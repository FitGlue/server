package enricher_providers

import (
	"context"
	"fmt"
	"math"

	"github.com/fitglue/server/src/go/pkg/plugin"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type VirtualGPSProvider struct{}

func init() {
	Register(NewVirtualGPSProvider())

	plugin.RegisterEnricher(pb.EnricherProviderType_ENRICHER_PROVIDER_VIRTUAL_GPS, &pb.PluginManifest{
		Id:          "virtual-gps",
		Type:        pb.PluginType_PLUGIN_TYPE_ENRICHER,
		Name:        "Virtual GPS",
		Description: "Adds GPS coordinates from a virtual route to indoor activities",
		Icon:        "🗺️",
		Enabled:     true,
		ConfigSchema: []*pb.ConfigFieldSchema{
			{
				Key:          "route",
				Label:        "Route",
				Description:  "Virtual route to use for GPS generation",
				FieldType:    pb.ConfigFieldType_CONFIG_FIELD_TYPE_SELECT,
				Required:     false,
				DefaultValue: "london",
				Options: []*pb.ConfigFieldOption{
					{Value: "london", Label: "London Hyde Park (~4km loop)"},
					{Value: "nyc", Label: "NYC Central Park (~10km loop)"},
				},
			},
			{
				Key:          "force",
				Label:        "Force Override",
				Description:  "Override existing GPS data if present",
				FieldType:    pb.ConfigFieldType_CONFIG_FIELD_TYPE_BOOLEAN,
				Required:     false,
				DefaultValue: "false",
			},
		},
	})
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

func (p *VirtualGPSProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*EnrichmentResult, error) {
	// 1. Validation
	if len(activity.Sessions) == 0 {
		return &EnrichmentResult{}, nil
	}
	session := activity.Sessions[0]
	duration := int(session.TotalElapsedTime)
	distance := session.TotalDistance

	// Only apply if distance > 0 and duration > 0
	if distance <= 0 || duration <= 0 {
		return &EnrichmentResult{}, nil
	}

	// 2. Check overlap logic: If we already have GPS, we probably shouldn't overwrite unless forced.
	// For now, assume if any record has Lat/Long != 0, we skip.
	hasGPS := false
	for _, lap := range session.Laps {
		for _, rec := range lap.Records {
			if rec.PositionLat != 0 || rec.PositionLong != 0 {
				hasGPS = true
				break
			}
		}
	}
	// Allow override via inputConfig
	force := inputConfig["force"] == "true"
	if hasGPS && !force {
		return &EnrichmentResult{}, nil
	}

	// 3. Select Route
	routeName := inputConfig["route"]
	if routeName == "" {
		routeName = "london"
	}
	route, ok := RoutesLibrary[routeName]
	if !ok {
		// Fallback to london if unknown
		route = RoutesLibrary["london"]
	}

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

	return &EnrichmentResult{
		PositionLatStream:  latStream,
		PositionLongStream: longStream,
		Description:        fmt.Sprintf("🗺️ Took a virtual tour of %s (GPS generated for this indoor workout)\n", route.Name),
		Metadata: map[string]string{
			"virtual_gps_route": route.Name,
		},
	}, nil
}
