package fit_file_hr

import (
	"context"
	"encoding/base64"
	"log/slog"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFitFileHR_Name(t *testing.T) {
	provider := NewFitFileHRProvider()
	expected := "fit-file-heart-rate"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestFitFileHR_ProviderType(t *testing.T) {
	provider := NewFitFileHRProvider()
	expected := pb.EnricherProviderType_ENRICHER_PROVIDER_FIT_FILE_HEART_RATE
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}

func TestFitFileHR_SkipsIfExistingHRData(t *testing.T) {
	provider := NewFitFileHRProvider()

	// Create activity WITH existing heart rate data
	startTime := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(startTime),
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 120}, // Existing HR data
							{HeartRate: 130},
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
	}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Metadata["hr_source"] != "skipped" {
		t.Errorf("Expected hr_source=skipped, got %s", result.Metadata["hr_source"])
	}
	if result.Metadata["force"] != "false" {
		t.Errorf("Expected force=false in metadata, got %s", result.Metadata["force"])
	}
}

func TestFitFileHR_NoServiceError(t *testing.T) {
	provider := NewFitFileHRProvider()
	// Don't set service

	// Activity without HR data
	startTime := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(startTime),
		Source:    "SOURCE_FILE_UPLOAD",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Timestamp: timestamppb.New(startTime)},
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
	}

	_, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err == nil {
		t.Error("Expected error when service not initialized")
	}
}

func TestFitFileHR_EnrichResume_InvalidBase64(t *testing.T) {
	provider := NewFitFileHRProvider()

	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
		Sessions:  []*pb.Session{{TotalElapsedTime: 3600}},
	}

	user := &pb.UserRecord{UserId: "test-user"}

	pendingInput := &pb.PendingInput{
		InputData: map[string]string{
			"fit_file_base64": "not-valid-base64!!!",
		},
	}

	_, err := provider.EnrichResume(context.Background(), activity, user, pendingInput)
	if err == nil {
		t.Error("Expected error for invalid base64")
	}
}

func TestFitFileHR_EnrichResume_EmptyInput(t *testing.T) {
	provider := NewFitFileHRProvider()

	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
		Sessions:  []*pb.Session{{TotalElapsedTime: 3600}},
	}

	user := &pb.UserRecord{UserId: "test-user"}

	pendingInput := &pb.PendingInput{
		InputData: map[string]string{},
	}

	_, err := provider.EnrichResume(context.Background(), activity, user, pendingInput)
	if err == nil {
		t.Error("Expected error for empty fit_file_base64")
	}
}

func TestHasExistingHeartRateData(t *testing.T) {
	tests := []struct {
		name     string
		activity *pb.StandardizedActivity
		expected bool
	}{
		{
			name: "has HR data",
			activity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{{
					Laps: []*pb.Lap{{
						Records: []*pb.Record{{HeartRate: 120}},
					}},
				}},
			},
			expected: true,
		},
		{
			name: "no HR data",
			activity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{{
					Laps: []*pb.Lap{{
						Records: []*pb.Record{{HeartRate: 0}},
					}},
				}},
			},
			expected: false,
		},
		{
			name: "empty sessions",
			activity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasExistingHeartRateData(tt.activity)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHasGPSData(t *testing.T) {
	tests := []struct {
		name     string
		activity *pb.StandardizedActivity
		expected bool
	}{
		{
			name: "has GPS data",
			activity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{{
					Laps: []*pb.Lap{{
						Records: []*pb.Record{{PositionLat: 53.07, PositionLong: -1.02}},
					}},
				}},
			},
			expected: true,
		},
		{
			name: "no GPS data",
			activity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{{
					Laps: []*pb.Lap{{
						Records: []*pb.Record{{PositionLat: 0, PositionLong: 0}},
					}},
				}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasGPSData(tt.activity)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractHRSamples(t *testing.T) {
	startTime := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{{
			Laps: []*pb.Lap{{
				Records: []*pb.Record{
					{HeartRate: 100, Timestamp: timestamppb.New(startTime)},
					{HeartRate: 110, Timestamp: timestamppb.New(startTime.Add(1 * time.Second))},
					{HeartRate: 0, Timestamp: timestamppb.New(startTime.Add(2 * time.Second))}, // Zero HR - should skip
					{HeartRate: 120, Timestamp: timestamppb.New(startTime.Add(3 * time.Second))},
				},
			}},
		}},
	}

	samples := extractHRSamples(activity)

	if len(samples) != 3 {
		t.Errorf("Expected 3 samples (skipping zero HR), got %d", len(samples))
	}

	if samples[0].Value != 100 {
		t.Errorf("Expected first sample value 100, got %d", samples[0].Value)
	}
}

func TestBuildStreamTimeBased(t *testing.T) {
	startTime := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	samples := []providers.TimedSample{
		{Timestamp: startTime, Value: 100},
		{Timestamp: startTime.Add(10 * time.Second), Value: 120},
		{Timestamp: startTime.Add(20 * time.Second), Value: 130},
	}

	stream := buildStreamTimeBased(samples, startTime, 30)

	if len(stream) != 30 {
		t.Errorf("Expected stream length 30, got %d", len(stream))
	}

	// First point
	if stream[0] != 100 {
		t.Errorf("Expected stream[0]=100, got %d", stream[0])
	}

	// At second 10
	if stream[10] != 120 {
		t.Errorf("Expected stream[10]=120, got %d", stream[10])
	}

	// At second 20
	if stream[20] != 130 {
		t.Errorf("Expected stream[20]=130, got %d", stream[20])
	}

	// Forward filled values
	if stream[5] != 100 {
		t.Errorf("Expected forward fill stream[5]=100, got %d", stream[5])
	}
	if stream[15] != 120 {
		t.Errorf("Expected forward fill stream[15]=120, got %d", stream[15])
	}
}

func TestMergeMetadata(t *testing.T) {
	base := map[string]string{"a": "1", "b": "2"}
	overlay := map[string]string{"b": "3", "c": "4"}

	result := mergeMetadata(base, overlay)

	if result["a"] != "1" {
		t.Errorf("Expected a=1, got %s", result["a"])
	}
	if result["b"] != "3" { // Overlay wins
		t.Errorf("Expected b=3 (overlay), got %s", result["b"])
	}
	if result["c"] != "4" {
		t.Errorf("Expected c=4, got %s", result["c"])
	}
}

// TestFitFileHR_EnrichResume_ValidFIT tests with a minimal valid FIT-like structure
// Note: This is a unit test with base64-encoded data - real FIT parsing is tested in fit_parser package
func TestFitFileHR_EnrichResume_InvalidFitFile(t *testing.T) {
	provider := NewFitFileHRProvider()

	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
		Sessions:  []*pb.Session{{TotalElapsedTime: 3600}},
	}

	user := &pb.UserRecord{UserId: "test-user"}

	// Valid base64 but not a valid FIT file
	invalidFitData := base64.StdEncoding.EncodeToString([]byte("not a fit file"))
	pendingInput := &pb.PendingInput{
		InputData: map[string]string{
			"fit_file_base64": invalidFitData,
		},
	}

	_, err := provider.EnrichResume(context.Background(), activity, user, pendingInput)
	if err == nil {
		t.Error("Expected error for invalid FIT file format")
	}
}
