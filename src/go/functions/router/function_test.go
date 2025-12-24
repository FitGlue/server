package router

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/mocks"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
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

	// Inject Mocks into Global Service
	svc = &bootstrap.Service{
		DB:  mockDB,
		Pub: mockPub,
		Config: &bootstrap.Config{
			ProjectID: "test-project",
		},
	}

	// Prepare Input
	eventPayload := pb.EnrichedActivityEvent{
		UserId:       "user_router",
		FitFileUri:   "gs://bucket/file.fit",
		Description:  "Test Description",
		ActivityType: "WEIGHT_TRAINING",
		Name:         "Test Workout",
		Source:       pb.ActivitySource_SOURCE_HEVY,
	}
	payloadBytes, _ := json.Marshal(&eventPayload)

	psMsg := types.PubSubMessage{
		Message: struct {
			Data       []byte            `json:"data"`
			Attributes map[string]string `json:"attributes"`
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
	err := RouteActivity(context.Background(), e)
	if err != nil {
		t.Fatalf("RouteActivity failed: %v", err)
	}

	// Verify
	if len(publishedTopics) != 1 {
		t.Errorf("Expected 1 published topic, got %d", len(publishedTopics))
	}
}
