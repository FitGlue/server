package goal_tracker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// GoalTracker tracks progress toward configurable goals.
// NOTE: Full persistence will be added in a future update using the user_data service.
// For now, it outputs progress from this activity toward the goal.
type GoalTracker struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewGoalTracker())
}

func NewGoalTracker() *GoalTracker {
	return &GoalTracker{}
}

func (p *GoalTracker) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *GoalTracker) Name() string {
	return "goal-tracker"
}

func (p *GoalTracker) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_GOAL_TRACKER
}

func (p *GoalTracker) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("goal_tracker: starting", "activity_name", activity.Name)

	// Parse config
	period := inputs["period"]
	if period == "" {
		period = "month"
	}
	metric := inputs["metric"]
	if metric == "" {
		metric = "distance"
	}
	target, _ := strconv.ParseFloat(inputs["target"], 64)
	if target <= 0 {
		target = 100 // Default 100km goal
	}

	// Get current metric value from this activity
	activityValue := getMetricValue(activity, metric)

	// Fetch accumulated progress from booster_data
	var accumulatedProgress float64
	var currentPeriod string
	boosterId := fmt.Sprintf("goal_tracker_%s_%s", period, metric)

	if p.Service != nil && p.Service.DB != nil {
		data, err := p.Service.DB.GetBoosterData(ctx, user.UserId, boosterId)
		if err != nil {
			logger.Warn("Failed to fetch goal progress", "error", err)
		} else if data != nil {
			// Check if data is from current period
			if storedPeriod, ok := data["period_key"].(string); ok {
				currentPeriod = getPeriodKey(period)
				if storedPeriod == currentPeriod {
					if val, ok := data["accumulated"].(float64); ok {
						accumulatedProgress = val
					}
				}
				// If period changed, reset (new week/month/year)
			}
		}
	}

	// Calculate new total
	newTotal := accumulatedProgress + activityValue
	percentage := (newTotal / target) * 100
	if percentage > 100 {
		percentage = 100
	}

	// Persist updated progress
	if p.Service != nil && p.Service.DB != nil && activityValue > 0 {
		if currentPeriod == "" {
			currentPeriod = getPeriodKey(period)
		}
		updateData := map[string]interface{}{
			"accumulated": newTotal,
			"period_key":  currentPeriod,
			"last_update": time.Now().Format(time.RFC3339),
		}
		if err := p.Service.DB.SetBoosterData(ctx, user.UserId, boosterId, updateData); err != nil {
			logger.Warn("Failed to save goal progress", "error", err)
		}
	}

	// Build output
	var sb strings.Builder
	periodLabel := getPeriodLabel(period)
	metricLabel := getMetricLabel(metric)

	// Progress bar
	progressBar := buildProgressBar(percentage)

	sb.WriteString(fmt.Sprintf("🎯 %s Goal Progress\n", periodLabel))
	sb.WriteString(fmt.Sprintf("• %s %.1f/%.0f %s\n", progressBar, newTotal, target, metricLabel))
	sb.WriteString(fmt.Sprintf("• ➕ This activity: +%.1f %s", activityValue, metricLabel))

	// Show remaining if not complete
	if newTotal < target {
		remaining := target - newTotal
		daysRemaining := getDaysRemaining(period)
		if daysRemaining > 0 {
			neededPerDay := remaining / float64(daysRemaining)
			sb.WriteString(fmt.Sprintf("\n• 💡 Need %.1f %s/day to hit goal", neededPerDay, metricLabel))
		}
	} else {
		sb.WriteString("\n• 🏆 Goal complete!")
	}

	logger.Info("Goal tracker processed",
		"period", period,
		"metric", metric,
		"activity_value", activityValue,
		"accumulated", newTotal,
		"target", target,
		"percentage", percentage,
	)

	return &providers.EnrichmentResult{
		Description: sb.String(),
		Metadata: map[string]string{
			"goal_status":      fmt.Sprintf("%.0f%%", percentage),
			"goal_accumulated": fmt.Sprintf("%.2f", newTotal),
			"goal_target":      fmt.Sprintf("%.0f", target),
			"goal_period":      period,
			"goal_metric":      metric,
		},
	}, nil
}

func buildProgressBar(percentage float64) string {
	filled := int(percentage / 10)
	empty := 10 - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s] %.0f%%", bar, percentage)
}

func getMetricValue(activity *pb.StandardizedActivity, metric string) float64 {
	var total float64
	for _, session := range activity.Sessions {
		switch metric {
		case "distance":
			total += session.TotalDistance / 1000 // Convert to km
		case "duration":
			total += session.TotalElapsedTime / 3600 // Convert to hours
		case "activities":
			total = 1
		}
	}
	return total
}

func getPeriodLabel(period string) string {
	now := time.Now()
	switch period {
	case "week":
		return "Weekly"
	case "year":
		return fmt.Sprintf("%d", now.Year())
	default:
		return now.Format("January")
	}
}

// getPeriodKey returns a unique key for the current period to track resets
func getPeriodKey(period string) string {
	now := time.Now()
	switch period {
	case "week":
		year, week := now.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "year":
		return fmt.Sprintf("%d", now.Year())
	default: // month
		return now.Format("2006-01")
	}
}

func getMetricLabel(metric string) string {
	switch metric {
	case "duration":
		return "hours"
	case "activities":
		return "activities"
	case "elevation":
		return "m elevation"
	default:
		return "km"
	}
}

func getDaysRemaining(period string) int {
	now := time.Now()
	switch period {
	case "week":
		return 7 - int(now.Weekday())
	case "year":
		endOfYear := time.Date(now.Year(), 12, 31, 0, 0, 0, 0, now.Location())
		return int(endOfYear.Sub(now).Hours() / 24)
	default: // month
		endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())
		return endOfMonth.Day() - now.Day()
	}
}
