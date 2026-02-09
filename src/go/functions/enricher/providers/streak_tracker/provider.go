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
			currentStreak = int(providers.ToFloat64(data["current_streak"]))
			longestStreak = int(providers.ToFloat64(data["longest_streak"]))
			if val, ok := data["last_activity_date"].(string); ok {
				lastActivityDate = val
			}
		}
	}

	// Determine streak continuation
	streakBroken := false
	isNewDay := lastActivityDate != activityDate

	if isNewDay && lastActivityDate != "" {
		actDate, _ := time.Parse("2006-01-02", activityDate)
		lastDate, _ := time.Parse("2006-01-02", lastActivityDate)

		// If this activity is from a date BEFORE or SAME as the last recorded date,
		// it's out-of-order or a race condition duplicate — don't modify the streak
		if !actDate.After(lastDate) {
			isNewDay = false
			logger.Info("streak_tracker: skipping out-of-order or duplicate activity",
				"activity_date", activityDate,
				"last_activity_date", lastActivityDate,
			)
		} else {
			// Check if streak continues: last activity must be exactly the day before this activity
			expectedPrev := actDate.AddDate(0, 0, -1).Format("2006-01-02")
			if lastActivityDate != expectedPrev {
				// Streak broken - reset
				streakBroken = true
				currentStreak = 0
			}
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
		}
		if err := p.Service.DB.SetBoosterData(ctx, user.UserId, boosterId, updateData); err != nil {
			logger.Warn("Failed to save streak data", "error", err)
		}
	}

	// Build output
	activityLabel := getActivityLabel(activityTypes)
	var sb strings.Builder
	sb.WriteString("🔥 Streak Tracker:\n")

	if streakBroken {
		sb.WriteString(fmt.Sprintf("• Day 1 of your %s streak - starting fresh!", activityLabel))
	} else if currentStreak == 1 {
		sb.WriteString(fmt.Sprintf("• %s streak started - let's go!", capitalise(activityLabel)))
	} else {
		emoji := getStreakEmoji(currentStreak)
		sb.WriteString(fmt.Sprintf("• %s %d-day %s streak!", emoji, currentStreak, activityLabel))

		if currentStreak == longestStreak && currentStreak > 1 {
			sb.WriteString("\n• 🏆 New personal best streak!")
		} else if longestStreak > currentStreak {
			sb.WriteString(fmt.Sprintf("\n• Best: %d days", longestStreak))
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
