package intervalsuploader

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestIsLoopOrigin(t *testing.T) {
	t.Run("should return false for event without origin metadata", func(t *testing.T) {
		event := &pb.EnrichedActivityEvent{}
		if isLoopOrigin(event) {
			t.Error("expected false for event without metadata")
		}
	})

	t.Run("should return true for event with intervals origin", func(t *testing.T) {
		event := &pb.EnrichedActivityEvent{
			EnrichmentMetadata: map[string]string{
				"origin_destination": "intervals",
			},
		}
		if !isLoopOrigin(event) {
			t.Error("expected true for event with intervals origin")
		}
	})

	t.Run("should return false for event with different origin", func(t *testing.T) {
		event := &pb.EnrichedActivityEvent{
			EnrichmentMetadata: map[string]string{
				"origin_destination": "strava",
			},
		}
		if isLoopOrigin(event) {
			t.Error("expected false for event with strava origin")
		}
	})
}

func TestUploadHandler(t *testing.T) {
	// Integration tests would require mocking HTTP client and service
	t.Skip("Integration tests require mock infrastructure")
}
