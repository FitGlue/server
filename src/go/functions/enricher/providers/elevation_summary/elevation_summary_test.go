package elevation_summary

import (
	"context"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestElevationSummary_Enrich_Success(t *testing.T) {
	provider := NewElevationSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity with elevation data
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Mountain Ride",
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Altitude: 100}, // Start at 100
							{Altitude: 150}, // Gain 50
							{Altitude: 200}, // Gain 50
							{Altitude: 180}, // Loss 20
							{Altitude: 250}, // Gain 70
							{Altitude: 220}, // Loss 30
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
	// Gain: 50 + 50 + 70 = 170
	// Loss: 20 + 30 = 50
	// Max: 250
	if result.Metadata["elevation_summary_status"] != "success" {
		t.Errorf("Expected elevation_summary_status=success, got %s", result.Metadata["elevation_summary_status"])
	}
	if result.Metadata["elevation_gain"] != "170.00" {
		t.Errorf("Expected elevation_gain=170.00, got %s", result.Metadata["elevation_gain"])
	}
	if result.Metadata["elevation_loss"] != "50.00" {
		t.Errorf("Expected elevation_loss=50.00, got %s", result.Metadata["elevation_loss"])
	}
	if result.Metadata["elevation_max"] != "250.00" {
		t.Errorf("Expected elevation_max=250.00, got %s", result.Metadata["elevation_max"])
	}

	// Verify description is appended
	expectedSummary := "\n\n⛰️ Elevation: +170m gain • -50m loss • 250m max"
	if result.Description != "Mountain Ride"+expectedSummary {
		t.Errorf("Expected description %q, got %q", "Mountain Ride"+expectedSummary, result.Description)
	}
}

func TestElevationSummary_Enrich_SkipNegative(t *testing.T) {
	provider := NewElevationSummary()
	provider.Service = &bootstrap.Service{}

	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Altitude: 100},
							{Altitude: -10}, // Should be skipped
							{Altitude: 150}, // Previous was 100 -> Gain 50
						},
					},
				},
			},
		},
	}

	result, _ := provider.Enrich(context.Background(), activity, nil, nil, false)

	if result.Metadata["elevation_gain"] != "50.00" {
		t.Errorf("Expected elevation_gain=50.00, got %s", result.Metadata["elevation_gain"])
	}
	if result.Metadata["elevation_record_count"] != "2" {
		t.Errorf("Expected elevation_record_count=2, got %s", result.Metadata["elevation_record_count"])
	}
}

func TestElevationSummary_Enrich_NoData(t *testing.T) {
	provider := NewElevationSummary()
	provider.Service = &bootstrap.Service{}

	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Altitude: 0},
							{Altitude: -5},
						},
					},
				},
			},
		},
	}

	result, _ := provider.Enrich(context.Background(), activity, nil, nil, false)

	if result.Metadata["elevation_summary_status"] != "skipped" {
		t.Errorf("Expected elevation_summary_status=skipped, got %s", result.Metadata["elevation_summary_status"])
	}
}
