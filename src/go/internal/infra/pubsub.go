package infra

import "context"

// Publisher defines an interface for publishing messages to a message broker (e.g. Pub/Sub).
type Publisher interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}
