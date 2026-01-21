package heart_rate_summary

import (
	"context"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHeartRateSummary_Enrich_Success(t *testing.T) {
	provider := NewHeartRateSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity with heart rate data
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 120},
							{HeartRate: 130},
							{HeartRate: 140},
							{HeartRate: 150},
							{HeartRate: 160},
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
	if result.Metadata["hr_summary_status"] != "success" {
		t.Errorf("Expected hr_summary_status=success, got %s", result.Metadata["hr_summary_status"])
	}
	if result.Metadata["hr_min"] != "120" {
		t.Errorf("Expected hr_min=120, got %s", result.Metadata["hr_min"])
	}
	if result.Metadata["hr_avg"] != "140" {
		t.Errorf("Expected hr_avg=140, got %s", result.Metadata["hr_avg"])
	}
	if result.Metadata["hr_max"] != "160" {
		t.Errorf("Expected hr_max=160, got %s", result.Metadata["hr_max"])
	}
	if result.Metadata["hr_sample_count"] != "5" {
		t.Errorf("Expected hr_sample_count=5, got %s", result.Metadata["hr_sample_count"])
	}

	// Verify description is appended
	if result.Description == "" {
		t.Error("Expected non-empty description")
	}
	if result.Description == "Morning Run" {
		t.Error("Expected description to be appended with HR summary")
	}
}

func TestHeartRateSummary_Enrich_NoHRData(t *testing.T) {
	provider := NewHeartRateSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity without heart rate data
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 0},
							{HeartRate: 0},
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

	if result.Metadata["hr_summary_status"] != "skipped" {
		t.Errorf("Expected hr_summary_status=skipped, got %s", result.Metadata["hr_summary_status"])
	}
}

func TestHeartRateSummary_Name(t *testing.T) {
	provider := NewHeartRateSummary()
	expected := "heart-rate-summary"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestHeartRateSummary_ProviderType(t *testing.T) {
	provider := NewHeartRateSummary()
	expected := pb.EnricherProviderType_ENRICHER_PROVIDER_HEART_RATE_SUMMARY
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}
