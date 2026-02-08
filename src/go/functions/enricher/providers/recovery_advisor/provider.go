package recovery_advisor

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

// RecoveryAdvisor calculates training load and suggests recovery time.
// Uses TRIMP (Training Impulse) to estimate training stress.
type RecoveryAdvisor struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewRecoveryAdvisor())
}

func NewRecoveryAdvisor() *RecoveryAdvisor {
	return &RecoveryAdvisor{}
}

func (p *RecoveryAdvisor) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *RecoveryAdvisor) Name() string {
	return "recovery-advisor"
}

func (p *RecoveryAdvisor) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_RECOVERY_ADVISOR
}

func (p *RecoveryAdvisor) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("recovery_advisor: starting", "activity_name", activity.Name)

	// Get activity duration and HR data
	var durationMinutes float64
	var avgHR float64
	var hrSamples int

	for _, session := range activity.Sessions {
		durationMinutes += session.TotalElapsedTime / 60
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.HeartRate > 0 {
					avgHR += float64(record.HeartRate)
					hrSamples++
				}
			}
		}
	}

	if hrSamples > 0 {
		avgHR = avgHR / float64(hrSamples)
	}

	// Calculate TRIMP (simplified Banister method)
	// TRIMP = duration × avg HR fraction × weighting factor
	maxHR := 190.0 // Could be configurable
	restHR := 60.0
	var trimp float64

	if avgHR > 0 {
		hrReserve := (avgHR - restHR) / (maxHR - restHR)
		if hrReserve < 0 {
			hrReserve = 0
		}
		if hrReserve > 1 {
			hrReserve = 1
		}
		// Exponential weighting for intensity
		trimp = durationMinutes * hrReserve * math.Pow(1.92, hrReserve)
	} else {
		// Estimate from duration only (less accurate)
		trimp = durationMinutes * 0.5
	}

	// Fetch 7-day training load history
	var weeklyLoad float64
	boosterId := "recovery_advisor"
	now := time.Now()
	var data map[string]interface{}

	if p.Service != nil && p.Service.DB != nil {
		var err error
		data, err = p.Service.DB.GetBoosterData(ctx, user.UserId, boosterId)
		if err != nil {
			logger.Warn("Failed to fetch recovery data", "error", err)
		} else if data != nil {
			// Get previous 7 days of load (excluding today, which will be added fresh)
			for i := 1; i <= 7; i++ {
				dateKey := now.AddDate(0, 0, -i).Format("2006-01-02")
				if val, ok := data[dateKey].(float64); ok {
					weeklyLoad += val
				}
			}
		}
	}

	// Add today's load, accumulating with any previously stored TRIMP for today
	// This handles users doing multiple activities in a single day
	today := now.Format("2006-01-02")
	todayLoad := trimp
	if data != nil {
		if existingToday, ok := data[today].(float64); ok {
			todayLoad += existingToday
		}
	}
	totalWeeklyLoad := weeklyLoad + todayLoad

	// Persist today's load
	if p.Service != nil && p.Service.DB != nil {
		updateData := map[string]interface{}{
			today:         todayLoad,
			"last_update": now.Format(time.RFC3339),
		}
		if err := p.Service.DB.SetBoosterData(ctx, user.UserId, boosterId, updateData); err != nil {
			logger.Warn("Failed to save recovery data", "error", err)
		}
	}

	// Calculate recovery recommendation
	recoveryHours, intensity := getRecoveryRecommendation(trimp, totalWeeklyLoad)

	// Build output
	var sb strings.Builder

	sb.WriteString("💤 Recovery Advisor:\n")
	sb.WriteString(fmt.Sprintf("• Session load: %.0f TRIMP (%s)\n", trimp, intensity))
	sb.WriteString(fmt.Sprintf("• 7-day load: %.0f TRIMP\n", totalWeeklyLoad))
	sb.WriteString(fmt.Sprintf("• 💡 Suggested recovery: %s", formatRecoveryTime(recoveryHours)))

	logger.Info("Recovery advisor processed",
		"trimp", trimp,
		"weekly_load", totalWeeklyLoad,
		"recovery_hours", recoveryHours,
		"intensity", intensity,
	)

	return &providers.EnrichmentResult{
		Description: sb.String(),
		Metadata: map[string]string{
			"trimp":          fmt.Sprintf("%.0f", trimp),
			"weekly_load":    fmt.Sprintf("%.0f", totalWeeklyLoad),
			"recovery_hours": fmt.Sprintf("%.0f", recoveryHours),
			"intensity":      intensity,
		},
	}, nil
}

func getRecoveryRecommendation(trimp, weeklyLoad float64) (hours float64, intensity string) {
	// Base recovery on session intensity
	switch {
	case trimp >= 150:
		intensity = "Very Hard"
		hours = 48
	case trimp >= 100:
		intensity = "Hard"
		hours = 36
	case trimp >= 50:
		intensity = "Moderate"
		hours = 24
	default:
		intensity = "Easy"
		hours = 12
	}

	// Adjust based on weekly load
	if weeklyLoad > 500 {
		hours += 12 // Extra recovery if high weekly load
	}

	return hours, intensity
}

func formatRecoveryTime(hours float64) string {
	if hours >= 48 {
		return fmt.Sprintf("%.0f hours (2 days)", hours)
	} else if hours >= 24 {
		return fmt.Sprintf("%.0f hours (1 day)", hours)
	}
	return fmt.Sprintf("%.0f hours", hours)
}
