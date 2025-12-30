package enricher_providers_test

import (
	"context"
	"testing"

	"github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestTypeMapperProvider_Enrich(t *testing.T) {
	provider := enricher_providers.NewTypeMapperProvider()
	ctx := context.Background()

	tests := []struct {
		name           string
		activityName   string
		rulesJson      string
		expectedType   string
		expectedNewTyp string
	}{
		{
			name:         "Matches substring (Yoga)",
			activityName: "Morning Yoga Flow",
			rulesJson:    `[{"substring": "Yoga", "target_type": "YOGA"}]`,
			expectedType: "YOGA",
		},
		{
			name:         "Matches substring case-insensitive",
			activityName: "sunday morning run",
			rulesJson:    `[{"substring": "run", "target_type": "RUNNING"}]`,
			expectedType: "RUNNING",
		},
		{
			name:         "No match keeps original type",
			activityName: "Heavy Lift",
			rulesJson:    `[{"substring": "Yoga", "target_type": "YOGA"}]`,
			expectedType: "WEIGHT_TRAINING",
		},
		{
			name:         "Empty rules JSON does nothing",
			activityName: "Any Activity",
			rulesJson:    "",
			expectedType: "WEIGHT_TRAINING",
		},
		{
			name:         "Invalid JSON does nothing",
			activityName: "Any Activity",
			rulesJson:    `{invalid}`,
			expectedType: "WEIGHT_TRAINING",
		},
		{
			name:         "First match wins",
			activityName: "Yoga and Run",
			rulesJson:    `[{"substring": "Yoga", "target_type": "YOGA"}, {"substring": "Run", "target_type": "RUNNING"}]`,
			expectedType: "YOGA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity := &pb.StandardizedActivity{
				Name: tt.activityName,
				Type: "WEIGHT_TRAINING", // Default
			}
			config := map[string]string{}
			if tt.rulesJson != "" {
				config["rules"] = tt.rulesJson
			}

			res, err := provider.Enrich(ctx, activity, nil, config)
			if err != nil {
				t.Fatalf("Enrich failed: %v", err)
			}

			if activity.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, activity.Type)
			}

			if activity.Type != "WEIGHT_TRAINING" {
				// If type changed, check metadata
				if res.Metadata["new_type"] != activity.Type {
					t.Errorf("Metadata new_type expected %s, got %s", activity.Type, res.Metadata["new_type"])
				}
			}
		})
	}
}
