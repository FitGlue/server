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
	UpdateExecutionFunc func(ctx context.Context, id string, data map[string]interface{}) error
	GetUserFunc         func(ctx context.Context, id string) (*pb.UserRecord, error)
	UpdateUserFunc      func(ctx context.Context, id string, data map[string]interface{}) error

	CreatePendingInputFunc func(ctx context.Context, input *pb.PendingInput) error
	GetPendingInputFunc    func(ctx context.Context, id string) (*pb.PendingInput, error)
	UpdatePendingInputFunc func(ctx context.Context, id string, data map[string]interface{}) error
	ListPendingInputsFunc  func(ctx context.Context, userID string) ([]*pb.PendingInput, error)

	GetCounterFunc              func(ctx context.Context, userId string, id string) (*pb.Counter, error)
	SetCounterFunc              func(ctx context.Context, userId string, counter *pb.Counter) error
	SetSynchronizedActivityFunc func(ctx context.Context, userId string, activity *pb.SynchronizedActivity) error
}

func (m *MockDatabase) SetExecution(ctx context.Context, record *pb.ExecutionRecord) error {
	if m.SetExecutionFunc != nil {
		return m.SetExecutionFunc(ctx, record)
	}
	return nil
}
func (m *MockDatabase) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	if m.UpdateExecutionFunc != nil {
		return m.UpdateExecutionFunc(ctx, id, data)
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

func (m *MockDatabase) CreatePendingInput(ctx context.Context, input *pb.PendingInput) error {
	if m.CreatePendingInputFunc != nil {
		return m.CreatePendingInputFunc(ctx, input)
	}
	return nil
}

func (m *MockDatabase) GetPendingInput(ctx context.Context, id string) (*pb.PendingInput, error) {
	if m.GetPendingInputFunc != nil {
		return m.GetPendingInputFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockDatabase) UpdatePendingInput(ctx context.Context, id string, data map[string]interface{}) error {
	if m.UpdatePendingInputFunc != nil {
		return m.UpdatePendingInputFunc(ctx, id, data)
	}
	return nil
}

func (m *MockDatabase) ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error) {
	if m.ListPendingInputsFunc != nil {
		return m.ListPendingInputsFunc(ctx, userID)
	}
	return nil, nil
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

func (m *MockDatabase) SetSynchronizedActivity(ctx context.Context, userId string, activity *pb.SynchronizedActivity) error {
	if m.SetSynchronizedActivityFunc != nil {
		return m.SetSynchronizedActivityFunc(ctx, userId, activity)
	}
	return nil
}

// --- Sync Count (for tier limits) ---

func (m *MockDatabase) IncrementSyncCount(ctx context.Context, userID string) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) ResetSyncCount(ctx context.Context, userID string) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) ListPendingParkrunActivities(ctx context.Context) ([]*pb.SynchronizedActivity, []string, error) {
	// No-op for tests by default
	return nil, nil, nil
}

func (m *MockDatabase) UpdateSynchronizedActivity(ctx context.Context, userId string, activityId string, data map[string]interface{}) error {
	// No-op for tests by default
	return nil
}

func (m *MockDatabase) GetSynchronizedActivity(ctx context.Context, userId string, activityId string) (*pb.SynchronizedActivity, error) {
	// No-op for tests by default
	return nil, nil
}

func (m *MockDatabase) ListPendingInputsByEnricher(ctx context.Context, enricherId string, status pb.PendingInput_Status) ([]*pb.PendingInput, error) {
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

// --- Mock Secrets ---
type MockSecretStore struct {
	GetSecretFunc func(ctx context.Context, projectID, name string) (string, error)
}

func (m *MockSecretStore) GetSecret(ctx context.Context, projectID, name string) (string, error) {
	if m.GetSecretFunc != nil {
		return m.GetSecretFunc(ctx, projectID, name)
	}
	return "mock-secret-value", nil
}
