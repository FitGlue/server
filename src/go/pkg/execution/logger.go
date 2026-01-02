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
	SetExecution(ctx context.Context, record *pb.ExecutionRecord) error
	UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error
}

// ExecutionOptions contains optional fields for execution logging
type ExecutionOptions struct {
	UserID      string
	TestRunID   string
	TriggerType string
	Inputs      interface{}
}

// LogPending creates an execution record with PENDING status and captured inputs
func LogPending(ctx context.Context, db Database, service string, opts ExecutionOptions) (string, error) {
	execID := fmt.Sprintf("%s-%d", service, time.Now().UnixNano())

	now := timestamppb.Now()

	record := &pb.ExecutionRecord{
		ExecutionId: execID,
		Service:     service,
		Status:      pb.ExecutionStatus_STATUS_PENDING,
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

	if err := db.SetExecution(ctx, record); err != nil {
		return execID, fmt.Errorf("failed to log execution pending: %w", err)
	}

	return execID, nil
}

// LogStart updates an execution record to STARTED status and adds inputs/metadata
func LogStart(ctx context.Context, db Database, execID string, inputs interface{}, opts *ExecutionOptions) error {
	now := timestamppb.Now()

	updates := map[string]interface{}{
		"status":     int32(pb.ExecutionStatus_STATUS_STARTED),
		"start_time": now.AsTime(),
	}

	// Update metadata if provided (wasn't available at Pending time)
	if opts != nil {
		if opts.UserID != "" {
			updates["user_id"] = opts.UserID
		}
		if opts.TestRunID != "" {
			updates["test_run_id"] = opts.TestRunID
		}
		if opts.TriggerType != "" {
			updates["trigger_type"] = opts.TriggerType
		}
	}

	// Encode inputs as JSON if provided
	if inputs != nil {
		inputsJSON, err := json.Marshal(inputs)
		if err == nil {
			updates["inputs_json"] = string(inputsJSON)
		}
	}

	if err := db.UpdateExecution(ctx, execID, updates); err != nil {
		return fmt.Errorf("failed to log execution start: %w", err)
	}

	return nil
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

	if err := db.SetExecution(ctx, record); err != nil {
		return execID, fmt.Errorf("failed to log child execution start: %w", err)
	}

	return execID, nil
}

// LogSuccess updates an execution record with SUCCESS status
func LogSuccess(ctx context.Context, db Database, execID string, outputs interface{}) error {
	now := timestamppb.Now()

	// Update using snake_case keys manually as we use map[string]interface{}
	updates := map[string]interface{}{
		"status":    int32(pb.ExecutionStatus_STATUS_SUCCESS),
		"timestamp": now.AsTime(),
		"end_time":  now.AsTime(),
	}

	// Encode outputs as JSON if provided
	if outputs != nil {
		outputsJSON, err := json.Marshal(outputs)
		if err == nil {
			updates["outputs_json"] = string(outputsJSON)
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
		"status":        int32(pb.ExecutionStatus_STATUS_FAILED),
		"timestamp":     now.AsTime(),
		"end_time":      now.AsTime(),
		"error_message": err.Error(),
	}

	// Encode outputs as JSON if provided
	if outputs != nil {
		outputsJSON, err := json.Marshal(outputs)
		if err == nil {
			updates["outputs_json"] = string(outputsJSON)
		}
	}

	if updateErr := db.UpdateExecution(ctx, execID, updates); updateErr != nil {
		return fmt.Errorf("failed to log execution failure: %w", updateErr)
	}

	return nil
}

// LogExecutionStatus updates an execution record with a custom status
func LogExecutionStatus(ctx context.Context, db Database, execID string, status pb.ExecutionStatus, outputs interface{}) error {
	now := timestamppb.Now()

	updates := map[string]interface{}{
		"status":    int32(status),
		"timestamp": now.AsTime(),
		"end_time":  now.AsTime(),
	}

	// Encode outputs as JSON if provided
	if outputs != nil {
		outputsJSON, err := json.Marshal(outputs)
		if err == nil {
			updates["outputs_json"] = string(outputsJSON)
		}
	}

	if err := db.UpdateExecution(ctx, execID, updates); err != nil {
		return fmt.Errorf("failed to log execution status %v: %w", status, err)
	}

	return nil
}
