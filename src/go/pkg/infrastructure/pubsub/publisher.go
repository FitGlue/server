package pubsub

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"

	"cloud.google.com/go/pubsub"
	"github.com/cloudevents/sdk-go/v2/event"
)

// PubSubAdapter provides message publishing using Google Cloud Pub/Sub
type PubSubAdapter struct {
	Client *pubsub.Client
}

func (a *PubSubAdapter) PublishCloudEvent(ctx context.Context, topicID string, e event.Event) (string, error) {
	bytes, err := json.Marshal(e)
	if err != nil {
		slog.Error("Failed to marshal CloudEvent", "topic", topicID, "error", err)
		return "", err
	}
	slog.Info("Publishing CloudEvent",
		"topic", topicID,
		"event_type", e.Type(),
		"event_id", e.ID(),
		"source", e.Source(),
		"size_bytes", len(bytes))
	return a.publish(ctx, topicID, bytes)
}

func (a *PubSubAdapter) publish(ctx context.Context, topicID string, data []byte) (string, error) {
	return a.publishWithAttrs(ctx, topicID, data, nil)
}

func (a *PubSubAdapter) publishWithAttrs(ctx context.Context, topicID string, data []byte, attributes map[string]string) (string, error) {
	topic := a.Client.Topic(topicID)
	msg := &pubsub.Message{
		Data: data,
	}
	if attributes != nil {
		msg.Attributes = attributes
	}
	res := topic.Publish(ctx, msg)
	msgID, err := res.Get(ctx)
	if err != nil {
		slog.Error("Failed to publish message", "topic", topicID, "error", err)
		return "", err
	}
	slog.Info("Message published successfully", "topic", topicID, "message_id", msgID, "size_bytes", len(data))
	return msgID, nil
}

// LogPublisher is a mock publisher for local development
type LogPublisher struct{}

func (p *LogPublisher) PublishCloudEvent(ctx context.Context, topicID string, e event.Event) (string, error) {
	bytes, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return p.publish(ctx, topicID, bytes)
}

func (p *LogPublisher) publish(ctx context.Context, topicID string, data []byte) (string, error) {
	return p.publishWithAttrs(ctx, topicID, data, nil)
}

func (p *LogPublisher) publishWithAttrs(ctx context.Context, topicID string, data []byte, attributes map[string]string) (string, error) {
	log.Printf("[LogPublisher] MOCK PUBLISH to %s: %s (attrs: %v)", topicID, string(data), attributes)
	return "mock-msg-id", nil
}
