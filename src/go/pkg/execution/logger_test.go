package execution_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ripixel/fitglue-server/src/go/pkg/execution"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

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
	return nil, nil // Not used in this test
}

func (m *MockDB) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}

func TestLogStart(t *testing.T) {
	mockDB := &MockDB{
		SetExecutionFunc: func(ctx context.Context, record *pb.ExecutionRecord) error {
			if record.Status != pb.ExecutionStatus_STATUS_STARTED {
				t.Errorf("Expected STATUS_STARTED, got %v", record.Status)
			}
			if record.Service != "test-service" {
				t.Errorf("Expected test-service, got %v", record.Service)
			}
			if record.UserId != "user-1" {
				t.Errorf("Expected user-1, got %v", record.UserId)
			}
			return nil
		},
	}

	opts := execution.ExecutionOptions{
		UserID: "user-1",
	}

	id, err := execution.LogStart(context.Background(), mockDB, "test-service", opts)
	if err != nil {
		t.Fatalf("LogStart failed: %v", err)
	}
	if id == "" {
		t.Error("Expected execution ID")
	}
}

func TestLogSuccess(t *testing.T) {
	mockDB := &MockDB{
		UpdateExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			if status, ok := data["status"].(int32); !ok || pb.ExecutionStatus(status) != pb.ExecutionStatus_STATUS_SUCCESS {
				t.Errorf("Expected STATUS_SUCCESS, got %v", data["status"])
			}
			return nil
		},
	}

	err := execution.LogSuccess(context.Background(), mockDB, "exec-1", nil)
	if err != nil {
		t.Fatalf("LogSuccess failed: %v", err)
	}
}

func TestLogFailure(t *testing.T) {
	mockDB := &MockDB{
		UpdateExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			if status, ok := data["status"].(int32); !ok || pb.ExecutionStatus(status) != pb.ExecutionStatus_STATUS_FAILED {
				t.Errorf("Expected STATUS_FAILED, got %v", data["status"])
			}
			if data["error_message"] != "oops" {
				t.Errorf("Expected oops, got %v", data["error_message"])
			}
			return nil
		},
	}

	err := execution.LogFailure(context.Background(), mockDB, "exec-1", &simpleError{}, nil)
	if err != nil {
		t.Fatalf("LogFailure failed: %v", err)
	}
}

func TestLogFailureWithOutputs(t *testing.T) {
	mockDB := &MockDB{
		UpdateExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			if status, ok := data["status"].(int32); !ok || pb.ExecutionStatus(status) != pb.ExecutionStatus_STATUS_FAILED {
				t.Errorf("Expected STATUS_FAILED, got %v", data["status"])
			}
			if data["error_message"] != "oops" {
				t.Errorf("Expected oops, got %v", data["error_message"])
			}
			if data["outputs_json"] != `{"foo":"bar"}` {
				t.Errorf("Expected outputs_json to be '{\"foo\":\"bar\"}', got %v", data["outputs_json"])
			}
			return nil
		},
	}

	outputs := map[string]string{"foo": "bar"}
	err := execution.LogFailure(context.Background(), mockDB, "exec-1", &simpleError{}, outputs)
	if err != nil {
		t.Fatalf("LogFailure failed: %v", err)
	}
}

func TestLogChildExecutionStart(t *testing.T) {
	mockDB := &MockDB{
		SetExecutionFunc: func(ctx context.Context, record *pb.ExecutionRecord) error {
			if record.Status != pb.ExecutionStatus_STATUS_STARTED {
				t.Errorf("Expected STATUS_STARTED, got %v", record.Status)
			}
			if record.Service != "child-service" {
				t.Errorf("Expected child-service, got %v", record.Service)
			}
			if record.ParentExecutionId != "parent-exec-123" {
				t.Errorf("Expected parent_execution_id parent-exec-123, got %v", record.ParentExecutionId)
			}
			if record.UserId != "user-1" {
				t.Errorf("Expected user-1, got %v", record.UserId)
			}
			return nil
		},
	}

	opts := execution.ExecutionOptions{
		UserID: "user-1",
	}

	id, err := execution.LogChildExecutionStart(context.Background(), mockDB, "child-service", "parent-exec-123", opts)
	if err != nil {
		t.Fatalf("LogChildExecutionStart failed: %v", err)
	}
	if id == "" {
		t.Error("Expected execution ID")
	}
	if !strings.Contains(id, "child-service-") {
		t.Errorf("Expected ID to contain 'child-service-', got %s", id)
	}
}

// Helper error type for testing
type simpleError struct{}

var _ error = (*simpleError)(nil)

func (e *simpleError) Error() string { return "oops" }
