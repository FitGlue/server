// Package personal_records provides Personal Record (PR) detection for cardio and strength activities.
package personal_records

import (
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// CardioRecordType defines the types of cardio records we track
type CardioRecordType string

const (
	// Sprint distance records
	RecordFastest100m CardioRecordType = "fastest_100m"
	RecordFastest200m CardioRecordType = "fastest_200m"
	RecordFastest400m CardioRecordType = "fastest_400m"
	RecordFastest500m CardioRecordType = "fastest_500m"

	// Metric distance records
	RecordFastest1K  CardioRecordType = "fastest_1k"
	RecordFastest2K  CardioRecordType = "fastest_2k"
	RecordFastest5K  CardioRecordType = "fastest_5k"
	RecordFastest10K CardioRecordType = "fastest_10k"
	RecordFastest20K CardioRecordType = "fastest_20k"
	RecordFastest30K CardioRecordType = "fastest_30k"
	RecordFastest40K CardioRecordType = "fastest_40k"
	RecordFastest50K CardioRecordType = "fastest_50k"

	// Race distance records
	RecordFastestHalfMarathon  CardioRecordType = "fastest_half_marathon"
	RecordFastestMarathon      CardioRecordType = "fastest_marathon"
	RecordFastestUltraMarathon CardioRecordType = "fastest_ultra_marathon"

	// Imperial distance records
	RecordFastest1Mile  CardioRecordType = "fastest_1_mile"
	RecordFastest2Mile  CardioRecordType = "fastest_2_mile"
	RecordFastest5Mile  CardioRecordType = "fastest_5_mile"
	RecordFastest10Mile CardioRecordType = "fastest_10_mile"
	RecordFastest15Mile CardioRecordType = "fastest_15_mile"
	RecordFastest20Mile CardioRecordType = "fastest_20_mile"
	RecordFastest25Mile CardioRecordType = "fastest_25_mile"
	RecordFastest30Mile CardioRecordType = "fastest_30_mile"

	// Distance records
	RecordLongestRun  CardioRecordType = "longest_run"
	RecordLongestRide CardioRecordType = "longest_ride"

	// Elevation records
	RecordHighestElevationGain CardioRecordType = "highest_elevation_gain"
)

// StrengthRecordSuffix defines the suffixes for strength records
type StrengthRecordSuffix string

const (
	Suffix1RM    StrengthRecordSuffix = "_1rm"
	SuffixVolume StrengthRecordSuffix = "_volume"
	SuffixReps   StrengthRecordSuffix = "_reps"
)

// Distance thresholds in meters
const (
	// Sprint distances
	Distance100m = 100.0
	Distance200m = 200.0
	Distance400m = 400.0
	Distance500m = 500.0

	// Metric distances
	Distance1K  = 1000.0
	Distance2K  = 2000.0
	Distance5K  = 5000.0
	Distance10K = 10000.0
	Distance20K = 20000.0
	Distance30K = 30000.0
	Distance40K = 40000.0
	Distance50K = 50000.0

	// Race distances
	DistanceHalfMarathon  = 21097.5  // 21.0975 km
	DistanceMarathon      = 42195.0  // 42.195 km
	DistanceUltraMarathon = 100000.0 // 100 km

	// Imperial distances (converted to meters)
	Distance1Mile  = 1609.344
	Distance2Mile  = 3218.688
	Distance5Mile  = 8046.72
	Distance10Mile = 16093.44
	Distance15Mile = 24140.16
	Distance20Mile = 32186.88
	Distance25Mile = 40233.6
	Distance30Mile = 48280.32
)

// DistanceThreshold pairs a record type with its target distance for iteration
type DistanceThreshold struct {
	RecordType CardioRecordType
	DistanceM  float64
	Display    string
}

// AllDistanceThresholds returns all distance thresholds sorted by distance (ascending).
// Used by checkCardioRecords to iterate over all applicable distances.
func AllDistanceThresholds() []DistanceThreshold {
	return []DistanceThreshold{
		{RecordFastest100m, Distance100m, "Fastest 100m"},                             // 100m
		{RecordFastest200m, Distance200m, "Fastest 200m"},                             // 200m
		{RecordFastest400m, Distance400m, "Fastest 400m"},                             // 400m
		{RecordFastest500m, Distance500m, "Fastest 500m"},                             // 500m
		{RecordFastest1K, Distance1K, "Fastest 1K"},                                   // 1,000m
		{RecordFastest1Mile, Distance1Mile, "Fastest 1 Mile"},                         // 1,609m
		{RecordFastest2K, Distance2K, "Fastest 2K"},                                   // 2,000m
		{RecordFastest2Mile, Distance2Mile, "Fastest 2 Mile"},                         // 3,219m
		{RecordFastest5K, Distance5K, "Fastest 5K"},                                   // 5,000m
		{RecordFastest5Mile, Distance5Mile, "Fastest 5 Mile"},                         // 8,047m
		{RecordFastest10K, Distance10K, "Fastest 10K"},                                // 10,000m
		{RecordFastest10Mile, Distance10Mile, "Fastest 10 Mile"},                      // 16,093m
		{RecordFastest20K, Distance20K, "Fastest 20K"},                                // 20,000m
		{RecordFastestHalfMarathon, DistanceHalfMarathon, "Fastest Half Marathon"},    // 21,098m
		{RecordFastest15Mile, Distance15Mile, "Fastest 15 Mile"},                      // 24,140m
		{RecordFastest30K, Distance30K, "Fastest 30K"},                                // 30,000m
		{RecordFastest20Mile, Distance20Mile, "Fastest 20 Mile"},                      // 32,187m
		{RecordFastest40K, Distance40K, "Fastest 40K"},                                // 40,000m
		{RecordFastest25Mile, Distance25Mile, "Fastest 25 Mile"},                      // 40,234m
		{RecordFastestMarathon, DistanceMarathon, "Fastest Marathon"},                 // 42,195m
		{RecordFastest30Mile, Distance30Mile, "Fastest 30 Mile"},                      // 48,280m
		{RecordFastest50K, Distance50K, "Fastest 50K"},                                // 50,000m
		{RecordFastestUltraMarathon, DistanceUltraMarathon, "Fastest Ultra Marathon"}, // 100,000m
	}
}

// NewPRResult holds the result of a PR check
type NewPRResult struct {
	RecordType     string
	NewValue       float64
	PreviousValue  *float64
	Improvement    *float64 // Percentage improvement (negative = faster for time records)
	Unit           string
	DisplayMessage string // Formatted message for description
}

// Calculate1RM calculates the estimated 1 Rep Max using the Epley formula
// If reps == 1, returns the weight directly
// Otherwise: weight * (1 + reps/30)
func Calculate1RM(weightKg float64, reps int32) float64 {
	if reps <= 0 {
		return 0
	}
	if reps == 1 {
		return weightKg
	}
	return weightKg * (1 + float64(reps)/30)
}

// CalculateSetVolume calculates the total volume for a set (weight * reps)
func CalculateSetVolume(weightKg float64, reps int32) float64 {
	if reps <= 0 || weightKg <= 0 {
		return 0
	}
	return weightKg * float64(reps)
}

// CalculateImprovement calculates the percentage improvement between old and new values
// For time-based records (lower is better), negative improvement = better
// For weight/distance records (higher is better), positive improvement = better
func CalculateImprovement(oldValue, newValue float64, lowerIsBetter bool) float64 {
	if oldValue == 0 {
		return 0
	}
	if lowerIsBetter {
		// For time records: negative improvement means faster
		return ((oldValue - newValue) / oldValue) * 100
	}
	// For weight/distance records: positive improvement means better
	return ((newValue - oldValue) / oldValue) * 100
}

// IsCardioActivity checks if the activity type is a cardio activity
func IsCardioActivity(activityType pb.ActivityType) bool {
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RUN,
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN,
		pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN,
		pb.ActivityType_ACTIVITY_TYPE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_EBIKE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_WALK,
		pb.ActivityType_ACTIVITY_TYPE_HIKE,
		pb.ActivityType_ACTIVITY_TYPE_SWIM,
		pb.ActivityType_ACTIVITY_TYPE_ROWING:
		return true
	default:
		return false
	}
}

// IsRunningActivity checks if the activity is a running type
func IsRunningActivity(activityType pb.ActivityType) bool {
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RUN,
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN,
		pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN:
		return true
	default:
		return false
	}
}

// IsCyclingActivity checks if the activity is a cycling type
func IsCyclingActivity(activityType pb.ActivityType) bool {
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_EBIKE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE:
		return true
	default:
		return false
	}
}

// IsStrengthActivity checks if the activity type is a strength/weight training activity
func IsStrengthActivity(activityType pb.ActivityType) bool {
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
		pb.ActivityType_ACTIVITY_TYPE_WORKOUT,
		pb.ActivityType_ACTIVITY_TYPE_CROSSFIT:
		return true
	default:
		return false
	}
}

// HybridRaceType identifies the type of hybrid race from activity metadata
type HybridRaceType string

const (
	HybridRaceHyrox HybridRaceType = "hyrox"
	HybridRaceATHX  HybridRaceType = "athx"
)

// HybridRaceStations lists the trackable stations for hybrid races
var HybridRaceStations = []string{
	"skierg",
	"sled_push",
	"sled_pull",
	"burpee_broad_jump",
	"rowing",
	"farmers_carry",
	"sandbag_lunges",
	"wall_balls",
}

// FormatHybridRaceRecordType creates a record type key for hybrid race PRs
// Format: hybrid_race_{race_type}_{category}
// Examples: hybrid_race_hyrox_total_time, hybrid_race_hyrox_skierg
func FormatHybridRaceRecordType(raceType, category string) string {
	return "hybrid_race_" + raceType + "_" + category
}

// ParseHybridRaceRecordType parses a hybrid race record type into components
// Returns empty strings if not a hybrid race record
func ParseHybridRaceRecordType(recordType string) (raceType, category string) {
	prefix := "hybrid_race_"
	if len(recordType) <= len(prefix) {
		return "", ""
	}
	suffix := recordType[len(prefix):]

	// Find the race type (first segment)
	for _, rt := range []string{"hyrox", "athx"} {
		if len(suffix) > len(rt)+1 && suffix[:len(rt)+1] == rt+"_" {
			return rt, suffix[len(rt)+1:]
		}
	}
	return "", ""
}
