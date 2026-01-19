package hevyuploader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/framework"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("UploadToHevy", UploadToHevy)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		baseSvc, err := bootstrap.NewService(ctx)
		if err != nil {
			slog.Error("Failed to initialize service", "error", err)
			svcErr = err
			return
		}
		svc = baseSvc
	})
	return svc, svcErr
}

// UploadToHevy is the Cloud Function entry point
func UploadToHevy(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("hevy-uploader", svc, uploadHandler())(ctx, e)
}

// uploadHandler contains the business logic
func uploadHandler() framework.HandlerFunc {
	return func(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		var eventPayload pb.EnrichedActivityEvent

		unmarshaler := protojson.UnmarshalOptions{
			DiscardUnknown: true,
			AllowPartial:   true,
		}
		if err := unmarshaler.Unmarshal(e.Data(), &eventPayload); err != nil {
			return nil, fmt.Errorf("protojson.Unmarshal: %w", err)
		}

		fwCtx.Logger.Info("Starting Hevy upload",
			"activity_id", eventPayload.ActivityId,
			"pipeline_id", eventPayload.PipelineId,
			"user_id", eventPayload.UserId,
		)

		// 1. Loop Prevention Check
		// Skip if this activity originated from Hevy (would create a loop)
		if isLoopOrigin(&eventPayload) {
			fwCtx.Logger.Info("Skipping Hevy upload - activity originated from Hevy",
				"activity_id", eventPayload.ActivityId)
			return map[string]interface{}{
				"status": "SKIPPED",
				"reason": "loop_prevention",
			}, nil
		}

		// 2. Get user's Hevy API key
		user, err := svc.DB.GetUser(ctx, eventPayload.UserId)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		if user.Integrations == nil || user.Integrations.Hevy == nil || user.Integrations.Hevy.ApiKey == "" {
			fwCtx.Logger.Warn("User has no Hevy API key configured", "userId", eventPayload.UserId)
			return map[string]interface{}{
				"status": "FAILED",
				"reason": "no_hevy_api_key",
			}, fmt.Errorf("user has no Hevy API key configured")
		}

		apiKey := user.Integrations.Hevy.ApiKey

		// 3. Check for existing activity in Hevy (dedup check)
		existingWorkoutID := checkExistingActivity(ctx, fwCtx, &eventPayload)
		if existingWorkoutID != "" {
			fwCtx.Logger.Info("Activity already exists in Hevy, using UPDATE mode",
				"existingWorkoutID", existingWorkoutID)
			return handleHevyUpdate(ctx, apiKey, &eventPayload, existingWorkoutID, fwCtx)
		}

		// 4. Map to Hevy workout format
		workout, err := mapToHevyWorkout(ctx, &eventPayload, apiKey, fwCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to map activity to Hevy format: %w", err)
		}

		// 5. POST to Hevy API
		workoutID, err := createHevyWorkout(ctx, apiKey, workout, fwCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to create Hevy workout: %w", err)
		}

		fwCtx.Logger.Info("Successfully created Hevy workout",
			"workoutId", workoutID,
			"activityId", eventPayload.ActivityId)

		// 6. Persist SynchronizedActivity
		syncedActivity := &pb.SynchronizedActivity{
			ActivityId:          eventPayload.ActivityId,
			Title:               eventPayload.Name,
			Description:         eventPayload.Description,
			Type:                eventPayload.ActivityType,
			Source:              eventPayload.Source.String(),
			StartTime:           eventPayload.StartTime,
			SyncedAt:            timestamppb.Now(),
			PipelineId:          eventPayload.PipelineId,
			PipelineExecutionId: fwCtx.PipelineExecutionId,
			Destinations: map[string]string{
				"hevy": workoutID,
			},
		}

		if err := svc.DB.SetSynchronizedActivity(ctx, eventPayload.UserId, syncedActivity); err != nil {
			fwCtx.Logger.Error("Failed to persist synchronized activity", "error", err)
			// Don't fail - just log
		}

		// Increment sync count for billing
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		return map[string]interface{}{
			"status":      "SUCCESS",
			"hevy_id":     workoutID,
			"activity_id": eventPayload.ActivityId,
			"pipeline_id": eventPayload.PipelineId,
		}, nil
	}
}

// isLoopOrigin checks if the activity originated from Hevy (source or destination)
func isLoopOrigin(event *pb.EnrichedActivityEvent) bool {
	// Check enrichment_metadata for origin_destination marker
	if event.EnrichmentMetadata != nil {
		if origin, ok := event.EnrichmentMetadata["origin_destination"]; ok && origin == "hevy" {
			return true
		}
	}

	// Also check if source is Hevy (self-loop prevention)
	if event.Source == pb.ActivitySource_SOURCE_HEVY {
		return true
	}

	return false
}

// checkExistingActivity looks up if we've already synced this activity to Hevy
func checkExistingActivity(ctx context.Context, fwCtx *framework.FrameworkContext, event *pb.EnrichedActivityEvent) string {
	// Look up in SynchronizedActivity by internal activity ID
	syncActivity, err := svc.DB.GetSynchronizedActivity(ctx, event.UserId, event.ActivityId)
	if err != nil {
		fwCtx.Logger.Debug("No existing synchronized activity found", "activityId", event.ActivityId)
		return ""
	}

	if syncActivity != nil && syncActivity.Destinations != nil {
		if hevyID, ok := syncActivity.Destinations["hevy"]; ok && hevyID != "" {
			return hevyID
		}
	}

	return ""
}

// handleHevyUpdate updates an existing workout in Hevy (PUT /v1/workouts/{workoutId})
// Used in resume mode for delayed enrichment or when activity already exists
func handleHevyUpdate(ctx context.Context, apiKey string, event *pb.EnrichedActivityEvent, workoutID string, fwCtx *framework.FrameworkContext) (interface{}, error) {
	fwCtx.Logger.Info("Starting Hevy UPDATE",
		"workoutId", workoutID,
		"activityId", event.ActivityId)

	client := &http.Client{Timeout: 30 * time.Second}

	// 1. GET current workout from Hevy to get existing description
	getURL := fmt.Sprintf("https://api.hevyapp.com/v1/workouts/%s", workoutID)
	getReq, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}
	getReq.Header.Set("api-key", apiKey)

	getResp, err := client.Do(getReq)
	if err != nil {
		return nil, fmt.Errorf("failed to GET existing workout: %w", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		var errorBody bytes.Buffer
		errorBody.ReadFrom(getResp.Body)
		return nil, fmt.Errorf("GET workout failed: status %d, body: %s", getResp.StatusCode, errorBody.String())
	}

	var existingWorkout struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&existingWorkout); err != nil {
		return nil, fmt.Errorf("failed to decode existing workout: %w", err)
	}

	fwCtx.Logger.Debug("Fetched existing Hevy workout",
		"workoutId", workoutID,
		"existingTitle", existingWorkout.Title,
		"existingDescLength", len(existingWorkout.Description))

	// 2. Merge new description with existing (append, don't overwrite)
	mergedDescription := existingWorkout.Description
	if event.Description != "" {
		if mergedDescription != "" {
			mergedDescription += "\n\n" + event.Description
		} else {
			mergedDescription = event.Description
		}
	}

	// 3. Build update payload
	updateBody := map[string]interface{}{}
	if event.Name != "" && event.Name != existingWorkout.Title {
		updateBody["title"] = event.Name
	}
	if mergedDescription != existingWorkout.Description {
		updateBody["description"] = mergedDescription
	}

	// If no changes, skip the PUT
	if len(updateBody) == 0 {
		fwCtx.Logger.Info("No changes to update, skipping PUT")
		return map[string]interface{}{
			"status":         "SUCCESS",
			"hevy_id":        workoutID,
			"activity_id":    event.ActivityId,
			"mode":           "UPDATE",
			"update_skipped": true,
			"reason":         "no_changes",
		}, nil
	}

	// 4. PUT to Hevy
	bodyJSON, err := json.Marshal(updateBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update body: %w", err)
	}

	putURL := fmt.Sprintf("https://api.hevyapp.com/v1/workouts/%s", workoutID)
	putReq, err := http.NewRequestWithContext(ctx, "PUT", putURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	putReq.Header.Set("api-key", apiKey)
	putReq.Header.Set("Content-Type", "application/json")

	fwCtx.Logger.Info("Updating Hevy workout (PUT)",
		"workoutId", workoutID,
		"updatedFields", updateBody,
		"descriptionLength", len(mergedDescription))

	putResp, err := client.Do(putReq)
	if err != nil {
		return nil, fmt.Errorf("failed to PUT workout: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode >= 400 {
		var errorBody bytes.Buffer
		errorBody.ReadFrom(putResp.Body)
		fwCtx.Logger.Error("Hevy PUT failed", "status", putResp.StatusCode, "body", errorBody.String())
		return nil, fmt.Errorf("hevy PUT failed: status %d", putResp.StatusCode)
	}

	fwCtx.Logger.Info("Successfully updated Hevy workout",
		"workoutId", workoutID,
		"updatedFields", updateBody)

	// 5. Update SynchronizedActivity with merged description
	if err := svc.DB.UpdateSynchronizedActivity(ctx, event.UserId, event.ActivityId, map[string]interface{}{
		"description": mergedDescription,
	}); err != nil {
		fwCtx.Logger.Warn("Failed to update synchronized activity description", "error", err)
		// Don't fail - just log
	}

	return map[string]interface{}{
		"status":         "SUCCESS",
		"hevy_id":        workoutID,
		"activity_id":    event.ActivityId,
		"mode":           "UPDATE",
		"updated_fields": updateBody,
	}, nil
}

// createHevyWorkout POSTs a new workout to Hevy
func createHevyWorkout(ctx context.Context, apiKey string, workout *HevyWorkoutRequest, fwCtx *framework.FrameworkContext) (string, error) {
	bodyJSON, err := json.Marshal(workout)
	if err != nil {
		return "", fmt.Errorf("failed to marshal workout: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.hevyapp.com/v1/workouts", bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	fwCtx.Logger.Debug("Sending POST to Hevy",
		"bodyLength", len(bodyJSON))

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Hevy API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errorBody bytes.Buffer
		errorBody.ReadFrom(resp.Body)
		fwCtx.Logger.Error("Hevy API error",
			"status", resp.StatusCode,
			"body", errorBody.String())
		return "", fmt.Errorf("Hevy API error: status %d", resp.StatusCode)
	}

	// Parse response to get workout ID
	var respBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return "", fmt.Errorf("failed to decode Hevy response: %w", err)
	}

	return respBody.ID, nil
}
