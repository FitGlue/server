package mocks

import (
	"context"
	"fmt"
)

// --- Mock Database ---
type MockDatabase struct {
	SetExecutionFunc    func(ctx context.Context, id string, data map[string]interface{}) error
	UpdateExecutionFunc func(ctx context.Context, id string, data map[string]interface{}) error
	GetUserFunc         func(ctx context.Context, id string) (map[string]interface{}, error)
}

func (m *MockDatabase) SetExecution(ctx context.Context, id string, data map[string]interface{}) error {
	if m.SetExecutionFunc != nil {
		return m.SetExecutionFunc(ctx, id, data)
	}
	return nil
}
func (m *MockDatabase) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	if m.UpdateExecutionFunc != nil {
		return m.UpdateExecutionFunc(ctx, id, data)
	}
	return nil
}
func (m *MockDatabase) GetUser(ctx context.Context, id string) (map[string]interface{}, error) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(ctx, id)
	}
	return nil, fmt.Errorf("user not found")
}

// --- Mock Publisher ---
type MockPublisher struct {
	PublishFunc func(ctx context.Context, topic string, data []byte) (string, error)
}

func (m *MockPublisher) Publish(ctx context.Context, topic string, data []byte) (string, error) {
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
