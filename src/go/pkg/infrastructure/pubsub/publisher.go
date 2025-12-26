package pubsub

import (
	"context"
	"log"

	"cloud.google.com/go/pubsub"
)

// PubSubAdapter provides message publishing using Google Cloud Pub/Sub
type PubSubAdapter struct {
	Client *pubsub.Client
}

func (a *PubSubAdapter) Publish(ctx context.Context, topicID string, data []byte) (string, error) {
	topic := a.Client.Topic(topicID)
	res := topic.Publish(ctx, &pubsub.Message{Data: data})
	return res.Get(ctx)
}

// LogPublisher is a mock publisher for local development
type LogPublisher struct{}

func (p *LogPublisher) Publish(ctx context.Context, topicID string, data []byte) (string, error) {
	log.Printf("[LogPublisher] MOCK PUBLISH to %s: %s", topicID, string(data))
	return "mock-msg-id", nil
}
