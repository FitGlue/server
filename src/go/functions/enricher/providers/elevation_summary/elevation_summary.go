package elevation_summary

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type ElevationSummary struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewElevationSummary())
}

func NewElevationSummary() *ElevationSummary {
	return &ElevationSummary{}
}

func (p *ElevationSummary) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *ElevationSummary) Name() string {
	return "elevation-summary"
}

func (p *ElevationSummary) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_ELEVATION_SUMMARY
}

func (p *ElevationSummary) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("elevation_summary: starting", "activity_name", activity.Name)
	var gain float64
	var loss float64
	var maxAltitude float64
	var previousAltitude float64
	var hasPrevious bool
	var recordCount int

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.Altitude > 0 {
					if record.Altitude > maxAltitude {
						maxAltitude = record.Altitude
					}

					if hasPrevious {
						diff := record.Altitude - previousAltitude
						if diff > 0 {
							gain += diff
						} else if diff < 0 {
							loss += math.Abs(diff)
						}
					}

					previousAltitude = record.Altitude
					hasPrevious = true
					recordCount++
				}
			}
		}
	}

	if recordCount == 0 {
		logger.Info("No elevation data found for elevation summary enricher")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"elevation_summary_status": "skipped",
				"status_detail":            "No altitude data found",
			},
		}, nil
	}

	logger.Info("Elevation summary calculated",
		"gain", gain,
		"loss", loss,
		"max_altitude", maxAltitude,
		"record_count", recordCount,
	)

	// Build the summary text
	// Output Format: "⛰️ Elevation: +342m gain • -289m loss • 1,245m max"
	summaryText := fmt.Sprintf("⛰️ Elevation: +%.0fm gain • -%.0fm loss • %.0fm max", gain, loss, maxAltitude)

	// Append to existing description
	newDescription := summaryText

	return &providers.EnrichmentResult{
		Description: newDescription,
		Metadata: map[string]string{
			"elevation_summary_status": "success",
			"elevation_gain":           fmt.Sprintf("%.2f", gain),
			"elevation_loss":           fmt.Sprintf("%.2f", loss),
			"elevation_max":            fmt.Sprintf("%.2f", maxAltitude),
			"elevation_record_count":   fmt.Sprintf("%d", recordCount),
		},
	}, nil
}
