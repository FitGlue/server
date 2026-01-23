package speed_summary

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// SpeedSummary calculates and appends speed statistics (km/h) to the activity description.
// Shows avg/max speed from GPS or sensor data.
type SpeedSummary struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewSpeedSummary())
}

func NewSpeedSummary() *SpeedSummary {
	return &SpeedSummary{}
}

func (p *SpeedSummary) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *SpeedSummary) Name() string {
	return "speed-summary"
}

func (p *SpeedSummary) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_SPEED_SUMMARY
}

func (p *SpeedSummary) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Collect all speed values from the activity (m/s)
	var speeds []float64

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.Speed > 0 {
					speeds = append(speeds, record.Speed)
				}
			}
		}
	}

	if len(speeds) == 0 {
		slog.Info("No speed data found for speed summary enricher")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"speed_summary_status": "skipped",
				"status_detail":        "No speed data found",
			},
		}, nil
	}

	// Calculate avg and max speed
	var sumSpeed float64
	var maxSpeed float64 = speeds[0]

	for _, speed := range speeds {
		sumSpeed += speed
		if speed > maxSpeed {
			maxSpeed = speed
		}
	}

	avgSpeed := sumSpeed / float64(len(speeds))

	// Convert m/s to km/h (* 3.6)
	avgSpeedKmh := avgSpeed * 3.6
	maxSpeedKmh := maxSpeed * 3.6

	slog.Info("Speed summary calculated",
		"avg_speed_kmh", avgSpeedKmh,
		"max_speed_kmh", maxSpeedKmh,
		"sample_count", len(speeds),
	)

	// Build the summary text to append to description
	summaryText := fmt.Sprintf("\n\n🚀 Speed: %.1f km/h avg • %.1f km/h max", avgSpeedKmh, maxSpeedKmh)

	// Append to existing description
	newDescription := activity.Description + summaryText

	return &providers.EnrichmentResult{
		Description: newDescription,
		Metadata: map[string]string{
			"speed_summary_status": "success",
			"speed_avg_kmh":        fmt.Sprintf("%.1f", avgSpeedKmh),
			"speed_max_kmh":        fmt.Sprintf("%.1f", maxSpeedKmh),
			"speed_sample_count":   fmt.Sprintf("%d", len(speeds)),
		},
	}, nil
}
