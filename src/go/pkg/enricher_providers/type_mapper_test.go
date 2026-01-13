package enricher_providers_test

import (
	"context"
	"testing"

	"github.com/ripixel/fitglue-server/src/go/pkg/domain/activity"
	"github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestTypeMapperProvider_Enrich(t *testing.T) {
	provider := enricher_providers.NewTypeMapperProvider()
	ctx := context.Background()

	tests := []struct {
		name           string
		activityType   pb.ActivityType
		typeMappings   string // JSON object: {"OriginalType": "DesiredType"}
		expectedType   pb.ActivityType
		expectMetadata bool
	}{
		{
			name:           "Maps WeightTraining to Yoga",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			typeMappings:   `{"WeightTraining": "Yoga"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_YOGA,
			expectMetadata: true,
		},
		{
			name:           "Maps Run to VirtualRun",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_RUN,
			typeMappings:   `{"Run": "VirtualRun"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN,
			expectMetadata: true,
		},
		{
			name:           "Case-insensitive matching",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_RIDE,
			typeMappings:   `{"ride": "VirtualRide"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE,
			expectMetadata: true,
		},
		{
			name:           "No matching mapping keeps original",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			typeMappings:   `{"Run": "VirtualRun"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			expectMetadata: false,
		},
		{
			name:           "Empty mappings does nothing",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			typeMappings:   "",
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			expectMetadata: false,
		},
		{
			name:           "Invalid JSON does nothing",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			typeMappings:   `{invalid}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			expectMetadata: false,
		},
		{
			name:           "Multiple mappings - first match used",
			activityType:   pb.ActivityType_ACTIVITY_TYPE_RUN,
			typeMappings:   `{"Run": "VirtualRun", "Ride": "VirtualRide"}`,
			expectedType:   pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN,
			expectMetadata: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			act := &pb.StandardizedActivity{
				Name: "Test Activity",
				Type: tt.activityType,
			}
			config := map[string]string{}
			if tt.typeMappings != "" {
				config["type_mappings"] = tt.typeMappings
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
				if res.Metadata["mapping_used"] == "" {
					t.Error("Expected mapping_used in metadata")
				}
			}
		})
	}
}
