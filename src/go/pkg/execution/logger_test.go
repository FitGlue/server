package execution_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ripixel/fitglue-server/src/go/pkg/execution"
)

type MockDB struct {
	SetExecutionFunc    func(ctx context.Context, id string, data map[string]interface{}) error
	UpdateExecutionFunc func(ctx context.Context, id string, data map[string]interface{}) error
}

func (m *MockDB) SetExecution(ctx context.Context, id string, data map[string]interface{}) error {
	if m.SetExecutionFunc != nil {
		return m.SetExecutionFunc(ctx, id, data)
	}
	return nil
}
func (m *MockDB) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	if m.UpdateExecutionFunc != nil {
		return m.UpdateExecutionFunc(ctx, id, data)
	}
	return nil
}

func TestLogStart(t *testing.T) {
	mockDB := &MockDB{
		SetExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			if data["status"] != "STATUS_STARTED" {
				t.Errorf("Expected STATUS_STARTED, got %v", data["status"])
			}
			if data["service"] != "test-service" {
				t.Errorf("Expected test-service, got %v", data["service"])
			}
			if data["user_id"] != "user-1" {
				t.Errorf("Expected user-1, got %v", data["user_id"])
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
			if data["status"] != "STATUS_SUCCESS" {
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
			if data["status"] != "STATUS_FAILED" {
				t.Errorf("Expected STATUS_FAILED, got %v", data["status"])
			}
			if data["errorMessage"] != "oops" {
				t.Errorf("Expected oops, got %v", data["errorMessage"])
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
			if data["status"] != "STATUS_FAILED" {
				t.Errorf("Expected STATUS_FAILED, got %v", data["status"])
			}
			if data["errorMessage"] != "oops" {
				t.Errorf("Expected oops, got %v", data["errorMessage"])
			}
			if data["outputsJson"] != `{"foo":"bar"}` {
				t.Errorf("Expected outputsJson to be '{\"foo\":\"bar\"}', got %v", data["outputsJson"])
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
		SetExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			if data["status"] != "STATUS_STARTED" {
				t.Errorf("Expected STATUS_STARTED, got %v", data["status"])
			}
			if data["service"] != "child-service" {
				t.Errorf("Expected child-service, got %v", data["service"])
			}
			if data["parent_execution_id"] != "parent-exec-123" {
				t.Errorf("Expected parent_execution_id parent-exec-123, got %v", data["parent_execution_id"])
			}
			if data["user_id"] != "user-1" {
				t.Errorf("Expected user-1, got %v", data["user_id"])
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
