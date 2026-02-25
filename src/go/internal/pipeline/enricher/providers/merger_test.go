package providers

import (
	"log/slog"
	"testing"
	"time"
)

func TestAlignTimeSeries_PerfectMatch(t *testing.T) {
	// GPS and HR have identical timelines
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	gpsTimestamps := make([]time.Time, 60)
	hrSamples := make([]TimedSample, 60)

	for i := 0; i < 60; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Second)
		gpsTimestamps[i] = ts
		hrSamples[i] = TimedSample{Timestamp: ts, Value: 120 + i}
	}

	result, err := AlignTimeSeries(gpsTimestamps, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.AlignedHR) != 60 {
		t.Errorf("Expected 60 aligned samples, got %d", len(result.AlignedHR))
	}

	// Values should match exactly
	for i := 0; i < 60; i++ {
		if result.AlignedHR[i] != 120+i {
			t.Errorf("At index %d: expected %d, got %d", i, 120+i, result.AlignedHR[i])
		}
	}

	if result.DriftPercent > 0.01 {
		t.Errorf("Expected near-zero drift, got %.2f%%", result.DriftPercent)
	}

	if result.Metadata["alignment_status"] != "success" {
		t.Errorf("Expected status 'success', got '%s'", result.Metadata["alignment_status"])
	}
}

func TestAlignTimeSeries_SmallDrift_0_5_Percent(t *testing.T) {
	// HR is 0.5% shorter than GPS (1 hour GPS, 59.7 min HR)
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	gpsDuration := 3600 * time.Second // 1 hour
	hrDuration := 3582 * time.Second  // 0.5% shorter

	gpsTimestamps := make([]time.Time, 3600)
	for i := 0; i < 3600; i++ {
		gpsTimestamps[i] = baseTime.Add(time.Duration(i) * time.Second)
	}

	// HR samples at same intervals but shorter total duration
	hrSamples := make([]TimedSample, 3582)
	for i := 0; i < 3582; i++ {
		hrSamples[i] = TimedSample{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     120,
		}
	}

	result, err := AlignTimeSeries(gpsTimestamps, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Drift should be ~0.5%
	if result.DriftPercent < 0.4 || result.DriftPercent > 0.6 {
		t.Errorf("Expected drift around 0.5%%, got %.2f%%", result.DriftPercent)
	}

	// Should be success (under 1% threshold)
	if result.Metadata["alignment_status"] != "success" {
		t.Errorf("Expected status 'success', got '%s'", result.Metadata["alignment_status"])
	}

	// All GPS timestamps should have aligned HR values
	if len(result.AlignedHR) != 3600 {
		t.Errorf("Expected 3600 aligned samples, got %d", len(result.AlignedHR))
	}

	// Check we don't have zeros (all should be 120)
	for i, val := range result.AlignedHR {
		if val == 0 {
			t.Errorf("Unexpected zero at index %d", i)
			break
		}
	}

	_ = gpsDuration
	_ = hrDuration
}

func TestAlignTimeSeries_LargeDrift_2_Percent(t *testing.T) {
	// HR is 2% shorter than GPS
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	gpsTimestamps := make([]time.Time, 1000)
	for i := 0; i < 1000; i++ {
		gpsTimestamps[i] = baseTime.Add(time.Duration(i) * time.Second)
	}

	// HR is 2% shorter (980 seconds instead of 1000)
	hrSamples := make([]TimedSample, 980)
	for i := 0; i < 980; i++ {
		hrSamples[i] = TimedSample{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     130,
		}
	}

	result, err := AlignTimeSeries(gpsTimestamps, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Drift should be ~2%
	if result.DriftPercent < 1.9 || result.DriftPercent > 2.1 {
		t.Errorf("Expected drift around 2%%, got %.2f%%", result.DriftPercent)
	}

	// Should have warning (over 1% threshold)
	if result.WarningMessage == "" {
		t.Error("Expected warning message for high drift")
	}

	if result.Metadata["alignment_status"] != "high_drift_best_effort" {
		t.Errorf("Expected status 'high_drift_best_effort', got '%s'", result.Metadata["alignment_status"])
	}

	// Should still produce aligned values
	if len(result.AlignedHR) != 1000 {
		t.Errorf("Expected 1000 aligned samples, got %d", len(result.AlignedHR))
	}
}

func TestAlignTimeSeries_OffsetStart(t *testing.T) {
	// HR starts 30s before GPS
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// GPS starts at 10:00:30
	gpsTimestamps := make([]time.Time, 60)
	for i := 0; i < 60; i++ {
		gpsTimestamps[i] = baseTime.Add(30*time.Second + time.Duration(i)*time.Second)
	}

	// HR starts at 10:00:00 (30s earlier)
	hrSamples := make([]TimedSample, 90) // Extra data before GPS start
	for i := 0; i < 90; i++ {
		hrSamples[i] = TimedSample{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     100 + i,
		}
	}

	result, err := AlignTimeSeries(gpsTimestamps, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.AlignedHR) != 60 {
		t.Errorf("Expected 60 aligned samples, got %d", len(result.AlignedHR))
	}

	// First GPS timestamp (10:00:30) should align with HR values around index 30
	// Due to elastic matching, the exact values depend on the algorithm
	// Just verify we get reasonable values (not zeros)
	if result.AlignedHR[0] == 0 {
		t.Error("Expected non-zero HR value at start")
	}
}

func TestAlignTimeSeries_MissingStartHR(t *testing.T) {
	// HR data starts 1 minute late
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// GPS from 10:00:00 to 10:05:00 (5 minutes)
	gpsTimestamps := make([]time.Time, 300)
	for i := 0; i < 300; i++ {
		gpsTimestamps[i] = baseTime.Add(time.Duration(i) * time.Second)
	}

	// HR from 10:01:00 to 10:05:00 (4 minutes, missing first minute)
	hrSamples := make([]TimedSample, 240)
	for i := 0; i < 240; i++ {
		hrSamples[i] = TimedSample{
			Timestamp: baseTime.Add(60*time.Second + time.Duration(i)*time.Second),
			Value:     140,
		}
	}

	result, err := AlignTimeSeries(gpsTimestamps, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should still have values for all GPS timestamps (forward fill for early ones)
	if len(result.AlignedHR) != 300 {
		t.Errorf("Expected 300 aligned samples, got %d", len(result.AlignedHR))
	}

	// First values should be forward-filled with the first available HR
	if result.AlignedHR[0] != 140 {
		t.Errorf("Expected forward-filled value 140, got %d", result.AlignedHR[0])
	}
}

func TestAlignTimeSeries_MissingEndHR(t *testing.T) {
	// HR data ends 1 minute early
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// GPS from 10:00:00 to 10:05:00 (5 minutes)
	gpsTimestamps := make([]time.Time, 300)
	for i := 0; i < 300; i++ {
		gpsTimestamps[i] = baseTime.Add(time.Duration(i) * time.Second)
	}

	// HR from 10:00:00 to 10:04:00 (4 minutes, missing last minute)
	hrSamples := make([]TimedSample, 240)
	for i := 0; i < 240; i++ {
		hrSamples[i] = TimedSample{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     150,
		}
	}

	result, err := AlignTimeSeries(gpsTimestamps, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.AlignedHR) != 300 {
		t.Errorf("Expected 300 aligned samples, got %d", len(result.AlignedHR))
	}

	// Last values should be backward-filled with the last available HR
	if result.AlignedHR[299] != 150 {
		t.Errorf("Expected backward-filled value 150, got %d", result.AlignedHR[299])
	}
}

func TestAlignTimeSeries_EmptyHR(t *testing.T) {
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	gpsTimestamps := make([]time.Time, 60)
	for i := 0; i < 60; i++ {
		gpsTimestamps[i] = baseTime.Add(time.Duration(i) * time.Second)
	}

	result, err := AlignTimeSeries(gpsTimestamps, []TimedSample{}, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Metadata["alignment_status"] != "skipped_no_hr" {
		t.Errorf("Expected status 'skipped_no_hr', got '%s'", result.Metadata["alignment_status"])
	}

	if result.WarningMessage == "" {
		t.Error("Expected warning message for empty HR")
	}

	// Should return zeros for all positions
	for i, val := range result.AlignedHR {
		if val != 0 {
			t.Errorf("Expected zero at index %d, got %d", i, val)
		}
	}
}

func TestAlignTimeSeries_EmptyGPS(t *testing.T) {
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	hrSamples := make([]TimedSample, 60)
	for i := 0; i < 60; i++ {
		hrSamples[i] = TimedSample{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     120,
		}
	}

	result, err := AlignTimeSeries([]time.Time{}, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Metadata["alignment_status"] != "skipped_no_gps" {
		t.Errorf("Expected status 'skipped_no_gps', got '%s'", result.Metadata["alignment_status"])
	}

	if len(result.AlignedHR) != 0 {
		t.Errorf("Expected empty aligned HR, got %d samples", len(result.AlignedHR))
	}
}

func TestLinearInterpolation(t *testing.T) {
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	samples := []TimedSample{
		{Timestamp: baseTime, Value: 100},
		{Timestamp: baseTime.Add(10 * time.Second), Value: 200},
	}

	// Test exact match at start
	val := interpolateHR(samples, baseTime)
	if val != 100 {
		t.Errorf("Expected 100 at start, got %d", val)
	}

	// Test exact match at end
	val = interpolateHR(samples, baseTime.Add(10*time.Second))
	if val != 200 {
		t.Errorf("Expected 200 at end, got %d", val)
	}

	// Test midpoint (should be 150)
	val = interpolateHR(samples, baseTime.Add(5*time.Second))
	if val != 150 {
		t.Errorf("Expected 150 at midpoint, got %d", val)
	}

	// Test quarter point (should be 125)
	val = interpolateHR(samples, baseTime.Add(2500*time.Millisecond))
	if val != 125 {
		t.Errorf("Expected 125 at quarter point, got %d", val)
	}

	// Test before first sample (forward fill)
	val = interpolateHR(samples, baseTime.Add(-5*time.Second))
	if val != 100 {
		t.Errorf("Expected 100 before start (forward fill), got %d", val)
	}

	// Test after last sample (backward fill)
	val = interpolateHR(samples, baseTime.Add(15*time.Second))
	if val != 200 {
		t.Errorf("Expected 200 after end (backward fill), got %d", val)
	}
}

func TestAlignTimeSeries_Gaps(t *testing.T) {
	// HR has a 30-second gap in the middle
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// GPS continuous for 2 minutes
	gpsTimestamps := make([]time.Time, 120)
	for i := 0; i < 120; i++ {
		gpsTimestamps[i] = baseTime.Add(time.Duration(i) * time.Second)
	}

	// HR has first 30 seconds, gap, then last 60 seconds
	hrSamples := make([]TimedSample, 0)
	for i := 0; i < 30; i++ {
		hrSamples = append(hrSamples, TimedSample{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     120,
		})
	}
	// 30-second gap (no samples from 30-60)
	for i := 60; i < 120; i++ {
		hrSamples = append(hrSamples, TimedSample{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     140,
		})
	}

	result, err := AlignTimeSeries(gpsTimestamps, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.AlignedHR) != 120 {
		t.Errorf("Expected 120 aligned samples, got %d", len(result.AlignedHR))
	}

	// All values should be filled (no zeros)
	for i, val := range result.AlignedHR {
		if val == 0 {
			t.Errorf("Unexpected zero at index %d", i)
		}
	}

	// During the gap, interpolation should provide values between 120 and 140
	// The exact values depend on the elastic matching algorithm
	// Just verify they're reasonable
	if result.AlignedHR[45] < 120 || result.AlignedHR[45] > 140 {
		t.Errorf("Gap value out of expected range: %d", result.AlignedHR[45])
	}
}

func TestFindSampleBefore(t *testing.T) {
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	samples := []TimedSample{
		{Timestamp: baseTime, Value: 100},
		{Timestamp: baseTime.Add(10 * time.Second), Value: 110},
		{Timestamp: baseTime.Add(20 * time.Second), Value: 120},
		{Timestamp: baseTime.Add(30 * time.Second), Value: 130},
	}

	// Exact match - first
	idx := findSampleBefore(samples, baseTime)
	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}

	// Exact match - last
	idx = findSampleBefore(samples, baseTime.Add(30*time.Second))
	if idx != 3 {
		t.Errorf("Expected index 3, got %d", idx)
	}

	// Between samples
	idx = findSampleBefore(samples, baseTime.Add(15*time.Second))
	if idx != 1 {
		t.Errorf("Expected index 1 (at 10s), got %d", idx)
	}

	// Before first
	idx = findSampleBefore(samples, baseTime.Add(-5*time.Second))
	if idx != 0 {
		t.Errorf("Expected index 0 for before first, got %d", idx)
	}
}

func TestConvertHRResponseToSamples(t *testing.T) {
	baseDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	dataset := []struct {
		Time  string `json:"time"`
		Value int    `json:"value"`
	}{
		{Time: "10:00:00", Value: 120},
		{Time: "10:00:30", Value: 125},
		{Time: "10:01:00", Value: 130},
	}

	samples := ConvertHRResponseToSamples(dataset, baseDate)

	if len(samples) != 3 {
		t.Fatalf("Expected 3 samples, got %d", len(samples))
	}

	// Check first sample
	expected := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	if !samples[0].Timestamp.Equal(expected) {
		t.Errorf("Expected timestamp %v, got %v", expected, samples[0].Timestamp)
	}
	if samples[0].Value != 120 {
		t.Errorf("Expected value 120, got %d", samples[0].Value)
	}

	// Check second sample
	expected = time.Date(2026, 1, 15, 10, 0, 30, 0, time.UTC)
	if !samples[1].Timestamp.Equal(expected) {
		t.Errorf("Expected timestamp %v, got %v", expected, samples[1].Timestamp)
	}
	if samples[1].Value != 125 {
		t.Errorf("Expected value 125, got %d", samples[1].Value)
	}
}

func TestAlignTimeSeries_RealWorldDrift(t *testing.T) {
	// Simulate realistic scenario: 45-minute run
	// Phone GPS started 3 seconds before Fitbit HR (common with device startup times)
	// HR clock runs 0.3% faster than GPS clock

	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	gpsDurationSec := 2700 // 45 minutes

	gpsTimestamps := make([]time.Time, gpsDurationSec)
	for i := 0; i < gpsDurationSec; i++ {
		gpsTimestamps[i] = baseTime.Add(time.Duration(i) * time.Second)
	}

	// HR starts 3 seconds later and runs 0.3% faster
	hrStartOffset := 3 * time.Second
	hrDurationSec := int(float64(gpsDurationSec) * 0.997) // 0.3% shorter

	hrSamples := make([]TimedSample, hrDurationSec)
	for i := 0; i < hrDurationSec; i++ {
		hrSamples[i] = TimedSample{
			Timestamp: baseTime.Add(hrStartOffset + time.Duration(i)*time.Second),
			Value:     140 + (i % 20), // Varying HR for realism
		}
	}

	result, err := AlignTimeSeries(gpsTimestamps, hrSamples, DefaultAlignmentConfig, slog.Default())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should successfully align with <1% drift triggering success
	if result.DriftPercent > 0.5 {
		t.Errorf("Expected drift <0.5%%, got %.2f%%", result.DriftPercent)
	}

	if result.Metadata["alignment_status"] != "success" {
		t.Errorf("Expected status 'success', got '%s'", result.Metadata["alignment_status"])
	}

	// All aligned values should be non-zero (proper forward/backward fill)
	for i, val := range result.AlignedHR {
		if val == 0 {
			t.Errorf("Unexpected zero at index %d", i)
			break
		}
	}

	// Check that values are in expected range
	for i, val := range result.AlignedHR {
		if val < 140 || val > 160 {
			t.Errorf("Value at index %d out of expected range: %d", i, val)
			break
		}
	}
}
