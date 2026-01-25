package virtual_gps

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type VirtualGPSProvider struct{}

func init() {
	providers.Register(NewVirtualGPSProvider())
}

func NewVirtualGPSProvider() *VirtualGPSProvider {
	return &VirtualGPSProvider{}
}

func (p *VirtualGPSProvider) Name() string {
	return "virtual-gps"
}

func (p *VirtualGPSProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_VIRTUAL_GPS
}

func (p *VirtualGPSProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("virtual_gps: starting",
		"activity_type", activity.Type.String(),
		"session_count", len(activity.Sessions),
		"configured_route", inputConfig["route"],
		"force", inputConfig["force"],
	)

	// 1. Validation
	if len(activity.Sessions) == 0 {
		logger.Debug("virtual_gps: skipping - no sessions")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "no_sessions"},
		}, nil
	}
	session := activity.Sessions[0]
	duration := int(session.TotalElapsedTime)
	distance := session.TotalDistance

	logger.Debug("virtual_gps: session data",
		"duration_seconds", duration,
		"distance_meters", distance,
	)

	// Only apply if distance > 0 and duration > 0
	if distance <= 0 || duration <= 0 {
		logger.Debug("virtual_gps: skipping - no distance or duration")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "no_distance_or_duration"},
		}, nil
	}

	// 2. Check overlap logic: If we already have GPS, we probably shouldn't overwrite unless forced.
	// For now, assume if any record has Lat/Long != 0, we skip.
	hasGPS := false
	gpsRecordCount := 0
	for _, lap := range session.Laps {
		for _, rec := range lap.Records {
			if rec.PositionLat != 0 || rec.PositionLong != 0 {
				hasGPS = true
				gpsRecordCount++
			}
		}
	}
	// Allow override via inputConfig
	force := inputConfig["force"] == "true"

	logger.Debug("virtual_gps: GPS check",
		"has_gps", hasGPS,
		"gps_record_count", gpsRecordCount,
		"force_override", force,
	)

	if hasGPS && !force {
		logger.Debug("virtual_gps: skipping - GPS already exists and force=false")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "gps_already_exists", "force": "false"},
		}, nil
	}

	// 3. Select Route
	routeName := inputConfig["route"]
	if routeName == "" {
		routeName = "london"
	}
	route, ok := RoutesLibrary[routeName]
	if !ok {
		// Fallback to london if unknown
		logger.Debug("virtual_gps: unknown route, falling back to london",
			"requested_route", routeName,
		)
		route = RoutesLibrary["london"]
	}

	logger.Debug("virtual_gps: generating GPS stream",
		"route_name", route.Name,
		"route_points", len(route.Points),
		"duration_seconds", duration,
		"distance_meters", distance,
	)

	// 4. Generate Streams
	latStream := make([]float64, duration)
	longStream := make([]float64, duration)

	// Pre-calculate cumulative distances for the route segments to make lookup faster
	routeTotalDist := 0.0
	segmentDists := make([]float64, len(route.Points)-1)

	for i := 0; i < len(route.Points)-1; i++ {
		d := haversine(route.Points[i], route.Points[i+1])
		segmentDists[i] = d
		routeTotalDist += d
	}

	avgSpeed := distance / float64(duration) // meters per second

	for t := 0; t < duration; t++ {
		// Current distance traveled in the workout
		curDist := avgSpeed * float64(t)

		// Map to route position (handling loops)
		routeDist := math.Mod(curDist, routeTotalDist)

		// Find segment
		accum := 0.0
		var p1, p2 LatLong
		var fraction float64

		for i := 0; i < len(segmentDists); i++ {
			if accum+segmentDists[i] >= routeDist {
				// We are in this segment
				remaining := routeDist - accum
				fraction = remaining / segmentDists[i]
				p1 = route.Points[i]
				p2 = route.Points[i+1]
				break
			}
			accum += segmentDists[i]
		}
		// Edge case: end of loop, use last points if loop finished exactly (rare with float)
		if p1 == (LatLong{}) && p2 == (LatLong{}) {
			p1 = route.Points[len(route.Points)-2]
			p2 = route.Points[len(route.Points)-1]
			fraction = 1.0
		}

		// Interpolate
		lat := p1.Lat + (p2.Lat-p1.Lat)*fraction
		long := p1.Long + (p2.Long-p1.Long)*fraction

		latStream[t] = lat
		longStream[t] = long
	}

	logger.Debug("virtual_gps: generated GPS stream successfully",
		"stream_length", len(latStream),
		"route_total_distance", routeTotalDist,
		"avg_speed_mps", avgSpeed,
	)

	return &providers.EnrichmentResult{
		PositionLatStream:  latStream,
		PositionLongStream: longStream,
		Description:        fmt.Sprintf("🗺️ Took a virtual tour of %s (GPS generated for this indoor workout)\n", route.Name),
		Metadata: map[string]string{
			"virtual_gps_route": route.Name,
		},
	}, nil
}
