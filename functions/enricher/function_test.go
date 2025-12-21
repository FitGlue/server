package function

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"

	"fitglue-enricher/pkg/shared/mocks"
	"fitglue-enricher/pkg/shared/types"
	pb "fitglue-enricher/pkg/shared/types/pb/proto"
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

	svc := &Service{
		DB:      mockDB,
		Pub:     mockPub,
		Store:   mockStore,
		Secrets: mockSecrets,
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
	err := svc.EnrichActivity(context.Background(), e)
	if err != nil {
		t.Fatalf("EnrichActivity failed: %v", err)
	}
}
