package webhook

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cloudevents/sdk-go/v2/event"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
)

// WebhookEvent represents a normalized event across all providers
type WebhookEvent struct {
	Provider    string
	ProviderUID string // The provider's internal user ID
	ActivityID  string // The external activity ID
	Event       string // "create", "update", "delete"
	RawPayload  []byte // The raw JSON body
}

// SourceProvider is the interface implemented by each integration
// to handle verification, parsing, and user resolution.
type SourceProvider interface {
	// ID returns the provider identifier (e.g. "strava", "fitbit")
	ID() string

	// VerifySubscription handles provider-specific GET/POST verification challenges
	VerifySubscription(w http.ResponseWriter, r *http.Request)

	// ParseEvent validates the incoming POST signature/payload and extracts uniform event data
	ParseEvent(r *http.Request) ([]*WebhookEvent, error)

	// FetchActivity retrieves the full payload from the provider's API.
	FetchActivity(ctx context.Context, userSvc userpb.UserServiceClient, internalUserID string, evt *WebhookEvent) (*pbevents.ActivityPayload, error)
}

// Publisher defines the outbound event bus interface
type Publisher interface {
	PublishCloudEvent(ctx context.Context, topicID string, e event.Event) (string, error)
}

// Processor manages routing webhooks to the correct SourceProvider
type Processor struct {
	providers map[string]SourceProvider
	userSvc   userpb.UserServiceClient
	publisher Publisher
}

// NewProcessor creates a new WebhookProcessor
func NewProcessor(userSvc userpb.UserServiceClient, publisher Publisher) *Processor {
	return &Processor{
		providers: make(map[string]SourceProvider),
		userSvc:   userSvc,
		publisher: publisher,
	}
}

// Register adds a new SourceProvider to the processor
func (p *Processor) Register(provider SourceProvider) {
	p.providers[provider.ID()] = provider
}

// HandleVerification routes GET requests for webhook subscription challenges
func (p *Processor) HandleVerification(w http.ResponseWriter, r *http.Request, providerID string) {
	provider, ok := p.providers[providerID]
	if !ok {
		http.Error(w, "Unknown provider", http.StatusNotFound)
		return
	}
	provider.VerifySubscription(w, r)
}

// HandleEvent routes POST requests containing webhook payloads
func (p *Processor) HandleEvent(w http.ResponseWriter, r *http.Request, providerID string) {
	provider, ok := p.providers[providerID]
	if !ok {
		http.Error(w, "Unknown provider", http.StatusNotFound)
		return
	}

	events, err := provider.ParseEvent(r)
	if err != nil {
		// Log the error but return 200 to acknowledge receipt (providers will retry otherwise)
		// unless it's a verification signature failure where we might return 401.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, evt := range events {
		// 1. Resolve internal user ID
		resolveResp, err := p.userSvc.ResolveUserByIntegration(r.Context(), &userpb.ResolveUserByIntegrationRequest{
			Provider:    evt.Provider,
			ProviderUid: evt.ProviderUID,
		})
		if err != nil {
			// User not found or other err. We just skip this event.
			continue
		}

		internalUserID := resolveResp.Profile.UserId

		// 2. Fetch the full activity data using SourceProvider
		activityPayload, err := provider.FetchActivity(r.Context(), p.userSvc, internalUserID, evt)
		if err != nil || activityPayload == nil {
			// Provider might return nil without error if the event shouldn't be processed (e.g. non-activity event)
			continue
		}

		// 3. Construct and export the CloudEvent
		ce := event.New()
		ce.SetSource(fmt.Sprintf("/integrations/%s/webhook", evt.Provider))
		ce.SetType("com.fitglue.activity.created")
		if err := ce.SetData(event.ApplicationJSON, activityPayload); err != nil {
			continue
		}

		_, _ = p.publisher.PublishCloudEvent(r.Context(), "topic-raw-activity", ce)
	}

	// Always acknowledge receipt successfully if parsing succeeded
	w.WriteHeader(http.StatusOK)
}
