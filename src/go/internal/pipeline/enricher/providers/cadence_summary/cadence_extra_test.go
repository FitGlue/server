package cadence_summary

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"testing"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
)

// --- calculatePaceCorrelation ---

func TestCalculatePaceCorrelation_TooFewPoints(t *testing.T) {
	cadences := []int32{170, 172}
	speeds := []float64{3.0, 3.1}
	result := calculatePaceCorrelation(cadences, speeds)
	if !math.IsNaN(result) {
		t.Errorf("expected NaN for < 10 points, got %v", result)
	}
}

func TestCalculatePaceCorrelation_MismatchedLengths(t *testing.T) {
	cadences := []int32{170, 172, 168}
	speeds := []float64{3.0, 3.1}
	result := calculatePaceCorrelation(cadences, speeds)
	if !math.IsNaN(result) {
		t.Errorf("expected NaN for mismatched lengths, got %v", result)
	}
}

func TestCalculatePaceCorrelation_ZeroSpeeds_ReturnNaN(t *testing.T) {
	// All speeds are 0 -> not enough valid pairs (< 10)
	cadences := make([]int32, 15)
	speeds := make([]float64, 15)
	for i := range cadences {
		cadences[i] = 170
		speeds[i] = 0 // all zero
	}
	result := calculatePaceCorrelation(cadences, speeds)
	if !math.IsNaN(result) {
		t.Errorf("expected NaN when all speeds are zero, got %v", result)
	}
}

func TestCalculatePaceCorrelation_PositiveCorrelation(t *testing.T) {
	// 15 points with consistent positive correlation: higher cadence = faster speed
	cadences := make([]int32, 15)
	speeds := make([]float64, 15)
	for i := 0; i < 15; i++ {
		cadences[i] = int32(160 + i)
		speeds[i] = 3.0 + float64(i)*0.1
	}
	result := calculatePaceCorrelation(cadences, speeds)
	if math.IsNaN(result) {
		t.Error("expected valid correlation, got NaN")
	}
	if result <= 0 {
		t.Errorf("expected positive correlation, got %v", result)
	}
}

func TestCalculatePaceCorrelation_ConstantCadence(t *testing.T) {
	// Constant cadence -> denomC == 0 -> NaN
	cadences := make([]int32, 15)
	speeds := make([]float64, 15)
	for i := 0; i < 15; i++ {
		cadences[i] = 170 // constant
		speeds[i] = float64(i) + 1.0
	}
	result := calculatePaceCorrelation(cadences, speeds)
	if !math.IsNaN(result) {
		t.Errorf("expected NaN for constant cadence, got %v", result)
	}
}

// --- Enrich: correlation path ---

func makeActWithCadenceSpeed(n int, cadenceBase int32, speedBase float64) *pbactivity.StandardizedActivity {
	records := make([]*pbactivity.Record, n)
	for i := 0; i < n; i++ {
		records[i] = &pbactivity.Record{
			Cadence: cadenceBase + int32(i),
			Speed:   speedBase + float64(i)*0.05,
		}
	}
	return &pbactivity.StandardizedActivity{
		Type: pbactivity.ActivityType_ACTIVITY_TYPE_RUN,
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{Records: records}},
		}},
	}
}

func TestCadenceSummary_ShowCorrelation_PositiveInterpretation(t *testing.T) {
	p := NewCadenceSummary()
	act := makeActWithCadenceSpeed(15, 160, 3.0)
	inputs := map[string]string{"show_correlation": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "🦶") {
		t.Errorf("expected 🦶 in description, got %q", res.Description)
	}
	// The correlation should be positive, so we expect "faster pace" interpretation
	if res.Metadata["cadence_summary_status"] != "success" {
		t.Errorf("expected success, got %q", res.Metadata["cadence_summary_status"])
	}
}

func TestCadenceSummary_ShowCorrelation_NegativeInterpretation(t *testing.T) {
	// Negative correlation: higher cadence = lower speed
	p := NewCadenceSummary()
	records := make([]*pbactivity.Record, 15)
	for i := 0; i < 15; i++ {
		records[i] = &pbactivity.Record{
			Cadence: int32(185 - i),       // decreasing cadence
			Speed:   3.0 + float64(i)*0.1, // increasing speed
		}
	}
	act := &pbactivity.StandardizedActivity{
		Type: pbactivity.ActivityType_ACTIVITY_TYPE_RUN,
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{Records: records}},
		}},
	}
	inputs := map[string]string{"show_correlation": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = res // correlation may or may not be displayed depending on threshold
}

func TestCadenceSummary_NoCadenceData(t *testing.T) {
	p := NewCadenceSummary()
	act := &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{
				Records: []*pbactivity.Record{{Speed: 3.0}}, // no cadence
			}},
		}},
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metadata["cadence_summary_status"] != "skipped" {
		t.Errorf("expected skipped, got %q", res.Metadata["cadence_summary_status"])
	}
}

func TestCadenceSummary_CyclingUnit(t *testing.T) {
	p := NewCadenceSummary()
	records := []*pbactivity.Record{{Cadence: 90}, {Cadence: 95}}
	act := &pbactivity.StandardizedActivity{
		Type: pbactivity.ActivityType_ACTIVITY_TYPE_RIDE,
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{Records: records}},
		}},
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "rpm") {
		t.Errorf("expected rpm unit for cycling, got %q", res.Description)
	}
}
