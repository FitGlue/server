package muscle_heatmap_image

import (
	"testing"

	"github.com/fitglue/server/src/go/functions/enricher/providers/muscle_heatmap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestMuscleHeatmapImageProvider_Name(t *testing.T) {
	provider := NewMuscleHeatmapImageProvider()
	expected := "muscle-heatmap-image"
	if provider.Name() != expected {
		t.Errorf("Expected name '%s', got '%s'", expected, provider.Name())
	}
}

func TestMuscleHeatmapImageProvider_ProviderType(t *testing.T) {
	provider := NewMuscleHeatmapImageProvider()
	if provider.ProviderType() != pb.EnricherProviderType_ENRICHER_PROVIDER_MUSCLE_HEATMAP_IMAGE {
		t.Errorf("Expected provider type ENRICHER_PROVIDER_MUSCLE_HEATMAP_IMAGE")
	}
}

func TestCalculateMuscleScores(t *testing.T) {
	provider := NewMuscleHeatmapImageProvider()

	sets := []*pb.StrengthSet{
		{
			ExerciseName:       "Bench Press",
			PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_CHEST,
			WeightKg:           100,
			Reps:               10,
		},
		{
			ExerciseName:       "Squat",
			PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
			WeightKg:           150,
			Reps:               8,
		},
	}

	scores := provider.calculateMuscleScores(sets, muscle_heatmap.StandardCoefficients)

	// Should have scores for chest and quads
	if len(scores) == 0 {
		t.Error("Expected non-empty scores")
	}

	// Check that scores are sorted by percentage descending
	for i := 1; i < len(scores); i++ {
		if scores[i].Percentage > scores[i-1].Percentage {
			t.Error("Scores should be sorted by percentage descending")
		}
	}

	// Check that all scores have valid colors
	for _, score := range scores {
		if len(score.SVGIDs) == 0 {
			t.Errorf("Score has empty SVGIDs")
		}
		if score.Color == "" {
			t.Errorf("Score has empty color")
		}
		if score.Percentage < 0 || score.Percentage > 1 {
			t.Errorf("Score for %v has invalid percentage: %.2f", score.SVGIDs, score.Percentage)
		}
	}
}

func TestGenerateSVG(t *testing.T) {
	provider := NewMuscleHeatmapImageProvider()

	scores := []MuscleScore{
		{SVGIDs: []string{"chest"}, Percentage: 1.0, Color: "#EC4899"},
		{SVGIDs: []string{"biceps"}, Percentage: 0.6, Color: "#7C3AED"},
		{SVGIDs: []string{"quads"}, Percentage: 0.3, Color: "#8B5CF6"},
	}

	svg, err := provider.GenerateSVG("man", scores)
	if err != nil {
		t.Fatalf("generateSVG failed: %v", err)
	}

	// Check SVG structure
	if len(svg) == 0 {
		t.Error("SVG should not be empty")
	}

	// Should contain SVG tags
	if !contains(svg, "<svg") {
		t.Error("SVG should contain opening svg tag")
	}
	if !contains(svg, "</svg>") {
		t.Error("SVG should contain closing svg tag")
	}

	// Should contain muscle colors
	if !contains(svg, "#EC4899") {
		t.Error("SVG should contain chest color")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
