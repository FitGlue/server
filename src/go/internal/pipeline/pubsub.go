package pipeline

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Publisher defines the contract for publishing events (e.g., to Pub/Sub).
type Publisher interface {
	// PublishCloudEvent publishes a CloudEvent to a specific topic and returns the message ID.
	PublishCloudEvent(ctx context.Context, topic string, ce cloudevents.Event) (string, error)
}
