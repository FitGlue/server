package enricher

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/mocks"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestEnrichActivity(t *testing.T) {
	// Setup Mocks
	mockDB := &mocks.MockDatabase{
		SetExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			return nil
		},
		GetUserFunc: func(ctx context.Context, id string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"user_id": id,
				"integrations": map[string]interface{}{
					"fitbit": map[string]interface{}{
						"enabled":      true,
						"access_token": "mock-token",
					},
				},
				"enrichments": map[string]interface{}{
					"SOURCE_HEVY": map[string]interface{}{
						"enrichers": []interface{}{
							map[string]interface{}{
								"name": "fitbit-hr",
								"inputs": map[string]interface{}{
									"priority": "high",
								},
							},
						},
					},
				},
			}, nil
		},
	}
	mockPub := &mocks.MockPublisher{
		PublishFunc: func(ctx context.Context, topic string, data []byte) (string, error) {
			// Verify payload if needed
			return "msg-123", nil
		},
	}
	mockStore := &mocks.MockBlobStore{
		WriteFunc: func(ctx context.Context, bucket, object string, data []byte) error {
			return nil
		},
	}
	mockSecrets := &mocks.MockSecretStore{}

	// Inject Mocks into Global Service
	svc = &bootstrap.Service{
		DB:      mockDB,
		Pub:     mockPub,
		Store:   mockStore,
		Secrets: mockSecrets,
		Config: &bootstrap.Config{
			ProjectID:     "test-project",
			EnablePublish: false,
		},
	}

	// Prepare Input
	activity := pb.ActivityPayload{
		Source:    pb.ActivitySource_SOURCE_HEVY,
		UserId:    "user_123",
		Timestamp: time.Now().Format(time.RFC3339),
		StandardizedActivity: &pb.StandardizedActivity{
			StartTime: time.Now().Format(time.RFC3339),
			Type: "WEIGHT_TRAINING",
			Sessions: []*pb.Session{
				{TotalElapsedTime: 3600},
			},
		},
	}
	activityBytes, _ := json.Marshal(&activity)

	// Create CloudEvent
	e := event.New()
	e.SetID("event-123")
	e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
	e.SetSource("//pubsub")

	// Create the PubSubMessage struct expected by the handler
	psMsg := types.PubSubMessage{
		Message: struct {
			Data       []byte            `json:"data"`
			Attributes map[string]string `json:"attributes"`
		}{
			Data: activityBytes,
		},
	}

	// Set it as event data
	e.SetData(event.ApplicationJSON, psMsg)

	// Execute
	err := EnrichActivity(context.Background(), e)
	if err != nil {
		t.Fatalf("EnrichActivity failed: %v", err)
	}
}
