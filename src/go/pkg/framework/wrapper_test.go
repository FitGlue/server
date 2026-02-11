package framework

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/types"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// MockDB for Wrapper Test
type MockDB struct {
	SetExecutionFunc    func(ctx context.Context, record *pb.ExecutionRecord) error
	UpdateExecutionFunc func(ctx context.Context, userId string, id string, data map[string]interface{}) error
}

func (m *MockDB) SetExecution(ctx context.Context, record *pb.ExecutionRecord) error {
	if m.SetExecutionFunc != nil {
		return m.SetExecutionFunc(ctx, record)
	}
	return nil
}
func (m *MockDB) UpdateExecution(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	if m.UpdateExecutionFunc != nil {
		return m.UpdateExecutionFunc(ctx, userId, id, data)
	}
	return nil
}
func (m *MockDB) GetUser(ctx context.Context, id string) (*pb.UserRecord, error) {
	return nil, nil
}
func (m *MockDB) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}
func (m *MockDB) CreatePendingInput(ctx context.Context, userId string, input *pb.PendingInput) error {
	return nil
}
func (m *MockDB) GetPendingInput(ctx context.Context, userId string, id string) (*pb.PendingInput, error) {
	return nil, nil
}
func (m *MockDB) UpdatePendingInput(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	return nil
}
func (m *MockDB) ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error) {
	return nil, nil
}
func (m *MockDB) DeletePendingInput(ctx context.Context, userId string, id string) error {
	return nil
}
func (m *MockDB) GetCounter(ctx context.Context, userId string, id string) (*pb.Counter, error) {
	return nil, nil
}
func (m *MockDB) SetCounter(ctx context.Context, userId string, counter *pb.Counter) error {
	return nil
}
func (m *MockDB) ListCounters(ctx context.Context, userId string) ([]*pb.Counter, error) {
	return nil, nil
}
func (m *MockDB) DeleteCounter(ctx context.Context, userId string, id string) error {
	return nil
}
func (m *MockDB) IncrementSyncCount(ctx context.Context, userID string) error {
	return nil
}
func (m *MockDB) IncrementPreventedSyncCount(ctx context.Context, userID string) error {
	return nil
}
func (m *MockDB) ResetSyncCount(ctx context.Context, userID string) error {
	return nil
}
func (m *MockDB) ListPendingInputsByEnricher(ctx context.Context, enricherId string, status pb.PendingInput_Status) ([]*pb.PendingInput, error) {
	return nil, nil
}
func (m *MockDB) ShowcaseActivityExists(ctx context.Context, showcaseId string) (bool, error) {
	return false, nil
}
func (m *MockDB) SetShowcasedActivity(ctx context.Context, activity *pb.ShowcasedActivity) error {
	return nil
}
func (m *MockDB) GetShowcasedActivity(ctx context.Context, showcaseId string) (*pb.ShowcasedActivity, error) {
	return nil, nil
}
func (m *MockDB) SetShowcaseProfile(ctx context.Context, profile *pb.ShowcaseProfile) error {
	return nil
}
func (m *MockDB) GetShowcaseProfile(ctx context.Context, slug string) (*pb.ShowcaseProfile, error) {
	return nil, nil
}
func (m *MockDB) GetShowcaseProfileByUserId(ctx context.Context, userId string) (*pb.ShowcaseProfile, error) {
	return nil, nil
}
func (m *MockDB) DeleteShowcaseProfile(ctx context.Context, slug string) error {
	return nil
}
func (m *MockDB) GetPersonalRecord(ctx context.Context, userId string, recordType string) (*pb.PersonalRecord, error) {
	return nil, nil
}
func (m *MockDB) SetPersonalRecord(ctx context.Context, userId string, record *pb.PersonalRecord) error {
	return nil
}
func (m *MockDB) ListPersonalRecords(ctx context.Context, userId string) ([]*pb.PersonalRecord, error) {
	return nil, nil
}
func (m *MockDB) DeletePersonalRecord(ctx context.Context, userId string, recordType string) error {
	return nil
}
func (m *MockDB) GetUserPipelines(ctx context.Context, userId string) ([]*pb.PipelineConfig, error) {
	return []*pb.PipelineConfig{}, nil
}
func (m *MockDB) GetPluginDefault(ctx context.Context, userId string, pluginId string) (*pb.PluginDefault, error) {
	return nil, nil
}
func (m *MockDB) SetPluginDefault(ctx context.Context, userId string, pluginDefault *pb.PluginDefault) error {
	return nil
}
func (m *MockDB) SetUploadedActivity(ctx context.Context, userId string, record *pb.UploadedActivityRecord) error {
	return nil
}
func (m *MockDB) GetUploadedActivity(ctx context.Context, userId string, destination pb.Destination, destinationId string) (*pb.UploadedActivityRecord, error) {
	return nil, nil
}
func (m *MockDB) CreatePipelineRun(ctx context.Context, userId string, run *pb.PipelineRun) error {
	return nil
}
func (m *MockDB) GetPipelineRun(ctx context.Context, userId string, id string) (*pb.PipelineRun, error) {
	return nil, nil
}
func (m *MockDB) GetPipelineRunByActivityId(ctx context.Context, userId string, activityId string) (*pb.PipelineRun, error) {
	return nil, nil
}
func (m *MockDB) UpdatePipelineRun(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	return nil
}
func (m *MockDB) SetDestinationOutcome(ctx context.Context, userId string, pipelineRunId string, outcome *pb.DestinationOutcome) error {
	return nil
}
func (m *MockDB) GetDestinationOutcomes(ctx context.Context, userId string, pipelineRunId string) ([]*pb.DestinationOutcome, error) {
	return nil, nil
}
func (m *MockDB) GetBoosterData(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *MockDB) SetBoosterData(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
	return nil
}
func (m *MockDB) DeleteBoosterData(ctx context.Context, userId string, boosterId string) error {
	return nil
}

// Update Wrapper Test to expect metadata in LogStart updates
func TestWrapCloudEvent(t *testing.T) {
	mockDB := &MockDB{
		SetExecutionFunc: func(ctx context.Context, record *pb.ExecutionRecord) error {
			if record.Status != pb.ExecutionStatus_STATUS_PENDING {
				t.Errorf("Expected status pending, got %v", record.Status)
			}
			return nil
		},
		UpdateExecutionFunc: func(ctx context.Context, userId string, id string, data map[string]interface{}) error {
			status, ok := data["status"].(int32)
			if !ok {
				// Check for metadata updates
				if _, ok := data["user_id"]; ok {
					return nil // User ID update
				}
				return nil // some other update
			}
			s := pb.ExecutionStatus(status)
			// Should be either STARTED or SUCCESS
			if s != pb.ExecutionStatus_STATUS_STARTED && s != pb.ExecutionStatus_STATUS_SUCCESS {
				t.Errorf("Unexpected status update: %v", s)
			}
			return nil
		},
	}
	// ... existing setup ...

	svc := &bootstrap.Service{
		DB: mockDB,
	}

	handler := func(ctx context.Context, e event.Event, fwCtx *FrameworkContext) (interface{}, error) {
		if fwCtx.Service != svc {
			t.Error("Service not injected correctly")
		}
		if fwCtx.ExecutionID == "" {
			t.Error("ExecutionID not generated")
		}
		return "ok", nil
	}

	wrapped := WrapCloudEvent("test-service", svc, handler)

	e := event.New()
	e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
	e.SetSource("test-source")

	err := wrapped(context.Background(), e)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
}

func TestWrapCloudEvent_Failure(t *testing.T) {
	mockDB := &MockDB{
		SetExecutionFunc: func(ctx context.Context, record *pb.ExecutionRecord) error {
			if record.Status != pb.ExecutionStatus_STATUS_PENDING {
				t.Errorf("Expected status pending, got %v", record.Status)
			}
			return nil
		},
		UpdateExecutionFunc: func(ctx context.Context, userId string, id string, data map[string]interface{}) error {
			status, ok := data["status"].(int32)
			if !ok {
				return nil
			}
			s := pb.ExecutionStatus(status)
			// Should be STARTED then FAILED
			if s != pb.ExecutionStatus_STATUS_STARTED && s != pb.ExecutionStatus_STATUS_FAILED {
				t.Errorf("Unexpected status update: %v", s)
			}
			return nil
		},
	}

	svc := &bootstrap.Service{
		DB: mockDB,
	}

	handler := func(ctx context.Context, e event.Event, fwCtx *FrameworkContext) (interface{}, error) {
		return nil, errors.New("simulated error")
	}

	wrapped := WrapCloudEvent("test-service", svc, handler)

	e := event.New()
	err := wrapped(context.Background(), e)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestWrapCloudEvent_UnwrapsNestedEvent(t *testing.T) {
	svc := &bootstrap.Service{
		DB: &MockDB{},
	}

	expectedID := "inner-event-123"
	expectedType := "com.fitglue.activity.created"

	handler := func(ctx context.Context, e event.Event, fwCtx *FrameworkContext) (interface{}, error) {
		// Assert that 'e' is the INNER event
		if e.ID() != expectedID {
			t.Errorf("Expected event ID %s, got %s", expectedID, e.ID())
		}
		if e.Type() != expectedType {
			t.Errorf("Expected event type %s, got %s", expectedType, e.Type())
		}
		return "ok", nil
	}

	wrapped := WrapCloudEvent("test-service", svc, handler)

	// 1. Create Inner CloudEvent
	inner := event.New()
	inner.SetID(expectedID)
	inner.SetType(expectedType)
	inner.SetSource("/test/source")
	inner.SetData(event.ApplicationJSON, map[string]string{"foo": "bar"})

	innerBytes, _ := json.Marshal(inner)

	// 2. Wrap in Pub/Sub Envelope (as if coming from GCP)
	psMsg := types.PubSubMessage{
		Message: struct {
			Data       []byte            `json:"data"`
			Attributes map[string]string `json:"attributes"`
		}{
			Data: innerBytes,
		},
	}

	outer := event.New()
	outer.SetID("outer-msg-id")
	outer.SetType("google.cloud.pubsub.topic.v1.messagePublished")
	outer.SetSource("//pubsub")
	outer.SetData(event.ApplicationJSON, psMsg)

	// 3. Execute
	err := wrapped(context.Background(), outer)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
}
