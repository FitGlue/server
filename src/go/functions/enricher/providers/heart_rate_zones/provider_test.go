package heart_rate_zones

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHeartRateZones_Enrich_Success(t *testing.T) {
	provider := NewHeartRateZonesProvider()
	provider.Service = &bootstrap.Service{}

	// Create activity with heart rate data spanning multiple zones
	baseTime := time.Now()
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(baseTime),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 1800, // 30 minutes
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							// Zone 1 (Recovery): 50-60% of 190 = 95-114 bpm
							{HeartRate: 100, Timestamp: timestamppb.New(baseTime)},
							{HeartRate: 105, Timestamp: timestamppb.New(baseTime.Add(1 * time.Minute))},
							// Zone 2 (Endurance): 60-70% of 190 = 114-133 bpm
							{HeartRate: 120, Timestamp: timestamppb.New(baseTime.Add(2 * time.Minute))},
							{HeartRate: 125, Timestamp: timestamppb.New(baseTime.Add(3 * time.Minute))},
							{HeartRate: 130, Timestamp: timestamppb.New(baseTime.Add(4 * time.Minute))},
							// Zone 3 (Tempo): 70-80% of 190 = 133-152 bpm
							{HeartRate: 140, Timestamp: timestamppb.New(baseTime.Add(5 * time.Minute))},
							{HeartRate: 145, Timestamp: timestamppb.New(baseTime.Add(6 * time.Minute))},
							// Zone 4 (Threshold): 80-90% of 190 = 152-171 bpm
							{HeartRate: 160, Timestamp: timestamppb.New(baseTime.Add(7 * time.Minute))},
							// Zone 5 (VO2 Max): 90-100% of 190 = 171-190 bpm
							{HeartRate: 180, Timestamp: timestamppb.New(baseTime.Add(8 * time.Minute))},
							{HeartRate: 175, Timestamp: timestamppb.New(baseTime.Add(9 * time.Minute))},
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
	if result.Metadata["hr_zones_status"] != "success" {
		t.Errorf("Expected hr_zones_status=success, got %s", result.Metadata["hr_zones_status"])
	}
	if result.Metadata["max_hr"] != "190" {
		t.Errorf("Expected max_hr=190, got %s", result.Metadata["max_hr"])
	}

	// Verify description
	if result.Description == "" {
		t.Error("Expected non-empty description")
	}
	if !contains(result.Description, "❤️ Heart Rate Zones:") {
		t.Error("Expected description to contain header")
	}
	if !contains(result.Description, "Zone 1") {
		t.Error("Expected description to contain Zone 1")
	}
}

func TestHeartRateZones_Enrich_CustomMaxHR(t *testing.T) {
	provider := NewHeartRateZonesProvider()
	provider.Service = &bootstrap.Service{}

	baseTime := time.Now()
	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(baseTime),
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 150, Timestamp: timestamppb.New(baseTime)},
							{HeartRate: 160, Timestamp: timestamppb.New(baseTime.Add(1 * time.Minute))},
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{UserId: "test-user"}
	inputs := map[string]string{"max_hr": "200"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, inputs, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result.Metadata["max_hr"] != "200" {
		t.Errorf("Expected max_hr=200, got %s", result.Metadata["max_hr"])
	}
}

func TestHeartRateZones_Enrich_PercentageStyle(t *testing.T) {
	provider := NewHeartRateZonesProvider()
	provider.Service = &bootstrap.Service{}

	baseTime := time.Now()
	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(baseTime),
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 120, Timestamp: timestamppb.New(baseTime)},
							{HeartRate: 125, Timestamp: timestamppb.New(baseTime.Add(1 * time.Minute))},
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{UserId: "test-user"}
	inputs := map[string]string{"style": "percentage"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, inputs, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Percentage style should include "%" in output
	if !contains(result.Description, "%") {
		t.Error("Expected percentage style to include % in description")
	}
}

func TestHeartRateZones_Enrich_TextStyle(t *testing.T) {
	provider := NewHeartRateZonesProvider()
	provider.Service = &bootstrap.Service{}

	baseTime := time.Now()
	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(baseTime),
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 120, Timestamp: timestamppb.New(baseTime)},
							{HeartRate: 125, Timestamp: timestamppb.New(baseTime.Add(1 * time.Minute))},
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{UserId: "test-user"}
	inputs := map[string]string{"style": "text"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, inputs, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Text style should include level descriptors
	hasLevel := contains(result.Description, "None") ||
		contains(result.Description, "Low") ||
		contains(result.Description, "Moderate") ||
		contains(result.Description, "High") ||
		contains(result.Description, "Primary")

	if !hasLevel {
		t.Error("Expected text style to include level descriptors")
	}
}

func TestHeartRateZones_Enrich_NoHRData(t *testing.T) {
	provider := NewHeartRateZonesProvider()
	provider.Service = &bootstrap.Service{}

	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{
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

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result.Metadata["hr_zones_status"] != "skipped" {
		t.Errorf("Expected hr_zones_status=skipped, got %s", result.Metadata["hr_zones_status"])
	}
}

func TestHeartRateZones_Name(t *testing.T) {
	provider := NewHeartRateZonesProvider()
	expected := "heart-rate-zones"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestHeartRateZones_ProviderType(t *testing.T) {
	provider := NewHeartRateZonesProvider()
	expected := pb.EnricherProviderType_ENRICHER_PROVIDER_HEART_RATE_ZONES
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}

func TestGetZoneIndex(t *testing.T) {
	tests := []struct {
		hrPct    float64
		expected int
	}{
		{0.40, -1}, // Below zone 1
		{0.55, 0},  // Zone 1
		{0.65, 1},  // Zone 2
		{0.75, 2},  // Zone 3
		{0.85, 3},  // Zone 4
		{0.95, 4},  // Zone 5
		{1.00, 4},  // Max HR (Zone 5)
		{1.05, 4},  // Above max (still Zone 5)
	}

	for _, tt := range tests {
		result := getZoneIndex(tt.hrPct)
		if result != tt.expected {
			t.Errorf("getZoneIndex(%.2f) = %d, expected %d", tt.hrPct, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
