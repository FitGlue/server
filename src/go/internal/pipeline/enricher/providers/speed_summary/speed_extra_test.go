package speed_summary

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
)

// --- calculateSpeedConsistency ---

func TestCalculateSpeedConsistency_TooFewSpeeds(t *testing.T) {
	result := calculateSpeedConsistency([]float64{3.0}, 3.0)
	if result != 0 {
		t.Errorf("expected 0 for < 2 speeds, got %v", result)
	}
}

func TestCalculateSpeedConsistency_ZeroMean(t *testing.T) {
	result := calculateSpeedConsistency([]float64{3.0, 4.0}, 0)
	if result != 0 {
		t.Errorf("expected 0 for zero mean, got %v", result)
	}
}

func TestCalculateSpeedConsistency_ConstantSpeed(t *testing.T) {
	// All same speed -> std dev = 0 -> consistency = 100
	speeds := []float64{3.0, 3.0, 3.0, 3.0, 3.0}
	result := calculateSpeedConsistency(speeds, 3.0)
	if result != 100 {
		t.Errorf("expected 100 for constant speed, got %v", result)
	}
}

func TestCalculateSpeedConsistency_HighVariance(t *testing.T) {
	// Very different speeds -> low consistency
	speeds := []float64{1.0, 10.0, 1.0, 10.0, 1.0, 10.0}
	mean := 5.5
	result := calculateSpeedConsistency(speeds, mean)
	if result > 50 {
		t.Errorf("expected low consistency for high variance, got %v", result)
	}
}

func TestCalculateSpeedConsistency_NeverNegative(t *testing.T) {
	// Extremely high variance should be capped at 0
	speeds := []float64{0.1, 100.0, 0.1, 100.0}
	result := calculateSpeedConsistency(speeds, 50.0)
	if result < 0 {
		t.Errorf("consistency should never be negative, got %v", result)
	}
}

// --- Enrich: show_analysis path ---

func makeActWithSpeeds(speeds []float64) *pbactivity.StandardizedActivity {
	records := make([]*pbactivity.Record, len(speeds))
	for i, s := range speeds {
		records[i] = &pbactivity.Record{Speed: s}
	}
	return &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{Records: records}},
		}},
	}
}

func TestSpeedSummary_ShowAnalysis_VeryConsistent(t *testing.T) {
	p := NewSpeedSummary()
	// 15 identical speeds -> very consistent
	speeds := make([]float64, 15)
	for i := range speeds {
		speeds[i] = 3.0
	}
	act := makeActWithSpeeds(speeds)
	inputs := map[string]string{"show_analysis": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "Consistency") {
		t.Errorf("expected Consistency in analysis output, got %q", res.Description)
	}
	if !strings.Contains(res.Description, "very consistent") {
		t.Errorf("expected 'very consistent' label, got %q", res.Description)
	}
}

func TestSpeedSummary_ShowAnalysis_ModerateVariance(t *testing.T) {
	p := NewSpeedSummary()
	// 15 values with moderate variance
	speeds := make([]float64, 15)
	for i := range speeds {
		speeds[i] = 3.0 + float64(i%4)*0.3
	}
	act := makeActWithSpeeds(speeds)
	inputs := map[string]string{"show_analysis": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = res // just verify no crash and valid result returned
}

func TestSpeedSummary_ShowAnalysis_FewPoints(t *testing.T) {
	p := NewSpeedSummary()
	// Only 5 speed records (< 10) -> simple format
	speeds := []float64{3.0, 3.5, 4.0, 2.8, 3.2}
	act := makeActWithSpeeds(speeds)
	inputs := map[string]string{"show_analysis": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With < 10 points, we get the simple format (no "Consistency" line)
	if strings.Contains(res.Description, "Consistency") {
		t.Errorf("expected simple format for < 10 speed points, got %q", res.Description)
	}
}

func TestSpeedSummary_NoSpeedData(t *testing.T) {
	p := NewSpeedSummary()
	act := &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{
				Records: []*pbactivity.Record{{Cadence: 180}}, // no speed
			}},
		}},
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metadata["speed_summary_status"] != "skipped" {
		t.Errorf("expected skipped, got %q", res.Metadata["speed_summary_status"])
	}
}
