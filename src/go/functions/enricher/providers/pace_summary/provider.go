package pace_summary

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// PaceSummary calculates and appends pace statistics (min/km) to the activity description.
// Uses speed (m/s) data from records, converts to pace, and shows avg/best pace.
type PaceSummary struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewPaceSummary())
}

func NewPaceSummary() *PaceSummary {
	return &PaceSummary{}
}

func (p *PaceSummary) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *PaceSummary) Name() string {
	return "pace-summary"
}

func (p *PaceSummary) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_PACE_SUMMARY
}

func (p *PaceSummary) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
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
		slog.Info("No speed data found for pace summary enricher")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"pace_summary_status": "skipped",
				"status_detail":       "No speed data found",
			},
		}, nil
	}

	// Calculate avg and best (fastest) pace
	// Pace = time per km = 1000 / speed_m_s / 60 (minutes per km)
	var sumSpeed float64
	var maxSpeed float64 = speeds[0]

	for _, speed := range speeds {
		sumSpeed += speed
		if speed > maxSpeed {
			maxSpeed = speed
		}
	}

	avgSpeed := sumSpeed / float64(len(speeds))

	// Convert to pace (min/km)
	avgPace := 1000.0 / avgSpeed / 60.0 // minutes per km
	bestPace := 1000.0 / maxSpeed / 60.0

	slog.Info("Pace summary calculated",
		"avg_pace_min_km", avgPace,
		"best_pace_min_km", bestPace,
		"sample_count", len(speeds),
	)

	// Format pace as MM:SS
	avgPaceStr := formatPace(avgPace)
	bestPaceStr := formatPace(bestPace)

	// Build the summary text to append to description
	summaryText := fmt.Sprintf("⚡ Pace: %s/km avg • %s/km best", avgPaceStr, bestPaceStr)

	// Append to existing description
	newDescription := summaryText

	return &providers.EnrichmentResult{
		Description: newDescription,
		Metadata: map[string]string{
			"pace_summary_status": "success",
			"pace_avg":            avgPaceStr,
			"pace_best":           bestPaceStr,
			"pace_sample_count":   fmt.Sprintf("%d", len(speeds)),
		},
	}, nil
}

// formatPace converts pace in minutes (float) to MM:SS format
func formatPace(paceMinutes float64) string {
	minutes := int(paceMinutes)
	seconds := int((paceMinutes - float64(minutes)) * 60)
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
