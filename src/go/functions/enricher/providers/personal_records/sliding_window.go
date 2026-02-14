// Package personal_records provides Personal Record (PR) detection for cardio and strength activities.
package personal_records

import (
	"math"
	"sort"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// distanceTimePoint represents a single data point with cumulative distance and timestamp (seconds from start)
type distanceTimePoint struct {
	CumulativeDistanceM float64
	ElapsedTimeSec      float64
}

// findFastestSegment scans through an activity's record-level, lap-level, or session-level data
// to find the minimum elapsed time for a contiguous segment covering targetDistanceM.
// Returns 0 if the activity doesn't cover the target distance.
//
// Fidelity levels (tried in order):
//  1. Record-level: Uses 1Hz speed data to build cumulative distance, then sliding window
//  2. Lap-level: Uses lap total_distance/total_elapsed_time with sliding window
//  3. Proportional fallback: Assumes even pacing across the entire activity
func findFastestSegment(activity *pb.StandardizedActivity, targetDistanceM float64) float64 {
	// Try record-level first (highest fidelity)
	if time := findFastestFromRecords(activity, targetDistanceM); time > 0 {
		return time
	}

	// Try lap-level (medium fidelity)
	if time := findFastestFromLaps(activity, targetDistanceM); time > 0 {
		return time
	}

	// Fallback: proportional extrapolation (lowest fidelity)
	return findFastestProportional(activity, targetDistanceM)
}

// findFastestFromRecords builds cumulative distance from 1Hz speed records and uses
// a sliding window to find the fastest contiguous segment.
func findFastestFromRecords(activity *pb.StandardizedActivity, targetDistanceM float64) float64 {
	points := buildDistanceTimePoints(activity)
	if len(points) < 2 {
		return 0
	}

	// Check total distance is sufficient
	totalDistance := points[len(points)-1].CumulativeDistanceM
	if totalDistance < targetDistanceM {
		return 0
	}

	return slidingWindowMinTime(points, targetDistanceM)
}

// buildDistanceTimePoints collects all records across sessions/laps and builds
// cumulative distance from speed × Δtime.
func buildDistanceTimePoints(activity *pb.StandardizedActivity) []distanceTimePoint {
	var points []distanceTimePoint

	var cumulativeDistance float64
	var cumulativeTime float64
	var hasSpeedData bool

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.Speed > 0 {
					hasSpeedData = true
				}
			}
		}
	}

	if !hasSpeedData {
		return nil
	}

	// Add initial zero point
	points = append(points, distanceTimePoint{0, 0})

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			var prevTimestamp int64
			for i, record := range lap.Records {
				ts := record.Timestamp.GetSeconds()
				if i == 0 && prevTimestamp == 0 && len(points) == 1 {
					// First record ever - just record the base timestamp
					prevTimestamp = ts
					continue
				}

				if prevTimestamp == 0 {
					prevTimestamp = ts
					continue
				}

				dt := float64(ts - prevTimestamp)
				if dt <= 0 {
					prevTimestamp = ts
					continue
				}

				// Distance = speed × time
				speed := record.Speed
				if speed < 0 {
					speed = 0
				}
				distDelta := speed * dt
				cumulativeDistance += distDelta
				cumulativeTime += dt

				points = append(points, distanceTimePoint{
					CumulativeDistanceM: cumulativeDistance,
					ElapsedTimeSec:      cumulativeTime,
				})

				prevTimestamp = ts
			}
		}
	}

	return points
}

// slidingWindowMinTime uses a two-pointer technique on cumulative distance/time points
// to find the minimum elapsed time for a contiguous segment covering targetDistanceM.
// It interpolates the exact start point for precision.
func slidingWindowMinTime(points []distanceTimePoint, targetDistanceM float64) float64 {
	minTime := math.MaxFloat64
	left := 0

	for right := 1; right < len(points); right++ {
		// Advance left pointer while the window still covers the target distance
		for left < right-1 {
			windowDist := points[right].CumulativeDistanceM - points[left+1].CumulativeDistanceM
			if windowDist >= targetDistanceM {
				left++
			} else {
				break
			}
		}

		// Check if current window covers target distance
		windowDist := points[right].CumulativeDistanceM - points[left].CumulativeDistanceM
		if windowDist >= targetDistanceM {
			// Interpolate the exact start point
			// We need exactly targetDistanceM ending at points[right]
			exactStartDist := points[right].CumulativeDistanceM - targetDistanceM

			// Find the interpolated start time
			startTime := interpolateTime(points, exactStartDist)
			endTime := points[right].ElapsedTimeSec

			elapsed := endTime - startTime
			if elapsed > 0 && elapsed < minTime {
				minTime = elapsed
			}
		}
	}

	if minTime == math.MaxFloat64 {
		return 0
	}
	return minTime
}

// interpolateTime finds the elapsed time at a given cumulative distance
// by interpolating between the surrounding data points.
func interpolateTime(points []distanceTimePoint, targetDist float64) float64 {
	// Binary search for the point just at or before targetDist
	idx := sort.Search(len(points), func(i int) bool {
		return points[i].CumulativeDistanceM > targetDist
	})

	if idx == 0 {
		return points[0].ElapsedTimeSec
	}
	if idx >= len(points) {
		return points[len(points)-1].ElapsedTimeSec
	}

	// Interpolate between points[idx-1] and points[idx]
	p1 := points[idx-1]
	p2 := points[idx]

	distRange := p2.CumulativeDistanceM - p1.CumulativeDistanceM
	if distRange <= 0 {
		return p1.ElapsedTimeSec
	}

	fraction := (targetDist - p1.CumulativeDistanceM) / distRange
	return p1.ElapsedTimeSec + fraction*(p2.ElapsedTimeSec-p1.ElapsedTimeSec)
}

// findFastestFromLaps uses lap-level distance/time data with a sliding window approach.
// Less precise than record-level but better than proportional.
func findFastestFromLaps(activity *pb.StandardizedActivity, targetDistanceM float64) float64 {
	// Build cumulative distance/time from laps
	var points []distanceTimePoint
	var cumulativeDistance float64
	var cumulativeTime float64
	var hasLapData bool

	points = append(points, distanceTimePoint{0, 0})

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			if lap.TotalDistance > 0 && lap.TotalElapsedTime > 0 {
				hasLapData = true
				cumulativeDistance += lap.TotalDistance
				cumulativeTime += lap.TotalElapsedTime

				points = append(points, distanceTimePoint{
					CumulativeDistanceM: cumulativeDistance,
					ElapsedTimeSec:      cumulativeTime,
				})
			}
		}
	}

	if !hasLapData || len(points) < 2 {
		return 0
	}

	// Only use lap-level if we have more than one lap (otherwise it's just proportional)
	if len(points) <= 2 {
		return 0
	}

	if cumulativeDistance < targetDistanceM {
		return 0
	}

	return slidingWindowMinTime(points, targetDistanceM)
}

// findFastestProportional estimates time using proportional extrapolation (assumes even pacing).
// This is the lowest fidelity fallback, equivalent to the old behaviour.
func findFastestProportional(activity *pb.StandardizedActivity, targetDistanceM float64) float64 {
	var totalDistanceM float64
	var totalDurationSec float64

	for _, session := range activity.Sessions {
		totalDistanceM += session.TotalDistance
		totalDurationSec += session.TotalElapsedTime
	}

	if totalDistanceM < targetDistanceM || totalDurationSec <= 0 {
		return 0
	}

	return (targetDistanceM / totalDistanceM) * totalDurationSec
}
