// nolint:proto-json
package github

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

// Provider implements webhook.SourceProvider for Github
type Provider struct{}

// NewProvider creates a new Github SourceProvider
func NewProvider() *Provider {
	return &Provider{}
}

// ID returns the provider identifier
func (p *Provider) ID() string {
	return "github"
}

// VerifySubscription handles Github webhook verification
func (p *Provider) VerifySubscription(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type githubPushEvent struct {
	Ref        string `json:"ref"`
	Repository struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Commits []struct {
		ID        string `json:"id"`
		Committer struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"committer"`
	} `json:"commits"`
	HeadCommit struct {
		ID string `json:"id"`
	} `json:"head_commit"`
}

// ParseEvent extracts events from a Github push webhook
func (p *Provider) ParseEvent(r *http.Request) ([]*webhook.WebhookEvent, error) {
	if r.Header.Get("X-GitHub-Event") != "push" {
		return nil, nil // Ignore non-push events
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var payload githubPushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	// Skip FitGlue Bot commits
	allBot := len(payload.Commits) > 0
	for _, c := range payload.Commits {
		if c.Committer.Name != "FitGlue Bot" && c.Committer.Email != "bot@fitglue.com" {
			allBot = false
			break
		}
	}
	if allBot {
		return nil, nil
	}

	commitID := payload.HeadCommit.ID
	if commitID == "" && len(payload.Commits) > 0 {
		commitID = payload.Commits[0].ID
	}
	if commitID == "" {
		return nil, nil
	}

	username := payload.Repository.Owner.Login
	if username == "" {
		return nil, fmt.Errorf("missing repository.owner.login")
	}

	evt := &webhook.WebhookEvent{
		Provider:    p.ID(),
		ProviderUID: username,
		ActivityID:  commitID,
		Event:       "push",
		RawPayload:  body,
	}

	return []*webhook.WebhookEvent{evt}, nil
}

func (p *Provider) FetchActivity(ctx context.Context, userSvc userpb.UserServiceClient, internalUserID string, evt *webhook.WebhookEvent) (*pbevents.ActivityPayload, error) {
	if len(evt.RawPayload) == 0 {
		return nil, fmt.Errorf("missing raw payload for github activity push")
	}

	payload := &pbevents.ActivityPayload{
		Source:              activitypb.ActivitySource_SOURCE_GITHUB,
		UserId:              internalUserID,
		OriginalPayloadJson: string(evt.RawPayload),
		ActivityId:          &evt.ActivityID,
	}

	return payload, nil
}
