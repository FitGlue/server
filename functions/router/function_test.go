package function

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"

	"fitglue-router/pkg/shared/mocks"
	"fitglue-router/pkg/shared/types"
	pb "fitglue-router/pkg/shared/types/pb/proto"
)

func TestRouteActivity(t *testing.T) {
	// Setup Mocks
	mockDB := &mocks.MockDatabase{
		SetExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			return nil
		},
		GetUserFunc: func(ctx context.Context, id string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"strava_enabled": true,
			}, nil
		},
	}

	publishedTopics := []string{}
	mockPub := &mocks.MockPublisher{
		PublishFunc: func(ctx context.Context, topic string, data []byte) (string, error) {
			publishedTopics = append(publishedTopics, topic)
			return "msg-routable", nil
		},
	}

	svc := &Service{
		DB:  mockDB,
		Pub: mockPub,
	}

	// Prepare Input
	eventPayload := pb.EnrichedActivityEvent{
		UserId: "user_router",
		GcsUri: "gs://bucket/file.fit",
	}
	payloadBytes, _ := json.Marshal(&eventPayload)

	psMsg := types.PubSubMessage{
		Message: struct {
			Data []byte `json:"data"`
		}{
			Data: payloadBytes,
		},
	}

	e := event.New()
	e.SetID("evt-router")
	e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
	e.SetSource("//pubsub")
	e.SetData(event.ApplicationJSON, psMsg)

	// Execute
	err := svc.RouteActivity(context.Background(), e)
	if err != nil {
		t.Fatalf("RouteActivity failed: %v", err)
	}

	// Verify
	if len(publishedTopics) != 1 {
		t.Errorf("Expected 1 published topic, got %d", len(publishedTopics))
	}
}
