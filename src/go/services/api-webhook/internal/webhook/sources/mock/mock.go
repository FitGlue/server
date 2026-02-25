// nolint:proto-json
package mock

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

// Provider implements webhook.SourceProvider for Mock
type Provider struct{}

// NewProvider creates a new Mock SourceProvider
func NewProvider() *Provider {
	return &Provider{}
}

// ID returns the provider identifier
func (p *Provider) ID() string {
	return "mock"
}

// VerifySubscription handles Mock webhook verification
func (p *Provider) VerifySubscription(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type mockPayload struct {
	UserID     string `json:"user_id"`
	ActivityID string `json:"activity_id"`
	Event      string `json:"event"`
}

// ParseEvent extracts events from a Mock webhook
func (p *Provider) ParseEvent(r *http.Request) ([]*webhook.WebhookEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var payload mockPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	if payload.ActivityID == "" || payload.UserID == "" {
		return nil, fmt.Errorf("missing activity_id or user_id")
	}

	evt := &webhook.WebhookEvent{
		Provider:    p.ID(),
		ProviderUID: payload.UserID,
		ActivityID:  payload.ActivityID,
		Event:       payload.Event,
		RawPayload:  body,
	}

	return []*webhook.WebhookEvent{evt}, nil
}

func (p *Provider) FetchActivity(ctx context.Context, userSvc userpb.UserServiceClient, internalUserID string, evt *webhook.WebhookEvent) (*pbevents.ActivityPayload, error) {
	// For mock, we just pass the activity ID along as the original payload
	// The splitter will use this to generate the mock StandardizedActivity
	mockJSON := fmt.Sprintf(`{"id": "%s"}`, evt.ActivityID)

	payload := &pbevents.ActivityPayload{
		Source:              activitypb.ActivitySource_SOURCE_TEST,
		UserId:              internalUserID,
		OriginalPayloadJson: mockJSON,
		ActivityId:          &evt.ActivityID,
	}

	return payload, nil
}
