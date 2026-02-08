package pace_summary

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// PaceSummary calculates and appends pace statistics (min/km) to the activity description.
// Uses speed (m/s) data from records, converts to pace, and shows avg/best pace.
// Enhanced features: splits, negative split detection, fatigue analysis.
type PaceSummary struct {
	Service *bootstrap.Service
}

// Split represents a single km/mile split
type Split struct {
	Distance float64       // in meters
	Duration time.Duration // time for this split
	Pace     float64       // min/km
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

func (p *PaceSummary) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("pace_summary: starting", "activity_name", activity.Name)

	// Parse config options
	showSplits := inputs["show_splits"] == "true"
	showNegativeSplit := inputs["negative_split_alert"] == "true"
	showFatigue := inputs["show_fatigue"] == "true"

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
		logger.Info("No speed data found for pace summary enricher")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"pace_summary_status": "skipped",
				"status_detail":       "No speed data found",
			},
		}, nil
	}

	// Calculate avg and best (fastest) pace
	var sumSpeed float64
	var maxSpeed float64 = speeds[0]

	for _, speed := range speeds {
		sumSpeed += speed
		if speed > maxSpeed {
			maxSpeed = speed
		}
	}

	avgSpeed := sumSpeed / float64(len(speeds))
	avgPace := 1000.0 / avgSpeed / 60.0 // minutes per km
	bestPace := 1000.0 / maxSpeed / 60.0

	logger.Info("Pace summary calculated",
		"avg_pace_min_km", avgPace,
		"best_pace_min_km", bestPace,
		"sample_count", len(speeds),
	)

	avgPaceStr := formatPace(avgPace)
	bestPaceStr := formatPace(bestPace)

	// Build the summary text
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚡ Pace: %s/km avg • %s/km best", avgPaceStr, bestPaceStr))

	// Calculate splits if requested
	var splits []Split
	if showSplits || showNegativeSplit || showFatigue {
		splits = calculateSplitsFromLaps(activity)
	}

	// Show splits
	if showSplits && len(splits) > 0 {
		sb.WriteString("\n📊 Splits:")
		fastestIdx, slowestIdx := 0, 0
		for i, split := range splits {
			if split.Pace < splits[fastestIdx].Pace {
				fastestIdx = i
			}
			if split.Pace > splits[slowestIdx].Pace {
				slowestIdx = i
			}
		}
		for i, split := range splits {
			marker := ""
			if i == fastestIdx {
				marker = " 🏆"
			} else if i == slowestIdx {
				marker = " 🐢"
			}
			sb.WriteString(fmt.Sprintf("\n• Km %d: %s%s", i+1, formatPace(split.Pace), marker))
		}
	}

	// Negative split detection
	if showNegativeSplit && len(splits) >= 2 {
		midpoint := len(splits) / 2
		var firstHalfPace, secondHalfPace float64
		for i := 0; i < midpoint; i++ {
			firstHalfPace += splits[i].Pace
		}
		for i := midpoint; i < len(splits); i++ {
			secondHalfPace += splits[i].Pace
		}
		firstHalfPace /= float64(midpoint)
		secondHalfPace /= float64(len(splits) - midpoint)

		if secondHalfPace < firstHalfPace {
			diff := firstHalfPace - secondHalfPace
			diffSeconds := int(diff * 60)
			sb.WriteString(fmt.Sprintf("\n🔥 Negative Split! Second half %ds/km faster", diffSeconds))
		}
	}

	// Fatigue analysis
	if showFatigue && len(splits) >= 4 {
		quarter := len(splits) / 4
		var firstQuarterPace, lastQuarterPace float64
		for i := 0; i < quarter; i++ {
			firstQuarterPace += splits[i].Pace
		}
		for i := len(splits) - quarter; i < len(splits); i++ {
			lastQuarterPace += splits[i].Pace
		}
		firstQuarterPace /= float64(quarter)
		lastQuarterPace /= float64(quarter)

		if lastQuarterPace > firstQuarterPace {
			fatiguePercent := ((lastQuarterPace - firstQuarterPace) / firstQuarterPace) * 100
			if fatiguePercent > 1 { // Only show if significant
				sb.WriteString(fmt.Sprintf("\n😓 Fatigue: %.0f%% pace drop in final quarter", fatiguePercent))
			}
		} else {
			// Strong finish!
			sb.WriteString("\n💪 Strong Finish: Final quarter faster than start")
		}
	}

	metadata := map[string]string{
		"pace_summary_status": "success",
		"pace_avg":            avgPaceStr,
		"pace_best":           bestPaceStr,
		"pace_sample_count":   fmt.Sprintf("%d", len(speeds)),
	}

	// Add split data to metadata
	if len(splits) > 0 {
		metadata["splits_count"] = fmt.Sprintf("%d", len(splits))
	}

	return &providers.EnrichmentResult{
		Description: sb.String(),
		Metadata:    metadata,
	}, nil
}

// calculateSplitsFromLaps attempts to derive km splits from lap data
func calculateSplitsFromLaps(activity *pb.StandardizedActivity) []Split {
	var splits []Split

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			// Each lap with distance >= 900m is roughly a km split
			if lap.TotalDistance >= 900 && lap.TotalDistance <= 1100 {
				duration := time.Duration(lap.TotalElapsedTime * float64(time.Second))
				pace := (lap.TotalElapsedTime / lap.TotalDistance) * 1000 / 60 // min/km
				splits = append(splits, Split{
					Distance: lap.TotalDistance,
					Duration: duration,
					Pace:     pace,
				})
			} else if lap.TotalDistance > 1100 {
				// Longer lap - estimate number of km splits within
				numKm := int(math.Floor(lap.TotalDistance / 1000))
				if numKm > 0 {
					avgPace := (lap.TotalElapsedTime / lap.TotalDistance) * 1000 / 60
					for i := 0; i < numKm; i++ {
						splits = append(splits, Split{
							Distance: 1000,
							Duration: time.Duration(lap.TotalElapsedTime/float64(numKm)) * time.Second,
							Pace:     avgPace,
						})
					}
				}
			}
		}
	}

	return splits
}

// formatPace converts pace in minutes (float) to MM:SS format
func formatPace(paceMinutes float64) string {
	minutes := int(paceMinutes)
	seconds := int((paceMinutes - float64(minutes)) * 60)
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
