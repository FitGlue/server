package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Database interface for Firestore operations
type Database interface {
	SetExecution(ctx context.Context, id string, data map[string]interface{}) error
	UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error
}

// ExecutionOptions contains optional fields for execution logging
type ExecutionOptions struct {
	UserID      string
	TestRunID   string
	TriggerType string
	Inputs      interface{}
}

// LogStart creates an execution record with STARTED status
func LogStart(ctx context.Context, db Database, service string, opts ExecutionOptions) (string, error) {
	execID := fmt.Sprintf("%s-%d", service, time.Now().UnixNano())

	now := timestamppb.Now()

	record := &pb.ExecutionRecord{
		ExecutionId: execID,
		Service:     service,
		Status:      pb.ExecutionStatus_STATUS_STARTED,
		Timestamp:   now,
		StartTime:   now,
		UserId:      opts.UserID,
		TestRunId:   opts.TestRunID,
		TriggerType: opts.TriggerType,
	}

	// Encode inputs as JSON if provided
	if opts.Inputs != nil {
		inputsJSON, err := json.Marshal(opts.Inputs)
		if err == nil {
			record.InputsJson = string(inputsJSON)
		}
	}

	// Convert to map for Firestore
	data := executionRecordToMap(record)

	if err := db.SetExecution(ctx, execID, data); err != nil {
		return execID, fmt.Errorf("failed to log execution start: %w", err)
	}

	return execID, nil
}

// LogChildExecutionStart creates an execution record with STARTED status and links it to a parent
func LogChildExecutionStart(ctx context.Context, db Database, service string, parentExecutionID string, opts ExecutionOptions) (string, error) {
	execID := fmt.Sprintf("%s-%d", service, time.Now().UnixNano())

	now := timestamppb.Now()

	record := &pb.ExecutionRecord{
		ExecutionId:       execID,
		Service:           service,
		Status:            pb.ExecutionStatus_STATUS_STARTED,
		Timestamp:         now,
		StartTime:         now,
		UserId:            opts.UserID,
		TestRunId:         opts.TestRunID,
		TriggerType:       opts.TriggerType,
		ParentExecutionId: parentExecutionID,
	}

	// Encode inputs as JSON if provided
	if opts.Inputs != nil {
		inputsJSON, err := json.Marshal(opts.Inputs)
		if err == nil {
			record.InputsJson = string(inputsJSON)
		}
	}

	// Convert to map for Firestore
	data := executionRecordToMap(record)

	if err := db.SetExecution(ctx, execID, data); err != nil {
		return execID, fmt.Errorf("failed to log child execution start: %w", err)
	}

	return execID, nil
}

// LogSuccess updates an execution record with SUCCESS status
func LogSuccess(ctx context.Context, db Database, execID string, outputs interface{}) error {
	now := timestamppb.Now()

	updates := map[string]interface{}{
		"status":    pb.ExecutionStatus_STATUS_SUCCESS.String(),
		"timestamp": now.AsTime(),
		"endTime":   now.AsTime(),
	}

	// Encode outputs as JSON if provided
	if outputs != nil {
		outputsJSON, err := json.Marshal(outputs)
		if err == nil {
			updates["outputsJson"] = string(outputsJSON)
		}
	}

	if err := db.UpdateExecution(ctx, execID, updates); err != nil {
		return fmt.Errorf("failed to log execution success: %w", err)
	}

	return nil
}

// LogFailure updates an execution record with FAILED status
func LogFailure(ctx context.Context, db Database, execID string, err error, outputs interface{}) error {
	now := timestamppb.Now()

	updates := map[string]interface{}{
		"status":       pb.ExecutionStatus_STATUS_FAILED.String(),
		"timestamp":    now.AsTime(),
		"endTime":      now.AsTime(),
		"errorMessage": err.Error(),
	}

	// Encode outputs as JSON if provided
	if outputs != nil {
		outputsJSON, err := json.Marshal(outputs)
		if err == nil {
			updates["outputsJson"] = string(outputsJSON)
		}
	}

	if updateErr := db.UpdateExecution(ctx, execID, updates); updateErr != nil {
		return fmt.Errorf("failed to log execution failure: %w", updateErr)
	}

	return nil
}

// executionRecordToMap converts a protobuf ExecutionRecord to a Firestore-compatible map
func executionRecordToMap(record *pb.ExecutionRecord) map[string]interface{} {
	data := map[string]interface{}{
		"service":   record.Service,
		"status":    record.Status.String(),
		"timestamp": record.Timestamp.AsTime(),
	}

	if record.UserId != "" {
		data["user_id"] = record.UserId
	}
	if record.TestRunId != "" {
		data["test_run_id"] = record.TestRunId
	}
	if record.TriggerType != "" {
		data["trigger_type"] = record.TriggerType
	}
	if record.StartTime != nil {
		data["startTime"] = record.StartTime.AsTime()
	}
	if record.InputsJson != "" {
		data["inputs"] = record.InputsJson
	}
	if record.ParentExecutionId != "" {
		data["parent_execution_id"] = record.ParentExecutionId
	}

	return data
}
