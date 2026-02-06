package streak_tracker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// StreakTracker tracks consecutive activity days/weeks.
// NOTE: Full persistence will be added in a future update using the user_data service.
// For now, it outputs a celebration message for each activity.
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

	// For now, we output a motivational message
	// Full streak persistence will be added via user_data service in a future update
	activityLabel := getActivityLabel(activityTypes, activity.Type)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔥 Keep the %s streak alive!", activityLabel))

	logger.Info("Streak tracker processed",
		"activity_types", activityTypes,
		"activity_type", activity.Type.String(),
	)

	return &providers.EnrichmentResult{
		Description: sb.String(),
		Metadata: map[string]string{
			"streak_status":        "success",
			"activity_type_filter": activityTypes,
		},
	}, nil
}

func getActivityLabel(filter string, actType pb.ActivityType) string {
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
		// Use actual activity type if "any"
		switch actType {
		case pb.ActivityType_ACTIVITY_TYPE_RUN, pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN:
			return "running"
		case pb.ActivityType_ACTIVITY_TYPE_RIDE, pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE:
			return "cycling"
		case pb.ActivityType_ACTIVITY_TYPE_SWIM:
			return "swimming"
		case pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING:
			return "strength"
		default:
			return "activity"
		}
	}
}
