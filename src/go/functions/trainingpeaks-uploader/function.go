package trainingpeaksuploader

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
	functions.CloudEvent("UploadToTrainingPeaks", UploadToTrainingPeaks)
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

// UploadToTrainingPeaks is the Cloud Function entry point
func UploadToTrainingPeaks(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("trainingpeaks-uploader", svc, uploadHandler(nil))(ctx, e)
}

// uploadHandler contains the business logic
// httpClient can be injected for testing; if nil, creates OAuth client
func uploadHandler(httpClient *http.Client) framework.HandlerFunc {
	return func(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		var eventPayload pb.EnrichedActivityEvent

		unmarshaler := protojson.UnmarshalOptions{
			DiscardUnknown: true,
			AllowPartial:   true,
		}
		if err := unmarshaler.Unmarshal(e.Data(), &eventPayload); err != nil {
			return nil, fmt.Errorf("protojson.Unmarshal: %w", err)
		}

		fwCtx.Logger.Info("Starting TrainingPeaks upload",
			"activity_id", eventPayload.ActivityId,
			"pipeline_id", eventPayload.PipelineId,
			"user_id", eventPayload.UserId,
		)

		// Note: Loop prevention is handled at source-handler level (isBounceback check)
		// The source handler checks uploaded_activities before publishing to the enricher

		// 1. Get user's TrainingPeaks integration
		user, err := svc.DB.GetUser(ctx, eventPayload.UserId)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		if user.Integrations == nil || user.Integrations.Trainingpeaks == nil || !user.Integrations.Trainingpeaks.Enabled {
			fwCtx.Logger.Warn("User has no TrainingPeaks integration configured", "userId", eventPayload.UserId)
			return map[string]interface{}{
				"status": "FAILED",
				"reason": "no_trainingpeaks_integration",
			}, fmt.Errorf("user has no TrainingPeaks integration configured")
		}

		athleteID := user.Integrations.Trainingpeaks.AthleteId

		// Initialize OAuth HTTP Client if not provided (for testing)
		if httpClient == nil {
			tokenSource := oauth.NewFirestoreTokenSource(fwCtx.Service, eventPayload.UserId, "trainingpeaks")
			httpClient = oauth.NewClientWithUsageTracking(tokenSource, fwCtx.Service, eventPayload.UserId, "trainingpeaks")
		}

		// Check if this is an UPDATE operation (resume mode with existing activity)
		if useUpdate, ok := eventPayload.EnrichmentMetadata["use_update_method"]; ok && useUpdate == "true" {
			return handleTrainingpeaksUpdate(ctx, httpClient, &eventPayload, athleteID, fwCtx)
		}

		// --- CREATE MODE ---
		return handleTrainingPeaksCreate(ctx, httpClient, &eventPayload, athleteID, fwCtx)
	}
}

// handleTrainingPeaksCreate creates a new workout on TrainingPeaks (POST)
func handleTrainingPeaksCreate(ctx context.Context, httpClient *http.Client, eventPayload *pb.EnrichedActivityEvent, athleteID string, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Build TrainingPeaks workout payload
	workout := buildTrainingPeaksWorkout(eventPayload)

	// POST to TrainingPeaks API
	workoutID, err := createTrainingPeaksWorkout(ctx, httpClient, athleteID, workout, fwCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create TrainingPeaks workout: %w", err)
	}

	fwCtx.Logger.Info("Successfully created TrainingPeaks workout",
		"workoutId", workoutID,
		"activityId", eventPayload.ActivityId)

	// Record upload for loop prevention
	if eventPayload.ActivityData != nil && eventPayload.ActivityData.ExternalId != "" {
		uploadRecord := &pb.UploadedActivityRecord{
			Id:            loopprevention.BuildUploadedActivityID(eventPayload.Source, eventPayload.ActivityData.ExternalId),
			UserId:        eventPayload.UserId,
			Source:        eventPayload.Source,
			ExternalId:    eventPayload.ActivityData.ExternalId,
			StartTime:     eventPayload.StartTime,
			Destination:   pb.Destination_DESTINATION_TRAININGPEAKS,
			DestinationId: workoutID,
			UploadedAt:    timestamppb.Now(),
		}
		if err := svc.DB.SetUploadedActivity(ctx, eventPayload.UserId, uploadRecord); err != nil {
			fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
		} else {
			fwCtx.Logger.Debug("Recorded upload for loop prevention", "id", uploadRecord.Id)
		}
	}

	// Persist SynchronizedActivity
	existingActivity, _ := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
	if existingActivity != nil {
		if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
			"destinations": map[string]interface{}{
				"trainingpeaks": workoutID,
			},
			"synced_at": timestamppb.Now().AsTime(),
		}); err != nil {
			fwCtx.Logger.Error("Failed to update synchronized activity destinations", "error", err)
		} else {
			fwCtx.Logger.Info("Updated synchronized activity destinations (preserved execution ID)", "activity_id", eventPayload.ActivityId)
		}
	} else {
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
				"trainingpeaks": workoutID,
			},
		}

		if err := svc.DB.SetSynchronizedActivity(ctx, eventPayload.UserId, syncedActivity); err != nil {
			fwCtx.Logger.Error("Failed to persist synchronized activity", "error", err)
		}
	}

	// Increment sync count for billing
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
	}

	return map[string]interface{}{
		"status":             "SUCCESS",
		"trainingpeaks_id":   workoutID,
		"activity_id":        eventPayload.ActivityId,
		"pipeline_id":        eventPayload.PipelineId,
		"activity_name":      eventPayload.Name,
		"activity_type":      eventPayload.ActivityType.String(),
		"trainingpeaks_type": mapToTrainingPeaksType(eventPayload.ActivityType),
		"description":        eventPayload.Description,
	}, nil
}

// handleTrainingpeaksUpdate modifies an existing TrainingPeaks workout (PUT)
// Used in resume mode for delayed enrichment
func handleTrainingpeaksUpdate(ctx context.Context, httpClient *http.Client, eventPayload *pb.EnrichedActivityEvent, athleteID string, fwCtx *framework.FrameworkContext) (interface{}, error) {
	fwCtx.Logger.Info("Starting TrainingPeaks UPDATE",
		"activity_id", eventPayload.ActivityId,
		"user_id", eventPayload.UserId)

	// 1. Lookup SynchronizedActivity to get TrainingPeaks workout ID
	syncActivity, err := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
	if err != nil {
		return nil, fmt.Errorf("failed to get synchronized activity: %w", err)
	}
	if syncActivity == nil {
		return nil, fmt.Errorf("synchronized activity not found for activity_id: %s", eventPayload.ActivityId)
	}

	workoutIDStr, ok := syncActivity.Destinations["trainingpeaks"]
	if !ok || workoutIDStr == "" {
		return nil, fmt.Errorf("no TrainingPeaks destination found in synchronized activity")
	}

	fwCtx.Logger.Info("Found existing TrainingPeaks workout", "trainingpeaks_workout_id", workoutIDStr)

	// 2. Build update payload - for UPDATE we merge descriptions
	existingDescription := syncActivity.Description
	mergedDescription := existingDescription
	if eventPayload.Description != "" {
		if mergedDescription != "" {
			mergedDescription += "\n\n" + eventPayload.Description
		} else {
			mergedDescription = eventPayload.Description
		}
	}

	updatePayload := &TrainingPeaksWorkout{}
	hasChanges := false

	if eventPayload.Name != "" && eventPayload.Name != syncActivity.Title {
		updatePayload.Title = eventPayload.Name
		hasChanges = true
	}
	if mergedDescription != existingDescription {
		updatePayload.Description = mergedDescription
		hasChanges = true
	}

	if !hasChanges {
		fwCtx.Logger.Info("No changes to update, skipping PUT")
		return map[string]interface{}{
			"status":           "SUCCESS",
			"trainingpeaks_id": workoutIDStr,
			"update_skipped":   true,
			"reason":           "no_changes",
			"activity_name":    eventPayload.Name,
			"activity_type":    eventPayload.ActivityType.String(),
			"description":      eventPayload.Description,
		}, nil
	}

	// 3. PUT to TrainingPeaks API
	if err := updateTrainingPeaksWorkout(ctx, httpClient, athleteID, workoutIDStr, updatePayload, fwCtx); err != nil {
		return nil, fmt.Errorf("failed to update TrainingPeaks workout: %w", err)
	}

	fwCtx.Logger.Info("Successfully updated TrainingPeaks workout",
		"workoutId", workoutIDStr,
		"activityId", eventPayload.ActivityId)

	// 4. Update SynchronizedActivity with new description
	if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
		"description": mergedDescription,
	}); err != nil {
		fwCtx.Logger.Warn("Failed to update synchronized activity description", "error", err)
	}

	// 5. Increment sync count for billing (per successful destination sync)
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
	}

	return map[string]interface{}{
		"status":           "SUCCESS",
		"trainingpeaks_id": workoutIDStr,
		"mode":             "UPDATE",
		"activity_name":    eventPayload.Name,
		"activity_type":    eventPayload.ActivityType.String(),
		"description":      mergedDescription,
	}, nil
}

// TrainingPeaksWorkout represents the workout payload for TrainingPeaks API
type TrainingPeaksWorkout struct {
	Title            string  `json:"title,omitempty"`
	Description      string  `json:"description,omitempty"`
	WorkoutType      string  `json:"workoutType,omitempty"`
	StartDate        string  `json:"startDate,omitempty"`
	TotalTimePlanned float64 `json:"totalTimePlanned,omitempty"` // Duration in seconds
	DistancePlanned  float64 `json:"distancePlanned,omitempty"`  // Distance in meters
	HeartRateAvg     *int    `json:"heartRateAvg,omitempty"`
	HeartRateMax     *int    `json:"heartRateMax,omitempty"`
}

// buildTrainingPeaksWorkout creates the workout payload from activity data
func buildTrainingPeaksWorkout(event *pb.EnrichedActivityEvent) *TrainingPeaksWorkout {
	workout := &TrainingPeaksWorkout{
		Title:       event.Name,
		Description: event.Description,
		WorkoutType: mapToTrainingPeaksType(event.ActivityType),
	}

	// Add start time
	if event.StartTime != nil {
		workout.StartDate = event.StartTime.AsTime().Format(time.RFC3339)
	}

	// Extract duration and distance from activity data if available
	if event.ActivityData != nil && len(event.ActivityData.Sessions) > 0 {
		var totalDuration float64
		var totalDistance float64
		var hrSum, hrCount int
		var hrMax int

		for _, session := range event.ActivityData.Sessions {
			// Duration in seconds
			totalDuration += session.TotalElapsedTime

			// Distance in meters
			totalDistance += session.TotalDistance

			// Extract heart rate from laps/records
			for _, lap := range session.Laps {
				for _, record := range lap.Records {
					if record.HeartRate > 0 {
						hrSum += int(record.HeartRate)
						hrCount++
						if int(record.HeartRate) > hrMax {
							hrMax = int(record.HeartRate)
						}
					}
				}
			}
		}

		if totalDuration > 0 {
			workout.TotalTimePlanned = totalDuration
		}
		if totalDistance > 0 {
			workout.DistancePlanned = totalDistance
		}
		if hrCount > 0 {
			avgHR := hrSum / hrCount
			workout.HeartRateAvg = &avgHR
			workout.HeartRateMax = &hrMax
		}
	}

	return workout
}

// mapToTrainingPeaksType maps FitGlue activity types to TrainingPeaks workout types
func mapToTrainingPeaksType(activityType pb.ActivityType) string {
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RUN,
		pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN,
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN:
		return "Run"
	case pb.ActivityType_ACTIVITY_TYPE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_EBIKE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_EMOUNTAIN_BIKE_RIDE:
		return "Bike"
	case pb.ActivityType_ACTIVITY_TYPE_SWIM:
		return "Swim"
	case pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
		pb.ActivityType_ACTIVITY_TYPE_CROSSFIT,
		pb.ActivityType_ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING:
		return "Strength"
	default:
		return "Other"
	}
}

// createTrainingPeaksWorkout POSTs a new workout to TrainingPeaks
func createTrainingPeaksWorkout(ctx context.Context, httpClient *http.Client, athleteID string, workout *TrainingPeaksWorkout, fwCtx *framework.FrameworkContext) (string, error) {
	bodyJSON, err := json.Marshal(workout)
	if err != nil {
		return "", fmt.Errorf("failed to marshal workout: %w", err)
	}

	url := fmt.Sprintf("https://api.trainingpeaks.com/v1/athlete/%s/workouts", athleteID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	fwCtx.Logger.Debug("Sending POST to TrainingPeaks",
		"url", url,
		"bodyLength", len(bodyJSON),
		"workoutType", workout.WorkoutType)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("TrainingPeaks API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		err := httputil.WrapResponseError(resp, "TrainingPeaks API error")
		fwCtx.Logger.Error("TrainingPeaks API error", "status", resp.StatusCode, "error", err)
		return "", err
	}

	// Parse response to get workout ID
	var respBody struct {
		ID        interface{} `json:"Id"`
		WorkoutId interface{} `json:"workoutId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return "", fmt.Errorf("failed to decode TrainingPeaks response: %w", err)
	}

	// Handle both possible ID field names and types
	workoutID := ""
	if respBody.ID != nil {
		workoutID = fmt.Sprintf("%v", respBody.ID)
	} else if respBody.WorkoutId != nil {
		workoutID = fmt.Sprintf("%v", respBody.WorkoutId)
	}

	if workoutID == "" {
		return "", fmt.Errorf("no workout ID in TrainingPeaks response")
	}

	return workoutID, nil
}

// updateTrainingPeaksWorkout PUTs updates to an existing workout on TrainingPeaks
func updateTrainingPeaksWorkout(ctx context.Context, httpClient *http.Client, athleteID, workoutID string, workout *TrainingPeaksWorkout, fwCtx *framework.FrameworkContext) error {
	bodyJSON, err := json.Marshal(workout)
	if err != nil {
		return fmt.Errorf("failed to marshal workout update: %w", err)
	}

	url := fmt.Sprintf("https://api.trainingpeaks.com/v1/athlete/%s/workouts/%s", athleteID, workoutID)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	fwCtx.Logger.Info("Updating TrainingPeaks workout (PUT)",
		"url", url,
		"workoutId", workoutID,
		"bodyLength", len(bodyJSON))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("TrainingPeaks API PUT request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		err := httputil.WrapResponseError(resp, "TrainingPeaks PUT error")
		fwCtx.Logger.Error("TrainingPeaks PUT error", "status", resp.StatusCode, "error", err)
		return err
	}

	return nil
}
