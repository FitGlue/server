package condition_matcher

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// ConditionMatcherProvider applies enrichments based on a set of conditions.
type ConditionMatcherProvider struct{}

func init() {
	enricher_providers.Register(NewConditionMatcherProvider())
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

func (p *ConditionMatcherProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	// 1. Check Conditions (AND logic for all provided inputs)

	// A. Activity Type
	if val, ok := inputs["activity_type"]; ok && val != "" {
		// Case-insensitive check
		if !strings.EqualFold(activity.Type, val) {
			return nil, nil
		}
	}

	// B. Days of Week (e.g. "Mon,Tue")
	startTime := activity.StartTime.AsTime()
	if val, ok := inputs["days"]; ok && val != "" {
		currentDay := startTime.Weekday().String()[:3] // "Mon"
		match := false
		for _, day := range strings.Split(val, ",") {
			if strings.TrimSpace(day) == currentDay {
				match = true
				break
			}
		}
		if !match {
			return nil, nil
		}
	}

	// C. Time Window (Local Time approximation from Longitude if available, else UTC?)
	// User request said "rough start time".
	// Ideally we need timezone. StandardizedActivity doesn't strictly have it, but we can infer or use UTC if config expects UTC.
	// For now, let's assume we use the inferred local time from Longitude logic derived in Parkrun, or just compare UTC if no location?
	// The implementation plan didn't specify timezone handling, but Parkrun uses Longitude. Let's reuse that logic if useful, or simpler: Hour matching.
	// Let's assume input matches the activity's time reference (which is UTC in proto). User probably configures "09:00" implying local time.
	// This is hard without timezone.
	// Let's attempt to estimate local time if coordinates exist, otherwise warn/skip?
	// Or we just check against UTC if the user configures it that way.
	// Let's stick to the Parkrun logic: Estimate offset from Longitude.

	localTime := startTime
	lat, long, hasLoc := getStartLocation(activity)

	if hasLoc {
		offset := long / 15.0
		localTime = startTime.Add(time.Duration(offset * float64(time.Hour)))
	}

	if startStr, ok := inputs["time_start"]; ok && startStr != "" {
		if !checkTime(localTime, startStr, true) {
			return nil, nil
		}
	}
	if endStr, ok := inputs["time_end"]; ok && endStr != "" {
		if !checkTime(localTime, endStr, false) {
			return nil, nil
		}
	}

	// D. Location (Lat/Long + Radius)
	if latStr, ok := inputs["location_lat"]; ok && latStr != "" {
		if !hasLoc {
			return nil, nil
		}
		targetLat, err := strconv.ParseFloat(latStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid location_lat: %v", err)
		}
		targetLong, err := strconv.ParseFloat(inputs["location_long"], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid location_long: %v", err)
		}
		radius, err := strconv.ParseFloat(inputs["radius_m"], 64)
		if err != nil {
			radius = 200 // Default 200m
		}

		dist := distanceMeters(lat, long, targetLat, targetLong)
		if dist > radius {
			return nil, nil
		}
	}

	// 2. Conditions Met - Apply Outputs
	result := &enricher_providers.EnrichmentResult{
		Metadata: map[string]string{
			"condition_matcher_applied": "true",
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
