package hevyuploader

import (
	"context"
	"fmt"
	"time"

	"github.com/fitglue/server/src/go/pkg/framework"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// HevyWorkoutRequest represents the POST body for creating a workout
type HevyWorkoutRequest struct {
	Workout HevyWorkoutData `json:"workout"`
}

// HevyWorkoutData is the inner workout object
type HevyWorkoutData struct {
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	StartTime   string         `json:"start_time"`
	EndTime     string         `json:"end_time"`
	IsPrivate   bool           `json:"is_private"`
	Exercises   []HevyExercise `json:"exercises"`
}

// HevyExercise represents an exercise in a Hevy workout
type HevyExercise struct {
	ExerciseTemplateID string    `json:"exercise_template_id"`
	SupersetID         *int      `json:"superset_id,omitempty"`
	Notes              string    `json:"notes,omitempty"`
	Sets               []HevySet `json:"sets"`
}

// HevySet represents a set in a Hevy exercise
type HevySet struct {
	Type            string   `json:"type"` // normal, warmup, dropset, failure
	WeightKg        *float64 `json:"weight_kg,omitempty"`
	Reps            *int     `json:"reps,omitempty"`
	DistanceMeters  *int     `json:"distance_meters,omitempty"`
	DurationSeconds *int     `json:"duration_seconds,omitempty"`
	RPE             *float64 `json:"rpe,omitempty"`
	CustomMetric    *float64 `json:"custom_metric,omitempty"`
}

// mapToHevyWorkout converts an EnrichedActivityEvent to Hevy's workout format
func mapToHevyWorkout(ctx context.Context, event *pb.EnrichedActivityEvent, resolver *TemplateResolver, fwCtx *framework.FrameworkContext) (*HevyWorkoutRequest, error) {
	startTime := time.Now()
	if event.StartTime != nil {
		startTime = event.StartTime.AsTime()
	}

	// Calculate end time based on activity duration
	endTime := startTime
	var totalDuration float64 = 0
	if event.ActivityData != nil {
		for _, session := range event.ActivityData.Sessions {
			totalDuration += session.TotalElapsedTime
		}
	}
	if totalDuration > 0 {
		endTime = startTime.Add(time.Duration(totalDuration) * time.Second)
	} else {
		// Default to 30 minutes if no duration
		endTime = startTime.Add(30 * time.Minute)
	}

	workout := &HevyWorkoutRequest{
		Workout: HevyWorkoutData{
			Title:       event.Name,
			Description: event.Description,
			StartTime:   startTime.Format(time.RFC3339),
			EndTime:     endTime.Format(time.RFC3339),
			IsPrivate:   false, // Activities synced via FitGlue are public by default
			Exercises:   []HevyExercise{},
		},
	}

	if event.ActivityData == nil || len(event.ActivityData.Sessions) == 0 {
		fwCtx.Logger.Warn("No activity data available for mapping")
		// Create a placeholder exercise for cardio activities
		exercise, err := mapCardioActivityToExercise(ctx, event, totalDuration, resolver)
		if err != nil {
			return nil, fmt.Errorf("failed to map cardio activity: %w", err)
		}
		workout.Workout.Exercises = append(workout.Workout.Exercises, exercise)
		return workout, nil
	}

	// Process each session
	for _, session := range event.ActivityData.Sessions {
		// Handle strength sets
		if len(session.StrengthSets) > 0 {
			exercises, err := mapStrengthSetsToExercises(ctx, session.StrengthSets, resolver, fwCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to map strength sets: %w", err)
			}
			workout.Workout.Exercises = append(workout.Workout.Exercises, exercises...)
		}

		// Check if laps have individual exercise names (hybrid race tagging)
		hasLapExerciseNames := false
		for _, lap := range session.Laps {
			if lap.ExerciseName != "" {
				hasLapExerciseNames = true
				break
			}
		}

		if hasLapExerciseNames {
			// Map each lap as a separate exercise (Hyrox, ATHX, etc.)
			for _, lap := range session.Laps {
				lapExercise, err := mapLapToExercise(ctx, lap, event.ActivityType, resolver)
				if err != nil {
					fwCtx.Logger.Warn("Failed to map lap to exercise",
						"error", err,
						"exercise_name", lap.ExerciseName)
					continue
				}
				workout.Workout.Exercises = append(workout.Workout.Exercises, lapExercise)
			}
		} else if len(session.StrengthSets) == 0 && (session.TotalDistance > 0 || session.TotalElapsedTime > 0) {
			// Handle cardio activities (runs, rides, etc.) as a single exercise
			cardioExercise, err := mapCardioSessionToExercise(ctx, event.ActivityType, session, resolver)
			if err != nil {
				return nil, fmt.Errorf("failed to map cardio session: %w", err)
			}
			workout.Workout.Exercises = append(workout.Workout.Exercises, cardioExercise)
		}
	}

	// If no exercises were created, create a generic workout exercise
	if len(workout.Workout.Exercises) == 0 {
		exercise, err := mapCardioActivityToExercise(ctx, event, totalDuration, resolver)
		if err != nil {
			return nil, fmt.Errorf("failed to map fallback activity: %w", err)
		}
		workout.Workout.Exercises = append(workout.Workout.Exercises, exercise)
	}

	fwCtx.Logger.Debug("Mapped workout",
		"exerciseCount", len(workout.Workout.Exercises),
		"totalDuration", totalDuration)

	return workout, nil
}

// mapStrengthSetsToExercises groups strength sets by exercise name and converts them
func mapStrengthSetsToExercises(ctx context.Context, sets []*pb.StrengthSet, resolver *TemplateResolver, fwCtx *framework.FrameworkContext) ([]HevyExercise, error) {
	// Group sets by exercise name
	exerciseMap := make(map[string][]HevySet)
	exerciseOrder := []string{}

	for _, set := range sets {
		name := set.ExerciseName
		if name == "" {
			name = "Unknown Exercise"
		}

		// Check if we've seen this exercise before
		if _, exists := exerciseMap[name]; !exists {
			exerciseOrder = append(exerciseOrder, name)
			exerciseMap[name] = []HevySet{}
		}

		// Convert set
		hevySet := convertStrengthSet(set)
		exerciseMap[name] = append(exerciseMap[name], hevySet)
	}

	// Build exercises in order
	exercises := []HevyExercise{}
	for _, name := range exerciseOrder {
		// Resolve template ID via API
		templateID, err := resolver.ResolveTemplateID(ctx, name)
		if err != nil {
			fwCtx.Logger.Error("Failed to resolve template ID",
				"exerciseName", name,
				"error", err)
			return nil, fmt.Errorf("failed to resolve template for %q: %w", name, err)
		}

		exercise := HevyExercise{
			ExerciseTemplateID: templateID,
			Sets:               exerciseMap[name],
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}

// convertStrengthSet converts a pb.StrengthSet to HevySet
func convertStrengthSet(set *pb.StrengthSet) HevySet {
	hevySet := HevySet{
		Type: mapSetType(set.SetType),
	}

	// Weight
	if set.WeightKg > 0 {
		weight := float64(set.WeightKg)
		hevySet.WeightKg = &weight
	}

	// Reps
	if set.Reps > 0 {
		reps := int(set.Reps)
		hevySet.Reps = &reps
	}

	// Distance
	if set.DistanceMeters > 0 {
		distance := int(set.DistanceMeters)
		hevySet.DistanceMeters = &distance
	}

	// Duration
	if set.DurationSeconds > 0 {
		duration := int(set.DurationSeconds)
		hevySet.DurationSeconds = &duration
	}

	return hevySet
}

// mapSetType converts FitGlue set type to Hevy set type
func mapSetType(setType string) string {
	switch setType {
	case "warmup":
		return "warmup"
	case "dropset":
		return "dropset"
	case "failure":
		return "failure"
	default:
		return "normal"
	}
}

// mapCardioSessionToExercise maps a cardio session to a Hevy exercise
func mapCardioSessionToExercise(ctx context.Context, activityType pb.ActivityType, session *pb.Session, resolver *TemplateResolver) (HevyExercise, error) {
	// Get the cardio exercise name for this activity type
	exerciseName := getCardioExerciseName(activityType)

	// Resolve to a real template ID
	templateID, err := resolver.ResolveTemplateID(ctx, exerciseName)
	if err != nil {
		return HevyExercise{}, fmt.Errorf("failed to resolve cardio template: %w", err)
	}

	distance := int(session.TotalDistance)
	duration := int(session.TotalElapsedTime)

	return HevyExercise{
		ExerciseTemplateID: templateID,
		Sets: []HevySet{{
			Type:            "normal",
			DistanceMeters:  &distance,
			DurationSeconds: &duration,
		}},
	}, nil
}

// mapLapToExercise maps a single lap to a Hevy exercise (for hybrid race tagging)
func mapLapToExercise(ctx context.Context, lap *pb.Lap, fallbackActivityType pb.ActivityType, resolver *TemplateResolver) (HevyExercise, error) {
	// Use lap's exercise name if set, otherwise fall back to activity type
	exerciseName := lap.ExerciseName
	if exerciseName == "" {
		exerciseName = getCardioExerciseName(fallbackActivityType)
	}

	// Resolve to a real template ID via Hevy's API
	templateID, err := resolver.ResolveTemplateID(ctx, exerciseName)
	if err != nil {
		return HevyExercise{}, fmt.Errorf("failed to resolve lap template for %q: %w", exerciseName, err)
	}

	distance := int(lap.TotalDistance)
	duration := int(lap.TotalElapsedTime)

	return HevyExercise{
		ExerciseTemplateID: templateID,
		Sets: []HevySet{{
			Type:            "normal",
			DistanceMeters:  &distance,
			DurationSeconds: &duration,
		}},
	}, nil
}

// mapCardioActivityToExercise creates a placeholder exercise for activities without session data
func mapCardioActivityToExercise(ctx context.Context, event *pb.EnrichedActivityEvent, durationSeconds float64, resolver *TemplateResolver) (HevyExercise, error) {
	exerciseName := getCardioExerciseName(event.ActivityType)

	// Resolve to a real template ID
	templateID, err := resolver.ResolveTemplateID(ctx, exerciseName)
	if err != nil {
		return HevyExercise{}, fmt.Errorf("failed to resolve cardio template: %w", err)
	}

	duration := int(durationSeconds)
	if duration == 0 {
		duration = 1800 // Default 30 min
	}

	return HevyExercise{
		ExerciseTemplateID: templateID,
		Notes:              event.Description,
		Sets: []HevySet{{
			Type:            "normal",
			DurationSeconds: &duration,
		}},
	}, nil
}

// getCardioExerciseName returns the exercise name to search for the given activity type
func getCardioExerciseName(activityType pb.ActivityType) string {
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RUN:
		return "Running (Outdoor)"
	case pb.ActivityType_ACTIVITY_TYPE_WALK:
		return "Walking"
	case pb.ActivityType_ACTIVITY_TYPE_RIDE:
		return "Cycling (Outdoor)"
	case pb.ActivityType_ACTIVITY_TYPE_SWIM:
		return "Swimming"
	case pb.ActivityType_ACTIVITY_TYPE_WORKOUT:
		return "Workout"
	case pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING:
		return "Weight Training"
	default:
		return "Workout"
	}
}
