package destination

import (
	"context"
	"log/slog"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Database interface subset needed for destination updates
// This matches the shared Database interface in interfaces.go
type Database interface {
	UpdatePipelineRun(ctx context.Context, userId string, id string, data map[string]interface{}) error
	SetDestinationOutcome(ctx context.Context, userId string, pipelineRunId string, outcome *pb.DestinationOutcome) error
	GetDestinationOutcomes(ctx context.Context, userId string, pipelineRunId string) ([]*pb.DestinationOutcome, error)
}

// UpdateStatus updates a single destination's status using the subcollection pattern.
// Each destination is written as a separate document in the destination_outcomes subcollection,
// eliminating race conditions between parallel uploaders.
// Parameters:
//   - db: the Database interface for Firestore operations
//   - userId: the user's ID
//   - pipelineRunId: the ID of the pipeline run (same as pipelineExecutionId)
//   - dest: the destination enum value (e.g., DESTINATION_STRAVA)
//   - status: the new status (e.g., DESTINATION_STATUS_SUCCESS, DESTINATION_STATUS_FAILED)
//   - externalId: optional external ID from the destination (e.g., Strava activity ID)
//   - errMsg: optional error message if status is FAILED
//   - logger: logger for debugging
func UpdateStatus(ctx context.Context, db Database, userId string, pipelineRunId string, dest pb.Destination, status pb.DestinationStatus, externalId string, errMsg string, logger *slog.Logger) {
	if pipelineRunId == "" {
		return // No pipeline run to update - legacy flow
	}

	// Build the outcome
	outcome := &pb.DestinationOutcome{
		Destination: dest,
		Status:      status,
		CompletedAt: timestamppb.Now(),
	}
	if externalId != "" {
		outcome.ExternalId = &externalId
	}
	if errMsg != "" {
		outcome.Error = &errMsg
	}

	// Write directly to subcollection - each destination has its own document
	// No read-modify-write needed, eliminating race conditions
	if err := db.SetDestinationOutcome(ctx, userId, pipelineRunId, outcome); err != nil {
		logger.Error("Failed to set destination outcome", "error", err, "pipeline_run_id", pipelineRunId, "destination", dest.String())
		return
	}

	logger.Debug("Set destination outcome in subcollection", "pipeline_run_id", pipelineRunId, "destination", dest.String(), "status", status.String())

	// Now compute and update the overall pipeline status
	// Read all destination outcomes from subcollection
	outcomes, err := db.GetDestinationOutcomes(ctx, userId, pipelineRunId)
	if err != nil {
		logger.Warn("Failed to get destination outcomes for status computation", "error", err, "pipeline_run_id", pipelineRunId)
		return
	}

	newStatus := ComputePipelineRunStatus(outcomes)

	// Update the parent pipeline run's overall status
	updateData := map[string]interface{}{
		"status":     int32(newStatus),
		"updated_at": timestamppb.Now(),
	}

	if err := db.UpdatePipelineRun(ctx, userId, pipelineRunId, updateData); err != nil {
		logger.Error("Failed to update pipeline run status", "error", err, "pipeline_run_id", pipelineRunId)
	} else {
		logger.Debug("Updated pipeline run status", "pipeline_run_id", pipelineRunId, "status", newStatus.String())
	}
}

// ComputePipelineRunStatus determines overall status from destination outcomes
func ComputePipelineRunStatus(destinations []*pb.DestinationOutcome) pb.PipelineRunStatus {
	if len(destinations) == 0 {
		return pb.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING
	}

	allSuccess := true
	anyFailed := false
	allComplete := true

	for _, d := range destinations {
		switch d.Status {
		case pb.DestinationStatus_DESTINATION_STATUS_PENDING:
			allComplete = false
			allSuccess = false
		case pb.DestinationStatus_DESTINATION_STATUS_FAILED:
			anyFailed = true
			allSuccess = false
		case pb.DestinationStatus_DESTINATION_STATUS_SUCCESS:
			// Good
		case pb.DestinationStatus_DESTINATION_STATUS_SKIPPED:
			// Skipped doesn't count as failure
		}
	}

	if !allComplete {
		return pb.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING
	}
	if allSuccess {
		return pb.PipelineRunStatus_PIPELINE_RUN_STATUS_SYNCED
	}
	if anyFailed {
		return pb.PipelineRunStatus_PIPELINE_RUN_STATUS_PARTIAL
	}
	return pb.PipelineRunStatus_PIPELINE_RUN_STATUS_SYNCED
}
