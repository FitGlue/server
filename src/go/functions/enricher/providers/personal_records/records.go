// Package personal_records provides Personal Record (PR) detection for cardio and strength activities.
package personal_records

import (
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// CardioRecordType defines the types of cardio records we track
type CardioRecordType string

const (
	// Distance-based time records
	RecordFastest5K           CardioRecordType = "fastest_5k"
	RecordFastest10K          CardioRecordType = "fastest_10k"
	RecordFastestHalfMarathon CardioRecordType = "fastest_half_marathon"

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
	Distance5K           = 5000.0
	Distance10K          = 10000.0
	DistanceHalfMarathon = 21097.5 // 21.0975 km
)

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
