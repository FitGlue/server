package hybrid_race_tagger

import (
	"strings"
	"testing"
	"time"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- getStationIcon ---

func TestGetStationIcon(t *testing.T) {
	cases := []struct {
		name     string
		contains string
	}{
		{"Run 1", "🏃"},
		{"SkiErg", "⛷️"},
		{"Sled Push 50m", "🛷"},
		{"Sled Pull 50m", "🛷"},
		{"Burpee Broad Jump", "🏋️"},
		{"Rowing", "🚣"},
		{"Farmers Carry", "🧳"},
		{"Sandbag Lunges", "🎒"},
		{"Wall Ball", "🏐"},
		{"Something Unknown", "▪️"},
	}
	for _, c := range cases {
		got := getStationIcon(c.name)
		if got != c.contains {
			t.Errorf("getStationIcon(%q) = %q, want %q", c.name, got, c.contains)
		}
	}
}

// --- formatDuration ---

func TestFormatDurationHRT(t *testing.T) {
	cases := []struct {
		seconds  float64
		expected string
	}{
		{0, "0:00"},
		{59, "0:59"},
		{90, "1:30"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
	}
	for _, c := range cases {
		got := formatDuration(c.seconds)
		if got != c.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", c.seconds, got, c.expected)
		}
	}
}

// --- generateTimeMarkers ---

func TestGenerateTimeMarkers(t *testing.T) {
	ts := timestamppb.New(time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC))

	results := []StationResult{
		{Name: "Run 1", StartTime: ts, IsRun: true},
		{Name: "SkiErg", StartTime: ts, IsRun: false},
		{Name: "No Time", StartTime: nil}, // should be skipped
	}

	markers := generateTimeMarkers(results)

	if len(markers) != 2 {
		t.Fatalf("expected 2 markers (nil start time skipped), got %d", len(markers))
	}
	if markers[0].MarkerType != "run_start" {
		t.Errorf("expected 'run_start' for run lap, got %q", markers[0].MarkerType)
	}
	if markers[1].MarkerType != "station_start" {
		t.Errorf("expected 'station_start' for non-run lap, got %q", markers[1].MarkerType)
	}
	if markers[0].Label != "Run 1" {
		t.Errorf("expected label 'Run 1', got %q", markers[0].Label)
	}
}

// --- generateDescription ---

func TestGenerateDescription_RunOnly(t *testing.T) {
	preset := RacePreset{Name: "Test Race", RaceType: "hyrox"}
	results := []StationResult{
		{Name: "Run 1", Duration: 300, IsRun: true},
		{Name: "Run 2", Duration: 280, IsRun: true},
	}
	desc := generateDescription(preset, results)
	if !strings.Contains(desc, "Test Race") {
		t.Errorf("expected preset name in description, got %q", desc)
	}
	if !strings.Contains(desc, "Run 1") {
		t.Errorf("expected Run 1 in description, got %q", desc)
	}
	if !strings.Contains(desc, "Total") {
		t.Errorf("expected Total in description, got %q", desc)
	}
}

func TestGenerateDescription_StationWithWeight(t *testing.T) {
	preset := RacePreset{Name: "Test Race", RaceType: "hyrox"}
	results := []StationResult{
		{Name: "Sled Push", Duration: 120, IsRun: false, Weight: 102.0},
	}
	desc := generateDescription(preset, results)
	if !strings.Contains(desc, "102") {
		t.Errorf("expected weight in description, got %q", desc)
	}
}

func TestGenerateDescription_StationWithReps(t *testing.T) {
	preset := RacePreset{Name: "Test Race", RaceType: "hyrox"}
	results := []StationResult{
		{Name: "Wall Ball", Duration: 180, IsRun: false, ExpectedReps: 100},
	}
	desc := generateDescription(preset, results)
	if !strings.Contains(desc, "100 reps") {
		t.Errorf("expected reps in description, got %q", desc)
	}
}

func TestGenerateDescription_StationWithRepsAndWeight(t *testing.T) {
	preset := RacePreset{Name: "Test Race", RaceType: "hyrox"}
	results := []StationResult{
		{Name: "Wall Ball", Duration: 180, IsRun: false, ExpectedReps: 100, Weight: 9.0},
	}
	desc := generateDescription(preset, results)
	if !strings.Contains(desc, "reps") || !strings.Contains(desc, "9") {
		t.Errorf("expected reps and weight in description, got %q", desc)
	}
}

func TestGenerateDescription_PlainStation(t *testing.T) {
	preset := RacePreset{Name: "Test Race", RaceType: "hyrox"}
	results := []StationResult{
		{Name: "SkiErg", Duration: 200, IsRun: false},
	}
	desc := generateDescription(preset, results)
	if !strings.Contains(desc, "SkiErg") {
		t.Errorf("expected station name in description, got %q", desc)
	}
}

// --- GetPresetList / GetPreset ---

func TestGetPresetList_NotEmpty(t *testing.T) {
	presets := GetPresetList()
	if len(presets) == 0 {
		t.Error("expected at least one preset")
	}
}

func TestGetPreset_HyroxExists(t *testing.T) {
	// At least Hyrox preset should exist
	presets := GetPresetList()
	found := false
	for _, p := range presets {
		if _, ok := GetPreset(p.ID); ok {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected GetPreset to return a valid preset for listed IDs")
	}
}

func TestGetPreset_NotFound(t *testing.T) {
	_, ok := GetPreset("nonexistent_preset_id_12345")
	if ok {
		t.Error("expected not found for unknown preset ID")
	}
}

// --- mapLapsToPreset ---

func TestMapLapsToPreset_ExtraLaps(t *testing.T) {
	// More laps than stations
	laps := []*pbactivity.Lap{
		{TotalElapsedTime: 60, TotalDistance: 100},
		{TotalElapsedTime: 120, TotalDistance: 200},
		{TotalElapsedTime: 90, TotalDistance: 150}, // extra
	}
	preset := RacePreset{
		Name:     "Mini Race",
		RaceType: "test",
		Stations: []Station{
			{Name: "Run 1", Type: StationTypeRun},
			{Name: "SkiErg", Type: StationTypeCardio, WeightKg: 0},
		},
	}
	newLaps, _, results := mapLapsToPreset(laps, preset)
	// 2 mapped + 1 extra = 3 laps total
	if len(newLaps) != 3 {
		t.Errorf("expected 3 laps (2 mapped + 1 extra), got %d", len(newLaps))
	}
	if len(results) != 2 {
		t.Errorf("expected 2 station results, got %d", len(results))
	}
}

func TestMapLapsToPreset_StrengthStation(t *testing.T) {
	ts := timestamppb.New(time.Now())
	laps := []*pbactivity.Lap{
		{TotalElapsedTime: 120, TotalDistance: 50, StartTime: ts},
	}
	preset := RacePreset{
		Name:     "Test",
		RaceType: "test",
		Stations: []Station{
			{Name: "Sled Push", Type: StationTypeStrength, WeightKg: 102, Reps: 0},
		},
	}
	newLaps, strengthSets, _ := mapLapsToPreset(laps, preset)
	if len(newLaps) != 0 {
		t.Errorf("expected 0 laps (strength becomes StrengthSet), got %d", len(newLaps))
	}
	if len(strengthSets) != 1 {
		t.Errorf("expected 1 strength set, got %d", len(strengthSets))
	}
	if strengthSets[0].WeightKg != 102 {
		t.Errorf("expected weight 102, got %v", strengthSets[0].WeightKg)
	}
}

func TestMapLapsToPreset_RepBasedStrengthStation(t *testing.T) {
	ts := timestamppb.New(time.Now())
	laps := []*pbactivity.Lap{
		{TotalElapsedTime: 180, TotalDistance: 0, StartTime: ts},
	}
	preset := RacePreset{
		Name:     "Test",
		RaceType: "test",
		Stations: []Station{
			{Name: "Wall Ball", Type: StationTypeStrength, WeightKg: 9, Reps: 100},
		},
	}
	_, strengthSets, _ := mapLapsToPreset(laps, preset)
	if len(strengthSets) != 1 {
		t.Fatalf("expected 1 strength set, got %d", len(strengthSets))
	}
	if strengthSets[0].Reps != 100 {
		t.Errorf("expected 100 reps, got %d", strengthSets[0].Reps)
	}
	// Rep-based: DistanceMeters should be zeroed out
	if strengthSets[0].DistanceMeters != 0 {
		t.Errorf("expected DistanceMeters=0 for rep-based station, got %v", strengthSets[0].DistanceMeters)
	}
}
