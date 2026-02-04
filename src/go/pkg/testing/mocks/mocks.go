package mocks

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/event"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// --- Mock Database ---
type MockDatabase struct {
	SetExecutionFunc    func(ctx context.Context, record *pb.ExecutionRecord) error
	UpdateExecutionFunc func(ctx context.Context, userId string, id string, data map[string]interface{}) error
	GetUserFunc         func(ctx context.Context, id string) (*pb.UserRecord, error)
	UpdateUserFunc      func(ctx context.Context, id string, data map[string]interface{}) error

	CreatePendingInputFunc func(ctx context.Context, userId string, input *pb.PendingInput) error
	GetPendingInputFunc    func(ctx context.Context, userId string, id string) (*pb.PendingInput, error)
	UpdatePendingInputFunc func(ctx context.Context, userId string, id string, data map[string]interface{}) error
	ListPendingInputsFunc  func(ctx context.Context, userID string) ([]*pb.PendingInput, error)

	GetCounterFunc       func(ctx context.Context, userId string, id string) (*pb.Counter, error)
	SetCounterFunc       func(ctx context.Context, userId string, counter *pb.Counter) error
	ListCountersFunc     func(ctx context.Context, userId string) ([]*pb.Counter, error)
	GetUserPipelinesFunc func(ctx context.Context, userId string) ([]*pb.PipelineConfig, error)
}

func (m *MockDatabase) SetExecution(ctx context.Context, record *pb.ExecutionRecord) error {
	if m.SetExecutionFunc != nil {
		return m.SetExecutionFunc(ctx, record)
	}
	return nil
}
func (m *MockDatabase) UpdateExecution(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	if m.UpdateExecutionFunc != nil {
		return m.UpdateExecutionFunc(ctx, userId, id, data)
	}
	return nil
}
func (m *MockDatabase) GetUser(ctx context.Context, id string) (*pb.UserRecord, error) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(ctx, id)
	}
	return nil, fmt.Errorf("user not found")
}
func (m *MockDatabase) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	if m.UpdateUserFunc != nil {
		return m.UpdateUserFunc(ctx, id, data)
	}
	return nil
}

func (m *MockDatabase) CreatePendingInput(ctx context.Context, userId string, input *pb.PendingInput) error {
	if m.CreatePendingInputFunc != nil {
		return m.CreatePendingInputFunc(ctx, userId, input)
	}
	return nil
}

func (m *MockDatabase) GetPendingInput(ctx context.Context, userId string, id string) (*pb.PendingInput, error) {
	if m.GetPendingInputFunc != nil {
		return m.GetPendingInputFunc(ctx, userId, id)
	}
	return nil, nil
}

func (m *MockDatabase) UpdatePendingInput(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	if m.UpdatePendingInputFunc != nil {
		return m.UpdatePendingInputFunc(ctx, userId, id, data)
	}
	return nil
}

func (m *MockDatabase) ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error) {
	if m.ListPendingInputsFunc != nil {
		return m.ListPendingInputsFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockDatabase) DeletePendingInput(ctx context.Context, userId string, id string) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) GetCounter(ctx context.Context, userId string, id string) (*pb.Counter, error) {
	if m.GetCounterFunc != nil {
		return m.GetCounterFunc(ctx, userId, id)
	}
	return nil, nil
}

func (m *MockDatabase) SetCounter(ctx context.Context, userId string, counter *pb.Counter) error {
	if m.SetCounterFunc != nil {
		return m.SetCounterFunc(ctx, userId, counter)
	}
	return nil
}

func (m *MockDatabase) ListCounters(ctx context.Context, userId string) ([]*pb.Counter, error) {
	if m.ListCountersFunc != nil {
		return m.ListCountersFunc(ctx, userId)
	}
	return nil, nil
}

func (m *MockDatabase) DeleteCounter(ctx context.Context, userId string, id string) error {
	// No-op for tests by default
	return nil
}

// --- Sync Count (for tier limits) ---

func (m *MockDatabase) IncrementSyncCount(ctx context.Context, userID string) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) IncrementPreventedSyncCount(ctx context.Context, userID string) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) ResetSyncCount(ctx context.Context, userID string) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) ListPendingInputsByEnricher(ctx context.Context, enricherId string, status pb.PendingInput_Status) ([]*pb.PendingInput, error) {
	// No-op for tests by default
	return nil, nil
}

// --- Showcased Activities (public shareable snapshots) ---

func (m *MockDatabase) ShowcaseActivityExists(ctx context.Context, showcaseId string) (bool, error) {
	// No-op for tests by default
	return false, nil
}

func (m *MockDatabase) SetShowcasedActivity(ctx context.Context, activity *pb.ShowcasedActivity) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) GetShowcasedActivity(ctx context.Context, showcaseId string) (*pb.ShowcasedActivity, error) {
	// No-op for tests by default
	return nil, nil
}

// --- Personal Records ---

func (m *MockDatabase) GetPersonalRecord(ctx context.Context, userId string, recordType string) (*pb.PersonalRecord, error) {
	// No-op for tests by default
	return nil, nil
}

func (m *MockDatabase) SetPersonalRecord(ctx context.Context, userId string, record *pb.PersonalRecord) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) ListPersonalRecords(ctx context.Context, userId string) ([]*pb.PersonalRecord, error) {
	// No-op for tests by default
	return nil, nil
}

func (m *MockDatabase) DeletePersonalRecord(ctx context.Context, userId string, recordType string) error {
	// No-op for tests by default
	return nil
}

// --- Pipelines (Sub-collection) ---

func (m *MockDatabase) GetUserPipelines(ctx context.Context, userId string) ([]*pb.PipelineConfig, error) {
	if m.GetUserPipelinesFunc != nil {
		return m.GetUserPipelinesFunc(ctx, userId)
	}
	// No-op for tests by default - return empty slice
	return []*pb.PipelineConfig{}, nil
}

// --- Uploaded Activities (for loop prevention) ---

func (m *MockDatabase) SetUploadedActivity(ctx context.Context, userId string, record *pb.UploadedActivityRecord) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) GetUploadedActivity(ctx context.Context, userId string, destination pb.Destination, destinationId string) (*pb.UploadedActivityRecord, error) {
	// No-op for tests by default - return nil (not found)
	return nil, nil
}

// --- Pipeline Runs (lifecycle tracking) ---

func (m *MockDatabase) CreatePipelineRun(ctx context.Context, userId string, run *pb.PipelineRun) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) GetPipelineRun(ctx context.Context, userId string, id string) (*pb.PipelineRun, error) {
	// No-op for tests by default
	return nil, nil
}

func (m *MockDatabase) GetPipelineRunByActivityId(ctx context.Context, userId string, activityId string) (*pb.PipelineRun, error) {
	// No-op for tests by default
	return nil, nil
}

func (m *MockDatabase) UpdatePipelineRun(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	// No-op for tests by default
	return nil
}

// --- Destination Outcomes (subcollection of Pipeline Runs) ---

func (m *MockDatabase) SetDestinationOutcome(ctx context.Context, userId string, pipelineRunId string, outcome *pb.DestinationOutcome) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) GetDestinationOutcomes(ctx context.Context, userId string, pipelineRunId string) ([]*pb.DestinationOutcome, error) {
	// No-op for tests by default
	return nil, nil
}

// --- Mock Publisher ---
type MockPublisher struct {
	PublishCloudEventFunc func(ctx context.Context, topic string, e event.Event) (string, error)
}

func (m *MockPublisher) PublishCloudEvent(ctx context.Context, topic string, e event.Event) (string, error) {
	if m.PublishCloudEventFunc != nil {
		return m.PublishCloudEventFunc(ctx, topic, e)
	}
	return "msg-id", nil
}

// --- Mock Storage ---
type MockBlobStore struct {
	WriteFunc func(ctx context.Context, bucket, object string, data []byte) error
	ReadFunc  func(ctx context.Context, bucket, object string) ([]byte, error)
}

func (m *MockBlobStore) Write(ctx context.Context, bucket, object string, data []byte) error {
	if m.WriteFunc != nil {
		return m.WriteFunc(ctx, bucket, object, data)
	}
	return nil
}
func (m *MockBlobStore) Read(ctx context.Context, bucket, object string) ([]byte, error) {
	if m.ReadFunc != nil {
		return m.ReadFunc(ctx, bucket, object)
	}
	return []byte("mock-data"), nil
}
