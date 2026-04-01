package hevy

import (
	"testing"

	activitypb "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	"github.com/stretchr/testify/assert"
)

func TestMapToStandardizedActivity(t *testing.T) {
	rawJSON := []byte(`{
		"workout": {
			"id": "123",
			"title": "Push Day",
			"start_time": "2025-12-29T08:00:06.000Z",
			"end_time": "2025-12-29T09:00:06.000Z",
			"exercises": [
				{
					"title": "Bench Press",
					"sets": [
						{
							"reps": 10,
							"weight_kg": 60,
							"duration_seconds": 30
						}
					]
				}
			]
		}
	}`)

	act, err := mapToStandardizedActivity(rawJSON, "user_uuid", activitypb.ActivitySource_SOURCE_HEVY)
	assert.NoError(t, err)
	assert.NotNil(t, act)

	assert.Equal(t, "123", act.ExternalId)
	assert.Equal(t, "Push Day", act.Name)
	assert.Equal(t, activitypb.ActivitySource_SOURCE_HEVY, act.Source)
	assert.Equal(t, "user_uuid", act.UserId)
	assert.Equal(t, activitypb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING, act.Type)

	assert.Len(t, act.Sessions, 1)
	assert.Equal(t, float64(3600), act.Sessions[0].TotalElapsedTime)

	assert.Len(t, act.Sessions[0].StrengthSets, 1)
	assert.Equal(t, "Bench Press", act.Sessions[0].StrengthSets[0].ExerciseName)
	assert.Equal(t, int32(10), act.Sessions[0].StrengthSets[0].Reps)
	assert.Equal(t, float64(60), act.Sessions[0].StrengthSets[0].WeightKg)
}
