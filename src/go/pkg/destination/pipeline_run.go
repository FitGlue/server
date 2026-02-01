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
	GetPipelineRun(ctx context.Context, userId string, id string) (*pb.PipelineRun, error)
}

// UpdateStatus updates a single destination's status in the PipelineRun.
// Uses additive-only updates to avoid race conditions between parallel uploaders.
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

	// Fetch current PipelineRun
	pipelineRun, err := db.GetPipelineRun(ctx, userId, pipelineRunId)
	if err != nil || pipelineRun == nil {
		logger.Warn("Failed to get pipeline run for destination update", "error", err, "pipeline_run_id", pipelineRunId)
		return
	}

	// Find and update the matching destination (additive - only updates the specific destination)
	found := false
	for i, outcome := range pipelineRun.Destinations {
		if outcome.Destination == dest {
			pipelineRun.Destinations[i].Status = status
			pipelineRun.Destinations[i].CompletedAt = timestamppb.Now()
			if externalId != "" {
				pipelineRun.Destinations[i].ExternalId = &externalId
			}
			if errMsg != "" {
				pipelineRun.Destinations[i].Error = &errMsg
			}
			found = true
			break
		}
	}

	if !found {
		logger.Warn("Destination not found in pipeline run", "destination", dest.String(), "pipeline_run_id", pipelineRunId)
		return
	}

	// Compute overall status based on all destinations
	newStatus := ComputePipelineRunStatus(pipelineRun.Destinations)

	// Update with the modified destinations array
	updateData := map[string]interface{}{
		"destinations": pipelineRun.Destinations,
		"status":       int32(newStatus),
		"updated_at":   timestamppb.Now(),
	}

	if err := db.UpdatePipelineRun(ctx, userId, pipelineRunId, updateData); err != nil {
		logger.Error("Failed to update pipeline run destination", "error", err, "pipeline_run_id", pipelineRunId, "destination", dest.String())
	} else {
		logger.Debug("Updated pipeline run destination", "pipeline_run_id", pipelineRunId, "destination", dest.String(), "status", status.String())
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
