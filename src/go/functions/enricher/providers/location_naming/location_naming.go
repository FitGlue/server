package location_naming

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// Rate limiting: Nominatim requires max 1 request per second
var (
	lastRequestTime time.Time
	rateLimitMutex  sync.Mutex
	// Simple in-memory cache for location lookups (lat/lon key -> location name)
	locationCache      = make(map[string]string)
	locationCacheMutex sync.RWMutex
)

type LocationNaming struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewLocationNaming())
}

func NewLocationNaming() *LocationNaming {
	return &LocationNaming{}
}

func (p *LocationNaming) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *LocationNaming) Name() string {
	return "location_naming"
}

func (p *LocationNaming) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_LOCATION_NAMING
}

func (p *LocationNaming) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Extract GPS coordinates from first record
	var latitude, longitude float64
	var hasGPS bool

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.PositionLat != 0 && record.PositionLong != 0 {
					latitude = record.PositionLat
					longitude = record.PositionLong
					hasGPS = true
					break
				}
			}
			if hasGPS {
				break
			}
		}
		if hasGPS {
			break
		}
	}

	if !hasGPS {
		slog.Info("No GPS data found for location naming enricher, skipping")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"location_naming_status": "skipped",
				"status_detail":          "No GPS data found",
			},
		}, nil
	}

	// Try to get location from cache first
	cacheKey := fmt.Sprintf("%.4f,%.4f", latitude, longitude)
	locationCacheMutex.RLock()
	cachedLocation, cached := locationCache[cacheKey]
	locationCacheMutex.RUnlock()

	var locationName, cityName string
	if cached {
		slog.Info("Using cached location", "key", cacheKey, "location", cachedLocation)
		// Parse cached value (format: "location|city")
		parts := strings.SplitN(cachedLocation, "|", 2)
		locationName = parts[0]
		if len(parts) > 1 {
			cityName = parts[1]
		}
	} else {
		// Rate limiting: ensure at least 1 second between requests
		rateLimitMutex.Lock()
		elapsed := time.Since(lastRequestTime)
		if elapsed < time.Second {
			time.Sleep(time.Second - elapsed)
		}
		lastRequestTime = time.Now()
		rateLimitMutex.Unlock()

		// Call Nominatim reverse geocode API
		url := fmt.Sprintf(
			"https://nominatim.openstreetmap.org/reverse?lat=%.6f&lon=%.6f&format=json&zoom=16",
			latitude, longitude,
		)

		slog.Info("Fetching location data from Nominatim",
			"latitude", latitude,
			"longitude", longitude,
			"url", url,
		)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			slog.Error("Failed to create Nominatim request", "error", err)
			return nil, &providers.RetryableError{Err: fmt.Errorf("failed to create request: %w", err)}
		}

		// Required by Nominatim usage policy
		req.Header.Set("User-Agent", "FitGlue/1.0")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("Failed to fetch location data", "error", err)
			return nil, &providers.RetryableError{Err: fmt.Errorf("nominatim API request failed: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			slog.Error("Nominatim API returned non-200 status", "status", resp.StatusCode, "body", string(body))
			return nil, &providers.RetryableError{Err: fmt.Errorf("nominatim API returned status %d", resp.StatusCode)}
		}

		// Parse response
		var nominatimResp NominatimResponse
		if err := json.NewDecoder(resp.Body).Decode(&nominatimResp); err != nil {
			slog.Error("Failed to decode nominatim response", "error", err)
			return &providers.EnrichmentResult{
				Metadata: map[string]string{
					"location_naming_status": "skipped",
					"status_detail":          "Failed to parse location response",
				},
			}, nil
		}

		// Extract location name with priority: park/leisure > suburb > city
		locationName = getLocationName(nominatimResp.Address)
		cityName = getCityName(nominatimResp.Address)

		// Cache the result
		cacheValue := locationName + "|" + cityName
		locationCacheMutex.Lock()
		locationCache[cacheKey] = cacheValue
		locationCacheMutex.Unlock()

		slog.Info("Location resolved",
			"location", locationName,
			"city", cityName,
		)
	}

	// Get config options
	mode := inputs["mode"]
	if mode == "" {
		mode = "title" // Default to title mode
	}
	titleTemplate := inputs["title_template"]
	if titleTemplate == "" {
		titleTemplate = "{activity_type} in {location}"
	}
	fallbackEnabled := inputs["fallback_enabled"] != "false" // Default to true

	// If no specific location found and fallback is disabled, skip
	if locationName == "" && !fallbackEnabled {
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"location_naming_status": "skipped",
				"status_detail":          "No specific location found and fallback disabled",
			},
		}, nil
	}

	// Use city as fallback if no specific location
	displayLocation := locationName
	if displayLocation == "" && fallbackEnabled {
		displayLocation = cityName
	}
	if displayLocation == "" {
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"location_naming_status": "skipped",
				"status_detail":          "Could not determine location name",
			},
		}, nil
	}

	result := &providers.EnrichmentResult{
		Metadata: map[string]string{
			"location_naming_status": "success",
			"location_name":          displayLocation,
			"city":                   cityName,
		},
	}

	// Build the activity type string for template
	activityType := getActivityTypeStr(activity.Type)

	switch mode {
	case "title":
		// Generate title using template
		newTitle := titleTemplate
		newTitle = strings.ReplaceAll(newTitle, "{activity_type}", activityType)
		newTitle = strings.ReplaceAll(newTitle, "{location}", displayLocation)
		result.Name = newTitle
		slog.Info("Generated location-based title", "title", newTitle)

	case "description":
		// Append location line to description
		locationLine := fmt.Sprintf("\n\n📍 Location: %s", displayLocation)
		if cityName != "" && cityName != displayLocation {
			locationLine = fmt.Sprintf("\n\n📍 Location: %s, %s", displayLocation, cityName)
		}
		result.Description = locationLine
		slog.Info("Generated location description", "location_line", locationLine)
	}

	return result, nil
}

// NominatimResponse represents the Nominatim reverse geocode response
type NominatimResponse struct {
	Address NominatimAddress `json:"address"`
}

type NominatimAddress struct {
	Leisure string `json:"leisure"`
	Park    string `json:"park"`
	Suburb  string `json:"suburb"`
	City    string `json:"city"`
	Town    string `json:"town"`
	Village string `json:"village"`
	County  string `json:"county"`
	State   string `json:"state"`
	Country string `json:"country"`
}

// getLocationName returns the most specific location name available
// Priority: park > leisure > suburb
func getLocationName(addr NominatimAddress) string {
	if addr.Park != "" {
		return addr.Park
	}
	if addr.Leisure != "" {
		return addr.Leisure
	}
	if addr.Suburb != "" {
		return addr.Suburb
	}
	return ""
}

// getCityName returns the city/town/village name
func getCityName(addr NominatimAddress) string {
	if addr.City != "" {
		return addr.City
	}
	if addr.Town != "" {
		return addr.Town
	}
	if addr.Village != "" {
		return addr.Village
	}
	return ""
}

// getActivityTypeStr returns a human-readable activity type string
func getActivityTypeStr(activityType pb.ActivityType) string {
	// Convert enum to user-friendly string
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RUN:
		return "Run"
	case pb.ActivityType_ACTIVITY_TYPE_RIDE:
		return "Ride"
	case pb.ActivityType_ACTIVITY_TYPE_WALK:
		return "Walk"
	case pb.ActivityType_ACTIVITY_TYPE_HIKE:
		return "Hike"
	case pb.ActivityType_ACTIVITY_TYPE_SWIM:
		return "Swim"
	case pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING:
		return "Workout"
	case pb.ActivityType_ACTIVITY_TYPE_YOGA:
		return "Yoga"
	case pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE:
		return "Virtual Ride"
	case pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN:
		return "Virtual Run"
	default:
		return "Activity"
	}
}
