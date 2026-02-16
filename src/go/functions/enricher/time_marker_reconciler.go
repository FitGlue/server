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
// Two matching strategies are used:
//  1. Position-based: When all StrengthSets share the same timestamp (e.g., Hevy sets
//     all start times to the workout start), group consecutive same-name sets into
//     exercise blocks and match TimeMarker[i] → block[i] by order.
//  2. Timestamp-based: When sets have distinct timestamps (e.g., FIT file uploads),
//     find the StrengthSet with the closest start time for each TimeMarker.
func reconcileTimeMarkerLabels(activity *pb.StandardizedActivity) {
	if activity == nil || len(activity.TimeMarkers) == 0 || len(activity.Sessions) == 0 {
		return
	}

	// Collect all StrengthSets with valid exercise names
	var sets []*pb.StrengthSet
	for _, session := range activity.Sessions {
		for _, s := range session.StrengthSets {
			if s.ExerciseName != "" {
				sets = append(sets, s)
			}
		}
	}

	if len(sets) == 0 {
		return
	}

	// Detect if all sets share the same timestamp (within 1 second).
	// Sources like Hevy don't provide per-set times, so all sets get the workout start time.
	// Position-based matching only activates with 2+ sets sharing the same timestamp.
	allSameTimestamp := false
	if len(sets) > 1 {
		allSameTimestamp = true // Assume same until proven otherwise
		firstTime := sets[0].GetStartTime().AsTime()
		for _, s := range sets[1:] {
			if s.StartTime == nil {
				break // Missing timestamps → treat as same
			}
			diff := firstTime.Sub(s.StartTime.AsTime())
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Second {
				allSameTimestamp = false
				break
			}
		}
	}

	if allSameTimestamp {
		reconcileByPosition(activity.TimeMarkers, sets)
	} else {
		reconcileByTimestamp(activity.TimeMarkers, sets)
	}
}

// reconcileByPosition matches TimeMarkers to exercise groups by order.
// Groups consecutive same-name StrengthSets into blocks, then maps
// TimeMarker[i] → block[i].
func reconcileByPosition(markers []*pb.TimeMarker, sets []*pb.StrengthSet) {
	// Build exercise groups (consecutive same-name sets → one group)
	var groups []string
	currentName := ""
	for _, s := range sets {
		if s.ExerciseName != currentName {
			groups = append(groups, s.ExerciseName)
			currentName = s.ExerciseName
		}
	}

	// Match markers to groups by position
	for i, marker := range markers {
		if i >= len(groups) {
			break
		}
		if groups[i] != marker.Label {
			marker.Label = groups[i]
		}
	}
}

// reconcileByTimestamp matches TimeMarkers to StrengthSets by closest timestamp.
func reconcileByTimestamp(markers []*pb.TimeMarker, sets []*pb.StrengthSet) {
	for _, marker := range markers {
		if marker.Timestamp == nil {
			continue
		}

		markerTime := marker.Timestamp.AsTime()
		bestName := ""
		bestDiff := time.Duration(math.MaxInt64)

		for _, set := range sets {
			if set.StartTime == nil {
				continue
			}
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
