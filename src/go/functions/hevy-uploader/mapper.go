package hevyuploader

import (
	"context"
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
	IsPrivate   bool           `json:"is_private,omitempty"`
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
func mapToHevyWorkout(ctx context.Context, event *pb.EnrichedActivityEvent, apiKey string, fwCtx *framework.FrameworkContext) (*HevyWorkoutRequest, error) {
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
			Exercises:   []HevyExercise{},
		},
	}

	if event.ActivityData == nil || len(event.ActivityData.Sessions) == 0 {
		fwCtx.Logger.Warn("No activity data available for mapping")
		// Create a placeholder exercise for cardio activities
		workout.Workout.Exercises = append(workout.Workout.Exercises,
			mapCardioActivityToExercise(event, totalDuration))
		return workout, nil
	}

	// Process each session
	for _, session := range event.ActivityData.Sessions {
		// Handle strength sets
		if len(session.StrengthSets) > 0 {
			exercises := mapStrengthSetsToExercises(session.StrengthSets, fwCtx)
			workout.Workout.Exercises = append(workout.Workout.Exercises, exercises...)
		}

		// Handle cardio activities (runs, rides, etc.) that don't have strength sets
		if len(session.StrengthSets) == 0 && (session.TotalDistance > 0 || session.TotalElapsedTime > 0) {
			cardioExercise := mapCardioSessionToExercise(event.ActivityType, session)
			workout.Workout.Exercises = append(workout.Workout.Exercises, cardioExercise)
		}
	}

	// If no exercises were created, create a generic workout exercise
	if len(workout.Workout.Exercises) == 0 {
		workout.Workout.Exercises = append(workout.Workout.Exercises,
			mapCardioActivityToExercise(event, totalDuration))
	}

	fwCtx.Logger.Debug("Mapped workout",
		"exerciseCount", len(workout.Workout.Exercises),
		"totalDuration", totalDuration)

	return workout, nil
}

// mapStrengthSetsToExercises groups strength sets by exercise name and converts them
func mapStrengthSetsToExercises(sets []*pb.StrengthSet, fwCtx *framework.FrameworkContext) []HevyExercise {
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
		exercise := HevyExercise{
			ExerciseTemplateID: getExerciseTemplateID(name), // TODO: Implement proper template lookup
			Sets:               exerciseMap[name],
		}
		exercises = append(exercises, exercise)
	}

	return exercises
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
func mapCardioSessionToExercise(activityType pb.ActivityType, session *pb.Session) HevyExercise {
	templateID := getCardioTemplateID(activityType)

	distance := int(session.TotalDistance)
	duration := int(session.TotalElapsedTime)

	return HevyExercise{
		ExerciseTemplateID: templateID,
		Sets: []HevySet{{
			Type:            "normal",
			DistanceMeters:  &distance,
			DurationSeconds: &duration,
		}},
	}
}

// mapCardioActivityToExercise creates a placeholder exercise for activities without session data
func mapCardioActivityToExercise(event *pb.EnrichedActivityEvent, durationSeconds float64) HevyExercise {
	templateID := getCardioTemplateID(event.ActivityType)

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
	}
}

// getExerciseTemplateID returns the Hevy template ID for an exercise name
// TODO: Implement proper fuzzy matching and template caching
func getExerciseTemplateID(exerciseName string) string {
	// For now, return a placeholder
	// In production, this should:
	// 1. Check a cache of user's exercise templates
	// 2. Fuzzy match against Hevy's standard library
	// 3. Create a custom template if no match found
	return "placeholder-needs-mapping"
}

// getCardioTemplateID returns the Hevy template ID for cardio activity types
func getCardioTemplateID(activityType pb.ActivityType) string {
	// Map FitGlue activity types to Hevy's cardio exercise templates
	// These IDs need to be verified against actual Hevy template library
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RUN:
		return "running" // Placeholder - need actual Hevy template ID
	case pb.ActivityType_ACTIVITY_TYPE_WALK:
		return "walking"
	case pb.ActivityType_ACTIVITY_TYPE_RIDE:
		return "cycling"
	case pb.ActivityType_ACTIVITY_TYPE_SWIM:
		return "swimming"
	case pb.ActivityType_ACTIVITY_TYPE_WORKOUT:
		return "other"
	case pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING:
		return "other"
	default:
		return "other"
	}
}
