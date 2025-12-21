package function

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/ripixel/fitglue-server/src/go/pkg/mocks"
	"github.com/ripixel/fitglue-server/src/go/pkg/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb/proto"
)

func TestEnrichActivity(t *testing.T) {
	// Setup Mocks
	mockDB := &mocks.MockDatabase{
		SetExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			return nil
		},
	}
	mockPub := &mocks.MockPublisher{
		PublishFunc: func(ctx context.Context, topic string, data []byte) (string, error) {
			// Verify payload if needed
			return "msg-123", nil
		},
	}
	mockStore := &mocks.MockBlobStore{} // Default successful write
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
			Data []byte `json:"data"`
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
