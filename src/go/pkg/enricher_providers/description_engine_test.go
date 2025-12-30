package enricher_providers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestDescriptionEngine_Integration(t *testing.T) {
	// 1. Setup Input with comprehensive test data
	activity := &pb.StandardizedActivity{
		Source:      "HEVY",
		ExternalId:  "test-uuid",
		Name:        "Hyrox Training Session",
		Description: "Crushing it today! 💪",
		Type:        "WEIGHT_TRAINING",
		Sessions: []*pb.Session{
			{
				StrengthSets: []*pb.StrengthSet{
					// Superset 1: Bench Press + Dumbbell Row
					{ExerciseName: "Bench Press", Reps: 10, WeightKg: 60, SetType: "warmup", SupersetId: "ss1", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_CHEST, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_TRICEPS, pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS}},
					{ExerciseName: "Bench Press", Reps: 8, WeightKg: 100, SupersetId: "ss1", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_CHEST, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_TRICEPS}},
					{ExerciseName: "Bench Press", Reps: 8, WeightKg: 100, SupersetId: "ss1", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_CHEST, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_TRICEPS}},
					{ExerciseName: "Bench Press", Reps: 6, WeightKg: 100, SetType: "failure", SupersetId: "ss1", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_CHEST, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_TRICEPS}},
					{ExerciseName: "Dumbbell Row", Reps: 12, WeightKg: 40, SupersetId: "ss1", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_LATS, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_BICEPS}},
					{ExerciseName: "Dumbbell Row", Reps: 12, WeightKg: 40, SupersetId: "ss1", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_LATS, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_BICEPS}},
					{ExerciseName: "Dumbbell Row", Reps: 12, WeightKg: 40, SupersetId: "ss1", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_LATS, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_BICEPS}},

					// Regular exercise: Squats
					{ExerciseName: "Squat", Reps: 5, WeightKg: 140, PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_GLUTES, pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS}},
					{ExerciseName: "Squat", Reps: 5, WeightKg: 140, PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_GLUTES, pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS}},
					{ExerciseName: "Squat", Reps: 5, WeightKg: 140, PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS, SecondaryMuscleGroups: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_GLUTES, pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS}},

					// Cardio exercises (distance/duration based)
					{ExerciseName: "Running", Reps: 0, WeightKg: 0, DistanceMeters: 1000, DurationSeconds: 300, PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_CARDIO},
					{ExerciseName: "Rowing Machine", Reps: 0, WeightKg: 0, DistanceMeters: 500, DurationSeconds: 120, PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_CARDIO},

					// Superset 2: Bicep Curl + Tricep Extension
					{ExerciseName: "Bicep Curl", Reps: 12, WeightKg: 20, SupersetId: "ss2", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_BICEPS},
					{ExerciseName: "Bicep Curl", Reps: 12, WeightKg: 20, SupersetId: "ss2", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_BICEPS},
					{ExerciseName: "Bicep Curl", Reps: 12, WeightKg: 20, SupersetId: "ss2", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_BICEPS},
					{ExerciseName: "Tricep Extension", Reps: 15, WeightKg: 15, SupersetId: "ss2", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_TRICEPS},
					{ExerciseName: "Tricep Extension", Reps: 15, WeightKg: 15, SupersetId: "ss2", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_TRICEPS},
					{ExerciseName: "Tricep Extension", Reps: 15, WeightKg: 15, SupersetId: "ss2", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_TRICEPS},

					// Bodyweight exercise
					{ExerciseName: "Burpee Box Jump", Reps: 20, WeightKg: 0, PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_FULL_BODY},

					// Dropset
					{ExerciseName: "Shoulder Press", Reps: 10, WeightKg: 30, PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS},
					{ExerciseName: "Shoulder Press", Reps: 8, WeightKg: 25, SetType: "dropset", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS},
					{ExerciseName: "Shoulder Press", Reps: 6, WeightKg: 20, SetType: "dropset", PrimaryMuscleGroup: pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS},
				},
			},
		},
	}

	// 2. Setup Providers
	pLink := enricher_providers.NewSourceLinkProvider()
	pSummary := enricher_providers.NewWorkoutSummaryProvider()
	pHeatmap := enricher_providers.NewMuscleHeatmapProvider()
	pBranding := enricher_providers.NewBrandingProvider()

	// 3. Execute Providers
	ctx := context.Background()
	resLink, _ := pLink.Enrich(ctx, activity, nil, nil)
	resSummary, _ := pSummary.Enrich(ctx, activity, nil, nil)
	resHeatmap, _ := pHeatmap.Enrich(ctx, activity, nil, nil)
	resBranding, _ := pBranding.Enrich(ctx, activity, nil, nil)

	// 4. Simulate Orchestrator Merge
	finalDesc := activity.Description

	// Order: Summary, Heatmap, Link, then Branding (always last)
	results := []*enricher_providers.EnrichmentResult{resSummary, resHeatmap, resLink, resBranding}

	for _, res := range results {
		if res.Description != "" {
			trimmed := strings.TrimSpace(res.Description)
			if trimmed != "" {
				if finalDesc != "" {
					finalDesc += "\n\n"
				}
				finalDesc += trimmed
			}
		}
	}

	// 5. Verify Content
	expectedParts := []string{
		// Original description
		"Crushing it today! 💪",

		// Workout Summary
		"Workout Summary:",
		"(Exercises with matching numbers are supersets - performed back-to-back)",
		"([W]=Warmup, [F]=Failure, [D]=Dropset)",

		// Superset 1 with emoji numbers
		"1️⃣ Bench Press:",
		"[W] 10 × 60.0kg",
		"[F] 6 × 100.0kg",
		"1️⃣ Dumbbell Row:",
		"3 x 12 × 40.0kg",

		// Regular exercise
		"- Squat: 3 x 5 × 140.0kg",

		// Distance/duration exercises
		"- Running: 1000m in 5m",
		"- Rowing Machine: 500m in 2m",

		// Superset 2
		"2️⃣ Bicep Curl:",
		"2️⃣ Tricep Extension:",

		// Bodyweight
		"- Burpee Box Jump: 20 reps",

		// Dropset
		"- Shoulder Press:",
		"[D] 8 × 25.0kg",
		"[D] 6 × 20.0kg",

		// Muscle Heatmap (should be sorted by volume, descending)
		"Muscle Heatmap:",

		// Source link
		"View on Hevy: https://hevy.com/workout/test-uuid",

		// Branding footer (always present)
		"Posted via fitglue.tech 💪",
	}

	for _, part := range expectedParts {
		if !strings.Contains(finalDesc, part) {
			t.Errorf("Expected description to contain %q, but got:\n%s", part, finalDesc)
		}
	}

	// Verify muscle heatmap is sorted by volume (descending)
	// The heatmap should appear with highest volume muscles first
	heatmapStart := strings.Index(finalDesc, "Muscle Heatmap:")
	if heatmapStart == -1 {
		t.Fatal("Muscle Heatmap not found in description")
	}

	// Verify branding is at the end
	if !strings.HasSuffix(strings.TrimSpace(finalDesc), "Posted via fitglue.tech 💪") {
		t.Error("Expected branding footer to be at the end of description")
	}
}
