package destination

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	shared "github.com/fitglue/server/src/go/pkg"
	"github.com/fitglue/server/src/go/pkg/types/formatters"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Database interface subset needed for destination updates
// This matches the shared Database interface in interfaces.go
type Database interface {
	UpdatePipelineRun(ctx context.Context, userId string, id string, data map[string]interface{}) error
	SetDestinationOutcome(ctx context.Context, userId string, pipelineRunId string, outcome *pb.DestinationOutcome) error
	GetDestinationOutcomes(ctx context.Context, userId string, pipelineRunId string) ([]*pb.DestinationOutcome, error)
	GetUser(ctx context.Context, id string) (*pb.UserRecord, error)
}

// UpdateStatus updates a single destination's status using the subcollection pattern.
// Each destination is written as a separate document in the destination_outcomes subcollection,
// eliminating race conditions between parallel uploaders.
// When all destinations have reached a terminal status, a push notification is sent to the user.
// Parameters:
//   - db: the Database interface for Firestore operations
//   - notifications: the notification service for sending push notifications (can be nil)
//   - userId: the user's ID
//   - pipelineRunId: the ID of the pipeline run (same as pipelineExecutionId)
//   - dest: the destination enum value (e.g., DESTINATION_STRAVA)
//   - status: the new status (e.g., DESTINATION_STATUS_SUCCESS, DESTINATION_STATUS_FAILED)
//   - externalId: optional external ID from the destination (e.g., Strava activity ID)
//   - errMsg: optional error message if status is FAILED
//   - activityName: the activity name for the push notification title
//   - logger: logger for debugging
func UpdateStatus(ctx context.Context, db Database, notifications shared.NotificationService, userId string, pipelineRunId string, dest pb.Destination, status pb.DestinationStatus, externalId string, errMsg string, activityName string, activityId string, logger *slog.Logger) {
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

	// Convert outcomes to Firestore-compatible format for the inline destinations array
	// This keeps the inline array in sync with the subcollection for UI consumers
	destinationsData := make([]map[string]interface{}, len(outcomes))
	for i, o := range outcomes {
		destData := map[string]interface{}{
			"destination": int32(o.Destination),
			"status":      int32(o.Status),
		}
		if o.ExternalId != nil {
			destData["external_id"] = *o.ExternalId
		}
		if o.Error != nil {
			destData["error"] = *o.Error
		}
		if o.CompletedAt != nil {
			destData["completed_at"] = o.CompletedAt.AsTime()
		}
		destinationsData[i] = destData
	}

	// Update the parent pipeline run's overall status AND inline destinations array
	updateData := map[string]interface{}{
		"status":       int32(newStatus),
		"updated_at":   timestamppb.Now(),
		"destinations": destinationsData,
	}

	if err := db.UpdatePipelineRun(ctx, userId, pipelineRunId, updateData); err != nil {
		logger.Error("Failed to update pipeline run status", "error", err, "pipeline_run_id", pipelineRunId)
	} else {
		logger.Debug("Updated pipeline run status and destinations", "pipeline_run_id", pipelineRunId, "status", newStatus.String(), "destinations_count", len(destinationsData))
	}

	// Send push notification when all destinations have reached a terminal status
	if newStatus == pb.PipelineRunStatus_PIPELINE_RUN_STATUS_SYNCED || newStatus == pb.PipelineRunStatus_PIPELINE_RUN_STATUS_PARTIAL {
		sendSyncNotification(ctx, db, notifications, userId, activityName, activityId, newStatus, outcomes, logger)
	}
}

// sendSyncNotification sends a push notification when all destinations have completed.
// For SYNCED: "Successfully synced to: Strava, Hevy"
// For PARTIAL: "Synced to Strava, but Hevy failed"
func sendSyncNotification(ctx context.Context, db Database, notifications shared.NotificationService, userId string, activityName string, activityId string, status pb.PipelineRunStatus, outcomes []*pb.DestinationOutcome, logger *slog.Logger) {
	if notifications == nil {
		return
	}

	user, err := db.GetUser(ctx, userId)
	if err != nil || user == nil || len(user.FcmTokens) == 0 {
		return
	}

	// Check notification preferences (default to true if not set)
	prefs := user.NotificationPreferences
	if status == pb.PipelineRunStatus_PIPELINE_RUN_STATUS_SYNCED {
		if prefs != nil && !prefs.NotifyPipelineSuccess {
			return
		}
	} else if status == pb.PipelineRunStatus_PIPELINE_RUN_STATUS_PARTIAL {
		if prefs != nil && !prefs.NotifyPipelineFailure {
			return
		}
	}

	// Build human-readable destination lists
	var succeeded []string
	var failed []string
	for _, o := range outcomes {
		name := FormatDestinationName(o.Destination)
		switch o.Status {
		case pb.DestinationStatus_DESTINATION_STATUS_SUCCESS:
			succeeded = append(succeeded, name)
		case pb.DestinationStatus_DESTINATION_STATUS_FAILED:
			failed = append(failed, name)
		}
	}

	var title, body string
	data := map[string]string{
		"type":        "PIPELINE_SUCCESS",
		"user_id":     userId,
		"activity_id": activityId,
	}

	if status == pb.PipelineRunStatus_PIPELINE_RUN_STATUS_SYNCED {
		title = fmt.Sprintf("Activity Synced: %s", activityName)
		body = fmt.Sprintf("Successfully synced to: %s", strings.Join(succeeded, ", "))
	} else {
		title = fmt.Sprintf("Partial Sync: %s", activityName)
		if len(succeeded) > 0 && len(failed) > 0 {
			body = fmt.Sprintf("Synced to %s, but %s failed", strings.Join(succeeded, ", "), strings.Join(failed, ", "))
		} else if len(failed) > 0 {
			body = fmt.Sprintf("Failed to sync to: %s", strings.Join(failed, ", "))
		}
		data["type"] = "PIPELINE_FAILED"
	}

	if err := notifications.SendPushNotification(ctx, userId, title, body, user.FcmTokens, data); err != nil {
		logger.Warn("Failed to send sync notification", "error", err, "user_id", userId)
	}
}

// FormatDestinationName returns a human-readable name for a destination.
// Delegates to the generated formatters package for consistency across Go and TypeScript.
func FormatDestinationName(dest pb.Destination) string {
	return formatters.FormatDestination(dest)
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
