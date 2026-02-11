package fit_parser

import (
	"os"
	"testing"
)

func TestParseFitFile_StructuredIntervals(t *testing.T) {
	data, err := os.ReadFile("../../../cmd/fit-inspect/examples/sprints.fit")
	if err != nil {
		t.Skipf("Skipping: sprints.fit not available: %v", err)
	}

	activity, err := ParseFitFile(data)
	if err != nil {
		t.Fatalf("ParseFitFile failed: %v", err)
	}

	if len(activity.Sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(activity.Sessions))
	}

	session := activity.Sessions[0]

	// sprints.fit has 22 laps: 1 warmup, 3×(active+recovery) at 40s/20s,
	// 4×(active+recovery) at 30s/30s, 3×(active+recovery) at 20s/40s, 2 cooldown
	if len(session.Laps) != 22 {
		t.Errorf("Expected 22 laps, got %d", len(session.Laps))
	}

	// Every lap should have an intensity value
	for i, lap := range session.Laps {
		if lap.Intensity == "" {
			t.Errorf("Lap %d: expected non-empty intensity", i+1)
		}
	}

	// Verify specific interval intensities
	tests := []struct {
		lapIndex          int
		expectedIntensity string
		minDuration       float64
		maxDuration       float64
	}{
		{0, "warmup", 299, 301},    // Lap 1: 300s warmup
		{1, "active", 39, 41},      // Lap 2: 40s sprint
		{2, "recovery", 19, 21},    // Lap 3: 20s recovery
		{3, "active", 39, 41},      // Lap 4: 40s sprint
		{7, "active", 29, 31},      // Lap 8: 30s sprint (second group)
		{8, "recovery", 29, 31},    // Lap 9: 30s recovery
		{15, "active", 19, 21},     // Lap 16: 20s sprint (third group)
		{16, "recovery", 39, 41},   // Lap 17: 40s recovery
		{20, "cooldown", 299, 301}, // Lap 21: 300s cooldown
	}

	for _, tc := range tests {
		if tc.lapIndex >= len(session.Laps) {
			t.Errorf("Lap index %d out of range", tc.lapIndex)
			continue
		}
		lap := session.Laps[tc.lapIndex]
		if lap.Intensity != tc.expectedIntensity {
			t.Errorf("Lap %d: expected intensity %q, got %q", tc.lapIndex+1, tc.expectedIntensity, lap.Intensity)
		}
		if lap.TotalElapsedTime < tc.minDuration || lap.TotalElapsedTime > tc.maxDuration {
			t.Errorf("Lap %d: expected duration %.0f-%.0fs, got %.1fs", tc.lapIndex+1, tc.minDuration, tc.maxDuration, lap.TotalElapsedTime)
		}
	}

	// Count intensities
	intensityCounts := make(map[string]int)
	for _, lap := range session.Laps {
		intensityCounts[lap.Intensity]++
	}

	if intensityCounts["warmup"] != 1 {
		t.Errorf("Expected 1 warmup lap, got %d", intensityCounts["warmup"])
	}
	if intensityCounts["active"] != 10 {
		t.Errorf("Expected 10 active laps, got %d", intensityCounts["active"])
	}
	if intensityCounts["recovery"] != 9 {
		t.Errorf("Expected 9 recovery laps, got %d", intensityCounts["recovery"])
	}
	if intensityCounts["cooldown"] != 2 {
		t.Errorf("Expected 2 cooldown laps, got %d", intensityCounts["cooldown"])
	}

	t.Logf("Interval extraction: %d laps, intensities=%v", len(session.Laps), intensityCounts)
}

func TestParseFitFile_WorkoutDefinition(t *testing.T) {
	data, err := os.ReadFile("../../../cmd/fit-inspect/examples/sprints.fit")
	if err != nil {
		t.Skipf("Skipping: sprints.fit not available: %v", err)
	}

	activity, err := ParseFitFile(data)
	if err != nil {
		t.Fatalf("ParseFitFile failed: %v", err)
	}

	if activity.Workout == nil {
		t.Fatal("Expected non-nil WorkoutDefinition")
	}

	if activity.Workout.Name != "20min Sprints" {
		t.Errorf("Expected workout name %q, got %q", "20min Sprints", activity.Workout.Name)
	}

	// 11 steps: warmup, 3×(active+recovery) repeat group, 4×(active+recovery) repeat group,
	// 3×(active+recovery) repeat group, cooldown
	if len(activity.Workout.Steps) != 11 {
		t.Fatalf("Expected 11 workout steps, got %d", len(activity.Workout.Steps))
	}

	// Step 1: warmup (300s)
	step1 := activity.Workout.Steps[0]
	if step1.Intensity != "warmup" {
		t.Errorf("Step 1: expected intensity %q, got %q", "warmup", step1.Intensity)
	}
	if step1.DurationType != "time" {
		t.Errorf("Step 1: expected duration type %q, got %q", "time", step1.DurationType)
	}
	if step1.DurationValue != 300000 { // 300s in ms
		t.Errorf("Step 1: expected duration value 300000, got %d", step1.DurationValue)
	}

	// Step 4: repeat group
	step4 := activity.Workout.Steps[3]
	if step4.DurationType != "repeat_until_steps_cmplt" {
		t.Errorf("Step 4: expected repeat, got %q", step4.DurationType)
	}

	// Step 11: cooldown
	step11 := activity.Workout.Steps[10]
	if step11.Intensity != "cooldown" {
		t.Errorf("Step 11: expected intensity %q, got %q", "cooldown", step11.Intensity)
	}

	t.Logf("Workout: %q with %d steps", activity.Workout.Name, len(activity.Workout.Steps))
}
