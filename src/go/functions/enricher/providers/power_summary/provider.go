package power_summary

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/functions/enricher/providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// PowerSummary calculates and appends power statistics (watts) to the activity description.
// Shows avg/max power from power meter data.
type PowerSummary struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewPowerSummary())
}

func NewPowerSummary() *PowerSummary {
	return &PowerSummary{}
}

func (p *PowerSummary) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *PowerSummary) Name() string {
	return "power-summary"
}

func (p *PowerSummary) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_POWER_SUMMARY
}

func (p *PowerSummary) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Collect all power values from the activity (watts)
	var powers []int32

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.Power > 0 {
					powers = append(powers, record.Power)
				}
			}
		}
	}

	if len(powers) == 0 {
		slog.Info("No power data found for power summary enricher")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"power_summary_status": "skipped",
				"status_detail":        "No power data found",
			},
		}, nil
	}

	// Calculate avg and max power
	var sumPower int64
	var maxPower int32 = powers[0]

	for _, power := range powers {
		sumPower += int64(power)
		if power > maxPower {
			maxPower = power
		}
	}

	avgPower := float64(sumPower) / float64(len(powers))

	slog.Info("Power summary calculated",
		"avg_power", avgPower,
		"max_power", maxPower,
		"sample_count", len(powers),
	)

	// Build the summary text to append to description
	summaryText := fmt.Sprintf("\n\n⚡ Power: %.0fW avg • %dW max", avgPower, maxPower)

	// Append to existing description
	newDescription := activity.Description + summaryText

	return &providers.EnrichmentResult{
		Description: newDescription,
		Metadata: map[string]string{
			"power_summary_status": "success",
			"power_avg":            fmt.Sprintf("%.0f", avgPower),
			"power_max":            fmt.Sprintf("%d", maxPower),
			"power_sample_count":   fmt.Sprintf("%d", len(powers)),
		},
	}, nil
}
