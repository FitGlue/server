// nolint:proto-json
package enricher

import (
	user "github.com/fitglue/server/src/go/pkg/domain/user"

	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"

	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"

	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"

	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/testing/mocks"

	"github.com/fitglue/server/src/go/internal/pipeline/enricher/providers"
)

func TestEnrichActivity(t *testing.T) {
	// Setup Mocks
	mockDB := &mocks.MockDatabase{
		SetExecutionFunc: func(ctx context.Context, record *pbpipeline.ExecutionRecord) error {
			return nil
		},
		UpdateExecutionFunc: func(ctx context.Context, userId string, id string, data map[string]interface{}) error {
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
		GetUserFunc: func(ctx context.Context, id string) (*user.Record, error) {
			return &user.Record{
				UserProfile: &pbuser.UserProfile{
					UserId: id,
				},
			}, nil
		},
		GetUserPipelinesFunc: func(ctx context.Context, userId string) ([]*pbpipeline.PipelineConfig, error) {
			return []*pbpipeline.PipelineConfig{
				{
					Id:           "test-pipeline-1",
					Source:       "SOURCE_HEVY",
					Destinations: []pbplugin.DestinationType{pbplugin.DestinationType_DESTINATION_STRAVA},
					Enrichers: []*pbpipeline.EnricherConfig{
						{
							ProviderType: pbplugin.EnricherProviderType_ENRICHER_PROVIDER_MOCK,
							TypedConfig:  map[string]string{},
						},
					},
				},
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
		ProviderTypeFunc: func() pbplugin.EnricherProviderType { return pbplugin.EnricherProviderType_ENRICHER_PROVIDER_MOCK },
		EnrichFunc: func(ctx context.Context, _ *slog.Logger, activity *pbactivity.StandardizedActivity, user *user.Record, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
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
			ProjectID: "test-project",
		},
	}

	// Prepare Input
	pipelineID := "test-pipeline-1"
	activity := pbevents.ActivityPayload{
		Source:     pbactivity.ActivitySource_SOURCE_HEVY,
		UserId:     "user_123",
		PipelineId: &pipelineID, // Required by Rule E25
		Timestamp:  timestamppb.New(time.Now()),
		StandardizedActivity: &pbactivity.StandardizedActivity{
			StartTime: timestamppb.New(time.Now()),
			Type:      pbactivity.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			Sessions: []*pbactivity.Session{
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

	err := EnrichActivity(context.Background(), e)
	if err != nil {
		t.Fatalf("EnrichActivity failed: %v", err)
	}
}
