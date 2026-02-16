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

	hevy "github.com/fitglue/server/src/go/pkg/api/hevy"
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
			destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("failed to get user: %s", err), eventPayload.Name, eventPayload.ActivityId, fwCtx.Logger)
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		if user.Integrations == nil || user.Integrations.Hevy == nil || user.Integrations.Hevy.ApiKey == "" {
			fwCtx.Logger.Warn("User has no Hevy API key configured", "userId", eventPayload.UserId)
			destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", "no Hevy API key configured", eventPayload.Name, eventPayload.ActivityId, fwCtx.Logger)
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

		// 3b. Same-Source Detection: If Hevy is both source AND destination,
		// the activity already exists natively on Hevy. Use the source's
		// external ID as the workout ID for the update path.
		if eventPayload.EnrichmentMetadata["same_source_destination_hevy"] == "true" {
			sourceExternalID := eventPayload.ActivityData.GetExternalId()
			if sourceExternalID != "" {
				fwCtx.Logger.Info("Same-source detected (Hevy→Hevy), routing to update",
					"sourceExternalID", sourceExternalID)
				return handleHevyUpdate(ctx, apiKey, &eventPayload, sourceExternalID, fwCtx)
			}
		}

		// 4. Create template resolver for exercise ID lookups
		resolver := NewTemplateResolver(apiKey, fwCtx.Logger)

		// 5. Map to Hevy workout format
		workout, err := mapToHevyWorkout(ctx, &eventPayload, resolver, fwCtx)
		if err != nil {
			destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("failed to map to Hevy format: %s", err), eventPayload.Name, eventPayload.ActivityId, fwCtx.Logger)
			return nil, fmt.Errorf("failed to map activity to Hevy format: %w", err)
		}

		// 6. POST to Hevy API
		workoutID, err := createHevyWorkout(ctx, apiKey, workout, fwCtx)
		if err != nil {
			destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("API error: %s", err), eventPayload.Name, eventPayload.ActivityId, fwCtx.Logger)
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

		// Note: synchronized_activities is deprecated - pipeline_runs is now the source of truth
		// The destination.UpdateStatus call at the end of this function updates pipeline_runs with the externalId

		// 8. Increment sync count for billing
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		// Update PipelineRun destination as synced
		destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, workoutID, "", eventPayload.Name, eventPayload.ActivityId, fwCtx.Logger)

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
	// Look up in PipelineRun by internal activity ID
	pipelineRun, err := svc.DB.GetPipelineRunByActivityId(ctx, event.UserId, event.ActivityId)
	if err != nil {
		fwCtx.Logger.Debug("No existing pipeline run found", "activityId", event.ActivityId)
		return ""
	}

	if pipelineRun != nil && pipelineRun.Destinations != nil {
		for _, dest := range pipelineRun.Destinations {
			if dest.Destination == pb.Destination_DESTINATION_HEVY && dest.ExternalId != nil && *dest.ExternalId != "" {
				return *dest.ExternalId
			}
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

	// Parse into generated Workout type (which includes exercises)
	// The Hevy GET API may return {"workout": {...}} or a flat object
	var existingWorkout hevy.Workout
	var getRespBytes bytes.Buffer
	getRespBytes.ReadFrom(getResp.Body)
	getRespData := getRespBytes.Bytes()

	fwCtx.Logger.Debug("Hevy GET response", "body", string(getRespData))

	// Try to detect the wrapped format first: {"workout": {...}}
	var rawGetResp map[string]interface{}
	if err := json.Unmarshal(getRespData, &rawGetResp); err == nil {
		if workoutObj, ok := rawGetResp["workout"]; ok {
			// Re-marshal just the workout object and decode into the typed struct
			workoutBytes, marshalErr := json.Marshal(workoutObj)
			if marshalErr == nil {
				if decodeErr := json.Unmarshal(workoutBytes, &existingWorkout); decodeErr != nil {
					return nil, fmt.Errorf("failed to decode unwrapped workout: %w", decodeErr)
				}
			}
		} else {
			// Flat response - decode directly
			if err := json.Unmarshal(getRespData, &existingWorkout); err != nil {
				return nil, fmt.Errorf("failed to decode existing workout: %w", err)
			}
		}
	} else {
		return nil, fmt.Errorf("failed to decode existing workout response: %w", err)
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

	// 2. Merge description
	// Same-source: straight overwrite of title/description (activity already exists on this platform)
	// Resume/other: section-based replacement preserving user customizations
	isSameSource := event.EnrichmentMetadata["same_source_destination_hevy"] == "true"

	var mergedDescription string
	var mergedTitle string
	if isSameSource {
		// Same-source update: overwrite with enriched values
		mergedDescription = event.Description
		mergedTitle = event.Name
		fwCtx.Logger.Info("Same-source update: overwriting title/description",
			"newTitle", mergedTitle,
			"newDescLength", len(mergedDescription))
	} else {
		// Resume/standard update: use destination's description as base
		mergedDescription = existingDesc
		mergedTitle = existingTitle
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
				newSectionContent := description.ExtractSection(event.Description, sectionHeader)
				if newSectionContent != "" {
					mergedDescription = description.ReplaceSection(mergedDescription, sectionHeader, newSectionContent)
					fwCtx.Logger.Info("Replaced description section", "header", sectionHeader)
				} else {
					fwCtx.Logger.Warn("Section header found in metadata but content not found in payload", "header", sectionHeader)
				}
			} else if mergedDescription != "" {
				mergedDescription += "\n\n" + event.Description
			} else {
				mergedDescription = event.Description
			}
		}
	}

	// 3. Determine what changed (for logging/response only)
	updatedFields := map[string]interface{}{}
	if mergedTitle != existingTitle {
		updatedFields["title"] = "updated"
	}
	if mergedDescription != existingDesc {
		updatedFields["description"] = "updated"
	}

	// If no changes, skip the PUT
	if len(updatedFields) == 0 {
		fwCtx.Logger.Info("No changes to update, skipping PUT")
		// Record upload for loop prevention even when skipping PUT
		// The activity IS on Hevy, so we need bounceback protection
		uploadRecord := &pb.UploadedActivityRecord{
			Id:            loopprevention.BuildUploadedActivityID(pb.Destination_DESTINATION_HEVY, workoutID),
			UserId:        event.UserId,
			Source:        event.Source,
			ExternalId:    event.ActivityData.GetExternalId(),
			StartTime:     event.StartTime,
			Destination:   pb.Destination_DESTINATION_HEVY,
			DestinationId: workoutID,
			UploadedAt:    timestamppb.Now(),
		}
		if err := svc.DB.SetUploadedActivity(ctx, event.UserId, uploadRecord); err != nil {
			fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
		}
		// Update PipelineRun destination as success (no changes needed, but activity is already synced)
		destination.UpdateStatus(ctx, svc.DB, svc.Notifications, event.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, workoutID, "", event.Name, event.ActivityId, fwCtx.Logger)
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

	// Convert exercises from GET format to PUT format
	var putExercises []hevy.PostWorkoutsRequestExercise
	if existingWorkout.Exercises != nil {
		for _, ex := range *existingWorkout.Exercises {
			var putSets []hevy.PostWorkoutsRequestSet
			if ex.Sets != nil {
				for _, s := range *ex.Sets {
					putSet := hevy.PostWorkoutsRequestSet{
						CustomMetric: s.CustomMetric,
						WeightKg:     s.WeightKg,
					}
					// Convert Rpe from *float32 to *PostWorkoutsRequestSetRpe
					if s.Rpe != nil {
						rpe := hevy.PostWorkoutsRequestSetRpe(*s.Rpe)
						putSet.Rpe = &rpe
					}
					// Convert Type from *string to *PostWorkoutsRequestSetType
					if s.Type != nil {
						setType := hevy.PostWorkoutsRequestSetType(*s.Type)
						putSet.Type = &setType
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

			putEx := hevy.PostWorkoutsRequestExercise{
				ExerciseTemplateId: ex.ExerciseTemplateId,
			}
			// Only set Notes if non-nil and non-empty — Hevy API rejects empty string notes
			if ex.Notes != nil && *ex.Notes != "" {
				putEx.Notes = ex.Notes
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
	putPayload := hevy.PostWorkoutsRequestBody{
		Workout: &struct {
			Description *string                             `json:"description"`
			EndTime     *string                             `json:"end_time,omitempty"`
			Exercises   *[]hevy.PostWorkoutsRequestExercise `json:"exercises,omitempty"`
			IsPrivate   *bool                               `json:"is_private,omitempty"`
			StartTime   *string                             `json:"start_time,omitempty"`
			Title       *string                             `json:"title,omitempty"`
		}{
			Title:       &mergedTitle,
			Description: &mergedDescription,
			StartTime:   existingWorkout.StartTime,
			EndTime:     existingWorkout.EndTime,
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

	// Note: synchronized_activities is deprecated - pipeline_runs is now the source of truth
	// We no longer update synchronized_activities here

	// Record upload for loop prevention (same as create path)
	uploadRecord := &pb.UploadedActivityRecord{
		Id:            loopprevention.BuildUploadedActivityID(pb.Destination_DESTINATION_HEVY, workoutID),
		UserId:        event.UserId,
		Source:        event.Source,
		ExternalId:    event.ActivityData.GetExternalId(),
		StartTime:     event.StartTime,
		Destination:   pb.Destination_DESTINATION_HEVY,
		DestinationId: workoutID,
		UploadedAt:    timestamppb.Now(),
	}
	if err := svc.DB.SetUploadedActivity(ctx, event.UserId, uploadRecord); err != nil {
		fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
	}

	// Increment sync count for billing (skip for same-source updates - activity already existed)
	if !isSameSource {
		if err := svc.DB.IncrementSyncCount(ctx, event.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", event.UserId)
		}
	}

	// Update PipelineRun destination as synced
	destination.UpdateStatus(ctx, svc.DB, svc.Notifications, event.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, workoutID, "", event.Name, event.ActivityId, fwCtx.Logger)

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
func createHevyWorkout(ctx context.Context, apiKey string, workout *hevy.PostWorkoutsRequestBody, fwCtx *framework.FrameworkContext) (string, error) {
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
	// The Hevy API returns: {"workout": [{"id": "...", ...}]}
	// It's a JSON object with a "workout" key containing an ARRAY of workout objects.
	var bodyBuf bytes.Buffer
	bodyBuf.ReadFrom(resp.Body)
	respBytes := bodyBuf.Bytes()

	fwCtx.Logger.Debug("Hevy POST response", "body", string(respBytes))

	var rawResp map[string]interface{}
	if err := json.Unmarshal(respBytes, &rawResp); err != nil {
		return "", fmt.Errorf("failed to decode Hevy response: %w", err)
	}

	// Extract workout ID from {"workout": [{"id": "..."}]}
	if workoutArr, ok := rawResp["workout"]; ok {
		if arr, ok := workoutArr.([]interface{}); ok && len(arr) > 0 {
			if first, ok := arr[0].(map[string]interface{}); ok {
				if id, ok := first["id"]; ok {
					if idStr, ok := id.(string); ok && idStr != "" {
						return idStr, nil
					}
				}
			}
		}
	}

	fwCtx.Logger.Warn("Could not extract workout ID from Hevy response", "body", string(respBytes))
	return "", nil
}
