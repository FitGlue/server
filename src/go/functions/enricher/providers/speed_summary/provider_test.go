package speed_summary

import (
	"context"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSpeedSummary_Enrich_Success(t *testing.T) {
	provider := NewSpeedSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity with speed data (m/s)
	// 5 m/s = 18 km/h, 10 m/s = 36 km/h
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Ride",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Speed: 5.0},  // 18 km/h
							{Speed: 7.0},  // 25.2 km/h
							{Speed: 8.0},  // 28.8 km/h
							{Speed: 10.0}, // 36 km/h
							{Speed: 5.0},  // 18 km/h
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{UserId: "test-user"}

	result, err := provider.Enrich(context.Background(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify metadata
	if result.Metadata["speed_summary_status"] != "success" {
		t.Errorf("Expected speed_summary_status=success, got %s", result.Metadata["speed_summary_status"])
	}

	// Avg speed = 7 m/s = 25.2 km/h
	if result.Metadata["speed_avg_kmh"] != "25.2" {
		t.Errorf("Expected speed_avg_kmh=25.2, got %s", result.Metadata["speed_avg_kmh"])
	}

	// Max speed = 10 m/s = 36 km/h
	if result.Metadata["speed_max_kmh"] != "36.0" {
		t.Errorf("Expected speed_max_kmh=36.0, got %s", result.Metadata["speed_max_kmh"])
	}

	// Verify description is appended
	if result.Description == "" {
		t.Error("Expected non-empty description")
	}
	if result.Description == "Morning Ride" {
		t.Error("Expected description to be appended with speed summary")
	}
}

func TestSpeedSummary_Enrich_NoSpeedData(t *testing.T) {
	provider := NewSpeedSummary()
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

	result, err := provider.Enrich(context.Background(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result.Metadata["speed_summary_status"] != "skipped" {
		t.Errorf("Expected speed_summary_status=skipped, got %s", result.Metadata["speed_summary_status"])
	}
}

func TestSpeedSummary_Name(t *testing.T) {
	provider := NewSpeedSummary()
	expected := "speed-summary"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestSpeedSummary_ProviderType(t *testing.T) {
	provider := NewSpeedSummary()
	expected := pb.EnricherProviderType_ENRICHER_PROVIDER_SPEED_SUMMARY
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}
