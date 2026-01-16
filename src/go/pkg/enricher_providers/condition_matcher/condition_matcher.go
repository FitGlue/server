package condition_matcher

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/pkg/domain/activity"
	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	"github.com/fitglue/server/src/go/pkg/plugin"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// ConditionMatcherProvider applies enrichments based on a set of conditions.
type ConditionMatcherProvider struct{}

func init() {
	enricher_providers.Register(NewConditionMatcherProvider())

	plugin.RegisterEnricher(pb.EnricherProviderType_ENRICHER_PROVIDER_CONDITION_MATCHER, &pb.PluginManifest{
		Id:          "condition-matcher",
		Type:        pb.PluginType_PLUGIN_TYPE_ENRICHER,
		Name:        "Condition Matcher",
		Description: "Applies title/description templates when conditions match (type, day, time, location)",
		Icon:        "🎯",
		Enabled:     true,
		ConfigSchema: []*pb.ConfigFieldSchema{
			{
				Key:         "activity_type",
				Label:       "Activity Type",
				Description: "Match specific activity type (e.g., Run, Ride, Walk)",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "days_of_week",
				Label:       "Days of Week",
				Description: "Comma-separated days (e.g., Mon,Wed,Sat or 0,3,6)",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "start_time",
				Label:       "Start Time",
				Description: "Earliest time to match (HH:MM format)",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "end_time",
				Label:       "End Time",
				Description: "Latest time to match (HH:MM format)",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "location_lat",
				Label:       "Location Latitude",
				Description: "Target location latitude",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "location_long",
				Label:       "Location Longitude",
				Description: "Target location longitude",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:          "radius_m",
				Label:        "Radius (meters)",
				Description:  "Match radius in meters",
				FieldType:    pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:     false,
				DefaultValue: "200",
			},
			{
				Key:         "title_template",
				Label:       "Title Template",
				Description: "Title to set when conditions match",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "description_template",
				Label:       "Description Template",
				Description: "Description to set when conditions match",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
		},
	})
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

func (p *ConditionMatcherProvider) Enrich(ctx context.Context, act *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	// Helper to handle mismatch
	returnMismatch := func(reason string) (*enricher_providers.EnrichmentResult, error) {
		// Log for visibility
		fmt.Printf("ConditionMatcher mismatch: %s\n", reason)
		return &enricher_providers.EnrichmentResult{
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
	}

	endTimeVal, hasEndTime := inputs["time_end"]
	if !hasEndTime {
		endTimeVal = inputs["end_time"]
	}
	if endTimeVal != "" {
		if !checkTime(localTime, endTimeVal, false) {
			return returnMismatch(fmt.Sprintf("End Time mismatch. Expected <= %s, got %s (Local Est.)", endTimeVal, localTime.Format("15:04")))
		}
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
			return nil, fmt.Errorf("invalid location_lat: %v", err)
		}
		targetLong, err := strconv.ParseFloat(longStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid location_long: %v", err)
		}

		radiusStr, hasRadius := inputs["radius_m"]
		if !hasRadius {
			radiusStr = inputs["location_radius"]
		}

		radius := 200.0 // Default 200m
		if radiusStr != "" {
			r, err := strconv.ParseFloat(radiusStr, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid radius: %v", err)
			}
			radius = r
		}

		dist := distanceMeters(lat, long, targetLat, targetLong)
		if dist > radius {
			return returnMismatch(fmt.Sprintf("Location mismatch. Distance %.2fm > Radius %.2fm. Act: (%.4f, %.4f), Target: (%.4f, %.4f)", dist, radius, lat, long, targetLat, targetLong))
		}
	}

	// 2. Conditions Met - Apply Outputs
	result := &enricher_providers.EnrichmentResult{
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
