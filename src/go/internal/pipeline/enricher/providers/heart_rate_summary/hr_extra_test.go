package heart_rate_summary

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
)

func makeActWithHeartRates(heartRates []int32) *pbactivity.StandardizedActivity {
	records := make([]*pbactivity.Record, len(heartRates))
	for i, hr := range heartRates {
		records[i] = &pbactivity.Record{HeartRate: hr}
	}
	return &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{Records: records}},
		}},
	}
}

// --- Enrich: show_drift path ---

func TestHeartRateSummary_ShowDrift_PositiveDrift(t *testing.T) {
	p := NewHeartRateSummary()
	// 30 samples: first half low HR, second half high HR -> positive drift
	heartRates := make([]int32, 30)
	for i := 0; i < 15; i++ {
		heartRates[i] = 130
	}
	for i := 15; i < 30; i++ {
		heartRates[i] = 155 // +25 bpm -> drift > 5
	}
	act := makeActWithHeartRates(heartRates)
	inputs := map[string]string{"show_drift": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "Drift") {
		t.Errorf("expected 'Drift' in description for significant positive drift, got %q", res.Description)
	}
	if !strings.Contains(res.Description, "hydration") {
		t.Errorf("expected hydration note for positive drift, got %q", res.Description)
	}
}

func TestHeartRateSummary_ShowDrift_NegativeDrift(t *testing.T) {
	p := NewHeartRateSummary()
	// 30 samples: first half high HR, second half low HR -> negative drift
	heartRates := make([]int32, 30)
	for i := 0; i < 15; i++ {
		heartRates[i] = 155
	}
	for i := 15; i < 30; i++ {
		heartRates[i] = 130 // -25 bpm
	}
	act := makeActWithHeartRates(heartRates)
	inputs := map[string]string{"show_drift": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "warm-up") {
		t.Errorf("expected warm-up note for negative drift, got %q", res.Description)
	}
}

func TestHeartRateSummary_ShowDrift_NoDrift(t *testing.T) {
	p := NewHeartRateSummary()
	// 30 identical samples -> drift ≈ 0
	heartRates := make([]int32, 30)
	for i := range heartRates {
		heartRates[i] = 140
	}
	act := makeActWithHeartRates(heartRates)
	inputs := map[string]string{"show_drift": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "❤️") {
		t.Errorf("expected emoji in output, got %q", res.Description)
	}
	// No significant drift -> no "Drift:" line
	if strings.Contains(res.Description, "Drift") {
		t.Errorf("expected no Drift line for constant HR, got %q", res.Description)
	}
}

func TestHeartRateSummary_ShowDrift_TooFewPoints(t *testing.T) {
	p := NewHeartRateSummary()
	// Only 10 samples (< 20 required) -> drift not shown even with show_drift
	heartRates := make([]int32, 10)
	for i := range heartRates {
		heartRates[i] = 140
	}
	act := makeActWithHeartRates(heartRates)
	inputs := map[string]string{"show_drift": "true"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(res.Description, "Drift") {
		t.Errorf("expected no Drift line for < 20 samples, got %q", res.Description)
	}
}

func TestHeartRateSummary_NoHeartRateData(t *testing.T) {
	p := NewHeartRateSummary()
	act := &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{
				Records: []*pbactivity.Record{{Speed: 3.0}}, // no HR
			}},
		}},
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metadata["hr_summary_status"] != "skipped" {
		t.Errorf("expected skipped, got %q", res.Metadata["hr_summary_status"])
	}
}

func TestHeartRateSummary_SimpleFormat(t *testing.T) {
	p := NewHeartRateSummary()
	heartRates := []int32{120, 140, 160}
	act := makeActWithHeartRates(heartRates)
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default: single-line format
	if !strings.Contains(res.Description, "min") || !strings.Contains(res.Description, "avg") || !strings.Contains(res.Description, "max") {
		t.Errorf("expected min/avg/max in simple format, got %q", res.Description)
	}
}
