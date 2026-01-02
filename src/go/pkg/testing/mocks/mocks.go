package mocks

import (
	"context"
	"fmt"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// --- Mock Database ---
type MockDatabase struct {
	SetExecutionFunc    func(ctx context.Context, record *pb.ExecutionRecord) error
	UpdateExecutionFunc func(ctx context.Context, id string, data map[string]interface{}) error
	GetUserFunc         func(ctx context.Context, id string) (*pb.UserRecord, error)
	UpdateUserFunc      func(ctx context.Context, id string, data map[string]interface{}) error
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

// --- Mock Publisher ---
type MockPublisher struct {
	PublishFunc          func(ctx context.Context, topic string, data []byte) (string, error)
	PublishWithAttrsFunc func(ctx context.Context, topic string, data []byte, attributes map[string]string) (string, error)
}

func (m *MockPublisher) Publish(ctx context.Context, topic string, data []byte) (string, error) {
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, topic, data)
	}
	return m.PublishWithAttrs(ctx, topic, data, nil)
}

func (m *MockPublisher) PublishWithAttrs(ctx context.Context, topic string, data []byte, attributes map[string]string) (string, error) {
	if m.PublishWithAttrsFunc != nil {
		return m.PublishWithAttrsFunc(ctx, topic, data, attributes)
	}
	// Fallback to PublishFunc if AttrsFunc not defined (ignoring attrs), or just return success
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, topic, data)
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
