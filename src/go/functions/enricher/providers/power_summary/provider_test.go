package power_summary

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPowerSummary_Enrich_Success(t *testing.T) {
	provider := NewPowerSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity with power data
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Ride",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Power: 200},
							{Power: 250},
							{Power: 300},
							{Power: 350},
							{Power: 400},
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
	if result.Metadata["power_summary_status"] != "success" {
		t.Errorf("Expected power_summary_status=success, got %s", result.Metadata["power_summary_status"])
	}
	if result.Metadata["power_avg"] != "300" {
		t.Errorf("Expected power_avg=300, got %s", result.Metadata["power_avg"])
	}
	if result.Metadata["power_max"] != "400" {
		t.Errorf("Expected power_max=400, got %s", result.Metadata["power_max"])
	}
	if result.Metadata["power_sample_count"] != "5" {
		t.Errorf("Expected power_sample_count=5, got %s", result.Metadata["power_sample_count"])
	}

	// Verify description is appended
	if result.Description == "" {
		t.Error("Expected non-empty description")
	}
	if result.Description == "Morning Ride" {
		t.Error("Expected description to be appended with power summary")
	}
}

func TestPowerSummary_Enrich_NoPowerData(t *testing.T) {
	provider := NewPowerSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity without power data
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Power: 0},
							{Power: 0},
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

	if result.Metadata["power_summary_status"] != "skipped" {
		t.Errorf("Expected power_summary_status=skipped, got %s", result.Metadata["power_summary_status"])
	}
}

func TestPowerSummary_Name(t *testing.T) {
	provider := NewPowerSummary()
	expected := "power-summary"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestPowerSummary_ProviderType(t *testing.T) {
	provider := NewPowerSummary()
	expected := pb.EnricherProviderType_ENRICHER_PROVIDER_POWER_SUMMARY
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}
