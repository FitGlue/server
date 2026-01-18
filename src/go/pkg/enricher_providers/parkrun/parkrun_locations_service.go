package parkrun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	eventsJSONURL     = "https://images.parkrun.com/events.json"
	cacheRefreshHours = 24
	gridCellSize      = 1.0 // 1 degree (~111km) for grid cells
)

// ParkrunLocation represents a Parkrun event location.
type ParkrunLocation struct {
	Name       string // e.g., "Newark Parkrun"
	EventSlug  string // e.g., "newark"
	CountryURL string // e.g., "www.parkrun.org.uk"
	Latitude   float64
	Longitude  float64
}

// ParkrunLocationsService provides efficient lookup of Parkrun locations.
type ParkrunLocationsService struct {
	mu        sync.RWMutex
	locations []ParkrunLocation
	grid      map[string][]ParkrunLocation // Grid cells for O(1) lookup
	lastFetch time.Time
	client    *http.Client
}

// NewParkrunLocationsService creates a new service with default HTTP client.
func NewParkrunLocationsService() *ParkrunLocationsService {
	return &ParkrunLocationsService{
		grid: make(map[string][]ParkrunLocation),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewParkrunLocationsServiceWithClient creates a service with a custom HTTP client for testing.
func NewParkrunLocationsServiceWithClient(client *http.Client) *ParkrunLocationsService {
	return &ParkrunLocationsService{
		grid:   make(map[string][]ParkrunLocation),
		client: client,
	}
}

// gridKey returns the grid cell key for a given lat/lng.
func gridKey(lat, lng float64) string {
	// Use 1-degree cells
	latCell := int(lat / gridCellSize)
	lngCell := int(lng / gridCellSize)
	return fmt.Sprintf("%d:%d", latCell, lngCell)
}

// FindNearest finds the nearest Parkrun location within the threshold distance.
// Uses grid-based pre-filtering for O(1) average lookup.
func (s *ParkrunLocationsService) FindNearest(lat, lng float64, thresholdMeters float64) *ParkrunLocation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.locations) == 0 {
		return nil
	}

	// Get candidate cells (current + adjacent for edge cases)
	candidateCells := s.getAdjacentCells(lat, lng)
	var candidates []ParkrunLocation

	for _, cellKey := range candidateCells {
		if locs, ok := s.grid[cellKey]; ok {
			candidates = append(candidates, locs...)
		}
	}

	// Find nearest within threshold
	var nearest *ParkrunLocation
	minDist := thresholdMeters + 1 // Start above threshold

	for i := range candidates {
		dist := distanceMeters(lat, lng, candidates[i].Latitude, candidates[i].Longitude)
		if dist <= thresholdMeters && dist < minDist {
			minDist = dist
			nearest = &candidates[i]
		}
	}

	return nearest
}

// getAdjacentCells returns the 9 cells (current + 8 adjacent) for edge coverage.
func (s *ParkrunLocationsService) getAdjacentCells(lat, lng float64) []string {
	latCell := int(lat / gridCellSize)
	lngCell := int(lng / gridCellSize)

	cells := make([]string, 0, 9)
	for dLat := -1; dLat <= 1; dLat++ {
		for dLng := -1; dLng <= 1; dLng++ {
			cells = append(cells, fmt.Sprintf("%d:%d", latCell+dLat, lngCell+dLng))
		}
	}
	return cells
}

// RefreshFromSource fetches the latest Parkrun locations from events.json.
func (s *ParkrunLocationsService) RefreshFromSource(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", eventsJSONURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FitGlue/1.0 (https://fitglue.com)")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching events.json: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	locations, err := parseEventsJSON(body)
	if err != nil {
		return fmt.Errorf("parsing events.json: %w", err)
	}

	// Update cache
	s.mu.Lock()
	defer s.mu.Unlock()

	s.locations = locations
	s.lastFetch = time.Now()

	// Rebuild grid index
	s.grid = make(map[string][]ParkrunLocation, len(locations)/10)
	for _, loc := range locations {
		key := gridKey(loc.Latitude, loc.Longitude)
		s.grid[key] = append(s.grid[key], loc)
	}

	return nil
}

// EnsureLoaded ensures locations are loaded, refreshing if cache is stale or empty.
func (s *ParkrunLocationsService) EnsureLoaded(ctx context.Context) error {
	s.mu.RLock()
	isStale := len(s.locations) == 0 || time.Since(s.lastFetch) > cacheRefreshHours*time.Hour
	s.mu.RUnlock()

	if isStale {
		return s.RefreshFromSource(ctx)
	}
	return nil
}

// LocationCount returns the number of cached locations.
func (s *ParkrunLocationsService) LocationCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.locations)
}

// eventsJSONFeatureCollection represents the GeoJSON structure from Parkrun.
type eventsJSONFeatureCollection struct {
	Type     string            `json:"type"`
	Features []eventsJSONEvent `json:"features"`
}

type eventsJSONEvent struct {
	ID         int                       `json:"id"`
	Properties eventsJSONEventProperties `json:"properties"`
	Geometry   eventsJSONGeometry        `json:"geometry"`
}

type eventsJSONEventProperties struct {
	EventName      string `json:"eventname"`      // Slug, e.g., "newark"
	EventLongName  string `json:"EventLongName"`  // Full name
	EventShortName string `json:"EventShortName"` // Short name
	CountryCode    int    `json:"countrycode"`
}

type eventsJSONGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"` // [longitude, latitude]
}

// parseEventsJSON parses the Parkrun events.json format.
// The events.json has the FeatureCollection nested under an "events" key.
func parseEventsJSON(data []byte) ([]ParkrunLocation, error) {
	// Try nested structure first (current API structure)
	var wrapper struct {
		Events eventsJSONFeatureCollection `json:"events"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshalling JSON: %w", err)
	}

	fc := wrapper.Events
	if len(fc.Features) == 0 {
		// Fallback: try direct FeatureCollection (in case API changes)
		var directFC eventsJSONFeatureCollection
		if err := json.Unmarshal(data, &directFC); err == nil && len(directFC.Features) > 0 {
			fc = directFC
		}
	}

	locations := make([]ParkrunLocation, 0, len(fc.Features))
	for _, feature := range fc.Features {
		if feature.Geometry.Type != "Point" || len(feature.Geometry.Coordinates) < 2 {
			continue
		}

		// Coordinates are [longitude, latitude] in GeoJSON
		lng := feature.Geometry.Coordinates[0]
		lat := feature.Geometry.Coordinates[1]

		// Convert EventShortName to proper formatting: "Newark Parkrun"
		name := feature.Properties.EventShortName + " Parkrun"
		if name == " Parkrun" {
			name = feature.Properties.EventLongName
		}

		// Determine country URL from countrycode (simplified mapping)
		countryURL := countryCodeToURL(feature.Properties.CountryCode)

		locations = append(locations, ParkrunLocation{
			Name:       name,
			EventSlug:  feature.Properties.EventName,
			CountryURL: countryURL,
			Latitude:   lat,
			Longitude:  lng,
		})
	}

	return locations, nil
}

// countryCodeToURL maps Parkrun country codes to their URLs.
// This is a simplified mapping - full list can be expanded.
func countryCodeToURL(code int) string {
	countryURLs := map[int]string{
		97: "www.parkrun.org.uk", // UK
		65: "www.parkrun.com.au", // Australia
		23: "www.parkrun.co.nz",  // New Zealand
		79: "www.parkrun.co.za",  // South Africa
		14: "www.parkrun.ie",     // Ireland
		64: "www.parkrun.ca",     // Canada
		3:  "www.parkrun.de",     // Germany
		4:  "www.parkrun.dk",     // Denmark
		32: "www.parkrun.fi",     // Finland
		5:  "www.parkrun.fr",     // France
		58: "www.parkrun.it",     // Italy
		59: "www.parkrun.jp",     // Japan
		85: "www.parkrun.my",     // Malaysia
		31: "www.parkrun.nl",     // Netherlands
		24: "www.parkrun.no",     // Norway
		67: "www.parkrun.pl",     // Poland
		74: "www.parkrun.sg",     // Singapore
		25: "www.parkrun.se",     // Sweden
		98: "www.parkrun.us",     // USA
	}

	if url, ok := countryURLs[code]; ok {
		return url
	}
	return "www.parkrun.com" // Default fallback
}
