package googlesheetsuploader

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestBuildSheetRow_BasicActivity(t *testing.T) {
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-123",
		Name:         "Morning Run",
		Description:  "A quick morning jog",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
	}

	row := buildSheetRow(event, false)

	if len(row) == 0 {
		t.Error("Expected non-empty row")
	}

	// Check that activity name is in row
	found := false
	for _, val := range row {
		if val == "Morning Run" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected activity name in row")
	}
}

func TestBuildSheetRow_ColumnCount(t *testing.T) {
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-col-count",
		Name:         "Test Activity",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
	}

	row := buildSheetRow(event, true)
	headers := getHeaderRow()

	if len(row) != len(headers) {
		t.Errorf("Row column count (%d) does not match header count (%d)", len(row), len(headers))
	}
}

// Column indices:
// 0=Synced At, 1=Date, 2=Source, 3=Activity Type, 4=Title, 5=Duration,
// 6=Distance, 7=Calories, 8=Avg HR, 9=Max HR,
// 10=Elevation Gain, 11=Generated Images, 12=Description

func TestBuildSheetRow_CaloriesFromSession(t *testing.T) {
	calories := float64(350)
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-cal-session",
		Name:         "Calorie Run",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
		ActivityData: &pb.StandardizedActivity{
			Sessions: []*pb.Session{
				{TotalCalories: &calories},
			},
		},
	}

	row := buildSheetRow(event, false)

	if row[7] != "350" {
		t.Errorf("Expected calories '350', got '%v'", row[7])
	}
}

func TestBuildSheetRow_CaloriesFromEnrichment(t *testing.T) {
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-cal-enrichment",
		Name:         "Calorie Run",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
		EnrichmentMetadata: map[string]string{
			"calories": "420",
		},
	}

	row := buildSheetRow(event, false)

	if row[7] != "420" {
		t.Errorf("Expected calories '420' from enrichment, got '%v'", row[7])
	}
}

func TestBuildSheetRow_CaloriesSessionTakesPrecedence(t *testing.T) {
	calories := float64(300)
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-cal-precedence",
		Name:         "Calorie Run",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
		ActivityData: &pb.StandardizedActivity{
			Sessions: []*pb.Session{
				{TotalCalories: &calories},
			},
		},
		EnrichmentMetadata: map[string]string{
			"calories": "999",
		},
	}

	row := buildSheetRow(event, false)

	if row[7] != "300" {
		t.Errorf("Expected session calories '300' to take precedence, got '%v'", row[7])
	}
}

func TestBuildSheetRow_MaxHR(t *testing.T) {
	maxHR := int32(185)
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-max-hr",
		Name:         "HR Test",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
		ActivityData: &pb.StandardizedActivity{
			Sessions: []*pb.Session{
				{MaxHeartRate: &maxHR},
			},
		},
	}

	row := buildSheetRow(event, false)

	if row[9] != "185" {
		t.Errorf("Expected max HR '185', got '%v'", row[9])
	}
}

func TestBuildSheetRow_DescriptionPreservesNewlines(t *testing.T) {
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-desc-newlines",
		Name:         "Newline Test",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
		Description:  "🔥 Calories: 350\n💓 Heart Rate Zones\nZone 1: 10 min",
	}

	row := buildSheetRow(event, false)

	desc, ok := row[12].(string)
	if !ok {
		t.Fatal("Expected description to be a string")
	}
	if desc != "🔥 Calories: 350\n💓 Heart Rate Zones\nZone 1: 10 min" {
		t.Errorf("Expected newlines preserved in description, got '%s'", desc)
	}
}

func TestBuildSheetRow_VisualsArePlainURLs(t *testing.T) {
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-visuals-urls",
		Name:         "Visual Test",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
		EnrichmentMetadata: map[string]string{
			"asset_muscle_heatmap":  "https://example.com/heatmap.png",
			"asset_route_thumbnail": "https://example.com/route.png",
		},
	}

	row := buildSheetRow(event, true)

	// Generated Images is column index 11
	expected := "https://example.com/heatmap.png\nhttps://example.com/route.png"
	if row[11] != expected {
		t.Errorf("Expected combined image URLs, got '%v'", row[11])
	}
}

func TestBuildSheetRow_SourceFormatted(t *testing.T) {
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-source",
		Name:         "Source Test",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
		Source:       pb.ActivitySource_SOURCE_STRAVA,
	}

	row := buildSheetRow(event, false)

	if row[2] != "STRAVA" {
		t.Errorf("Expected source 'STRAVA', got '%v'", row[2])
	}
}

func TestGetHeaderRow_MatchesDataColumns(t *testing.T) {
	headers := getHeaderRow()

	expectedHeaders := []string{
		"Synced At", "Date", "Source", "Activity Type", "Title", "Duration",
		"Distance (km)", "Calories", "Avg HR", "Max HR",
		"Elevation Gain (m)", "Generated Images", "Description",
	}

	if len(headers) != len(expectedHeaders) {
		t.Fatalf("Expected %d headers, got %d", len(expectedHeaders), len(headers))
	}

	for i, expected := range expectedHeaders {
		if headers[i] != expected {
			t.Errorf("Header[%d]: expected '%s', got '%v'", i, expected, headers[i])
		}
	}
}
