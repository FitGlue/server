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

	// Build output
	var sb strings.Builder
	periodLabel := getPeriodLabel(period)
	metricLabel := getMetricLabel(metric)

	// Show activity contribution toward goal
	sb.WriteString(fmt.Sprintf("📅 %s Goal: %.0f %s", periodLabel, target, metricLabel))
	sb.WriteString(fmt.Sprintf("\n➕ This activity: +%.1f %s", activityValue, metricLabel))

	// Calculate days remaining
	daysRemaining := getDaysRemaining(period)
	if daysRemaining > 0 {
		avgPerDay := target / float64(getDaysInPeriod(period))
		sb.WriteString(fmt.Sprintf("\n💡 Target: %.1f %s/day", avgPerDay, metricLabel))
	}

	logger.Info("Goal tracker processed",
		"period", period,
		"metric", metric,
		"activity_value", activityValue,
		"target", target,
	)

	return &providers.EnrichmentResult{
		Description: sb.String(),
		Metadata: map[string]string{
			"goal_status":         "success",
			"goal_activity_value": fmt.Sprintf("%.2f", activityValue),
			"goal_target":         fmt.Sprintf("%.0f", target),
			"goal_period":         period,
			"goal_metric":         metric,
		},
	}, nil
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

func getDaysInPeriod(period string) int {
	now := time.Now()
	switch period {
	case "week":
		return 7
	case "year":
		return 365
	default: // month
		endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())
		return endOfMonth.Day()
	}
}
