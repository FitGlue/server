package distance_milestones

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

// DistanceMilestones celebrates lifetime distance achievements.
type DistanceMilestones struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewDistanceMilestones())
}

func NewDistanceMilestones() *DistanceMilestones {
	return &DistanceMilestones{}
}

func (p *DistanceMilestones) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *DistanceMilestones) Name() string {
	return "distance-milestones"
}

func (p *DistanceMilestones) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_DISTANCE_MILESTONES
}

// Milestone thresholds in km
var milestones = []float64{100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000, 100000}

func (p *DistanceMilestones) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("distance_milestones: starting", "activity_name", activity.Name)

	// Parse config
	sport := inputs["sport"]
	if sport == "" {
		sport = "any"
	}

	// Get activity distance
	var distanceKm float64
	for _, session := range activity.Sessions {
		distanceKm += session.TotalDistance / 1000
	}

	if distanceKm == 0 {
		logger.Debug("distance_milestones: no distance in activity")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{"milestone_status": "no_distance"},
		}, nil
	}

	// Check sport filter
	if sport != "any" && !matchesSport(activity.Type, sport) {
		logger.Debug("distance_milestones: activity does not match sport filter", "sport", sport, "type", activity.Type)
		return &providers.EnrichmentResult{
			Metadata: map[string]string{"milestone_status": "filtered"},
		}, nil
	}

	// Fetch lifetime distance from booster_data
	var lifetimeDistance float64
	boosterId := fmt.Sprintf("distance_milestones_%s", sport)

	if p.Service != nil && p.Service.DB != nil {
		data, err := p.Service.DB.GetBoosterData(ctx, user.UserId, boosterId)
		if err != nil {
			logger.Warn("Failed to fetch lifetime distance", "error", err)
		} else if data != nil {
			if val, ok := data["lifetime_distance"].(float64); ok {
				lifetimeDistance = val
			}
		}
	}

	// Calculate new total
	previousDistance := lifetimeDistance
	newDistance := lifetimeDistance + distanceKm

	// Check for milestone crossings
	var crossedMilestones []float64
	for _, m := range milestones {
		if previousDistance < m && newDistance >= m {
			crossedMilestones = append(crossedMilestones, m)
		}
	}

	// Persist updated lifetime distance
	if p.Service != nil && p.Service.DB != nil {
		updateData := map[string]interface{}{
			"lifetime_distance": newDistance,
			"last_update":       time.Now().Format(time.RFC3339),
		}
		if err := p.Service.DB.SetBoosterData(ctx, user.UserId, boosterId, updateData); err != nil {
			logger.Warn("Failed to save lifetime distance", "error", err)
		}
	}

	// Build output
	var sb strings.Builder

	if len(crossedMilestones) > 0 {
		// Celebrate milestone!
		biggest := crossedMilestones[len(crossedMilestones)-1]
		emoji := getMilestoneEmoji(biggest)
		sb.WriteString(fmt.Sprintf("%s MILESTONE: %.0f km lifetime!\n", emoji, biggest))
		sb.WriteString(fmt.Sprintf("• Total: %.1f km\n", newDistance))
		sb.WriteString(fmt.Sprintf("• This %s: +%.1f km", getSportLabel(sport), distanceKm))

		logger.Info("Distance milestone reached",
			"milestone", biggest,
			"lifetime_distance", newDistance,
			"sport", sport,
		)

		return &providers.EnrichmentResult{
			Description: sb.String(),
			Metadata: map[string]string{
				"milestone_reached": fmt.Sprintf("%.0f", biggest),
				"lifetime_distance": fmt.Sprintf("%.1f", newDistance),
				"activity_distance": fmt.Sprintf("%.1f", distanceKm),
			},
		}, nil
	}

	// No milestone, just add to description
	nextMilestone := getNextMilestone(newDistance)
	remaining := nextMilestone - newDistance

	sb.WriteString(fmt.Sprintf("📊 Lifetime %s:\n", getSportLabel(sport)))
	sb.WriteString(fmt.Sprintf("• %.1f km total\n", newDistance))
	sb.WriteString(fmt.Sprintf("• Next milestone: %.0f km (%.1f km to go)", nextMilestone, remaining))

	logger.Info("Distance milestones processed",
		"lifetime_distance", newDistance,
		"next_milestone", nextMilestone,
		"sport", sport,
	)

	return &providers.EnrichmentResult{
		Description: sb.String(),
		Metadata: map[string]string{
			"lifetime_distance": fmt.Sprintf("%.1f", newDistance),
			"next_milestone":    fmt.Sprintf("%.0f", nextMilestone),
		},
	}, nil
}

func getMilestoneEmoji(km float64) string {
	switch {
	case km >= 10000:
		return "🏆🎉🏅"
	case km >= 5000:
		return "🏆🎉"
	case km >= 1000:
		return "🎉🏅"
	case km >= 500:
		return "🎉"
	default:
		return "✨"
	}
}

func getSportLabel(sport string) string {
	switch sport {
	case "running":
		return "running"
	case "cycling":
		return "cycling"
	case "swimming":
		return "swimming"
	default:
		return "distance"
	}
}

func getNextMilestone(current float64) float64 {
	for _, m := range milestones {
		if current < m {
			return m
		}
	}
	return milestones[len(milestones)-1] // Cap at highest
}

func matchesSport(actType pb.ActivityType, sport string) bool {
	switch sport {
	case "running":
		return actType == pb.ActivityType_ACTIVITY_TYPE_RUN ||
			actType == pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN ||
			actType == pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN
	case "cycling":
		return actType == pb.ActivityType_ACTIVITY_TYPE_RIDE ||
			actType == pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE ||
			actType == pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE ||
			actType == pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE
	case "swimming":
		return actType == pb.ActivityType_ACTIVITY_TYPE_SWIM
	default:
		return true
	}
}
