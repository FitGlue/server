// Package parkrun provides shared utilities for fetching and parsing Parkrun results.
package parkrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
)

// Result represents fetched Parkrun results with PB tracking and location stats.
type Result struct {
	// Current run
	Time     string // e.g., "24:12"
	Position int    // e.g., 30
	AgeGrade string // e.g., "54.76%"

	// All-time PB tracking
	TimeAllTimePB     bool // Is this a new all-time time PB?
	PosAllTimePB      bool // Is this a new all-time position PB?
	AgeGradeAllTimePB bool // Is this a new all-time age grade PB?

	// This-year PB tracking (Jan 1st cutoff)
	TimeThisYearPB     bool
	PosThisYearPB      bool
	AgeGradeThisYearPB bool

	// Location stats
	TotalAtLocation int  // How many times at this location (including this run)
	TotalAllTime    int  // Total parkruns ever (including this run)
	FirstAtLocation bool // First time at this location

	// Event info
	EventName string
	EventDate string
}

// PlaywrightFetchRequest is the request body for the Playwright fetcher service.
type PlaywrightFetchRequest struct {
	URL string `json:"url"`
}

// PlaywrightFetchResponse is the response from the Playwright fetcher service.
type PlaywrightFetchResponse struct {
	HTML       string `json:"html"`
	ByteLength int    `json:"byteLength"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

// FetchResultsForAthlete fetches and parses results from Parkrun website.
// Uses the Playwright fetcher service to bypass AWS WAF bot protection.
func FetchResultsForAthlete(ctx context.Context, logger *slog.Logger, athleteID, countryURL, eventSlug string) (*Result, error) {
	// Extract numeric athlete ID from barcode (A12345 -> 12345)
	athleteID = strings.TrimPrefix(athleteID, "A")

	// Build URL: https://www.parkrun.org.uk/parkrunner/{athlete_id}/all/
	baseURL := countryURL
	if baseURL == "" {
		baseURL = "www.parkrun.org.uk"
	}
	parkrunURL := fmt.Sprintf("https://%s/parkrunner/%s/all/", baseURL, athleteID)

	// Get HTML via Playwright fetcher service (bypasses AWS WAF)
	html, err := FetchViaPlaywright(ctx, logger, parkrunURL)
	if err != nil {
		return nil, fmt.Errorf("fetch via playwright: %w", err)
	}

	// Parse the HTML to find matching event by slug
	return ParseAthleteResultsBySlug(logger, html, eventSlug)
}

// FetchViaPlaywright calls the Playwright fetcher Cloud Run service to get HTML.
// This bypasses AWS WAF JavaScript challenges by using a real browser.
func FetchViaPlaywright(ctx context.Context, logger *slog.Logger, url string) (string, error) {
	fetcherURL := os.Getenv("PARKRUN_FETCHER_URL")
	if fetcherURL == "" {
		// Fallback to direct fetch for local development/testing
		logger.Warn("PARKRUN_FETCHER_URL not set, falling back to direct HTTP fetch")
		return fetchDirectHTTP(ctx, &http.Client{Timeout: 30 * time.Second}, url)
	}

	// Create an authenticated HTTP client for Cloud Run service-to-service auth
	// The idtoken.NewClient automatically obtains identity tokens from the metadata service
	authClient, err := idtoken.NewClient(ctx, fetcherURL)
	if err != nil {
		return "", fmt.Errorf("create authenticated client: %w", err)
	}

	// Build request to Playwright service
	reqBody := PlaywrightFetchRequest{URL: url}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fetcherURL+"/fetch", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := authClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call playwright service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("playwright service error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var fetchResp PlaywrightFetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&fetchResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if !fetchResp.Success {
		return "", fmt.Errorf("playwright fetch failed: %s", fetchResp.Error)
	}

	logger.Info("Fetched HTML via Playwright",
		"url", url,
		"bytes", fetchResp.ByteLength)

	// Warn if HTML is suspiciously small (likely an error page)
	const minExpectedHTMLBytes = 5000
	if fetchResp.ByteLength < minExpectedHTMLBytes {
		logger.Warn("Parkrun HTML response unusually small",
			"bytes", fetchResp.ByteLength,
			"expected_min", minExpectedHTMLBytes,
			"url", url)
	}

	return fetchResp.HTML, nil
}

// fetchDirectHTTP is a fallback for local development when Playwright service is not available.
func fetchDirectHTTP(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("http_status_%d: unexpected response (body_len=%d)", resp.StatusCode, len(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(body), nil
}

// ParseAthleteResultsBySlug parses the athlete's results page HTML to find result by event slug
// and calculate PBs/stats from historical data.
// The /all/ page has a table with columns: Event, Run Date, Run Number, Pos, Time, Age Grade, PB?
func ParseAthleteResultsBySlug(logger *slog.Logger, html string, eventSlug string) (*Result, error) {
	// Find rows in the "All Results" table (look for tbody rows to skip header)
	// Using (?s) for dot-all mode to match across newlines
	rowPattern := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	rows := rowPattern.FindAllStringSubmatch(html, -1)

	logger.Debug("parseAthleteResultsBySlug starting",
		"html_len", len(html),
		"event_slug", eventSlug,
		"total_rows", len(rows))

	// Track our target result and all historical data for PB calculations
	var targetResult *Result
	var targetEventSlugLower = strings.ToLower(eventSlug)
	var targetRowDate string

	// Historical tracking for PB calculations
	var allTimes []string
	var allPositions []int
	var allAgeGrades []float64
	var thisYearTimes []string
	var thisYearPositions []int
	var thisYearAgeGrades []float64

	// Location tracking
	locationVisits := make(map[string]int)
	totalRuns := 0

	// Get current year for this-year PB calculations
	currentYear := time.Now().Year()

	// Cell pattern for extraction
	cellPattern := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)

	headerRows := 0
	insufficientCellRows := 0
	invalidPositionRows := 0
	validDataRows := 0

	for i, rowMatch := range rows {
		row := rowMatch[1]

		// Skip header rows (they contain <th> elements)
		if strings.Contains(row, "<th") {
			headerRows++
			continue
		}

		// Extract table cells
		cells := cellPattern.FindAllStringSubmatch(row, -1)

		// Expect 7 columns: Event (0), Run Date (1), Run Number (2), Pos (3), Time (4), Age Grade (5), PB? (6)
		if len(cells) < 7 {
			insufficientCellRows++
			continue
		}

		eventCell := stripTags(cells[0][1])
		dateCell := stripTags(cells[1][1])
		positionStr := stripTags(cells[3][1])
		timeStr := stripTags(cells[4][1])
		ageGradeStr := stripTags(cells[5][1])

		// Parse position
		var position int
		fmt.Sscanf(positionStr, "%d", &position)
		if position == 0 {
			invalidPositionRows++
			continue // Skip invalid rows
		}

		validDataRows++

		// Parse age grade (remove % if present)
		ageGradeStr = strings.TrimSuffix(ageGradeStr, "%")
		var ageGrade float64
		fmt.Sscanf(ageGradeStr, "%f", &ageGrade)

		// Parse date to determine year (format: DD/MM/YYYY)
		runYear := 0
		if len(dateCell) >= 10 {
			fmt.Sscanf(dateCell[6:10], "%d", &runYear)
		}

		// Extract event slug from this row's event link
		rowEventSlug := extractEventSlugFromRow(row)

		// Track location visits
		locationVisits[rowEventSlug]++
		totalRuns++

		// Track historical data for PB calculations (excluding the target row itself later)
		allTimes = append(allTimes, timeStr)
		allPositions = append(allPositions, position)
		allAgeGrades = append(allAgeGrades, ageGrade)

		if runYear == currentYear {
			thisYearTimes = append(thisYearTimes, timeStr)
			thisYearPositions = append(thisYearPositions, position)
			thisYearAgeGrades = append(thisYearAgeGrades, ageGrade)
		}

		// Check if this is our target row (most recent match for the event slug)
		rowLower := strings.ToLower(row)
		containsTarget := strings.Contains(rowLower, targetEventSlugLower)

		if i < 25 || containsTarget { // Log first 25 rows or any matching rows
			logger.Debug("Row parsing",
				"row", i,
				"event", eventCell,
				"row_slug", rowEventSlug,
				"contains_target", containsTarget,
				"target_result_nil", targetResult == nil)
		}

		if targetResult == nil && containsTarget {
			logger.Debug("Match found",
				"row", i,
				"event", eventCell,
				"date", dateCell,
				"position", position,
				"time", timeStr,
				"age_grade", ageGrade)
			targetResult = &Result{
				Time:            timeStr,
				Position:        position,
				AgeGrade:        fmt.Sprintf("%.2f%%", ageGrade),
				EventName:       eventCell,
				EventDate:       dateCell,
				TotalAtLocation: locationVisits[rowEventSlug],
				TotalAllTime:    totalRuns,
				FirstAtLocation: locationVisits[rowEventSlug] == 1,
			}
			targetRowDate = dateCell
		}
	}

	logger.Debug("Parsing complete",
		"header_rows", headerRows,
		"insufficient_cells", insufficientCellRows,
		"invalid_pos", invalidPositionRows,
		"valid_data_rows", validDataRows,
		"target_found", targetResult != nil)

	// If no matching result found
	if targetResult == nil {
		return nil, nil
	}

	// Now calculate PBs by comparing against all OTHER results (excluding target row)
	targetResult.TimeAllTimePB = isTimePB(targetResult.Time, allTimes, targetRowDate)
	targetResult.PosAllTimePB = isPositionPB(targetResult.Position, allPositions)
	targetResult.AgeGradeAllTimePB = isAgeGradePB(parseAgeGrade(targetResult.AgeGrade), allAgeGrades)

	// This-year PBs
	targetResult.TimeThisYearPB = isTimePBThisYear(targetResult.Time, thisYearTimes)
	targetResult.PosThisYearPB = isPositionPBThisYear(targetResult.Position, thisYearPositions)
	targetResult.AgeGradeThisYearPB = isAgeGradePBThisYear(parseAgeGrade(targetResult.AgeGrade), thisYearAgeGrades)

	// Update totals (we want to show counts including this run)
	eventSlugLower := strings.ToLower(eventSlug)
	targetResult.TotalAtLocation = locationVisits[eventSlugLower]
	targetResult.TotalAllTime = totalRuns
	// FirstAtLocation is true only if this is the only run ever at this location
	targetResult.FirstAtLocation = locationVisits[eventSlugLower] == 1

	return targetResult, nil
}

// extractEventSlugFromRow extracts the event slug from a row's event link.
func extractEventSlugFromRow(row string) string {
	// Look for href pattern like https://www.parkrun.org.uk/newark/results/
	hrefPattern := regexp.MustCompile(`href="https?://[^/]+/([^/]+)/results/"`)
	match := hrefPattern.FindStringSubmatch(row)
	if len(match) >= 2 {
		return strings.ToLower(match[1])
	}
	return ""
}

// parseAgeGrade parses age grade string to float.
func parseAgeGrade(ag string) float64 {
	ag = strings.TrimSuffix(ag, "%")
	var val float64
	fmt.Sscanf(ag, "%f", &val)
	return val
}

// parseTimeToSeconds converts time string (MM:SS or HH:MM:SS) to seconds for comparison.
func parseTimeToSeconds(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	seconds := 0
	switch len(parts) {
	case 2: // MM:SS
		var mins, secs int
		fmt.Sscanf(parts[0], "%d", &mins)
		fmt.Sscanf(parts[1], "%d", &secs)
		seconds = mins*60 + secs
	case 3: // HH:MM:SS
		var hours, mins, secs int
		fmt.Sscanf(parts[0], "%d", &hours)
		fmt.Sscanf(parts[1], "%d", &mins)
		fmt.Sscanf(parts[2], "%d", &secs)
		seconds = hours*3600 + mins*60 + secs
	}
	return seconds
}

// isTimePB checks if the target time is a new all-time PB (lower is better).
func isTimePB(targetTime string, allTimes []string, targetDate string) bool {
	targetSeconds := parseTimeToSeconds(targetTime)
	if targetSeconds == 0 {
		return false
	}
	for _, t := range allTimes {
		otherSeconds := parseTimeToSeconds(t)
		if otherSeconds > 0 && otherSeconds < targetSeconds {
			return false // Found a faster time
		}
	}
	return len(allTimes) > 1 // Only a PB if there were previous runs
}

// isPositionPB checks if the target position is a new all-time PB (lower is better).
func isPositionPB(targetPos int, allPositions []int) bool {
	for _, pos := range allPositions {
		if pos > 0 && pos < targetPos {
			return false // Found a better position
		}
	}
	return len(allPositions) > 1
}

// isAgeGradePB checks if the target age grade is a new all-time PB (higher is better).
func isAgeGradePB(targetAG float64, allAgeGrades []float64) bool {
	for _, ag := range allAgeGrades {
		if ag > targetAG {
			return false // Found a higher age grade
		}
	}
	return len(allAgeGrades) > 1
}

// isTimePBThisYear checks if the target time is a this-year PB.
func isTimePBThisYear(targetTime string, thisYearTimes []string) bool {
	targetSeconds := parseTimeToSeconds(targetTime)
	if targetSeconds == 0 {
		return false
	}
	for _, t := range thisYearTimes {
		otherSeconds := parseTimeToSeconds(t)
		if otherSeconds > 0 && otherSeconds < targetSeconds {
			return false
		}
	}
	return len(thisYearTimes) > 1
}

// isPositionPBThisYear checks if the target position is a this-year PB.
func isPositionPBThisYear(targetPos int, thisYearPositions []int) bool {
	for _, pos := range thisYearPositions {
		if pos > 0 && pos < targetPos {
			return false
		}
	}
	return len(thisYearPositions) > 1
}

// isAgeGradePBThisYear checks if the target age grade is a this-year PB.
func isAgeGradePBThisYear(targetAG float64, thisYearAgeGrades []float64) bool {
	for _, ag := range thisYearAgeGrades {
		if ag > targetAG {
			return false
		}
	}
	return len(thisYearAgeGrades) > 1
}

// FormatResultsDescription formats results into a nice description with PB badges.
func FormatResultsDescription(results *Result, eventName string) string {
	if results == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("🏃 Parkrun Results:\n")

	// Position line with PB badges
	sb.WriteString(fmt.Sprintf("• Position: %s", Ordinal(results.Position)))
	if results.PosAllTimePB {
		sb.WriteString(" · 🏆 New all-time PB!")
	}
	if results.PosThisYearPB {
		sb.WriteString(" · 🏅 New this-year PB!")
	}
	sb.WriteString("\n")

	// Time line with PB badges
	sb.WriteString(fmt.Sprintf("• Time: %s", results.Time))
	if results.TimeAllTimePB {
		sb.WriteString(" · 🏆 New all-time PB!")
	}
	if results.TimeThisYearPB {
		sb.WriteString(" · 🏅 New this-year PB!")
	}
	sb.WriteString("\n")

	// Age Grade line with PB badges
	if results.AgeGrade != "" {
		sb.WriteString(fmt.Sprintf("• Age Grade: %s", results.AgeGrade))
		if results.AgeGradeAllTimePB {
			sb.WriteString(" · 🏆 New all-time PB!")
		}
		if results.AgeGradeThisYearPB {
			sb.WriteString(" · 🏅 New this-year PB!")
		}
		sb.WriteString("\n")
	}

	// Location line
	sb.WriteString(fmt.Sprintf("• Location: %s, %s Parkrun here (%d total)",
		eventName, Ordinal(results.TotalAtLocation), results.TotalAllTime))
	if results.FirstAtLocation {
		sb.WriteString(" · 🌟 First time at this location!")
	}

	return sb.String()
}

// Ordinal converts an integer to its ordinal string (1st, 2nd, 3rd, 4th, etc.).
func Ordinal(n int) string {
	if n <= 0 {
		return fmt.Sprintf("%d", n)
	}
	switch n % 100 {
	case 11, 12, 13:
		return fmt.Sprintf("%dth", n)
	}
	switch n % 10 {
	case 1:
		return fmt.Sprintf("%dst", n)
	case 2:
		return fmt.Sprintf("%dnd", n)
	case 3:
		return fmt.Sprintf("%drd", n)
	default:
		return fmt.Sprintf("%dth", n)
	}
}

// tagRegex for HTML tag stripping.
var tagRegex = regexp.MustCompile(`<[^>]*>`)

// stripTags removes HTML tags from a string.
func stripTags(s string) string {
	return strings.TrimSpace(tagRegex.ReplaceAllString(s, ""))
}
