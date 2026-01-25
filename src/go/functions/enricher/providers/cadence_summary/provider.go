package cadence_summary

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// CadenceSummary calculates and appends cadence statistics to the activity description.
// Shows avg/max cadence in rpm (revolutions/steps per minute).
type CadenceSummary struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewCadenceSummary())
}

func NewCadenceSummary() *CadenceSummary {
	return &CadenceSummary{}
}

func (p *CadenceSummary) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *CadenceSummary) Name() string {
	return "cadence-summary"
}

func (p *CadenceSummary) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_CADENCE_SUMMARY
}

func (p *CadenceSummary) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("cadence_summary: starting", "activity_name", activity.Name)
	// Collect all cadence values from the activity
	var cadences []int32

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.Cadence > 0 {
					cadences = append(cadences, record.Cadence)
				}
			}
		}
	}

	if len(cadences) == 0 {
		logger.Info("No cadence data found for cadence summary enricher")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"cadence_summary_status": "skipped",
				"status_detail":          "No cadence data found",
			},
		}, nil
	}

	// Calculate avg and max cadence
	var sumCadence int64
	var maxCadence int32 = cadences[0]

	for _, cadence := range cadences {
		sumCadence += int64(cadence)
		if cadence > maxCadence {
			maxCadence = cadence
		}
	}

	avgCadence := float64(sumCadence) / float64(len(cadences))

	// Determine unit based on activity type (spm for running, rpm for cycling)
	unit := "rpm"
	if isRunningActivity(activity.Type) {
		unit = "spm"
	}

	logger.Info("Cadence summary calculated",
		"avg_cadence", avgCadence,
		"max_cadence", maxCadence,
		"sample_count", len(cadences),
	)

	// Build the summary text to append to description
	summaryText := fmt.Sprintf("🦶 Cadence: %.0f %s avg • %d %s max", avgCadence, unit, maxCadence, unit)

	// Append to existing description
	newDescription := summaryText

	return &providers.EnrichmentResult{
		Description: newDescription,
		Metadata: map[string]string{
			"cadence_summary_status": "success",
			"cadence_avg":            fmt.Sprintf("%.0f", avgCadence),
			"cadence_max":            fmt.Sprintf("%d", maxCadence),
			"cadence_sample_count":   fmt.Sprintf("%d", len(cadences)),
		},
	}, nil
}

// isRunningActivity returns true if the activity type is a running/walking activity
func isRunningActivity(activityType pb.ActivityType) bool {
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RUN,
		pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN,
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN,
		pb.ActivityType_ACTIVITY_TYPE_WALK,
		pb.ActivityType_ACTIVITY_TYPE_HIKE:
		return true
	default:
		return false
	}
}
