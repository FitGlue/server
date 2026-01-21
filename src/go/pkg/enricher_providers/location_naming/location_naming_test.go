package location_naming

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestGetLocationName(t *testing.T) {
	tests := []struct {
		name     string
		address  NominatimAddress
		expected string
	}{
		{
			name: "park takes priority",
			address: NominatimAddress{
				Park:    "Hyde Park",
				Leisure: "Sports Centre",
				Suburb:  "Kensington",
			},
			expected: "Hyde Park",
		},
		{
			name: "leisure takes priority over suburb",
			address: NominatimAddress{
				Leisure: "Sports Centre",
				Suburb:  "Kensington",
			},
			expected: "Sports Centre",
		},
		{
			name: "suburb used as fallback",
			address: NominatimAddress{
				Suburb: "Kensington",
				City:   "London",
			},
			expected: "Kensington",
		},
		{
			name:     "empty when no location available",
			address:  NominatimAddress{City: "London"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLocationName(tt.address)
			if result != tt.expected {
				t.Errorf("getLocationName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetCityName(t *testing.T) {
	tests := []struct {
		name     string
		address  NominatimAddress
		expected string
	}{
		{
			name:     "city preferred",
			address:  NominatimAddress{City: "London", Town: "Westminster"},
			expected: "London",
		},
		{
			name:     "town as fallback",
			address:  NominatimAddress{Town: "Windsor"},
			expected: "Windsor",
		},
		{
			name:     "village as last resort",
			address:  NominatimAddress{Village: "Little Gaddesden"},
			expected: "Little Gaddesden",
		},
		{
			name:     "empty when no city available",
			address:  NominatimAddress{County: "Hertfordshire"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCityName(tt.address)
			if result != tt.expected {
				t.Errorf("getCityName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetActivityTypeStr(t *testing.T) {
	tests := []struct {
		name         string
		activityType pb.ActivityType
		expected     string
	}{
		{name: "run", activityType: pb.ActivityType_ACTIVITY_TYPE_RUN, expected: "Run"},
		{name: "ride", activityType: pb.ActivityType_ACTIVITY_TYPE_RIDE, expected: "Ride"},
		{name: "walk", activityType: pb.ActivityType_ACTIVITY_TYPE_WALK, expected: "Walk"},
		{name: "hike", activityType: pb.ActivityType_ACTIVITY_TYPE_HIKE, expected: "Hike"},
		{name: "unknown defaults to Activity", activityType: pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED, expected: "Activity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getActivityTypeStr(tt.activityType)
			if result != tt.expected {
				t.Errorf("getActivityTypeStr() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProviderMetadata(t *testing.T) {
	provider := NewLocationNaming()

	if provider.Name() != "location_naming" {
		t.Errorf("Name() = %q, want 'location_naming'", provider.Name())
	}

	if provider.ProviderType() != pb.EnricherProviderType_ENRICHER_PROVIDER_LOCATION_NAMING {
		t.Errorf("ProviderType() = %v, want ENRICHER_PROVIDER_LOCATION_NAMING", provider.ProviderType())
	}
}
