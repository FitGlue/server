package elevation_summary

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
)

// --- generateElevationProfile ---

func TestGenerateElevationProfile_EmptyAltitudes(t *testing.T) {
	result := generateElevationProfile(nil, 0, 100, 10)
	if result != "" {
		t.Errorf("expected empty string for nil altitudes, got %q", result)
	}
}

func TestGenerateElevationProfile_FlatCourse(t *testing.T) {
	// maxAlt == minAlt -> returns ""
	altitudes := []float64{100, 100, 100, 100, 100}
	result := generateElevationProfile(altitudes, 100, 100, 5)
	if result != "" {
		t.Errorf("expected empty string for flat course (max==min), got %q", result)
	}
}

func TestGenerateElevationProfile_SimpleProfile(t *testing.T) {
	// 20 altitude points ascending then descending
	altitudes := make([]float64, 20)
	for i := 0; i < 10; i++ {
		altitudes[i] = float64(100 + i*10)    // 100-190
		altitudes[19-i] = float64(100 + i*10) // mirror
	}
	result := generateElevationProfile(altitudes, 100, 190, 10)
	if len(result) == 0 {
		t.Error("expected non-empty elevation profile")
	}
	// Each bucket should be a bar character
	for _, r := range result {
		found := false
		for _, bar := range profileBars {
			if string(r) == bar {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected character in profile: %q", string(r))
		}
	}
}

func TestGenerateElevationProfile_FewAltitudes(t *testing.T) {
	// Fewer altitudes than numBuckets -> pointsPerBucket becomes 1
	altitudes := []float64{100, 110, 120, 130, 140}
	result := generateElevationProfile(altitudes, 100, 140, 20)
	// Should still produce output (won't crash)
	if len(result) == 0 {
		t.Error("expected non-empty result even with few altitudes")
	}
}

// --- Enrich: profile path ---

func makeActWithAltitude(altitudes []float64) *pbactivity.StandardizedActivity {
	records := make([]*pbactivity.Record, len(altitudes))
	for i, alt := range altitudes {
		records[i] = &pbactivity.Record{Altitude: alt}
	}
	return &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{Records: records}},
		}},
	}
}

func TestElevationSummary_WithProfile(t *testing.T) {
	p := NewElevationSummary()
	// 15 altitude points (>= 10 required for profile)
	altitudes := make([]float64, 15)
	for i := 0; i < 15; i++ {
		altitudes[i] = float64(100 + i*5)
	}
	act := makeActWithAltitude(altitudes)
	inputs := map[string]string{"style": "profile"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metadata["elevation_summary_status"] != "success" {
		t.Errorf("expected success, got %q", res.Metadata["elevation_summary_status"])
	}
	if !strings.Contains(res.Description, "📈") {
		t.Errorf("expected 📈 profile in description, got %q", res.Description)
	}
}

func TestElevationSummary_WithProfile_FewPoints(t *testing.T) {
	p := NewElevationSummary()
	// Only 5 altitude points (< 10 required) -> no profile shown
	altitudes := []float64{100, 105, 110, 115, 120}
	act := makeActWithAltitude(altitudes)
	inputs := map[string]string{"style": "profile"}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(res.Description, "📈") {
		t.Error("expected no profile for < 10 altitude points")
	}
}

func TestElevationSummary_NoAltitudeData(t *testing.T) {
	p := NewElevationSummary()
	act := &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{
				Records: []*pbactivity.Record{{Speed: 3.0}}, // no altitude
			}},
		}},
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metadata["elevation_summary_status"] != "skipped" {
		t.Errorf("expected skipped, got %q", res.Metadata["elevation_summary_status"])
	}
}

func TestElevationSummary_GainAndLoss(t *testing.T) {
	p := NewElevationSummary()
	// Ascending then descending: gain 50, loss 50
	altitudes := []float64{100, 110, 120, 130, 150, 130, 110, 100}
	act := makeActWithAltitude(altitudes)
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "+") {
		t.Errorf("expected elevation gain in description, got %q", res.Description)
	}
}
