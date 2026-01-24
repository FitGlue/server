package parkrun_results_source

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// discardLogger returns a logger that discards all output (for tests)
var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// loadExampleHTML loads the example-results.html test fixture
func loadExampleHTML(t *testing.T) string {
	t.Helper()
	// Get the directory of this test file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Could not get test file location")
	}
	dir := filepath.Dir(filename)
	htmlPath := filepath.Join(dir, "example-results.html")

	data, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("Failed to load example-results.html: %v", err)
	}
	return string(data)
}

func TestParseAthleteResultsBySlug_NewarkLatestRun(t *testing.T) {
	html := loadExampleHTML(t)

	result, err := parseAthleteResultsBySlug(discardLogger, html, "newark")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// The most recent Newark run is 24/01/2026: position 30, time 23:50, age grade 54.76%
	if result.Position != 30 {
		t.Errorf("Position: got %d, want 30", result.Position)
	}
	if result.Time != "23:50" {
		t.Errorf("Time: got %q, want %q", result.Time, "23:50")
	}
	if result.AgeGrade != "54.76%" {
		t.Errorf("AgeGrade: got %q, want %q", result.AgeGrade, "54.76%")
	}
	if result.EventDate != "24/01/2026" {
		t.Errorf("EventDate: got %q, want %q", result.EventDate, "24/01/2026")
	}

	// PB checks: This 23:50 is the all-time fastest, so TimeAllTimePB should be true
	if !result.TimeAllTimePB {
		t.Error("TimeAllTimePB: expected true (23:50 is fastest ever)")
	}

	// Location stats: After full iteration, we get the total counts
	// There are 15 Newark runs in the test data
	if result.TotalAtLocation != 15 {
		t.Errorf("TotalAtLocation: got %d, want 15", result.TotalAtLocation)
	}

	// Total runs should be 19
	if result.TotalAllTime != 19 {
		t.Errorf("TotalAllTime: got %d, want 19", result.TotalAllTime)
	}

	// Not first at location since there are 15 Newark runs
	if result.FirstAtLocation {
		t.Error("FirstAtLocation: expected false (15 Newark runs in history)")
	}
}

func TestParseAthleteResultsBySlug_Colwick(t *testing.T) {
	html := loadExampleHTML(t)

	result, err := parseAthleteResultsBySlug(discardLogger, html, "colwick")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result for Colwick, got nil")
	}

	// Most recent Colwick run: 18/10/2025, position 263, time 29:10, age grade 44.74%
	if result.Position != 263 {
		t.Errorf("Position: got %d, want 263", result.Position)
	}
	if result.Time != "29:10" {
		t.Errorf("Time: got %q, want %q", result.Time, "29:10")
	}

	// There are 2 Colwick runs in the data
	if result.TotalAtLocation != 2 {
		t.Errorf("TotalAtLocation: got %d, want 2", result.TotalAtLocation)
	}
}

func TestParseAthleteResultsBySlug_RobertsPark(t *testing.T) {
	html := loadExampleHTML(t)

	result, err := parseAthleteResultsBySlug(discardLogger, html, "robertspark")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result for Roberts Park, got nil")
	}

	// Only one Roberts Park run: 29/11/2025
	if result.Position != 142 {
		t.Errorf("Position: got %d, want 142", result.Position)
	}
	if result.Time != "28:00" {
		t.Errorf("Time: got %q, want %q", result.Time, "28:00")
	}

	// First time at this location
	if !result.FirstAtLocation {
		t.Error("FirstAtLocation: expected true (only one Roberts Park run)")
	}
	if result.TotalAtLocation != 1 {
		t.Errorf("TotalAtLocation: got %d, want 1", result.TotalAtLocation)
	}
}

func TestParseAthleteResultsBySlug_Lancaster(t *testing.T) {
	html := loadExampleHTML(t)

	result, err := parseAthleteResultsBySlug(discardLogger, html, "lancaster")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result for Lancaster, got nil")
	}

	// Only one Lancaster run: 09/08/2025, position 98, time 29:00
	if result.Position != 98 {
		t.Errorf("Position: got %d, want 98", result.Position)
	}
	if result.Time != "29:00" {
		t.Errorf("Time: got %q, want %q", result.Time, "29:00")
	}

	// First time at this location
	if !result.FirstAtLocation {
		t.Error("FirstAtLocation: expected true (only one Lancaster run)")
	}
}

func TestParseAthleteResultsBySlug_NonExistentEvent(t *testing.T) {
	html := loadExampleHTML(t)

	result, err := parseAthleteResultsBySlug(discardLogger, html, "bushypark")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil for non-existent event, got %+v", result)
	}
}

func TestOrdinal(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "1st"},
		{2, "2nd"},
		{3, "3rd"},
		{4, "4th"},
		{10, "10th"},
		{11, "11th"},
		{12, "12th"},
		{13, "13th"},
		{14, "14th"},
		{21, "21st"},
		{22, "22nd"},
		{23, "23rd"},
		{24, "24th"},
		{30, "30th"},
		{100, "100th"},
		{101, "101st"},
		{111, "111th"},
		{112, "112th"},
		{113, "113th"},
		{121, "121st"},
		{0, "0"},
		{-1, "-1"},
	}

	for _, tt := range tests {
		got := ordinal(tt.n)
		if got != tt.want {
			t.Errorf("ordinal(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestParseTimeToSeconds(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"23:50", 23*60 + 50},
		{"24:36", 24*60 + 36},
		{"31:07", 31*60 + 7},
		{"1:05:30", 1*3600 + 5*60 + 30},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		got := parseTimeToSeconds(tt.input)
		if got != tt.want {
			t.Errorf("parseTimeToSeconds(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseAgeGrade(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"54.76%", 54.76},
		{"44.74%", 44.74},
		{"50.00", 50.00},
		{"", 0},
	}

	for _, tt := range tests {
		got := parseAgeGrade(tt.input)
		if got != tt.want {
			t.Errorf("parseAgeGrade(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestFormatResultsDescription(t *testing.T) {
	result := &ParkrunResult{
		Time:               "23:50",
		Position:           30,
		AgeGrade:           "54.76%",
		TimeAllTimePB:      true,
		TimeThisYearPB:     true,
		PosAllTimePB:       false,
		PosThisYearPB:      false,
		AgeGradeAllTimePB:  true,
		AgeGradeThisYearPB: false,
		TotalAtLocation:    15,
		TotalAllTime:       19,
		FirstAtLocation:    false,
		EventName:          "Newark",
	}

	desc := formatResultsDescription(result, "Newark")
	if desc == nil {
		t.Fatal("Expected description, got nil")
	}

	// Check that key elements are present
	if len(*desc) == 0 {
		t.Error("Description is empty")
	}

	// Should contain ordinal position
	if !containsStr(*desc, "30th") {
		t.Error("Description should contain '30th'")
	}

	// Should contain time
	if !containsStr(*desc, "23:50") {
		t.Error("Description should contain time '23:50'")
	}

	// Should contain PB badges for time (all-time and this-year)
	if !containsStr(*desc, "🏆") {
		t.Error("Description should contain all-time PB emoji 🏆")
	}
	if !containsStr(*desc, "🏅") {
		t.Error("Description should contain this-year PB emoji 🏅")
	}

	// Should contain location count
	if !containsStr(*desc, "15th Parkrun here") {
		t.Error("Description should contain '15th Parkrun here'")
	}
}

func TestFormatResultsDescription_FirstAtLocation(t *testing.T) {
	result := &ParkrunResult{
		Time:            "28:00",
		Position:        142,
		AgeGrade:        "46.61%",
		TotalAtLocation: 1,
		TotalAllTime:    12,
		FirstAtLocation: true,
		EventName:       "Roberts Park",
	}

	desc := formatResultsDescription(result, "Roberts Park")
	if desc == nil {
		t.Fatal("Expected description, got nil")
	}

	// Should contain first-time badge
	if !containsStr(*desc, "🌟") {
		t.Error("Description should contain first-time emoji 🌟")
	}
	if !containsStr(*desc, "First time at this location") {
		t.Error("Description should mention first time at location")
	}
}

func TestFormatResultsDescription_Nil(t *testing.T) {
	desc := formatResultsDescription(nil, "Newark")
	if desc != nil {
		t.Errorf("Expected nil for nil input, got %q", *desc)
	}
}

// Helper function to check if string contains substring
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsStrImpl(s, substr)))
}

func containsStrImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
