package enricher

import (
	"testing"

	"github.com/fitglue/server/src/go/internal/pipeline/enricher/providers/user_input"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHumanizeFieldName tests the humanizeFieldName utility function.
func TestHumanizeFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"fit_file_base64", "Fit File Base64"},
		{"user_id", "User Id"},
		{"name", "Name"},
		{"", ""},
		{"single", "Single"},
		{"a_b_c", "A B C"},
		{"hello_world", "Hello World"},
		{"multi_word_field_name", "Multi Word Field Name"},
		{"already_capitalized", "Already Capitalized"},
		{"x", "X"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := humanizeFieldName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestBuildPendingInputStatusMessage tests the buildPendingInputStatusMessage function.
func TestBuildPendingInputStatusMessage(t *testing.T) {
	t.Run("UsesDisplaySummary", func(t *testing.T) {
		waitErr := &user_input.WaitForInputError{
			RequiredFields: []string{"fit_file"},
			Metadata: map[string]string{
				"display.summary": "Please upload your FIT file",
			},
		}
		msg := buildPendingInputStatusMessage(waitErr)
		assert.Equal(t, "Waiting for user input: Please upload your FIT file", msg)
	})

	t.Run("UsesDisplayFieldLabels", func(t *testing.T) {
		waitErr := &user_input.WaitForInputError{
			RequiredFields: []string{"fit_file", "race_name"},
			Metadata: map[string]string{
				"display.field_labels": `{"fit_file":"FIT File","race_name":"Race Name"}`,
			},
		}
		msg := buildPendingInputStatusMessage(waitErr)
		assert.Equal(t, "Waiting for user input: FIT File, Race Name", msg)
	})

	t.Run("FallsBackToHumanizeForMissingLabel", func(t *testing.T) {
		waitErr := &user_input.WaitForInputError{
			RequiredFields: []string{"fit_file", "unknown_field"},
			Metadata: map[string]string{
				"display.field_labels": `{"fit_file":"FIT File"}`,
			},
		}
		msg := buildPendingInputStatusMessage(waitErr)
		// "unknown_field" has no label → humanized to "Unknown Field"
		assert.Equal(t, "Waiting for user input: FIT File, Unknown Field", msg)
	})

	t.Run("FallsBackToRawHumanize", func(t *testing.T) {
		waitErr := &user_input.WaitForInputError{
			RequiredFields: []string{"fit_file_base64", "athlete_name"},
			Metadata:       map[string]string{},
		}
		msg := buildPendingInputStatusMessage(waitErr)
		assert.Equal(t, "Waiting for user input: Fit File Base64, Athlete Name", msg)
	})

	t.Run("EmptyFieldsNoMetadata", func(t *testing.T) {
		waitErr := &user_input.WaitForInputError{
			RequiredFields: []string{},
			Metadata:       map[string]string{},
		}
		msg := buildPendingInputStatusMessage(waitErr)
		assert.Equal(t, "Waiting for user input: ", msg)
	})

	t.Run("InvalidFieldLabelsJSON", func(t *testing.T) {
		waitErr := &user_input.WaitForInputError{
			RequiredFields: []string{"fit_file"},
			Metadata: map[string]string{
				"display.field_labels": `not_valid_json`,
			},
		}
		msg := buildPendingInputStatusMessage(waitErr)
		// Falls back to humanized raw field names
		assert.Equal(t, "Waiting for user input: Fit File", msg)
	})

	t.Run("SummaryTakesPriorityOverFieldLabels", func(t *testing.T) {
		waitErr := &user_input.WaitForInputError{
			RequiredFields: []string{"fit_file"},
			Metadata: map[string]string{
				"display.summary":      "This should win",
				"display.field_labels": `{"fit_file":"FIT File"}`,
			},
		}
		msg := buildPendingInputStatusMessage(waitErr)
		assert.Equal(t, "Waiting for user input: This should win", msg)
	})
}

// TestGroupDestinationsByExclusions tests the groupDestinationsByExclusions function.
func TestGroupDestinationsByExclusions(t *testing.T) {
	t.Run("EmptyDestinations", func(t *testing.T) {
		result := groupDestinationsByExclusions(nil, nil)
		assert.Empty(t, result)
	})

	t.Run("NoExclusions", func(t *testing.T) {
		dests := []pbplugin.DestinationType{
			pbplugin.DestinationType_DESTINATION_STRAVA,
			pbplugin.DestinationType_DESTINATION_HEVY,
		}
		result := groupDestinationsByExclusions(dests, nil)
		assert.Len(t, result, 1)
		defaultGroup, ok := result[""]
		assert.True(t, ok)
		assert.Len(t, defaultGroup, 2)
	})

	t.Run("SingleExclusionGroup", func(t *testing.T) {
		dests := []pbplugin.DestinationType{
			pbplugin.DestinationType_DESTINATION_STRAVA,
		}
		configs := map[string]*pbpipeline.DestinationConfig{
			"strava": {
				ExcludedEnrichers: []string{"hr_zones", "distance_milestones"},
			},
		}
		result := groupDestinationsByExclusions(dests, configs)
		assert.Len(t, result, 1)
		// Key should be sorted comma-joined exclusions
		expectedKey := "hr_zones,distance_milestones"
		// Sort the expected key
		import_key := "distance_milestones,hr_zones" // sorted alphabetically
		assert.Contains(t, result, import_key)
		_ = expectedKey
	})

	t.Run("MixedExclusionGroups", func(t *testing.T) {
		dests := []pbplugin.DestinationType{
			pbplugin.DestinationType_DESTINATION_STRAVA,
			pbplugin.DestinationType_DESTINATION_HEVY,
		}
		configs := map[string]*pbpipeline.DestinationConfig{
			"strava": {ExcludedEnrichers: []string{"hr_zones"}},
		}
		result := groupDestinationsByExclusions(dests, configs)
		// strava → "hr_zones" group, hevy → "" (no exclusions) group
		assert.Len(t, result, 2)
		assert.Contains(t, result, "")
		assert.Contains(t, result, "hr_zones")
	})

	t.Run("EmptyExclusionsConfigPresent", func(t *testing.T) {
		dests := []pbplugin.DestinationType{
			pbplugin.DestinationType_DESTINATION_STRAVA,
		}
		configs := map[string]*pbpipeline.DestinationConfig{
			"strava": {ExcludedEnrichers: []string{}}, // empty exclusions
		}
		result := groupDestinationsByExclusions(dests, configs)
		// Empty exclusions → default group
		assert.Len(t, result, 1)
		assert.Contains(t, result, "")
	})
}

// TestBoostersToFirestoreMaps tests the boostersToFirestoreMaps function.
func TestBoostersToFirestoreMaps(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		result := boostersToFirestoreMaps(nil)
		assert.Empty(t, result)
	})

	t.Run("SingleExecution", func(t *testing.T) {
		execs := []ProviderExecution{
			{
				ProviderName: "hr_zones",
				Status:       "SUCCESS",
				DurationMs:   42,
				Metadata:     map[string]string{"key": "val"},
			},
		}
		result := boostersToFirestoreMaps(execs)
		require.Len(t, result, 1)
		assert.Equal(t, "hr_zones", result[0]["provider_name"])
		assert.Equal(t, "SUCCESS", result[0]["status"])
		assert.Equal(t, int64(42), result[0]["duration_ms"])
		assert.Equal(t, map[string]string{"key": "val"}, result[0]["metadata"])
		_, hasError := result[0]["error"]
		assert.False(t, hasError, "error key should not be present when empty")
	})

	t.Run("WithError", func(t *testing.T) {
		execs := []ProviderExecution{
			{
				ProviderName: "branding",
				Status:       "FAILED",
				DurationMs:   100,
				Error:        "connection timeout",
			},
		}
		result := boostersToFirestoreMaps(execs)
		require.Len(t, result, 1)
		assert.Equal(t, "branding", result[0]["provider_name"])
		assert.Equal(t, "FAILED", result[0]["status"])
		assert.Equal(t, "connection timeout", result[0]["error"])
	})

	t.Run("MultipleExecutions", func(t *testing.T) {
		execs := []ProviderExecution{
			{ProviderName: "hr_zones", Status: "SUCCESS"},
			{ProviderName: "distance_milestones", Status: "SKIPPED"},
			{ProviderName: "branding", Status: "SUCCESS"},
		}
		result := boostersToFirestoreMaps(execs)
		assert.Len(t, result, 3)
		assert.Equal(t, "hr_zones", result[0]["provider_name"])
		assert.Equal(t, "distance_milestones", result[1]["provider_name"])
		assert.Equal(t, "branding", result[2]["provider_name"])
	})
}

// TestCloneEnrichedEvent tests that cloneEnrichedEvent produces independent copies.
func TestCloneEnrichedEvent(t *testing.T) {
	t.Run("CloneIsIndependent", func(t *testing.T) {
		pipelineExecId := "test-exec-id"
		src := &pbevents.EnrichedActivityEvent{
			UserId:              "user-123",
			ActivityId:          "activity-456",
			PipelineExecutionId: &pipelineExecId,
			AppliedEnrichments:  []string{"hr_zones", "branding"},
			EnrichmentMetadata:  map[string]string{"key": "value"},
		}

		cloned := cloneEnrichedEvent(src)

		// Check values match
		assert.Equal(t, src.UserId, cloned.UserId)
		assert.Equal(t, src.ActivityId, cloned.ActivityId)
		assert.Equal(t, src.AppliedEnrichments, cloned.AppliedEnrichments)

		// Mutate the clone and verify the original is unchanged
		cloned.UserId = "mutated"
		cloned.AppliedEnrichments = append(cloned.AppliedEnrichments, "extra")
		cloned.EnrichmentMetadata["new_key"] = "new_value"

		assert.Equal(t, "user-123", src.UserId)
		assert.Len(t, src.AppliedEnrichments, 2)
		assert.NotContains(t, src.EnrichmentMetadata, "new_key")
	})
}
