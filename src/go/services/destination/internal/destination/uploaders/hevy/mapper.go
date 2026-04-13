package hevy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	hevy "github.com/fitglue/server/src/go/pkg/api/hevy"
	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
)

// mapToHevyWorkout converts an ActivityPayload to Hevy's workout format
func mapToHevyWorkout(ctx context.Context, payload *pbevents.ActivityPayload, resolver *TemplateResolver, logger *slog.Logger, isPrivate bool) (*hevy.PostWorkoutsRequestBody, error) {
	startTime := time.Now()
	if payload.Timestamp != nil {
		startTime = payload.Timestamp.AsTime()
	}

	endTime := startTime
	var totalDuration float64 = 0
	if payload.StandardizedActivity != nil {
		for _, session := range payload.StandardizedActivity.Sessions {
			totalDuration += session.TotalElapsedTime
		}
	}
	if totalDuration > 0 {
		endTime = startTime.Add(time.Duration(totalDuration) * time.Second)
	} else {
		endTime = startTime.Add(30 * time.Minute)
	}

	startTimeStr := startTime.Format(time.RFC3339)
	endTimeStr := endTime.Format(time.RFC3339)
	// isPrivate is passed in from the pipeline config (hevy_is_private metadata key)
	exercises := []hevy.PostWorkoutsRequestExercise{}

	activityName := payload.Metadata["activity_name"]
	description := payload.Metadata["description"]

	activityTypeVal, ok := pbactivity.ActivityType_value[payload.Metadata["activity_type"]]
	activityType := pbactivity.ActivityType_ACTIVITY_TYPE_UNSPECIFIED
	if ok {
		activityType = pbactivity.ActivityType(activityTypeVal)
	}

	workout := &hevy.PostWorkoutsRequestBody{
		Workout: &struct {
			Description *string                             `json:"description"`
			EndTime     *string                             `json:"end_time,omitempty"`
			Exercises   *[]hevy.PostWorkoutsRequestExercise `json:"exercises,omitempty"`
			IsPrivate   *bool                               `json:"is_private,omitempty"`
			StartTime   *string                             `json:"start_time,omitempty"`
			Title       *string                             `json:"title,omitempty"`
		}{
			Title:       &activityName,
			Description: &description,
			StartTime:   &startTimeStr,
			EndTime:     &endTimeStr,
			IsPrivate:   &isPrivate,
			Exercises:   &exercises,
		},
	}

	if payload.StandardizedActivity == nil || len(payload.StandardizedActivity.Sessions) == 0 {
		logger.Warn("No activity data available for mapping")
		exercise, err := mapCardioActivityToExercise(ctx, activityName, description, activityType, totalDuration, resolver)
		if err != nil {
			return nil, fmt.Errorf("failed to map cardio activity: %w", err)
		}
		workout.Workout.Exercises = &[]hevy.PostWorkoutsRequestExercise{exercise}
		return workout, nil
	}

	for _, session := range payload.StandardizedActivity.Sessions {
		if len(session.StrengthSets) > 0 {
			exercises, err := mapStrengthSetsToExercises(ctx, session.StrengthSets, resolver, logger)
			if err != nil {
				return nil, fmt.Errorf("failed to map strength sets: %w", err)
			}
			workout.Workout.Exercises = appendExercises(workout.Workout.Exercises, exercises...)
		}

		hasLapExerciseNames := false
		for _, lap := range session.Laps {
			if lap.ExerciseName != "" {
				hasLapExerciseNames = true
				break
			}
		}

		if hasLapExerciseNames {
			for _, lap := range session.Laps {
				if lap.IsTelemetryContainerOnly {
					logger.Debug("Skipping telemetry-only lap for Hevy upload", "exercise_name", lap.ExerciseName)
					continue
				}

				lapExercise, err := mapLapToExercise(ctx, lap, activityType, resolver)
				if err != nil {
					logger.Warn("Failed to map lap to exercise", "error", err, "exercise_name", lap.ExerciseName)
					continue
				}
				workout.Workout.Exercises = appendExercises(workout.Workout.Exercises, lapExercise)
			}
		} else if len(session.StrengthSets) == 0 && (session.TotalDistance > 0 || session.TotalElapsedTime > 0) {
			cardioExercise, err := mapCardioSessionToExercise(ctx, activityType, session, resolver)
			if err != nil {
				return nil, fmt.Errorf("failed to map cardio session: %w", err)
			}
			workout.Workout.Exercises = appendExercises(workout.Workout.Exercises, cardioExercise)
		}
	}

	if workout.Workout.Exercises == nil || len(*workout.Workout.Exercises) == 0 {
		exercise, err := mapCardioActivityToExercise(ctx, activityName, description, activityType, totalDuration, resolver)
		if err != nil {
			return nil, fmt.Errorf("failed to map fallback activity: %w", err)
		}
		workout.Workout.Exercises = &[]hevy.PostWorkoutsRequestExercise{exercise}
	}

	return workout, nil
}

func mapStrengthSetsToExercises(ctx context.Context, sets []*pbactivity.StrengthSet, resolver *TemplateResolver, logger *slog.Logger) ([]hevy.PostWorkoutsRequestExercise, error) {
	exerciseMap := make(map[string][]hevy.PostWorkoutsRequestSet)
	exerciseOrder := []string{}

	for _, set := range sets {
		name := set.ExerciseName
		if name == "" {
			name = "Unknown Exercise"
		}

		if _, exists := exerciseMap[name]; !exists {
			exerciseOrder = append(exerciseOrder, name)
			exerciseMap[name] = []hevy.PostWorkoutsRequestSet{}
		}

		// Pass the exercise name so the converter can restrict fields to those
		// accepted by Hevy for this exercise type (distance_duration, weight_duration, etc.)
		hevySet := convertStrengthSet(set, name)
		exerciseMap[name] = append(exerciseMap[name], hevySet)
	}

	exercises := []hevy.PostWorkoutsRequestExercise{}
	for _, name := range exerciseOrder {
		templateID, err := resolver.ResolveTemplateID(ctx, name)
		if err != nil {
			logger.Error("Failed to resolve template ID", "exerciseName", name, "error", err)
			return nil, fmt.Errorf("failed to resolve template for %q: %w", name, err)
		}

		setsCopy := exerciseMap[name]
		exercise := hevy.PostWorkoutsRequestExercise{
			ExerciseTemplateId: &templateID,
			Sets:               &setsCopy,
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}

// convertStrengthSet converts a StrengthSet to a Hevy set, restricting the populated
// fields to those accepted by the exercise's Hevy template type:
//
//   - distance_duration  → distance + duration only  (Sled Push/Pull, SkiErg, Rowing, Carries, Burpee, Sandbag)
//   - weight_duration    → weight + duration only     (Wall Balls — Hevy has no weight+reps+duration type)
//   - weight_reps        → weight + reps only         (generic strength)
//
// Sending fields outside the template type causes Hevy to reject the set with a 400 error.
func convertStrengthSet(set *pbactivity.StrengthSet, exerciseName string) hevy.PostWorkoutsRequestSet {
	setType := hevy.PostWorkoutsRequestSetType(mapSetType(set.SetType))
	hevySet := hevy.PostWorkoutsRequestSet{
		Type: &setType,
	}

	config := getExerciseTypeConfig(exerciseName)

	switch config.ExerciseType {
	case "distance_duration":
		// Only distance + duration are accepted.
		if set.DistanceMeters > 0 {
			distance := int(set.DistanceMeters)
			hevySet.DistanceMeters = &distance
		}
		if set.DurationSeconds > 0 {
			duration := int(set.DurationSeconds)
			hevySet.DurationSeconds = &duration
		}

	case "weight_duration":
		// Only weight + duration are accepted. Reps are noted in the StrengthSet
		// but Hevy has no weight+reps+duration template type, so reps are omitted here.
		if set.WeightKg > 0 {
			weight := float32(set.WeightKg)
			hevySet.WeightKg = &weight
		}
		if set.DurationSeconds > 0 {
			duration := int(set.DurationSeconds)
			hevySet.DurationSeconds = &duration
		}

	default: // "weight_reps" and any unknown types
		// Standard strength: weight + reps.
		if set.WeightKg > 0 {
			weight := float32(set.WeightKg)
			hevySet.WeightKg = &weight
		}
		if set.Reps > 0 {
			reps := int(set.Reps)
			hevySet.Reps = &reps
		}
		// Carry duration through for timed strength sets when present.
		if set.DurationSeconds > 0 {
			duration := int(set.DurationSeconds)
			hevySet.DurationSeconds = &duration
		}
	}

	return hevySet
}

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

func mapCardioSessionToExercise(ctx context.Context, activityType pbactivity.ActivityType, session *pbactivity.Session, resolver *TemplateResolver) (hevy.PostWorkoutsRequestExercise, error) {
	exerciseName := getCardioExerciseName(activityType)
	templateID, err := resolver.ResolveTemplateID(ctx, exerciseName)
	if err != nil {
		return hevy.PostWorkoutsRequestExercise{}, fmt.Errorf("failed to resolve cardio template: %w", err)
	}

	distance := int(session.TotalDistance)
	duration := int(session.TotalElapsedTime)
	setType := hevy.PostWorkoutsRequestSetType("normal")

	sets := []hevy.PostWorkoutsRequestSet{{
		Type:            &setType,
		DistanceMeters:  &distance,
		DurationSeconds: &duration,
	}}

	return hevy.PostWorkoutsRequestExercise{
		ExerciseTemplateId: &templateID,
		Sets:               &sets,
	}, nil
}

func mapLapToExercise(ctx context.Context, lap *pbactivity.Lap, fallbackActivityType pbactivity.ActivityType, resolver *TemplateResolver) (hevy.PostWorkoutsRequestExercise, error) {
	exerciseName := lap.ExerciseName
	if exerciseName == "" {
		exerciseName = getCardioExerciseName(fallbackActivityType)
	}

	templateID, err := resolver.ResolveTemplateID(ctx, exerciseName)
	if err != nil {
		return hevy.PostWorkoutsRequestExercise{}, fmt.Errorf("failed to resolve lap template for %q: %w", exerciseName, err)
	}

	distance := int(lap.TotalDistance)
	duration := int(lap.TotalElapsedTime)
	setType := hevy.PostWorkoutsRequestSetType("normal")

	sets := []hevy.PostWorkoutsRequestSet{{
		Type:            &setType,
		DistanceMeters:  &distance,
		DurationSeconds: &duration,
	}}

	return hevy.PostWorkoutsRequestExercise{
		ExerciseTemplateId: &templateID,
		Sets:               &sets,
	}, nil
}

func mapCardioActivityToExercise(ctx context.Context, activityName, description string, activityType pbactivity.ActivityType, durationSeconds float64, resolver *TemplateResolver) (hevy.PostWorkoutsRequestExercise, error) {
	exerciseName := getCardioExerciseName(activityType)
	templateID, err := resolver.ResolveTemplateID(ctx, exerciseName)
	if err != nil {
		return hevy.PostWorkoutsRequestExercise{}, fmt.Errorf("failed to resolve cardio template: %w", err)
	}

	duration := int(durationSeconds)
	if duration == 0 {
		duration = 1800
	}

	setType := hevy.PostWorkoutsRequestSetType("normal")
	sets := []hevy.PostWorkoutsRequestSet{{
		Type:            &setType,
		DurationSeconds: &duration,
	}}

	exercise := hevy.PostWorkoutsRequestExercise{
		ExerciseTemplateId: &templateID,
		Sets:               &sets,
	}
	if description != "" {
		exercise.Notes = &description
	}

	return exercise, nil
}

func getCardioExerciseName(activityType pbactivity.ActivityType) string {
	switch activityType {
	case pbactivity.ActivityType_ACTIVITY_TYPE_RUN:
		return "Running (Outdoor)"
	case pbactivity.ActivityType_ACTIVITY_TYPE_WALK:
		return "Walking"
	case pbactivity.ActivityType_ACTIVITY_TYPE_RIDE:
		return "Cycling (Outdoor)"
	case pbactivity.ActivityType_ACTIVITY_TYPE_SWIM:
		return "Swimming"
	case pbactivity.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING:
		return "Weightlifting"
	default:
		return "Other Cardio"
	}
}

func appendExercises(existing *[]hevy.PostWorkoutsRequestExercise, items ...hevy.PostWorkoutsRequestExercise) *[]hevy.PostWorkoutsRequestExercise {
	if existing == nil {
		return &items
	}
	result := append(*existing, items...)
	return &result
}
