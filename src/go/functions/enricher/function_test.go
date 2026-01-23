package enricher

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/testing/mocks"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
)

func TestEnrichActivity(t *testing.T) {
	// Setup Mocks
	mockDB := &mocks.MockDatabase{
		SetExecutionFunc: func(ctx context.Context, record *pb.ExecutionRecord) error {
			return nil
		},
		UpdateExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			// Verify rich output structure
			if outputsJSON, ok := data["outputs_json"].(string); ok {
				var outputs map[string]interface{}
				if err := json.Unmarshal([]byte(outputsJSON), &outputs); err != nil {
					t.Errorf("Failed to unmarshal outputs: %v", err)
					return nil
				}

				// Verify expected fields
				if status, ok := outputs["status"].(string); !ok || status == "" {
					t.Error("Expected 'status' field in outputs")
				}
				if _, ok := outputs["published_events"]; !ok {
					t.Error("Expected 'published_events' field in outputs")
				}
				if _, ok := outputs["provider_executions"]; !ok {
					t.Error("Expected 'provider_executions' field in outputs")
				}
			}
			return nil
		},
		GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				UserId: id,
			}, nil
		},
	}
	mockPub := &mocks.MockPublisher{
		PublishCloudEventFunc: func(ctx context.Context, topic string, e cloudevents.Event) (string, error) {
			// Verify payload if needed
			return "msg-123", nil
		},
	}
	mockStore := &mocks.MockBlobStore{
		WriteFunc: func(ctx context.Context, bucket, object string, data []byte) error {
			return nil
		},
	}

	// Override provider registry with mock provider for testing
	providers.ClearRegistry()
	providers.Register(&MockProvider{
		NameFunc:         func() string { return "mock-enricher" },
		ProviderTypeFunc: func() pb.EnricherProviderType { return pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK },
		EnrichFunc: func(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
			return &providers.EnrichmentResult{
				Name:        "Mock Enriched Activity",
				Description: "Enriched by mock provider",
				Metadata: map[string]string{
					"mock_key": "mock_value",
				},
			}, nil
		},
	})
	// Ensure cleanup
	defer providers.ClearRegistry()

	// Inject Mocks into Global Service
	svc = &bootstrap.Service{
		DB:    mockDB,
		Pub:   mockPub,
		Store: mockStore,
		Config: &bootstrap.Config{
			ProjectID:     "test-project",
			EnablePublish: false,
		},
	}

	// Prepare Input
	activity := pb.ActivityPayload{
		Source:    pb.ActivitySource_SOURCE_HEVY,
		UserId:    "user_123",
		Timestamp: timestamppb.New(time.Now()),
		StandardizedActivity: &pb.StandardizedActivity{
			StartTime: timestamppb.New(time.Now()),
			Type:      pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			Sessions: []*pb.Session{
				{TotalElapsedTime: 3600},
			},
		},
	}
	marshalOpts := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}
	activityBytes, _ := marshalOpts.Marshal(&activity)

	// Create CloudEvent
	e := cloudevents.NewEvent()
	e.SetID("event-123")
	e.SetType("com.fitglue.activity.created") // Use a realistic type
	e.SetSource("/fitbit-ingest")

	// Set the payload directly as data
	e.SetData(cloudevents.ApplicationJSON, activityBytes)

	// Execute
	err := EnrichActivity(context.Background(), e)
	if err != nil {
		t.Fatalf("EnrichActivity failed: %v", err)
	}
}
