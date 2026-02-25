package power_summary

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
)

// --- calculatePeakPower ---

func TestCalculatePeakPower_TooFewSamples(t *testing.T) {
	powers := []int32{200, 220}
	result := calculatePeakPower(powers, 5)
	if result != 0 {
		t.Errorf("expected 0 when fewer samples than duration, got %d", result)
	}
}

func TestCalculatePeakPower_ExactWindow(t *testing.T) {
	powers := []int32{200, 200, 200, 200, 200} // 5 values, window=5
	result := calculatePeakPower(powers, 5)
	if result != 200 {
		t.Errorf("expected 200W avg for constant 200W power, got %d", result)
	}
}

func TestCalculatePeakPower_SlidingWindow(t *testing.T) {
	// Peak is the last 3 values
	powers := []int32{100, 100, 100, 300, 300, 300}
	result := calculatePeakPower(powers, 3)
	if result != 300 {
		t.Errorf("expected 300W for peak 3s, got %d", result)
	}
}

// --- Enrich: show_curve path ---

func makeActWithPowers(powers []int32) *pbactivity.StandardizedActivity {
	records := make([]*pbactivity.Record, len(powers))
	for i, p := range powers {
		records[i] = &pbactivity.Record{Power: p}
	}
	return &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{Records: records}},
		}},
	}
}

func TestPowerSummary_ShowCurve_WithPeaks(t *testing.T) {
	p := NewPowerSummary()
	// 300 values to test all peak intervals (5s, 60s, 300s)
	powers := make([]int32, 300)
	for i := range powers {
		powers[i] = 250
	}
	act := makeActWithPowers(powers)
	inputs := map[string]string{"show_curve": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "Peak 5s") {
		t.Errorf("expected Peak 5s in description, got %q", res.Description)
	}
	if !strings.Contains(res.Description, "Peak 1m") {
		t.Errorf("expected Peak 1m in description, got %q", res.Description)
	}
}

func TestPowerSummary_ShowCurve_FewPoints(t *testing.T) {
	p := NewPowerSummary()
	// Only 3 power records (< 5) -> simple format
	powers := []int32{200, 220, 210}
	act := makeActWithPowers(powers)
	inputs := map[string]string{"show_curve": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// < 5 points: simple format, no "Peak" data
	if strings.Contains(res.Description, "Peak") {
		t.Errorf("expected simple format for < 5 power points, got %q", res.Description)
	}
}

func TestPowerSummary_NoPowerData(t *testing.T) {
	p := NewPowerSummary()
	act := &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{
				Records: []*pbactivity.Record{{Speed: 3.0}}, // no power
			}},
		}},
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metadata["power_summary_status"] != "skipped" {
		t.Errorf("expected skipped, got %q", res.Metadata["power_summary_status"])
	}
}

func TestPowerSummary_ShowCurve_With20MinPower_ShowsFTP(t *testing.T) {
	p := NewPowerSummary()
	// 1200 values (20 minutes) -> 20m peak should trigger FTP estimate
	powers := make([]int32, 1200)
	for i := range powers {
		powers[i] = 300
	}
	act := makeActWithPowers(powers)
	inputs := map[string]string{"show_curve": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "Est. FTP") {
		t.Errorf("expected Est. FTP in description for 20min data, got %q", res.Description)
	}
}
