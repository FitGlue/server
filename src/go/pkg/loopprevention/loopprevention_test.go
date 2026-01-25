package loopprevention

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestGetCorrespondingDestination(t *testing.T) {
	tests := []struct {
		name     string
		source   pb.ActivitySource
		expected pb.Destination
	}{
		{
			name:     "Hevy source maps to Hevy destination",
			source:   pb.ActivitySource_SOURCE_HEVY,
			expected: pb.Destination_DESTINATION_HEVY,
		},
		{
			name:     "Strava source maps to Strava destination",
			source:   pb.ActivitySource_SOURCE_STRAVA,
			expected: pb.Destination_DESTINATION_STRAVA,
		},
		{
			name:     "File upload has no destination",
			source:   pb.ActivitySource_SOURCE_FILE_UPLOAD,
			expected: pb.Destination_DESTINATION_UNSPECIFIED,
		},
		{
			name:     "Parkrun results has no destination",
			source:   pb.ActivitySource_SOURCE_PARKRUN_RESULTS,
			expected: pb.Destination_DESTINATION_UNSPECIFIED,
		},
		{
			name:     "Unknown source has no destination",
			source:   pb.ActivitySource_SOURCE_UNKNOWN,
			expected: pb.Destination_DESTINATION_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCorrespondingDestination(tt.source)
			if result != tt.expected {
				t.Errorf("GetCorrespondingDestination(%v) = %v, want %v", tt.source, result, tt.expected)
			}
		})
	}
}

func TestBuildUploadedActivityID(t *testing.T) {
	tests := []struct {
		name          string
		destination   pb.Destination
		destinationId string
		expected      string
	}{
		{
			name:          "Hevy destination ID",
			destination:   pb.Destination_DESTINATION_HEVY,
			destinationId: "abc123",
			expected:      "hevy:abc123",
		},
		{
			name:          "Strava destination ID",
			destination:   pb.Destination_DESTINATION_STRAVA,
			destinationId: "12345678",
			expected:      "strava:12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildUploadedActivityID(tt.destination, tt.destinationId)
			if result != tt.expected {
				t.Errorf("BuildUploadedActivityID(%v, %s) = %s, want %s", tt.destination, tt.destinationId, result, tt.expected)
			}
		})
	}
}
