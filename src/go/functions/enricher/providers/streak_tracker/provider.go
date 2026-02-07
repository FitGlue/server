package streak_tracker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// StreakTracker tracks consecutive activity days/weeks.
type StreakTracker struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewStreakTracker())
}

func NewStreakTracker() *StreakTracker {
	return &StreakTracker{}
}

func (p *StreakTracker) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *StreakTracker) Name() string {
	return "streak-tracker"
}

func (p *StreakTracker) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_STREAK_TRACKER
}

func (p *StreakTracker) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("streak_tracker: starting", "activity_name", activity.Name)

	// Parse config
	activityTypes := inputs["activity_types"]
	if activityTypes == "" {
		activityTypes = "any"
	}

	// Get activity date (use start time if available)
	activityDate := time.Now().Format("2006-01-02")
	if activity.StartTime != nil {
		activityDate = activity.StartTime.AsTime().Format("2006-01-02")
	}

	// Fetch streak data from booster_data
	var currentStreak int
	var longestStreak int
	var lastActivityDate string
	boosterId := fmt.Sprintf("streak_tracker_%s", activityTypes)

	if p.Service != nil && p.Service.DB != nil {
		data, err := p.Service.DB.GetBoosterData(ctx, user.UserId, boosterId)
		if err != nil {
			logger.Warn("Failed to fetch streak data", "error", err)
		} else if data != nil {
			if val, ok := data["current_streak"].(float64); ok {
				currentStreak = int(val)
			}
			if val, ok := data["longest_streak"].(float64); ok {
				longestStreak = int(val)
			}
			if val, ok := data["last_activity_date"].(string); ok {
				lastActivityDate = val
			}
		}
	}

	// Determine streak continuation
	streakBroken := false
	isNewDay := lastActivityDate != activityDate

	if isNewDay && lastActivityDate != "" {
		// Check if streak continues: last activity must be exactly the day before this activity
		actDate, _ := time.Parse("2006-01-02", activityDate)
		expectedPrev := actDate.AddDate(0, 0, -1).Format("2006-01-02")
		if lastActivityDate != expectedPrev {
			// Streak broken - reset
			streakBroken = true
			currentStreak = 0
		}
	}

	// Increment streak only if this is a new day
	if isNewDay {
		currentStreak++
	}

	// Update longest streak
	if currentStreak > longestStreak {
		longestStreak = currentStreak
	}

	// Persist updated streak
	if p.Service != nil && p.Service.DB != nil && isNewDay {
		updateData := map[string]interface{}{
			"current_streak":     currentStreak,
			"longest_streak":     longestStreak,
			"last_activity_date": activityDate,
			"last_update":        time.Now().Format(time.RFC3339),
		}
		if err := p.Service.DB.SetBoosterData(ctx, user.UserId, boosterId, updateData); err != nil {
			logger.Warn("Failed to save streak data", "error", err)
		}
	}

	// Build output
	activityLabel := getActivityLabel(activityTypes)
	var sb strings.Builder

	if streakBroken {
		sb.WriteString("🔥 Starting a new streak!\n")
		sb.WriteString(fmt.Sprintf("• Day 1 of your %s streak", activityLabel))
	} else if currentStreak == 1 {
		sb.WriteString(fmt.Sprintf("🔥 %s streak started!\n", capitalise(activityLabel)))
		sb.WriteString("• Day 1 - let's go!")
	} else {
		emoji := getStreakEmoji(currentStreak)
		sb.WriteString(fmt.Sprintf("%s %d-day %s streak!\n", emoji, currentStreak, activityLabel))

		if currentStreak == longestStreak && currentStreak > 1 {
			sb.WriteString("• 🏆 New personal best streak!")
		} else if longestStreak > currentStreak {
			sb.WriteString(fmt.Sprintf("• Best: %d days", longestStreak))
		}
	}

	logger.Info("Streak tracker processed",
		"activity_types", activityTypes,
		"current_streak", currentStreak,
		"longest_streak", longestStreak,
		"streak_broken", streakBroken,
	)

	return &providers.EnrichmentResult{
		Description: sb.String(),
		Metadata: map[string]string{
			"streak_current":       fmt.Sprintf("%d", currentStreak),
			"streak_longest":       fmt.Sprintf("%d", longestStreak),
			"activity_type_filter": activityTypes,
		},
	}, nil
}

func getStreakEmoji(streak int) string {
	switch {
	case streak >= 30:
		return "🏆🔥"
	case streak >= 14:
		return "💪🔥"
	case streak >= 7:
		return "🔥🔥"
	default:
		return "🔥"
	}
}

func getActivityLabel(filter string) string {
	switch filter {
	case "running":
		return "running"
	case "cycling":
		return "cycling"
	case "swimming":
		return "swimming"
	case "strength":
		return "strength training"
	default:
		return "activity"
	}
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
