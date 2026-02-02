package hevyuploader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/description"
	"github.com/fitglue/server/src/go/pkg/destination"
	"github.com/fitglue/server/src/go/pkg/domain/activity"
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
			// Error returned to caller
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

		// Resolve activity data from GCS if needed (for large payloads offloaded by enricher)
		if err := activity.ResolveEnrichedEvent(ctx, &eventPayload, fwCtx.Service.Store); err != nil {
			fwCtx.Logger.Warn("Failed to resolve activity data from GCS", "error", err)
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
			destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", "no Hevy API key configured", fwCtx.Logger)
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
			destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("failed to map to Hevy format: %s", err), fwCtx.Logger)
			return nil, fmt.Errorf("failed to map activity to Hevy format: %w", err)
		}

		// 6. POST to Hevy API
		workoutID, err := createHevyWorkout(ctx, apiKey, workout, fwCtx)
		if err != nil {
			destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("API error: %s", err), fwCtx.Logger)
			return nil, fmt.Errorf("failed to create Hevy workout: %w", err)
		}

		fwCtx.Logger.Info("Successfully created Hevy workout",
			"workoutId", workoutID,
			"activityId", eventPayload.ActivityId)

		// Record upload for loop prevention
		// Key is destination:destinationId so when Hevy sends a webhook with workoutID,
		// we can look it up by HEVY:{workoutID} and detect the bounceback
		if workoutID != "" {
			uploadRecord := &pb.UploadedActivityRecord{
				Id:            loopprevention.BuildUploadedActivityID(pb.Destination_DESTINATION_HEVY, workoutID),
				UserId:        eventPayload.UserId,
				Source:        eventPayload.Source,
				ExternalId:    eventPayload.ActivityData.GetExternalId(),
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

		// Update PipelineRun destination as synced
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, workoutID, "", fwCtx.Logger)

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
// CRITICAL: Hevy PUT API requires the FULL workout payload (same as POST), not partial updates.
// We must GET the existing workout, merge only title/description, then PUT the entire workout back.
func handleHevyUpdate(ctx context.Context, apiKey string, event *pb.EnrichedActivityEvent, workoutID string, fwCtx *framework.FrameworkContext) (interface{}, error) {
	fwCtx.Logger.Info("Starting Hevy UPDATE",
		"workoutId", workoutID,
		"activityId", event.ActivityId)

	client := oauth.NewClientWithErrorLogging(fwCtx.Logger, "hevy", 30*time.Second)

	// 1. GET the FULL current workout from Hevy (including exercises)
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

	// Parse the FULL workout response including exercises
	// Using a struct that matches the Hevy API GET response
	type HevySet struct {
		CustomMetric    *float32 `json:"custom_metric"`
		DistanceMeters  *float32 `json:"distance_meters"`
		DurationSeconds *float32 `json:"duration_seconds"`
		Index           *float32 `json:"index,omitempty"`
		Reps            *float32 `json:"reps"`
		Rpe             *float32 `json:"rpe"`
		Type            *string  `json:"type,omitempty"`
		WeightKg        *float32 `json:"weight_kg"`
	}
	type HevyExercise struct {
		ExerciseTemplateId *string    `json:"exercise_template_id,omitempty"`
		Index              *float32   `json:"index,omitempty"`
		Notes              *string    `json:"notes,omitempty"`
		Sets               *[]HevySet `json:"sets,omitempty"`
		SupersetId         *float32   `json:"superset_id"`
		Title              *string    `json:"title,omitempty"`
	}
	type HevyWorkoutFull struct {
		ID          *string         `json:"id,omitempty"`
		Title       *string         `json:"title,omitempty"`
		Description *string         `json:"description,omitempty"`
		StartTime   *string         `json:"start_time,omitempty"`
		EndTime     *string         `json:"end_time,omitempty"`
		IsPrivate   *bool           `json:"is_private,omitempty"`
		Exercises   *[]HevyExercise `json:"exercises,omitempty"`
		RoutineId   *string         `json:"routine_id,omitempty"`
		CreatedAt   *string         `json:"created_at,omitempty"`
		UpdatedAt   *string         `json:"updated_at,omitempty"`
	}

	var existingWorkout HevyWorkoutFull
	if err := json.NewDecoder(getResp.Body).Decode(&existingWorkout); err != nil {
		return nil, fmt.Errorf("failed to decode existing workout: %w", err)
	}

	existingTitle := ""
	if existingWorkout.Title != nil {
		existingTitle = *existingWorkout.Title
	}
	existingDesc := ""
	if existingWorkout.Description != nil {
		existingDesc = *existingWorkout.Description
	}

	exerciseCount := 0
	if existingWorkout.Exercises != nil {
		exerciseCount = len(*existingWorkout.Exercises)
	}

	fwCtx.Logger.Debug("Fetched existing Hevy workout",
		"workoutId", workoutID,
		"existingTitle", existingTitle,
		"existingDescLength", len(existingDesc),
		"exerciseCount", exerciseCount)

	// 2. Merge description: use DESTINATION's description as base (fetched via GET above)
	// then apply section replacement with the enricher's new content
	mergedDescription := existingDesc // From destination API, NOT event
	if event.Description != "" {
		// Check for section header in metadata (signals replaceable section)
		sectionHeader := ""
		for key, val := range event.EnrichmentMetadata {
			if strings.HasPrefix(key, "section_header_") {
				sectionHeader = val
				break
			}
		}

		if sectionHeader != "" && description.HasSection(mergedDescription, sectionHeader) {
			// Extract just the section content from event.Description
			// The payload contains the full description, but we only want the specific section
			newSectionContent := description.ExtractSection(event.Description, sectionHeader)
			if newSectionContent != "" {
				mergedDescription = description.ReplaceSection(mergedDescription, sectionHeader, newSectionContent)
				fwCtx.Logger.Info("Replaced description section", "header", sectionHeader)
			} else {
				fwCtx.Logger.Warn("Section header found in metadata but content not found in payload", "header", sectionHeader)
			}
		} else if mergedDescription != "" {
			// Fallback to append
			mergedDescription += "\n\n" + event.Description
		} else {
			mergedDescription = event.Description
		}
	}

	// 3. Determine what changed (for logging/response only)
	// In UPDATE mode, we intentionally do NOT update the title.
	// The activity already exists on Hevy, and the user may have customized the title there.
	updatedFields := map[string]interface{}{}
	if mergedDescription != existingDesc {
		updatedFields["description"] = "updated"
	}

	// If no changes, skip the PUT
	if len(updatedFields) == 0 {
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

	// 4. Build the FULL workout payload for PUT (Hevy requires complete workout, not partial)
	// Convert exercises from GET response format to POST/PUT request format
	type PutSet struct {
		CustomMetric    *float32 `json:"custom_metric"`
		DistanceMeters  *int     `json:"distance_meters"`
		DurationSeconds *int     `json:"duration_seconds"`
		Reps            *int     `json:"reps"`
		Rpe             *float32 `json:"rpe"`
		Type            *string  `json:"type,omitempty"`
		WeightKg        *float32 `json:"weight_kg"`
	}
	type PutExercise struct {
		ExerciseTemplateId *string   `json:"exercise_template_id,omitempty"`
		Notes              *string   `json:"notes"`
		Sets               *[]PutSet `json:"sets,omitempty"`
		SupersetId         *int      `json:"superset_id"`
	}
	type PutWorkout struct {
		Title       *string        `json:"title,omitempty"`
		Description *string        `json:"description"`
		StartTime   *string        `json:"start_time,omitempty"`
		EndTime     *string        `json:"end_time,omitempty"`
		IsPrivate   *bool          `json:"is_private,omitempty"`
		Exercises   *[]PutExercise `json:"exercises,omitempty"`
	}
	type PutWorkoutRequest struct {
		Workout *PutWorkout `json:"workout,omitempty"`
	}

	// Convert exercises from GET format to PUT format
	var putExercises []PutExercise
	if existingWorkout.Exercises != nil {
		for _, ex := range *existingWorkout.Exercises {
			var putSets []PutSet
			if ex.Sets != nil {
				for _, s := range *ex.Sets {
					putSet := PutSet{
						CustomMetric: s.CustomMetric,
						Rpe:          s.Rpe,
						Type:         s.Type,
						WeightKg:     s.WeightKg,
					}
					// Convert float32 pointers to int pointers (API schema difference)
					if s.DistanceMeters != nil {
						v := int(*s.DistanceMeters)
						putSet.DistanceMeters = &v
					}
					if s.DurationSeconds != nil {
						v := int(*s.DurationSeconds)
						putSet.DurationSeconds = &v
					}
					if s.Reps != nil {
						v := int(*s.Reps)
						putSet.Reps = &v
					}
					putSets = append(putSets, putSet)
				}
			}

			putEx := PutExercise{
				ExerciseTemplateId: ex.ExerciseTemplateId,
				Notes:              ex.Notes,
			}
			if len(putSets) > 0 {
				putEx.Sets = &putSets
			}
			// Convert superset_id from float32 to int
			if ex.SupersetId != nil {
				v := int(*ex.SupersetId)
				putEx.SupersetId = &v
			}
			putExercises = append(putExercises, putEx)
		}
	}

	// Build the full PUT payload
	putPayload := PutWorkoutRequest{
		Workout: &PutWorkout{
			Title:       &existingTitle, // Preserve existing title, don't update
			Description: &mergedDescription,
			StartTime:   existingWorkout.StartTime,
			EndTime:     existingWorkout.EndTime,
			IsPrivate:   existingWorkout.IsPrivate,
		},
	}
	if len(putExercises) > 0 {
		putPayload.Workout.Exercises = &putExercises
	}

	bodyJSON, err := json.Marshal(putPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update body: %w", err)
	}

	fwCtx.Logger.Debug("PUT payload prepared",
		"payloadLength", len(bodyJSON),
		"exerciseCount", len(putExercises))

	// 5. PUT the FULL workout to Hevy
	putURL := fmt.Sprintf("https://api.hevyapp.com/v1/workouts/%s", workoutID)
	putReq, err := http.NewRequestWithContext(ctx, "PUT", putURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	putReq.Header.Set("api-key", apiKey)
	putReq.Header.Set("Content-Type", "application/json")

	fwCtx.Logger.Info("Updating Hevy workout (PUT)",
		"workoutId", workoutID,
		"updatedFields", updatedFields,
		"descriptionLength", len(mergedDescription),
		"exerciseCount", len(putExercises))

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
		"updatedFields", updatedFields)

	// 6. Update SynchronizedActivity with merged description
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
		"updated_fields": updatedFields,
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
