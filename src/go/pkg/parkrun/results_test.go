package parkrun

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

// buildTestHTML generates a minimal Parkrun athlete results page HTML with given rows.
// Matches the real 7-column structure: Event, Run Date, Run Number, Pos, Time, Age Grade, PB?
func buildTestHTML(rows []testRow) string {
	html := `<html><body><table><thead><tr><th>Event</th><th>Run Date</th><th>Run Number</th><th>Pos</th><th>Time</th><th>Age Grade</th><th>PB?</th></tr></thead><tbody>`
	for _, r := range rows {
		html += `<tr>`
		html += `<td><a href="https://www.parkrun.org.uk/` + r.slug + `/results/">` + r.event + `</a></td>`
		html += `<td><a href="https://www.parkrun.org.uk/` + r.slug + `/results/` + r.runNumber + `/"><span class="format-date">` + r.date + `</span></a></td>`
		html += `<td><a href="https://www.parkrun.org.uk/` + r.slug + `/results/` + r.runNumber + `/">` + r.runNumber + `</a></td>`
		html += `<td>` + r.position + `</td>`
		html += `<td>` + r.time + `</td>`
		html += `<td>` + r.ageGrade + `</td>`
		html += `<td></td>`
		html += `</tr>`
	}
	html += `</tbody></table></body></html>`
	return html
}

type testRow struct {
	event     string
	slug      string
	date      string
	runNumber string
	position  string
	time      string
	ageGrade  string
}

func TestParseAthleteResultsBySlug_DateFiltering(t *testing.T) {
	// Simulate an athlete page with two results for the same event on different dates
	// (most recent first, as Parkrun does)
	rows := []testRow{
		{event: "Newark", slug: "newark", date: "29/03/2026", runNumber: "420", position: "15", time: "24:30", ageGrade: "55.00%"},
		{event: "Newark", slug: "newark", date: "22/03/2026", runNumber: "419", position: "20", time: "25:10", ageGrade: "52.00%"},
		{event: "Newark", slug: "newark", date: "15/03/2026", runNumber: "418", position: "18", time: "24:50", ageGrade: "53.50%"},
	}
	html := buildTestHTML(rows)
	logger := slog.Default()

	t.Run("Matching date returns correct result", func(t *testing.T) {
		expectedDate := time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "newark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected a result, got nil")
		}
		if result.Position != 15 {
			t.Errorf("Expected position 15, got %d", result.Position)
		}
		if result.Time != "24:30" {
			t.Errorf("Expected time 24:30, got %s", result.Time)
		}
		if result.EventDate != "29/03/2026" {
			t.Errorf("Expected date 29/03/2026, got %s", result.EventDate)
		}
	})

	t.Run("Previous week date returns nil (results not yet available for this week)", func(t *testing.T) {
		// Asking for a date that's NOT in the results page → should return nil
		expectedDate := time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "newark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("Expected nil result for future date not in results, got position=%d date=%s", result.Position, result.EventDate)
		}
	})

	t.Run("Old date returns correct historical result", func(t *testing.T) {
		expectedDate := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "newark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected a result for 22/03/2026, got nil")
		}
		if result.Position != 20 {
			t.Errorf("Expected position 20, got %d", result.Position)
		}
		if result.Time != "25:10" {
			t.Errorf("Expected time 25:10, got %s", result.Time)
		}
	})

	t.Run("Wrong event slug returns nil regardless of date", func(t *testing.T) {
		expectedDate := time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "bushypark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("Expected nil for non-matching slug, got position=%d", result.Position)
		}
	})
}

func TestParseAthleteResultsBySlug_MultipleEvents_DateFiltering(t *testing.T) {
	// Simulate mixed events — different slugs on the same and different dates
	rows := []testRow{
		{event: "Bushy Park", slug: "bushypark", date: "29/03/2026", runNumber: "1200", position: "50", time: "22:00", ageGrade: "60.00%"},
		{event: "Newark", slug: "newark", date: "22/03/2026", runNumber: "419", position: "20", time: "25:10", ageGrade: "52.00%"},
		{event: "Newark", slug: "newark", date: "15/03/2026", runNumber: "418", position: "18", time: "24:50", ageGrade: "53.50%"},
	}
	html := buildTestHTML(rows)
	logger := slog.Default()

	t.Run("Correct event and date matches despite other events on same date", func(t *testing.T) {
		expectedDate := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "newark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected a result, got nil")
		}
		if result.Position != 20 {
			t.Errorf("Expected position 20, got %d", result.Position)
		}
	})

	t.Run("Newark on 29/03 returns nil (only Bushy Park ran that day)", func(t *testing.T) {
		expectedDate := time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "newark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("Expected nil for newark on 29/03 (athlete was at bushypark), got position=%d", result.Position)
		}
	})
}

func TestParseAthleteResultsBySlug_RealFixture(t *testing.T) {
	// Load the real Parkrun HTML fixture
	htmlBytes, err := os.ReadFile("example_results_page.html")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}
	html := string(htmlBytes)
	logger := slog.Default()

	t.Run("Newark on 14/03/2026 returns correct result", func(t *testing.T) {
		expectedDate := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "newark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected a result for Newark on 14/03/2026, got nil")
		}
		if result.Position != 48 {
			t.Errorf("Expected position 48, got %d", result.Position)
		}
		if result.Time != "26:45" {
			t.Errorf("Expected time 26:45, got %s", result.Time)
		}
	})

	t.Run("Newark on 29/03/2026 returns nil (results not published yet)", func(t *testing.T) {
		// Today is 29/03/2026 — this week's Newark run happened but results aren't on page yet
		// The most recent Newark result on the page is 14/03/2026
		// Without date validation, this would incorrectly return the 14/03 result
		expectedDate := time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "newark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("Expected nil for Newark on 29/03 (results not published), got position=%d date=%s",
				result.Position, result.EventDate)
		}
	})

	t.Run("Doddington Hall on 21/03/2026 returns correct result", func(t *testing.T) {
		expectedDate := time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "doddingtonhall", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected a result for Doddington Hall on 21/03/2026, got nil")
		}
		if result.Position != 72 {
			t.Errorf("Expected position 72, got %d", result.Position)
		}
		if result.Time != "22:31" {
			t.Errorf("Expected time 22:31, got %s", result.Time)
		}
	})

	t.Run("Correctly skips Summary Stats and Annual tables", func(t *testing.T) {
		// The page has 3 tables; only the "All Results" table should be parsed
		// Summary Stats has 4 cells per row, Annual has 3 — both should be skipped by the < 7 check
		expectedDate := time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC)
		result, err := ParseAthleteResultsBySlug(logger, html, "newark", expectedDate)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected result for Newark on 07/03/2026")
		}
		if result.Position != 16 {
			t.Errorf("Expected position 16, got %d", result.Position)
		}
	})
}
