package intervals

import (
	"context"
	"log/slog"
	"testing"

	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestIntervals_NoIntervalData(t *testing.T) {
	p := NewIntervals()
	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						StartTime:        timestamppb.New(time.Now()),
						TotalElapsedTime: 600,
						TotalDistance:    1000,
						Intensity:        "", // No intensity = not interval data
					},
				},
			},
		},
	}

	result, err := p.Enrich(context.Background(), slog.Default(), activity, &pb.UserRecord{}, nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Metadata["intervals_status"] != "skipped" {
		t.Errorf("Expected skipped, got %s", result.Metadata["intervals_status"])
	}
}

func TestIntervals_BasicIntervals(t *testing.T) {
	p := NewIntervals()
	now := time.Now()

	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{StartTime: timestamppb.New(now), TotalElapsedTime: 300, TotalDistance: 863, Intensity: "warmup"},
					{StartTime: timestamppb.New(now.Add(5 * time.Minute)), TotalElapsedTime: 40, TotalDistance: 193, Intensity: "active"},
					{StartTime: timestamppb.New(now.Add(6 * time.Minute)), TotalElapsedTime: 20, TotalDistance: 63, Intensity: "recovery"},
					{StartTime: timestamppb.New(now.Add(7 * time.Minute)), TotalElapsedTime: 40, TotalDistance: 177, Intensity: "active"},
					{StartTime: timestamppb.New(now.Add(8 * time.Minute)), TotalElapsedTime: 20, TotalDistance: 59, Intensity: "recovery"},
					{StartTime: timestamppb.New(now.Add(9 * time.Minute)), TotalElapsedTime: 40, TotalDistance: 167, Intensity: "active"},
					{StartTime: timestamppb.New(now.Add(10 * time.Minute)), TotalElapsedTime: 300, TotalDistance: 763, Intensity: "cooldown"},
				},
			},
		},
		Workout: &pb.WorkoutDefinition{
			Name: "Test Sprints",
		},
	}

	result, err := p.Enrich(context.Background(), slog.Default(), activity, &pb.UserRecord{}, nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Metadata["intervals_status"] != "success" {
		t.Errorf("Expected success, got %s", result.Metadata["intervals_status"])
	}

	if result.Metadata["intervals_workout"] != "Test Sprints" {
		t.Errorf("Expected workout name 'Test Sprints', got %q", result.Metadata["intervals_workout"])
	}

	if result.Metadata["intervals_active"] != "3" {
		t.Errorf("Expected 3 active intervals, got %s", result.Metadata["intervals_active"])
	}

	if result.SectionHeader != "⏱️ Intervals — Test Sprints:" {
		t.Errorf("Expected SectionHeader '⏱️ Intervals — Test Sprints:', got %q", result.SectionHeader)
	}

	// Description should start with the section header (inlined, same pattern as Parkrun)
	if !contains(result.Description, "⏱️ Intervals — Test Sprints:") {
		t.Errorf("Description should contain section header '⏱️ Intervals — Test Sprints:', got: %s", result.Description)
	}
	if !contains(result.Description, "Warmup") {
		t.Errorf("Description should contain Warmup, got: %s", result.Description)
	}
	if !contains(result.Description, "Cooldown") {
		t.Errorf("Description should contain Cooldown, got: %s", result.Description)
	}
	if !contains(result.Description, "3×0:40 intervals") {
		t.Errorf("Description should contain grouped intervals '3×0:40 intervals', got: %s", result.Description)
	}

	// Verify time markers were generated
	// 1 warmup + 3 active + 2 recovery + 1 cooldown = 7 markers
	if len(result.TimeMarkers) != 7 {
		t.Errorf("Expected 7 time markers, got %d", len(result.TimeMarkers))
	}
	if len(result.TimeMarkers) > 0 {
		if result.TimeMarkers[0].MarkerType != "warmup_start" {
			t.Errorf("Expected first marker type 'warmup_start', got %q", result.TimeMarkers[0].MarkerType)
		}
		if result.TimeMarkers[len(result.TimeMarkers)-1].MarkerType != "cooldown_start" {
			t.Errorf("Expected last marker type 'cooldown_start', got %q", result.TimeMarkers[len(result.TimeMarkers)-1].MarkerType)
		}
	}
	if result.Metadata["time_markers"] != "7" {
		t.Errorf("Expected metadata time_markers='7', got %q", result.Metadata["time_markers"])
	}

	t.Logf("Description:\n%s", result.Description)
}

func TestIntervals_ShowAllIntervals(t *testing.T) {
	p := NewIntervals()
	now := time.Now()

	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{StartTime: timestamppb.New(now), TotalElapsedTime: 40, TotalDistance: 193, Intensity: "active"},
					{StartTime: timestamppb.New(now.Add(1 * time.Minute)), TotalElapsedTime: 20, TotalDistance: 63, Intensity: "recovery"},
					{StartTime: timestamppb.New(now.Add(2 * time.Minute)), TotalElapsedTime: 40, TotalDistance: 177, Intensity: "active"},
				},
			},
		},
	}

	inputs := map[string]string{
		"show_all_intervals": "true",
	}

	result, err := p.Enrich(context.Background(), slog.Default(), activity, &pb.UserRecord{}, inputs, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// With show_all_intervals, should have "Run 1" and "Run 2"
	if !contains(result.Description, "Run 1") {
		t.Errorf("Expected individual intervals with show_all_intervals, got: %s", result.Description)
	}

	t.Logf("Description (show_all):\n%s", result.Description)
}

func TestIntervals_FallbackWorkoutName(t *testing.T) {
	p := NewIntervals()
	now := time.Now()

	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{StartTime: timestamppb.New(now), TotalElapsedTime: 40, TotalDistance: 193, Intensity: "active"},
					{StartTime: timestamppb.New(now.Add(1 * time.Minute)), TotalElapsedTime: 20, TotalDistance: 63, Intensity: "recovery"},
				},
			},
		},
		// No WorkoutDefinition
	}

	result, err := p.Enrich(context.Background(), slog.Default(), activity, &pb.UserRecord{}, nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Metadata["intervals_workout"] != "Structured Intervals" {
		t.Errorf("Expected fallback workout name 'Structured Intervals', got %q", result.Metadata["intervals_workout"])
	}

	// SectionHeader should NOT include the generic subtitle
	if result.SectionHeader != "⏱️ Intervals:" {
		t.Errorf("Expected SectionHeader '⏱️ Intervals:', got %q", result.SectionHeader)
	}
	// Description should also contain the inlined section header
	if !contains(result.Description, "⏱️ Intervals:") {
		t.Errorf("Description should contain inlined section header '⏱️ Intervals:', got: %s", result.Description)
	}
}

func TestIntervals_AutoSplitSkipped(t *testing.T) {
	p := NewIntervals()
	now := time.Now()

	// Simulate auto-split laps: all laps have "active" intensity only (every km)
	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{StartTime: timestamppb.New(now), TotalElapsedTime: 360, TotalDistance: 1000, Intensity: "active"},
					{StartTime: timestamppb.New(now.Add(6 * time.Minute)), TotalElapsedTime: 340, TotalDistance: 1000, Intensity: "active"},
					{StartTime: timestamppb.New(now.Add(12 * time.Minute)), TotalElapsedTime: 350, TotalDistance: 1000, Intensity: "active"},
					{StartTime: timestamppb.New(now.Add(18 * time.Minute)), TotalElapsedTime: 355, TotalDistance: 984, Intensity: "active"},
				},
			},
		},
	}

	result, err := p.Enrich(context.Background(), slog.Default(), activity, &pb.UserRecord{}, nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Metadata["intervals_status"] != "skipped" {
		t.Errorf("Expected skipped for auto-split laps, got %s", result.Metadata["intervals_status"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
