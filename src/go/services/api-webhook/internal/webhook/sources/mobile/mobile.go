// nolint:proto-json
package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	activitypb "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
)

// Provider implements webhook.SourceProvider for Mobile Sync
type Provider struct{}

// NewProvider creates a new Mobile SourceProvider
func NewProvider() *Provider {
	return &Provider{}
}

// ID returns the provider identifier
func (p *Provider) ID() string {
	return "mobile"
}

// VerifySubscription handles Mobile webhook verification
func (p *Provider) VerifySubscription(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type mobileSyncRequest struct {
	Activities []struct {
		Source       string `json:"source"`
		ExternalID   string `json:"externalId"`
		ActivityName string `json:"activityName"`
		StartTime    string `json:"startTime"`
	} `json:"activities"`
}

// ParseEvent extracts events from a Mobile API sync request
func (p *Provider) ParseEvent(r *http.Request) ([]*webhook.WebhookEvent, error) {
	token := r.Header.Get("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var payload mobileSyncRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	var events []*webhook.WebhookEvent
	for _, act := range payload.Activities {
		actID := act.ExternalID
		if actID == "" {
			actID = act.Source + "-" + act.StartTime
		}

		// Keep the full payload of this specific activity
		// as we don't know the exact struct without decoding it all
		actBytes, _ := json.Marshal(act)

		events = append(events, &webhook.WebhookEvent{
			Provider:    p.ID(),
			ProviderUID: token, // Pass JWT token as UID to be verified by user resolver
			ActivityID:  actID,
			Event:       "sync",
			RawPayload:  actBytes,
		})
	}

	return events, nil
}

func (p *Provider) FetchActivity(ctx context.Context, userSvc userpb.UserServiceClient, internalUserID string, evt *webhook.WebhookEvent) (*pbevents.ActivityPayload, error) {
	if len(evt.RawPayload) == 0 {
		return nil, fmt.Errorf("missing raw payload for mobile activity sync")
	}

	var act struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(evt.RawPayload, &act); err != nil {
		return nil, fmt.Errorf("invalid mobile sync payload: %w", err)
	}

	sourceEnum := activitypb.ActivitySource_SOURCE_UNSPECIFIED
	switch act.Source {
	case "SOURCE_APPLE_HEALTH":
		sourceEnum = activitypb.ActivitySource_SOURCE_APPLE_HEALTH
	case "SOURCE_HEALTH_CONNECT":
		sourceEnum = activitypb.ActivitySource_SOURCE_HEALTH_CONNECT
	default:
		// Some mobile specific raw source
		sourceEnum = activitypb.ActivitySource_SOURCE_UNSPECIFIED
	}

	payload := &pbevents.ActivityPayload{
		Source:              sourceEnum,
		UserId:              internalUserID,
		OriginalPayloadJson: string(evt.RawPayload),
		ActivityId:          &evt.ActivityID,
	}

	return payload, nil
}
