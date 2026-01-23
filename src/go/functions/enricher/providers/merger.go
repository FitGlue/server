package providers

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"
)

// TimedSample represents a single data point with timestamp
type TimedSample struct {
	Timestamp time.Time
	Value     int
}

// AlignmentResult contains the merged HR data aligned to GPS timestamps
type AlignmentResult struct {
	AlignedHR      []int             // HR values aligned to target timestamps
	DriftPercent   float64           // Duration difference percentage
	WarningMessage string            // If drift > threshold
	Metadata       map[string]string // Alignment metadata for logging
}

// AlignmentConfig contains parameters for alignment
type AlignmentConfig struct {
	MaxDriftPercent float64       // Threshold for warning (default: 1.0 = 1%)
	TargetAccuracy  time.Duration // Target accuracy for alignment (default: 2s)
}

// DefaultAlignmentConfig provides sensible defaults
var DefaultAlignmentConfig = AlignmentConfig{
	MaxDriftPercent: 1.0,
	TargetAccuracy:  2 * time.Second,
}

// AlignTimeSeries performs the "Elastic Match" alignment between GPS timestamps and HR samples.
// It aligns HR data to the GPS timeline, handling clock drift between devices.
//
// Algorithm:
// 1. Align start times of both streams
// 2. Calculate duration difference (drift)
// 3. If drift < MaxDriftPercent: apply elastic stretch/compress with linear interpolation
// 4. If drift >= MaxDriftPercent: still apply alignment but log warning
// 5. Handle edge cases (missing data at start/end, gaps)
func AlignTimeSeries(gpsTimestamps []time.Time, hrSamples []TimedSample, config AlignmentConfig) (*AlignmentResult, error) {
	result := &AlignmentResult{
		AlignedHR: make([]int, len(gpsTimestamps)),
		Metadata:  make(map[string]string),
	}

	// Edge case: No GPS timestamps
	if len(gpsTimestamps) == 0 {
		result.Metadata["alignment_status"] = "skipped_no_gps"
		slog.Info("HR alignment skipped: no GPS timestamps provided")
		return result, nil
	}

	// Edge case: No HR samples
	if len(hrSamples) == 0 {
		result.Metadata["alignment_status"] = "skipped_no_hr"
		result.WarningMessage = "No HR data available for alignment"
		slog.Warn("HR alignment skipped: no HR samples provided")
		return result, nil
	}

	// Sort samples by timestamp to ensure correct ordering
	sortedHR := make([]TimedSample, len(hrSamples))
	copy(sortedHR, hrSamples)
	sort.Slice(sortedHR, func(i, j int) bool {
		return sortedHR[i].Timestamp.Before(sortedHR[j].Timestamp)
	})

	sortedGPS := make([]time.Time, len(gpsTimestamps))
	copy(sortedGPS, gpsTimestamps)
	sort.Slice(sortedGPS, func(i, j int) bool {
		return sortedGPS[i].Before(sortedGPS[j])
	})

	// Calculate durations
	gpsStart := sortedGPS[0]
	gpsEnd := sortedGPS[len(sortedGPS)-1]
	gpsDuration := gpsEnd.Sub(gpsStart)

	hrStart := sortedHR[0].Timestamp
	hrEnd := sortedHR[len(sortedHR)-1].Timestamp
	hrDuration := hrEnd.Sub(hrStart)

	// Calculate drift percentage
	if gpsDuration > 0 {
		driftDuration := math.Abs(float64(gpsDuration - hrDuration))
		result.DriftPercent = (driftDuration / float64(gpsDuration)) * 100
	}

	// Log drift detection
	result.Metadata["gps_duration_sec"] = fmt.Sprintf("%.1f", gpsDuration.Seconds())
	result.Metadata["hr_duration_sec"] = fmt.Sprintf("%.1f", hrDuration.Seconds())
	result.Metadata["drift_percent"] = fmt.Sprintf("%.2f", result.DriftPercent)
	result.Metadata["gps_samples"] = fmt.Sprintf("%d", len(sortedGPS))
	result.Metadata["hr_samples"] = fmt.Sprintf("%d", len(sortedHR))

	// Check drift threshold
	if result.DriftPercent > config.MaxDriftPercent {
		result.WarningMessage = fmt.Sprintf("Clock drift of %.2f%% detected (threshold: %.2f%%), applying best-effort alignment", result.DriftPercent, config.MaxDriftPercent)
		slog.Warn("High clock drift detected during HR alignment",
			"drift_percent", result.DriftPercent,
			"threshold_percent", config.MaxDriftPercent,
			"gps_duration_sec", gpsDuration.Seconds(),
			"hr_duration_sec", hrDuration.Seconds(),
		)
		result.Metadata["alignment_status"] = "high_drift_best_effort"
	} else {
		result.Metadata["alignment_status"] = "success"
	}

	// Calculate scale factor for elastic stretch/compress
	// scaleFactor > 1 means HR is shorter than GPS (stretch HR)
	// scaleFactor < 1 means HR is longer than GPS (compress HR)
	var scaleFactor float64 = 1.0
	if hrDuration > 0 {
		scaleFactor = float64(gpsDuration) / float64(hrDuration)
	}
	result.Metadata["scale_factor"] = fmt.Sprintf("%.4f", scaleFactor)

	// Align each GPS timestamp to an HR value
	for i, gpsTime := range sortedGPS {
		// Calculate the relative position in GPS timeline (0.0 to 1.0)
		var relativePos float64 = 0.0
		if gpsDuration > 0 {
			relativePos = float64(gpsTime.Sub(gpsStart)) / float64(gpsDuration)
		}

		// Map to corresponding position in HR timeline
		hrRelativeTime := time.Duration(float64(hrDuration) * relativePos)
		targetHRTime := hrStart.Add(hrRelativeTime)

		// Interpolate HR value at this target time
		hrValue := interpolateHR(sortedHR, targetHRTime)
		result.AlignedHR[i] = hrValue
	}

	slog.Info("HR alignment completed",
		"gps_duration_sec", gpsDuration.Seconds(),
		"hr_duration_sec", hrDuration.Seconds(),
		"drift_percent", result.DriftPercent,
		"samples_aligned", len(result.AlignedHR),
		"scale_factor", scaleFactor,
	)

	return result, nil
}

// interpolateHR finds the HR value at a specific target time using linear interpolation.
// If the target time is before the first sample, returns the first sample's value.
// If the target time is after the last sample, returns the last sample's value.
// Otherwise, linearly interpolates between the two surrounding samples.
func interpolateHR(samples []TimedSample, targetTime time.Time) int {
	if len(samples) == 0 {
		return 0
	}

	// Before first sample - forward fill
	if targetTime.Before(samples[0].Timestamp) || targetTime.Equal(samples[0].Timestamp) {
		return samples[0].Value
	}

	// After last sample - backward fill
	lastIdx := len(samples) - 1
	if targetTime.After(samples[lastIdx].Timestamp) || targetTime.Equal(samples[lastIdx].Timestamp) {
		return samples[lastIdx].Value
	}

	// Find surrounding samples using binary search
	beforeIdx := findSampleBefore(samples, targetTime)
	afterIdx := beforeIdx + 1

	if afterIdx >= len(samples) {
		return samples[beforeIdx].Value
	}

	before := samples[beforeIdx]
	after := samples[afterIdx]

	// If timestamps are the same (shouldn't happen but be safe)
	if after.Timestamp.Equal(before.Timestamp) {
		return before.Value
	}

	// Linear interpolation
	totalDuration := float64(after.Timestamp.Sub(before.Timestamp))
	elapsed := float64(targetTime.Sub(before.Timestamp))
	ratio := elapsed / totalDuration

	interpolatedValue := float64(before.Value) + ratio*float64(after.Value-before.Value)
	return int(math.Round(interpolatedValue))
}

// findSampleBefore returns the index of the sample immediately before or at the target time.
// Uses binary search for efficiency.
func findSampleBefore(samples []TimedSample, targetTime time.Time) int {
	left, right := 0, len(samples)-1

	for left < right {
		mid := (left + right + 1) / 2
		if samples[mid].Timestamp.After(targetTime) {
			right = mid - 1
		} else {
			left = mid
		}
	}

	return left
}

// ConvertHRResponseToSamples converts the Fitbit API response format to TimedSamples.
// The baseDate is the date of the activity (used to construct full timestamps).
func ConvertHRResponseToSamples(dataset []struct {
	Time  string `json:"time"`
	Value int    `json:"value"`
}, baseDate time.Time) []TimedSample {
	samples := make([]TimedSample, 0, len(dataset))

	for _, point := range dataset {
		// Parse time in "15:04:05" format
		ptTime, err := time.Parse("15:04:05", point.Time)
		if err != nil {
			slog.Warn("Failed to parse HR timestamp", "time", point.Time, "error", err)
			continue
		}

		// Combine with base date
		fullTime := time.Date(
			baseDate.Year(), baseDate.Month(), baseDate.Day(),
			ptTime.Hour(), ptTime.Minute(), ptTime.Second(), 0,
			baseDate.Location(),
		)

		samples = append(samples, TimedSample{
			Timestamp: fullTime,
			Value:     point.Value,
		})
	}

	return samples
}
