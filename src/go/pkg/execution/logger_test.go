package execution_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fitglue/server/src/go/pkg/execution"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

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
	return nil, nil // Not used in this test
}

func (m *MockDB) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}

func TestLogPending(t *testing.T) {
	mockDB := &MockDB{
		SetExecutionFunc: func(ctx context.Context, record *pb.ExecutionRecord) error {
			if record.Status != pb.ExecutionStatus_STATUS_PENDING {
				t.Errorf("Expected STATUS_PENDING, got %v", record.Status)
			}
			// Inputs should be empty/default as we don't pass them in wrapper
			if record.InputsJson != nil && *record.InputsJson != "" {
				t.Errorf("Expected empty inputs JSON, got %v", record.InputsJson)
			}
			return nil
		},
	}

	opts := execution.ExecutionOptions{
		UserID: "user-1",
		// wrapper doesn't pass inputs to LogPending anymore
	}

	id, err := execution.LogPending(context.Background(), mockDB, "test-service", opts)
	if err != nil {
		t.Fatalf("LogPending failed: %v", err)
	}
	if id == "" {
		t.Error("Expected execution ID")
	}
}

func TestLogStart(t *testing.T) {
	mockDB := &MockDB{
		UpdateExecutionFunc: func(ctx context.Context, userId string, id string, data map[string]interface{}) error {
			if status, ok := data["status"].(int32); !ok || pb.ExecutionStatus(status) != pb.ExecutionStatus_STATUS_STARTED {
				t.Errorf("Expected STATUS_STARTED, got %v", data["status"])
			}
			if data["inputs_json"] != `{"foo":"bar"}` {
				t.Errorf("Expected inputs_json to be '{\"foo\":\"bar\"}', got %v", data["inputs_json"])
			}
			// Check updated metadata
			if data["user_id"] != "user-updated" {
				t.Errorf("Expected user_id 'user-updated', got %v", data["user_id"])
			}
			return nil
		},
	}

	inputs := map[string]string{"foo": "bar"}
	opts := &execution.ExecutionOptions{
		UserID: "user-updated",
	}
	err := execution.LogStart(context.Background(), mockDB, "user-updated", "exec-1", inputs, opts)
	if err != nil {
		t.Fatalf("LogStart failed: %v", err)
	}
}

func TestLogSuccess(t *testing.T) {
	mockDB := &MockDB{
		UpdateExecutionFunc: func(ctx context.Context, userId string, id string, data map[string]interface{}) error {
			if status, ok := data["status"].(int32); !ok || pb.ExecutionStatus(status) != pb.ExecutionStatus_STATUS_SUCCESS {
				t.Errorf("Expected STATUS_SUCCESS, got %v", data["status"])
			}
			return nil
		},
	}

	err := execution.LogSuccess(context.Background(), mockDB, "test-user", "exec-1", nil)
	if err != nil {
		t.Fatalf("LogSuccess failed: %v", err)
	}
}

func TestLogFailure(t *testing.T) {
	mockDB := &MockDB{
		UpdateExecutionFunc: func(ctx context.Context, userId string, id string, data map[string]interface{}) error {
			if status, ok := data["status"].(int32); !ok || pb.ExecutionStatus(status) != pb.ExecutionStatus_STATUS_FAILED {
				t.Errorf("Expected STATUS_FAILED, got %v", data["status"])
			}
			if data["error_message"] != "oops" {
				t.Errorf("Expected oops, got %v", data["error_message"])
			}
			return nil
		},
	}

	err := execution.LogFailure(context.Background(), mockDB, "test-user", "exec-1", &simpleError{}, nil)
	if err != nil {
		t.Fatalf("LogFailure failed: %v", err)
	}
}

func TestLogFailureWithOutputs(t *testing.T) {
	mockDB := &MockDB{
		UpdateExecutionFunc: func(ctx context.Context, userId string, id string, data map[string]interface{}) error {
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
	err := execution.LogFailure(context.Background(), mockDB, "test-user", "exec-1", &simpleError{}, outputs)
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
			if record.PipelineExecutionId == nil || *record.PipelineExecutionId != "pipeline-123" {
				t.Errorf("Expected PipelineExecutionId 'pipeline-123', got %v", record.PipelineExecutionId)
			}
			if record.UserId == nil || *record.UserId != "user-1" {
				t.Errorf("Expected user-1, got %v", record.UserId)
			}
			return nil
		},
	}

	opts := execution.ExecutionOptions{
		UserID: "user-1",
	}

	id, err := execution.LogChildExecutionStart(context.Background(), mockDB, "child-service", "pipeline-123", opts)
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
