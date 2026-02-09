package enricher

import (
	"math"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// reconcileTimeMarkerLabels updates TimeMarker labels with better exercise names
// from StrengthSets. This is called after all enrichers have run, so the StrengthSets
// may contain superior names (e.g., "Bench Press" from Hevy) compared to the generic
// FIT category names on the TimeMarkers (e.g., "Bench Press" from category enum).
//
// The matching strategy is timestamp-based: for each TimeMarker, find the StrengthSet
// with the closest start time and adopt its exercise name.
func reconcileTimeMarkerLabels(activity *pb.StandardizedActivity) {
	if activity == nil || len(activity.TimeMarkers) == 0 || len(activity.Sessions) == 0 {
		return
	}

	// Collect all StrengthSets with valid timestamps
	var sets []*pb.StrengthSet
	for _, session := range activity.Sessions {
		for _, s := range session.StrengthSets {
			if s.StartTime != nil && s.ExerciseName != "" {
				sets = append(sets, s)
			}
		}
	}

	if len(sets) == 0 {
		return
	}

	// For each TimeMarker, find the best matching StrengthSet by timestamp
	for _, marker := range activity.TimeMarkers {
		if marker.Timestamp == nil {
			continue
		}

		markerTime := marker.Timestamp.AsTime()
		bestName := ""
		bestDiff := time.Duration(math.MaxInt64)

		for _, set := range sets {
			diff := markerTime.Sub(set.StartTime.AsTime())
			if diff < 0 {
				diff = -diff
			}
			if diff < bestDiff {
				bestDiff = diff
				bestName = set.ExerciseName
			}
		}

		// Only update if we found a match within a reasonable window (5 minutes)
		// and the name is actually different
		if bestName != "" && bestDiff <= 5*time.Minute && bestName != marker.Label {
			marker.Label = bestName
		}
	}
}
