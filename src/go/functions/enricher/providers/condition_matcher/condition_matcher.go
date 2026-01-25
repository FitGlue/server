package condition_matcher

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/domain/activity"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// ConditionMatcherProvider applies enrichments based on a set of conditions.
type ConditionMatcherProvider struct{}

func init() {
	providers.Register(NewConditionMatcherProvider())
}

func NewConditionMatcherProvider() *ConditionMatcherProvider {
	return &ConditionMatcherProvider{}
}

func (p *ConditionMatcherProvider) Name() string {
	return "condition_matcher"
}

func (p *ConditionMatcherProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_CONDITION_MATCHER
}

func (p *ConditionMatcherProvider) Enrich(ctx context.Context, logger *slog.Logger, act *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("condition_matcher: starting",
		"activity_type", act.Type.String(),
		"activity_name", act.Name,
		"has_activity_type_condition", inputs["activity_type"] != "",
		"has_days_condition", inputs["days"] != "" || inputs["days_of_week"] != "",
		"has_time_condition", inputs["time_start"] != "" || inputs["time_end"] != "",
		"has_location_condition", inputs["location_lat"] != "",
	)

	// Helper to handle mismatch
	returnMismatch := func(reason string) (*providers.EnrichmentResult, error) {
		logger.Debug("condition_matcher: condition not met",
			"reason", reason,
		)
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"condition_matcher_applied": "false",
				"condition_fail_reason":     reason,
			},
		}, nil
	}

	// 1. Check Conditions (AND logic)

	// A. Activity Type
	if val, ok := inputs["activity_type"]; ok && val != "" {
		expectedType := activity.ParseActivityTypeFromString(val)
		if expectedType != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED && act.Type != expectedType {
			return returnMismatch(fmt.Sprintf("Activity Type mismatch. Expected %v, got %v", expectedType, act.Type))
		}
		logger.Debug("condition_matcher: activity_type check passed",
			"expected", expectedType.String(),
			"actual", act.Type.String(),
		)
	}

	// B. Days of Week
	startTime := act.StartTime.AsTime()
	daysVal, hasDays := inputs["days"]
	if !hasDays {
		daysVal = inputs["days_of_week"]
	}
	if daysVal != "" {
		currentDay := startTime.Weekday().String()[:3] // "Mon"
		currentDayInt := int(startTime.Weekday())      // 0-6 (Sun-Sat)
		match := false
		for _, dayStr := range strings.Split(daysVal, ",") {
			val := strings.TrimSpace(dayStr)
			if val == currentDay {
				match = true
				break
			}
			if valInt, err := strconv.Atoi(val); err == nil {
				if valInt == currentDayInt {
					match = true
					break
				}
			}
		}
		if !match {
			return returnMismatch(fmt.Sprintf("Day mismatch. Expected one of [%s], got %s (%d)", daysVal, currentDay, currentDayInt))
		}
		logger.Debug("condition_matcher: days check passed",
			"allowed_days", daysVal,
			"current_day", currentDay,
		)
	}

	// C. Time Window
	localTime := startTime
	lat, long, hasLoc := getStartLocation(act)

	if hasLoc {
		offset := long / 15.0
		localTime = startTime.Add(time.Duration(offset * float64(time.Hour)))
	}

	startTimeVal, hasStartTime := inputs["time_start"]
	if !hasStartTime {
		startTimeVal = inputs["start_time"]
	}
	if startTimeVal != "" {
		if !checkTime(localTime, startTimeVal, true) {
			return returnMismatch(fmt.Sprintf("Start Time mismatch. Expected >= %s, got %s (Local Est.)", startTimeVal, localTime.Format("15:04")))
		}
		logger.Debug("condition_matcher: start_time check passed",
			"min_time", startTimeVal,
			"local_time", localTime.Format("15:04"),
		)
	}

	endTimeVal, hasEndTime := inputs["time_end"]
	if !hasEndTime {
		endTimeVal = inputs["end_time"]
	}
	if endTimeVal != "" {
		if !checkTime(localTime, endTimeVal, false) {
			return returnMismatch(fmt.Sprintf("End Time mismatch. Expected <= %s, got %s (Local Est.)", endTimeVal, localTime.Format("15:04")))
		}
		logger.Debug("condition_matcher: end_time check passed",
			"max_time", endTimeVal,
			"local_time", localTime.Format("15:04"),
		)
	}

	// D. Location
	latStr := inputs["location_lat"]
	longStr := inputs["location_long"]
	if longStr == "" {
		longStr = inputs["location_lng"]
	}

	if latStr != "" || longStr != "" {
		if latStr == "" || longStr == "" {
			return nil, fmt.Errorf("both location_lat and location_long are required for location proximity matching")
		}

		if !hasLoc {
			return returnMismatch("Location mismatch. No GPS data in activity.")
		}

		targetLat, err := strconv.ParseFloat(latStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid location_lat: %w", err)
		}
		targetLong, err := strconv.ParseFloat(longStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid location_long: %w", err)
		}

		radiusStr, hasRadius := inputs["radius_m"]
		if !hasRadius {
			radiusStr = inputs["location_radius"]
		}

		radius := 200.0 // Default 200m
		if radiusStr != "" {
			r, err := strconv.ParseFloat(radiusStr, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid radius: %w", err)
			}
			radius = r
		}

		dist := distanceMeters(lat, long, targetLat, targetLong)
		if dist > radius {
			return returnMismatch(fmt.Sprintf("Location mismatch. Distance %.2fm > Radius %.2fm. Act: (%.4f, %.4f), Target: (%.4f, %.4f)", dist, radius, lat, long, targetLat, targetLong))
		}
		logger.Debug("condition_matcher: location check passed",
			"distance", dist,
			"radius", radius,
			"activity_lat", lat,
			"activity_long", long,
		)
	}

	// 2. Conditions Met - Apply Outputs
	logger.Debug("condition_matcher: all conditions matched - applying outputs",
		"has_title_template", inputs["title_template"] != "",
		"has_description_template", inputs["description_template"] != "",
	)

	result := &providers.EnrichmentResult{
		Metadata: map[string]string{
			"condition_matcher_applied": "true",
			"match_debug":               "success",
		},
	}

	if titleTmpl, ok := inputs["title_template"]; ok && titleTmpl != "" {
		result.Name = titleTmpl
	}

	if descTmpl, ok := inputs["description_template"]; ok && descTmpl != "" {
		result.Description = descTmpl
	}

	return result, nil
}

// Helpers (Duplicated from Parkrun for now, should move to shared/geo?)
func getStartLocation(activity *pb.StandardizedActivity) (float64, float64, bool) {
	if len(activity.Sessions) == 0 {
		return 0, 0, false
	}
	for _, session := range activity.Sessions {
		if len(session.Laps) == 0 {
			continue
		}
		for _, lap := range session.Laps {
			if len(lap.Records) == 0 {
				continue
			}
			for _, rec := range lap.Records {
				if rec.PositionLat != 0 || rec.PositionLong != 0 {
					return rec.PositionLat, rec.PositionLong, true
				}
			}
		}
	}
	return 0, 0, false
}

func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func checkTime(t time.Time, limitStr string, isStart bool) bool {
	parts := strings.Split(limitStr, ":")
	if len(parts) < 2 {
		return false
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	limitMins := h*60 + m
	currentMins := t.Hour()*60 + t.Minute()

	if isStart {
		return currentMins >= limitMins
	}
	return currentMins <= limitMins
}
