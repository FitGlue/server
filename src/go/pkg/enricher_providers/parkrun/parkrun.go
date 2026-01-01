package parkrun

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// ParkrunProvider detects if an activity is a Parkrun event.
type ParkrunProvider struct{}

func init() {
	enricher_providers.Register(NewParkrunProvider())
}

func NewParkrunProvider() *ParkrunProvider {
	return &ParkrunProvider{}
}

func (p *ParkrunProvider) Name() string {
	return "parkrun"
}

func (p *ParkrunProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_PARKRUN
}

func (p *ParkrunProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	// 0. Parse Inputs
	enableTitling := inputs["enable_titling"] != "false" // Default true
	tagValueStr := inputs["tags"]
	if _, ok := inputs["tags"]; !ok {
		tagValueStr = "Parkrun"
	}

	specialEvents := inputs["special_events"]
	if specialEvents == "" {
		specialEvents = "12-25,01-01"
	}

	// 1. Basic Checks
	// Only care about Runs
	if activity.Type != "Run" && activity.Type != "RUNNING" {
		// Check both casing? StandardizedActivity Type usage is string, often "Run" or "RUNNING" depending on source.
		// Let's match case-insensitive "run"
		if !strings.Contains(strings.ToLower(activity.Type), "run") {
			return nil, nil
		}
	}

	// 2. Location Check (Deep traversal)
	lat, long, found := getStartLocation(activity)
	if !found {
		return nil, nil // No location data
	}

	// 3. Time Check
	startTime, err := time.Parse(time.RFC3339, activity.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	matchedLocation := p.findNearestParkrun(lat, long)

	if matchedLocation == nil {
		return nil, nil // Not near any parkrun
	}

	// Estimate Local Time of the EVENT
	estimatedOffsetHours := matchedLocation.Longitude / 15.0
	estimatedLocalTime := startTime.Add(time.Duration(estimatedOffsetHours * float64(time.Hour)))

	isSaturday := estimatedLocalTime.Weekday() == time.Saturday
	monthDay := estimatedLocalTime.Format("01-02")

	isSpecial := false
	for _, evt := range strings.Split(specialEvents, ",") {
		if strings.TrimSpace(evt) == monthDay {
			isSpecial = true
			break
		}
	}

	if !isSaturday && !isSpecial {
		// Not a parkrun day
		return nil, nil
	}

	// Time Window Check (Local Time)
	// 08:30 to 10:30
	hour := estimatedLocalTime.Hour()
	minute := estimatedLocalTime.Minute()
	totalMinutes := hour*60 + minute

	startWindow := 7*60 + 30 // 07:30 (Matches offset errors or early starts)
	endWindow := 11*60 + 0   // 11:00

	if totalMinutes < startWindow || totalMinutes > endWindow {
		// Outside time window
		return nil, nil
	}

	// 4. Match Found! Apply Changes.

	result := &enricher_providers.EnrichmentResult{
		Metadata: map[string]string{
			"is_parkrun":    "true",
			"parkrun_event": matchedLocation.Name,
		},
	}

	// Titling
	if enableTitling {
		result.Name = matchedLocation.Name
	}

	// Tagging
	if tagValueStr != "" {
		tags := strings.Split(tagValueStr, ",")
		result.Tags = make([]string, 0, len(tags))
		for _, t := range tags {
			if val := strings.TrimSpace(t); val != "" {
				result.Tags = append(result.Tags, val)
			}
		}
	}

	return result, nil
}

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
			// Check first record
			for _, rec := range lap.Records {
				if rec.PositionLat != 0 || rec.PositionLong != 0 {
					return rec.PositionLat, rec.PositionLong, true
				}
			}
		}
	}
	return 0, 0, false
}

func (p *ParkrunProvider) findNearestParkrun(lat, long float64) *ParkrunLocation {
	thresholdMeters := 200.0

	for _, loc := range KnownLocations {
		dist := distanceMeters(lat, long, loc.Latitude, loc.Longitude)
		if dist <= thresholdMeters {
			return &loc
		}
	}
	return nil
}

// Haversine formula
func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in meters
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
