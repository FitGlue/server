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
	httputil "github.com/fitglue/server/src/go/pkg/infrastructure/http"
	"github.com/fitglue/server/src/go/pkg/infrastructure/oauth"
	"github.com/fitglue/server/src/go/pkg/loopprevention"
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

		// Note: Loop prevention is handled at source-handler level (isBounceback check)
		// The source handler checks uploaded_activities before publishing to the enricher

		// 1. Get user's Hevy API key
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

		// 4. Create template resolver for exercise ID lookups
		resolver := NewTemplateResolver(apiKey, fwCtx.Logger)

		// 5. Map to Hevy workout format
		workout, err := mapToHevyWorkout(ctx, &eventPayload, resolver, fwCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to map activity to Hevy format: %w", err)
		}

		// 6. POST to Hevy API
		workoutID, err := createHevyWorkout(ctx, apiKey, workout, fwCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to create Hevy workout: %w", err)
		}

		fwCtx.Logger.Info("Successfully created Hevy workout",
			"workoutId", workoutID,
			"activityId", eventPayload.ActivityId)

		// Record upload for loop prevention
		// When Hevy sends webhooks, we'll check if we just uploaded this activity
		if eventPayload.ActivityData != nil && eventPayload.ActivityData.ExternalId != "" {
			uploadRecord := &pb.UploadedActivityRecord{
				Id:            loopprevention.BuildUploadedActivityID(eventPayload.Source, eventPayload.ActivityData.ExternalId),
				UserId:        eventPayload.UserId,
				Source:        eventPayload.Source,
				ExternalId:    eventPayload.ActivityData.ExternalId,
				StartTime:     eventPayload.StartTime,
				Destination:   pb.Destination_DESTINATION_HEVY,
				DestinationId: workoutID,
				UploadedAt:    timestamppb.Now(),
			}
			if err := svc.DB.SetUploadedActivity(ctx, eventPayload.UserId, uploadRecord); err != nil {
				fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
				// Don't fail the upload - this is just for loop prevention
			} else {
				fwCtx.Logger.Debug("Recorded upload for loop prevention", "id", uploadRecord.Id)
			}
		}

		// 7. Persist SynchronizedActivity
		// Check if activity already exists (e.g., repost scenario)
		// If it does, only update destinations to preserve original pipelineExecutionId
		existingActivity, _ := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
		if existingActivity != nil {
			// Activity exists - update only destinations (preserves original pipelineExecutionId for boosters display)
			// Use nested map structure so MergeAll properly merges into destinations
			if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
				"destinations": map[string]interface{}{
					"hevy": workoutID,
				},
				"synced_at": timestamppb.Now().AsTime(),
			}); err != nil {
				fwCtx.Logger.Error("Failed to update synchronized activity destinations", "error", err)
			} else {
				fwCtx.Logger.Info("Updated synchronized activity destinations (preserved execution ID)", "activity_id", eventPayload.ActivityId)
			}
		} else {
			// New activity - create full record including pipelineExecutionId
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
		}

		// 8. Increment sync count for billing
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		return map[string]interface{}{
			"status":        "SUCCESS",
			"hevy_id":       workoutID,
			"activity_id":   eventPayload.ActivityId,
			"pipeline_id":   eventPayload.PipelineId,
			"activity_name": eventPayload.Name,
			"activity_type": eventPayload.ActivityType.String(),
			"description":   eventPayload.Description,
		}, nil
	}
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

	client := oauth.NewClientWithErrorLogging(fwCtx.Logger, "hevy", 30*time.Second)

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
			"activity_name":  event.Name,
			"activity_type":  event.ActivityType.String(),
			"description":    event.Description,
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
		err := httputil.WrapResponseError(putResp, "Hevy PUT failed")
		fwCtx.Logger.Error("Hevy PUT failed", "status", putResp.StatusCode, "error", err)
		return nil, err
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
		"activity_name":  event.Name,
		"activity_type":  event.ActivityType.String(),
	}, nil
}

// createHevyWorkout POSTs a new workout to Hevy
func createHevyWorkout(ctx context.Context, apiKey string, workout *HevyWorkoutRequest, fwCtx *framework.FrameworkContext) (string, error) {
	bodyJSON, err := json.Marshal(workout)
	if err != nil {
		return "", fmt.Errorf("failed to marshal workout: %w", err)
	}

	client := oauth.NewClientWithErrorLogging(fwCtx.Logger, "hevy", 30*time.Second)
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
		err := httputil.WrapResponseError(resp, "Hevy API error")
		fwCtx.Logger.Error("Hevy API error", "status", resp.StatusCode, "error", err)
		return "", err
	}

	// Parse response to get workout ID
	var respBody struct {
		ID string `json:"workoutId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return "", fmt.Errorf("failed to decode Hevy response: %w", err)
	}

	return respBody.ID, nil
}
