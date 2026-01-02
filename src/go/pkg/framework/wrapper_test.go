package framework

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// MockDB for Wrapper Test
type MockDB struct {
	SetExecutionFunc    func(ctx context.Context, record *pb.ExecutionRecord) error
	UpdateExecutionFunc func(ctx context.Context, id string, data map[string]interface{}) error
}

func (m *MockDB) SetExecution(ctx context.Context, record *pb.ExecutionRecord) error {
	if m.SetExecutionFunc != nil {
		return m.SetExecutionFunc(ctx, record)
	}
	return nil
}
func (m *MockDB) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	if m.UpdateExecutionFunc != nil {
		return m.UpdateExecutionFunc(ctx, id, data)
	}
	return nil
}
func (m *MockDB) GetUser(ctx context.Context, id string) (*pb.UserRecord, error) {
	return nil, nil
}
func (m *MockDB) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}

func TestWrapCloudEvent(t *testing.T) {
	mockDB := &MockDB{
		SetExecutionFunc: func(ctx context.Context, record *pb.ExecutionRecord) error {
			if record.Status != pb.ExecutionStatus_STATUS_STARTED {
				t.Errorf("Expected status started, got %v", record.Status)
			}
			return nil
		},
		UpdateExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			if status, ok := data["status"].(int32); !ok || pb.ExecutionStatus(status) != pb.ExecutionStatus_STATUS_SUCCESS {
				t.Errorf("Expected status success, got %v", data["status"])
			}
			return nil
		},
	}

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
		UpdateExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			if status, ok := data["status"].(int32); !ok || pb.ExecutionStatus(status) != pb.ExecutionStatus_STATUS_FAILED {
				t.Errorf("Expected status failed, got %v", data["status"])
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
