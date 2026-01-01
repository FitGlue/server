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
	return a.PublishWithAttrs(ctx, topicID, data, nil)
}

func (a *PubSubAdapter) PublishWithAttrs(ctx context.Context, topicID string, data []byte, attributes map[string]string) (string, error) {
	topic := a.Client.Topic(topicID)
	msg := &pubsub.Message{
		Data: data,
	}
	if attributes != nil {
		msg.Attributes = attributes
	}
	res := topic.Publish(ctx, msg)
	return res.Get(ctx)
}

// LogPublisher is a mock publisher for local development
type LogPublisher struct{}

func (p *LogPublisher) Publish(ctx context.Context, topicID string, data []byte) (string, error) {
	return p.PublishWithAttrs(ctx, topicID, data, nil)
}

func (p *LogPublisher) PublishWithAttrs(ctx context.Context, topicID string, data []byte, attributes map[string]string) (string, error) {
	log.Printf("[LogPublisher] MOCK PUBLISH to %s: %s (attrs: %v)", topicID, string(data), attributes)
	return "mock-msg-id", nil
}
