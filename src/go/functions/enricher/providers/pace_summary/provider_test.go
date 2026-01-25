package pace_summary

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPaceSummary_Enrich_Success(t *testing.T) {
	provider := NewPaceSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity with speed data (5 m/s = 3:20/km, 4 m/s = 4:10/km, 6 m/s = 2:47/km)
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Speed: 4.0}, // 4:10/km
							{Speed: 5.0}, // 3:20/km
							{Speed: 5.0}, // 3:20/km
							{Speed: 6.0}, // 2:47/km (best)
							{Speed: 5.0}, // 3:20/km
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{UserId: "test-user"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify metadata
	if result.Metadata["pace_summary_status"] != "success" {
		t.Errorf("Expected pace_summary_status=success, got %s", result.Metadata["pace_summary_status"])
	}

	// Verify best pace (6 m/s = ~2:47/km)
	if result.Metadata["pace_best"] != "2:46" && result.Metadata["pace_best"] != "2:47" {
		t.Errorf("Expected pace_best around 2:46-2:47, got %s", result.Metadata["pace_best"])
	}

	// Verify description is appended
	if result.Description == "" {
		t.Error("Expected non-empty description")
	}
	if result.Description == "Morning Run" {
		t.Error("Expected description to be appended with pace summary")
	}
}

func TestPaceSummary_Enrich_NoSpeedData(t *testing.T) {
	provider := NewPaceSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity without speed data
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Strength Workout",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Speed: 0},
							{Speed: 0},
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{UserId: "test-user"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result.Metadata["pace_summary_status"] != "skipped" {
		t.Errorf("Expected pace_summary_status=skipped, got %s", result.Metadata["pace_summary_status"])
	}
}

func TestPaceSummary_Name(t *testing.T) {
	provider := NewPaceSummary()
	expected := "pace-summary"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestPaceSummary_ProviderType(t *testing.T) {
	provider := NewPaceSummary()
	expected := pb.EnricherProviderType_ENRICHER_PROVIDER_PACE_SUMMARY
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}

func TestFormatPace(t *testing.T) {
	tests := []struct {
		paceMinutes float64
		expected    string
	}{
		{5.5, "5:30"},
		{4.0, "4:00"},
		{3.333, "3:19"},
		{6.75, "6:45"},
	}

	for _, tt := range tests {
		result := formatPace(tt.paceMinutes)
		if result != tt.expected {
			t.Errorf("formatPace(%.3f) = %s, want %s", tt.paceMinutes, result, tt.expected)
		}
	}
}
