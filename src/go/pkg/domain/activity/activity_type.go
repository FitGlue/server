package activity

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/fitglue/server/src/go/pkg/types/formatters"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// GetStravaActivityType returns the Strava API string for a given ActivityType enum
// using the custom strava_name option (e.g., "HighIntensityIntervalTraining").
func GetStravaActivityType(t pb.ActivityType) string {
	// Get the Enum Descriptor
	ed := t.Descriptor()
	// Get the specific Enum Value Descriptor
	ev := ed.Values().ByNumber(protoreflect.EnumNumber(t))
	if ev == nil {
		return "Workout" // Default fallback
	}

	// Access options
	opts := ev.Options()

	// Use proto.GetExtension to retrieve the custom option
	if proto.HasExtension(opts, pb.E_StravaName) {
		val := proto.GetExtension(opts, pb.E_StravaName)
		if strVal, ok := val.(string); ok && strVal != "" {
			return strVal
		}
	}
	return "Workout" // Default fallback for UNSPECIFIED
}

// GetIntervalsActivityType returns the Intervals.icu API string for a given ActivityType enum.
// Intervals.icu uses activity type strings like "Ride", "Run", "Swim", "WeightTraining".
func GetIntervalsActivityType(t pb.ActivityType) string {
	// Intervals.icu uses similar types to Strava but with some differences
	intervalsTypes := map[pb.ActivityType]string{
		pb.ActivityType_ACTIVITY_TYPE_RUN:                              "Run",
		pb.ActivityType_ACTIVITY_TYPE_RIDE:                             "Ride",
		pb.ActivityType_ACTIVITY_TYPE_SWIM:                             "Swim",
		pb.ActivityType_ACTIVITY_TYPE_WALK:                             "Walk",
		pb.ActivityType_ACTIVITY_TYPE_HIKE:                             "Hike",
		pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING:                  "WeightTraining",
		pb.ActivityType_ACTIVITY_TYPE_YOGA:                             "Yoga",
		pb.ActivityType_ACTIVITY_TYPE_WORKOUT:                          "Workout",
		pb.ActivityType_ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING: "HIIT",
		pb.ActivityType_ACTIVITY_TYPE_CROSSFIT:                         "Crossfit",
		pb.ActivityType_ACTIVITY_TYPE_ELLIPTICAL:                       "Elliptical",
		pb.ActivityType_ACTIVITY_TYPE_ROWING:                           "Rowing",
		pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN:                        "TrailRun",
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE:                     "VirtualRide",
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN:                      "VirtualRun",
		pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE:               "MountainBikeRide",
		pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE:                      "GravelRide",
		pb.ActivityType_ACTIVITY_TYPE_PILATES:                          "Pilates",
		pb.ActivityType_ACTIVITY_TYPE_TENNIS:                           "Tennis",
		pb.ActivityType_ACTIVITY_TYPE_SOCCER:                           "Soccer",
	}

	if intervalsType, ok := intervalsTypes[t]; ok {
		return intervalsType
	}
	return "Workout" // Default fallback
}

// ParseActivityTypeFromString parses a friendly string into an ActivityType enum.
// Accepts enum names (e.g., "ACTIVITY_TYPE_RUN"), display names (e.g., "Run"),
// and informal aliases (e.g., "running", "cycling", "bike") via the generated parser.
func ParseActivityTypeFromString(input string) pb.ActivityType {
	// 1. Try exact proto enum name (fast path)
	if v, ok := pb.ActivityType_value[input]; ok {
		return pb.ActivityType(v)
	}

	// 2. Try generated parser (handles display names, short names, aliases, case-insensitive)
	parsed := formatters.ParseActivityType(input)
	if parsed != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
		return parsed
	}

	// 3. Try matching strava_name (case-insensitive, for backward compat)
	for _, enumVal := range pb.ActivityType_value {
		at := pb.ActivityType(enumVal)
		stravaName := GetStravaActivityType(at)
		if stravaName != "" && equalFold(stravaName, input) {
			return at
		}
	}

	return pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED
}

// equalFold is a simple case-insensitive string comparison
func equalFold(a, b string) bool {
	return toLower(a) == toLower(b)
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
