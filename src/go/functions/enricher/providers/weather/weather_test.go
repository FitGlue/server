package weather

import (
	"context"
	"log/slog"
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWeatherEnrich_NoGPS(t *testing.T) {
	provider := NewWeather()

	activity := &pb.StandardizedActivity{
		Description: "Indoor workout",
		StartTime:   timestamppb.New(time.Now()),
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 120}, // No GPS data
						},
					},
				},
			},
		},
	}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, &pb.UserRecord{}, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Expected no error for activity without GPS, got: %v", err)
	}

	if result.Description != "" {
		t.Errorf("Expected description to be unchanged, got: %s", result.Description)
	}

	if result.Metadata["weather_status"] != "skipped" {
		t.Errorf("Expected weather_status=skipped, got: %s", result.Metadata["weather_status"])
	}
}

func TestWeatherCodeMapping(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{0, "Clear"},
		{1, "Partly Cloudy"},
		{2, "Partly Cloudy"},
		{3, "Partly Cloudy"},
		{45, "Fog"},
		{48, "Fog"},
		{51, "Rain"},
		{61, "Rain"},
		{67, "Rain"},
		{71, "Snow"},
		{75, "Snow"},
		{77, "Snow"},
		{95, "Thunderstorm"},
		{99, "Thunderstorm"},
		{100, "Unknown"},
	}

	for _, tt := range tests {
		result := mapWeatherCode(tt.code)
		if result != tt.expected {
			t.Errorf("mapWeatherCode(%d) = %s, expected %s", tt.code, result, tt.expected)
		}
	}
}

func TestWindDirectionMapping(t *testing.T) {
	tests := []struct {
		degrees  float64
		expected string
	}{
		{0, "N"},
		{22, "N"},
		{45, "NE"},
		{67, "NE"},
		{90, "E"},
		{135, "SE"},
		{180, "S"},
		{225, "SW"},
		{270, "W"},
		{315, "NW"},
		{360, "N"},
		{-45, "NW"}, // Negative wraps around
	}

	for _, tt := range tests {
		result := mapWindDirection(tt.degrees)
		if result != tt.expected {
			t.Errorf("mapWindDirection(%.0f) = %s, expected %s", tt.degrees, result, tt.expected)
		}
	}
}

func TestFindClosestHourIndex(t *testing.T) {
	times := []string{
		"2026-01-21T10:00",
		"2026-01-21T11:00",
		"2026-01-21T12:00",
		"2026-01-21T13:00",
	}

	tests := []struct {
		target   string
		expected int
	}{
		{"2026-01-21T10:15", 0}, // Closer to 10:00
		{"2026-01-21T10:45", 1}, // Closer to 11:00
		{"2026-01-21T12:30", 2}, // Exactly between, should pick 12:00
		{"2026-01-21T13:30", 3}, // Closer to 13:00
	}

	for _, tt := range tests {
		target, _ := time.Parse("2006-01-02T15:04", tt.target)
		result := findClosestHourIndex(times, target)
		if result != tt.expected {
			t.Errorf("findClosestHourIndex(%s) = %d, expected %d", tt.target, result, tt.expected)
		}
	}
}

func TestFindClosestHourIndex_EmptyArray(t *testing.T) {
	result := findClosestHourIndex([]string{}, time.Now())
	if result != -1 {
		t.Errorf("Expected -1 for empty array, got %d", result)
	}
}
