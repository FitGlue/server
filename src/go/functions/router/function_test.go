package router

import (
	"context"
	"encoding/json"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/testing/mocks"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestRouteActivity(t *testing.T) {
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
				if _, ok := outputs["routed_destinations"]; !ok {
					t.Error("Expected 'routed_destinations' field in outputs")
				}
				if _, ok := outputs["activity_id"]; !ok {
					t.Error("Expected 'activity_id' field in outputs")
				}
				if _, ok := outputs["pipeline_id"]; !ok {
					t.Error("Expected 'pipeline_id' field in outputs")
				}
			}
			return nil
		},
		GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
			return &pb.UserRecord{}, nil
		},
	}

	publishedTopics := []string{}
	mockPub := &mocks.MockPublisher{
		PublishCloudEventFunc: func(ctx context.Context, topic string, e cloudevents.Event) (string, error) {
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
		ActivityId:   "activity-123",
		FitFileUri:   "gs://bucket/file.fit",
		Description:  "Test Description",
		ActivityType: "WEIGHT_TRAINING",
		Name:         "Test Workout",
		Source:       pb.ActivitySource_SOURCE_HEVY,
		Destinations: []string{"strava"},
		PipelineId:   "pipe-test-1",
	}
	marshalOpts := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}
	payloadBytes, _ := marshalOpts.Marshal(&eventPayload)

	e := cloudevents.NewEvent()
	e.SetID("evt-router")
	e.SetType("com.fitglue.activity.enriched")
	e.SetSource("/enricher")
	e.SetData(cloudevents.ApplicationJSON, payloadBytes)

	// Execute
	err := RouteActivity(context.Background(), e)
	if err != nil {
		t.Fatalf("RouteActivity failed: %v", err)
	}

	// Verify
	if len(publishedTopics) != 1 {
		t.Errorf("Expected 1 published topic, got %d", len(publishedTopics))
	}
	if publishedTopics[0] != "topic-job-upload-strava" {
		t.Errorf("Expected topic 'topic-job-upload-strava', got '%s'", publishedTopics[0])
	}
}
