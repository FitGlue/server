package hevy

import (
	"testing"

	hevy "github.com/fitglue/server/src/go/pkg/api/hevy"
	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapSetType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"warmup", "warmup"},
		{"dropset", "dropset"},
		{"failure", "failure"},
		{"normal", "normal"},
		{"", "normal"},
		{"unknown", "normal"},
		{"WARMUP", "normal"}, // case sensitive
		{"drop_set", "normal"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := mapSetType(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetCardioExerciseName(t *testing.T) {
	tests := []struct {
		activityType pbactivity.ActivityType
		expected     string
	}{
		{pbactivity.ActivityType_ACTIVITY_TYPE_RUN, "Running (Outdoor)"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_WALK, "Walking"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_RIDE, "Cycling (Outdoor)"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_SWIM, "Swimming"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING, "Weight Training"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_UNSPECIFIED, "Workout"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_HIKE, "Workout"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_YOGA, "Workout"},
	}

	for _, tc := range tests {
		t.Run(tc.activityType.String(), func(t *testing.T) {
			result := getCardioExerciseName(tc.activityType)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAppendExercises(t *testing.T) {
	t.Run("NilExistingSlice", func(t *testing.T) {
		e1 := hevy.PostWorkoutsRequestExercise{}
		result := appendExercises(nil, e1)
		require.NotNil(t, result)
		assert.Len(t, *result, 1)
	})

	t.Run("EmptyExistingSlice", func(t *testing.T) {
		existing := []hevy.PostWorkoutsRequestExercise{}
		e1 := hevy.PostWorkoutsRequestExercise{}
		e2 := hevy.PostWorkoutsRequestExercise{}
		result := appendExercises(&existing, e1, e2)
		require.NotNil(t, result)
		assert.Len(t, *result, 2)
	})

	t.Run("ExistingWithItems", func(t *testing.T) {
		id1 := "existing"
		id2 := "appended"
		existing := []hevy.PostWorkoutsRequestExercise{{ExerciseTemplateId: &id1}}
		e := hevy.PostWorkoutsRequestExercise{ExerciseTemplateId: &id2}
		result := appendExercises(&existing, e)
		require.NotNil(t, result)
		assert.Len(t, *result, 2)
		assert.Equal(t, "existing", *(*result)[0].ExerciseTemplateId)
		assert.Equal(t, "appended", *(*result)[1].ExerciseTemplateId)
	})

	t.Run("AppendMultiple", func(t *testing.T) {
		existing := []hevy.PostWorkoutsRequestExercise{}
		items := []hevy.PostWorkoutsRequestExercise{{}, {}, {}}
		result := appendExercises(&existing, items...)
		assert.Len(t, *result, 3)
	})
}

func TestConvertStrengthSet(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		set := &pbactivity.StrengthSet{
			SetType:         "warmup",
			WeightKg:        80.5,
			Reps:            10,
			DistanceMeters:  100,
			DurationSeconds: 60,
		}
		result := convertStrengthSet(set)
		require.NotNil(t, result.Type)
		assert.Equal(t, hevy.PostWorkoutsRequestSetType("warmup"), *result.Type)
		require.NotNil(t, result.WeightKg)
		assert.InDelta(t, 80.5, float64(*result.WeightKg), 0.01)
		require.NotNil(t, result.Reps)
		assert.Equal(t, 10, *result.Reps)
		require.NotNil(t, result.DistanceMeters)
		assert.Equal(t, 100, *result.DistanceMeters)
		require.NotNil(t, result.DurationSeconds)
		assert.Equal(t, 60, *result.DurationSeconds)
	})

	t.Run("ZeroFields", func(t *testing.T) {
		set := &pbactivity.StrengthSet{
			SetType:         "normal",
			WeightKg:        0,
			Reps:            0,
			DistanceMeters:  0,
			DurationSeconds: 0,
		}
		result := convertStrengthSet(set)
		require.NotNil(t, result.Type)
		assert.Equal(t, hevy.PostWorkoutsRequestSetType("normal"), *result.Type)
		// Zero values should NOT be set (only set when > 0)
		assert.Nil(t, result.WeightKg)
		assert.Nil(t, result.Reps)
		assert.Nil(t, result.DistanceMeters)
		assert.Nil(t, result.DurationSeconds)
	})

	t.Run("DropsetType", func(t *testing.T) {
		set := &pbactivity.StrengthSet{SetType: "dropset"}
		result := convertStrengthSet(set)
		require.NotNil(t, result.Type)
		assert.Equal(t, hevy.PostWorkoutsRequestSetType("dropset"), *result.Type)
	})

	t.Run("UnknownSetType", func(t *testing.T) {
		set := &pbactivity.StrengthSet{SetType: "custom"}
		result := convertStrengthSet(set)
		require.NotNil(t, result.Type)
		assert.Equal(t, hevy.PostWorkoutsRequestSetType("normal"), *result.Type)
	})
}
