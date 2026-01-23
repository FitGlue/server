package heart_rate_summary

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type HeartRateSummary struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewHeartRateSummary())
}

func NewHeartRateSummary() *HeartRateSummary {
	return &HeartRateSummary{}
}

func (p *HeartRateSummary) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *HeartRateSummary) Name() string {
	return "heart-rate-summary"
}

func (p *HeartRateSummary) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_HEART_RATE_SUMMARY
}

func (p *HeartRateSummary) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Collect all heart rate values from the activity
	var heartRates []int32

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.HeartRate > 0 {
					heartRates = append(heartRates, record.HeartRate)
				}
			}
		}
	}

	if len(heartRates) == 0 {
		slog.Info("No heart rate data found for heart rate summary enricher")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"hr_summary_status": "skipped",
				"status_detail":     "No heart rate data found",
			},
		}, nil
	}

	// Calculate min, avg, max
	minHR, maxHR := heartRates[0], heartRates[0]
	var sumHR int64 = 0

	for _, hr := range heartRates {
		if hr < minHR {
			minHR = hr
		}
		if hr > maxHR {
			maxHR = hr
		}
		sumHR += int64(hr)
	}

	avgHR := float64(sumHR) / float64(len(heartRates))

	slog.Info("Heart rate summary calculated",
		"min_hr", minHR,
		"avg_hr", avgHR,
		"max_hr", maxHR,
		"sample_count", len(heartRates),
	)

	// Build the summary text to append to description (single line format like workout summary)
	summaryText := fmt.Sprintf("\n\n❤️ Heart Rate: %d bpm min • %.0f bpm avg • %d bpm max", minHR, avgHR, maxHR)

	// Append to existing description
	newDescription := activity.Description + summaryText

	return &providers.EnrichmentResult{
		Description: newDescription,
		Metadata: map[string]string{
			"hr_summary_status": "success",
			"hr_min":            fmt.Sprintf("%d", minHR),
			"hr_avg":            fmt.Sprintf("%.0f", avgHR),
			"hr_max":            fmt.Sprintf("%d", maxHR),
			"hr_sample_count":   fmt.Sprintf("%d", len(heartRates)),
		},
	}, nil
}
