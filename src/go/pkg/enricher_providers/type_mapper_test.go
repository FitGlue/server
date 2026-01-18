package enricher_providers_test

import (
	"context"
	"testing"

	"github.com/fitglue/server/src/go/pkg/domain/activity"
	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestTypeMapperProvider_Enrich(t *testing.T) {
	provider := enricher_providers.NewTypeMapperProvider()
	ctx := context.Background()

	// The type mapper works by matching title substrings to target activity types.
	// Config key is "type_rules" with format: {"title substring": "TargetActivityType"}
	tests := []struct {
		name           string
		activityName   string
		activityType   pb.ActivityType
		typeRules      string // JSON object: {"title substring": "TargetActivityType"}
		expectedType   pb.ActivityType
		expectMetadata bool
	}{
		{
			name:           "Maps activity with 'morning' in title to Yoga",
			activityName:   "Morning Stretch Session",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			typeRules:      `{"morning": "Yoga"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_YOGA,
			expectMetadata: true,
		},
		{
			name:           "Maps activity with 'treadmill' in title to VirtualRun",
			activityName:   "Treadmill Run",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_RUN,
			typeRules:      `{"treadmill": "VirtualRun"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN,
			expectMetadata: true,
		},
		{
			name:           "Case-insensitive matching",
			activityName:   "ZWIFT Ride",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_RIDE,
			typeRules:      `{"zwift": "VirtualRide"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE,
			expectMetadata: true,
		},
		{
			name:           "No matching substring keeps original",
			activityName:   "Weight Training Session",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			typeRules:      `{"treadmill": "VirtualRun"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			expectMetadata: false,
		},
		{
			name:           "Empty rules does nothing",
			activityName:   "Morning Run",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_RUN,
			typeRules:      "",
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_RUN,
			expectMetadata: false,
		},
		{
			name:           "Invalid JSON does nothing",
			activityName:   "Morning Run",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_RUN,
			typeRules:      `{invalid}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_RUN,
			expectMetadata: false,
		},
		{
			name:           "Multiple rules - only one matches",
			activityName:   "Outdoor Treadmill Session",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_RUN,
			typeRules:      `{"zwift": "VirtualRide", "treadmill": "VirtualRun"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN,
			expectMetadata: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			act := &pb.StandardizedActivity{
				Name: tt.activityName,
				Type: tt.activityType,
			}
			config := map[string]string{}
			if tt.typeRules != "" {
				config["type_rules"] = tt.typeRules
			}

			res, err := provider.Enrich(ctx, act, nil, config, false)
			if err != nil {
				t.Fatalf("Enrich failed: %v", err)
			}

			if act.Type != tt.expectedType {
				t.Errorf("expected type %v, got %v", tt.expectedType, act.Type)
			}

			if tt.expectMetadata {
				expectedStravaName := activity.GetStravaActivityType(act.Type)
				if res.Metadata["new_type"] != expectedStravaName {
					t.Errorf("Metadata new_type expected %s, got %s", expectedStravaName, res.Metadata["new_type"])
				}
				if res.Metadata["matched_pattern"] == "" {
					t.Error("Expected matched_pattern in metadata")
				}
			}
		})
	}
}
