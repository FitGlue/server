package parkrun_results_source

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	shared "github.com/fitglue/server/src/go/pkg"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/framework"
	"github.com/fitglue/server/src/go/pkg/infrastructure/oauth"
	infrapubsub "github.com/fitglue/server/src/go/pkg/infrastructure/pubsub"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("PollParkrunResults", PollParkrunResults)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		svc, svcErr = bootstrap.NewService(ctx)
	})
	return svc, svcErr
}

// PollParkrunResults is triggered by Cloud Scheduler to check for pending Parkrun results
func PollParkrunResults(ctx context.Context, e cloudevents.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("parkrun-results-source", svc, pollHandler(nil))(ctx, e)
}

// pollHandler contains the business logic
// Uses the Pipeline Resume Pattern - queries auto-populated pending inputs and triggers enricher resume
func pollHandler(httpClient *http.Client) framework.HandlerFunc {
	return func(ctx context.Context, e cloudevents.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		fwCtx.Logger.Info("Starting Parkrun results poll")

		// Default HTTP client with error logging for fetching results
		if httpClient == nil {
			httpClient = oauth.NewClientWithErrorLogging(fwCtx.Logger, "parkrun", 30*time.Second)
		}

		// Query for auto-populated pending inputs from the Parkrun enricher
		pendingInputs, err := fwCtx.Service.DB.ListPendingInputsByEnricher(ctx, "parkrun", pb.PendingInput_STATUS_WAITING)
		if err != nil {
			fwCtx.Logger.Error("Failed to query pending inputs", "error", err)
			return nil, fmt.Errorf("query pending inputs: %w", err)
		}

		if len(pendingInputs) == 0 {
			fwCtx.Logger.Info("No pending Parkrun inputs found")
			return map[string]interface{}{
				"status":    "SUCCESS",
				"processed": 0,
				"updated":   0,
				"message":   "No pending Parkrun results to process",
			}, nil
		}

		fwCtx.Logger.Info("Found pending Parkrun inputs", "count", len(pendingInputs))

		var processed, updated, failed int
		for _, input := range pendingInputs {
			processed++

			// Only process auto-populated inputs that continued without resolution
			if !input.AutoPopulated || !input.ContinuedWithoutResolution {
				fwCtx.Logger.Debug("Skipping non-auto-populated input", "input_id", input.ActivityId)
				continue
			}

			// Get user for Parkrun integration credentials
			user, err := fwCtx.Service.DB.GetUser(ctx, input.UserId)
			if err != nil || user == nil {
				fwCtx.Logger.Warn("Failed to get user", "user_id", input.UserId, "error", err)
				continue
			}

			if user.Integrations == nil || user.Integrations.Parkrun == nil || !user.Integrations.Parkrun.Enabled {
				fwCtx.Logger.Debug("User has no Parkrun integration", "user_id", input.UserId)
				continue
			}

			// Extract event info from pending input metadata
			eventSlug := ""
			eventName := ""
			if input.OriginalPayload != nil && input.OriginalPayload.Metadata != nil {
				eventSlug = input.OriginalPayload.Metadata["parkrun_event_slug"]
				eventName = input.OriginalPayload.Metadata["parkrun_event_name"]
			}

			if eventSlug == "" {
				fwCtx.Logger.Warn("No event slug in pending input", "input_id", input.ActivityId)
				continue
			}

			// Fetch results from Parkrun
			results, err := fetchParkrunResultsForAthlete(ctx, httpClient, user.Integrations.Parkrun, eventSlug)
			if err != nil {
				fwCtx.Logger.Warn("Failed to fetch results (may not be published yet)",
					"activity_id", input.ActivityId,
					"event_slug", eventSlug,
					"error", err)
				continue
			}

			if results == nil {
				fwCtx.Logger.Info("Results not yet available",
					"activity_id", input.ActivityId,
					"event_slug", eventSlug)
				continue
			}

			// Update the pending input with the resolved data
			resultDescription := formatResultsDescription(results, eventName)
			err = fwCtx.Service.DB.UpdatePendingInput(ctx, input.ActivityId, map[string]interface{}{
				"status":       int32(pb.PendingInput_STATUS_COMPLETED),
				"completed_at": timestamppb.Now(),
				"input_data": map[string]string{
					"description": *resultDescription,
					"position":    fmt.Sprintf("%d", results.Position),
					"time":        results.Time,
					"age_grade":   results.AgeGrade,
				},
			})
			if err != nil {
				fwCtx.Logger.Error("Failed to update pending input", "error", err)
				failed++
				continue
			}

			// Trigger pipeline resume by publishing ActivityPayload with resume flags
			resumePayload := &pb.ActivityPayload{
				UserId:               input.UserId,
				ActivityId:           &input.LinkedActivityId,
				PipelineId:           &input.PipelineId,
				IsResume:             true,
				ResumeOnlyEnrichers:  []string{"parkrun"},
				UseUpdateMethod:      true,
				ResumePendingInputId: &input.ActivityId,
			}

			eventData, err := protojson.Marshal(resumePayload)
			if err != nil {
				fwCtx.Logger.Error("Failed to marshal resume payload", "error", err)
				failed++
				continue
			}

			ceEvent, err := infrapubsub.NewCloudEvent(
				"/integrations/parkrun/results",
				"com.fitglue.activity.enriched", // Re-trigger enricher with resume mode
				eventData,
			)
			if err != nil {
				fwCtx.Logger.Error("Failed to create CloudEvent", "error", err)
				failed++
				continue
			}

			// Publish to the enricher topic to resume the pipeline
			_, err = fwCtx.Service.Pub.PublishCloudEvent(ctx, shared.TopicActivityEnrichment, ceEvent)
			if err != nil {
				fwCtx.Logger.Error("Failed to publish resume event", "error", err)
				failed++
				continue
			}

			updated++
			fwCtx.Logger.Info("Published pipeline resume for Parkrun results",
				"activity_id", input.LinkedActivityId,
				"position", results.Position,
				"time", results.Time)
		}

		return map[string]interface{}{
			"status":    "SUCCESS",
			"processed": processed,
			"updated":   updated,
			"failed":    failed,
		}, nil
	}
}

// PendingParkrunActivity represents an activity awaiting results
type PendingParkrunActivity struct {
	ActivityID       string
	UserID           string
	StravaActivityID int64
	EventName        string
	EventSlug        string
	StartTime        time.Time
	PollingDeadline  *timestamppb.Timestamp
}

// ParkrunResult represents fetched results with PB tracking and location stats
type ParkrunResult struct {
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

// queryPendingParkrunActivities queries Firestore for activities awaiting results
func queryPendingParkrunActivities(ctx context.Context, svc *bootstrap.Service) ([]PendingParkrunActivity, error) {
	// Use the database adapter to query for pending Parkrun activities
	activities, userIDs, err := svc.DB.ListPendingParkrunActivities(ctx)
	if err != nil {
		return nil, fmt.Errorf("query pending activities: %w", err)
	}

	var pending []PendingParkrunActivity
	for i, activity := range activities {
		// Convert to our internal struct
		pending = append(pending, PendingParkrunActivity{
			ActivityID:       activity.ActivityId,
			UserID:           userIDs[i],
			StravaActivityID: extractStravaActivityID(activity.Destinations),
			EventName:        activity.ParkrunEventName,
			EventSlug:        activity.ParkrunEventSlug,
			StartTime:        activity.StartTime.AsTime(),
			PollingDeadline:  activity.ParkrunPollingDeadline,
		})
	}

	return pending, nil
}

// extractStravaActivityID extracts Strava activity ID from destinations map
func extractStravaActivityID(destinations map[string]string) int64 {
	if destinations == nil {
		return 0
	}
	// Destinations map has format: {"strava": "activity_id"}
	if stravaID, ok := destinations["strava"]; ok {
		var id int64
		fmt.Sscanf(stravaID, "%d", &id)
		return id
	}
	return 0
}

// fetchParkrunResultsForAthlete fetches and parses results from Parkrun website
// Uses eventSlug directly instead of activity struct
func fetchParkrunResultsForAthlete(ctx context.Context, client *http.Client, integration *pb.ParkrunIntegration, eventSlug string) (*ParkrunResult, error) {
	// Extract numeric athlete ID from barcode (A12345 -> 12345)
	athleteID := strings.TrimPrefix(integration.AthleteId, "A")

	// Build URL: https://www.parkrun.org.uk/parkrunner/{athlete_id}/all/
	baseURL := integration.CountryUrl
	if baseURL == "" {
		baseURL = "www.parkrun.org.uk"
	}
	url := fmt.Sprintf("https://%s/parkrunner/%s/all/", baseURL, athleteID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch results: %w", err)
	}
	defer resp.Body.Close()

	// Accept 200 OK and 202 Accepted (Parkrun sometimes returns 202 during caching)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Parse the HTML to find matching event by slug
	return parseAthleteResultsBySlug(string(body), eventSlug)
}

// parseAthleteResultsBySlug parses the athlete's results page HTML to find result by event slug
// and calculate PBs/stats from historical data.
// The /all/ page has a table with columns: Event, Run Date, Run Number, Pos, Time, Age Grade, PB?
func parseAthleteResultsBySlug(html string, eventSlug string) (*ParkrunResult, error) {
	// Find rows in the "All Results" table (look for tbody rows to skip header)
	// Using (?s) for dot-all mode to match across newlines
	rowPattern := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	rows := rowPattern.FindAllStringSubmatch(html, -1)

	log.Printf("[DEBUG] parseAthleteResultsBySlug: html_len=%d, event_slug=%s, total_rows=%d", len(html), eventSlug, len(rows))

	// Track our target result and all historical data for PB calculations
	var targetResult *ParkrunResult
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
			log.Printf("[DEBUG] Row %d: event=%s, rowSlug=%s, containsTarget=%v, targetResult_nil=%v",
				i, eventCell, rowEventSlug, containsTarget, targetResult == nil)
		}

		if targetResult == nil && containsTarget {
			log.Printf("[DEBUG] MATCH FOUND! Row %d: event=%s, date=%s, pos=%d, time=%s, ag=%.2f%%",
				i, eventCell, dateCell, position, timeStr, ageGrade)
			targetResult = &ParkrunResult{
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

	log.Printf("[DEBUG] Parsing complete: headerRows=%d, insufficientCells=%d, invalidPos=%d, validDataRows=%d, targetFound=%v",
		headerRows, insufficientCellRows, invalidPositionRows, validDataRows, targetResult != nil)

	// If no matching result found
	if targetResult == nil {
		return nil, nil
	}

	// Now calculate PBs by comparing against all OTHER results (excluding target row)
	targetResult.TimeAllTimePB = isTimePB(targetResult.Time, allTimes, targetRowDate)
	targetResult.PosAllTimePB = isPositionPB(targetResult.Position, allPositions, targetRowDate)
	targetResult.AgeGradeAllTimePB = isAgeGradePB(parseAgeGrade(targetResult.AgeGrade), allAgeGrades, targetRowDate)

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

// extractEventSlugFromRow extracts the event slug from a row's event link
func extractEventSlugFromRow(row string) string {
	// Look for href pattern like https://www.parkrun.org.uk/newark/results/
	hrefPattern := regexp.MustCompile(`href="https?://[^/]+/([^/]+)/results/"`)
	match := hrefPattern.FindStringSubmatch(row)
	if len(match) >= 2 {
		return strings.ToLower(match[1])
	}
	return ""
}

// parseAgeGrade parses age grade string to float
func parseAgeGrade(ag string) float64 {
	ag = strings.TrimSuffix(ag, "%")
	var val float64
	fmt.Sscanf(ag, "%f", &val)
	return val
}

// parseTimeToSeconds converts time string (MM:SS or HH:MM:SS) to seconds for comparison
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

// isTimePB checks if the target time is a new all-time PB (lower is better)
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

// isPositionPB checks if the target position is a new all-time PB (lower is better)
func isPositionPB(targetPos int, allPositions []int, targetDate string) bool {
	for _, pos := range allPositions {
		if pos > 0 && pos < targetPos {
			return false // Found a better position
		}
	}
	return len(allPositions) > 1
}

// isAgeGradePB checks if the target age grade is a new all-time PB (higher is better)
func isAgeGradePB(targetAG float64, allAgeGrades []float64, targetDate string) bool {
	for _, ag := range allAgeGrades {
		if ag > targetAG {
			return false // Found a higher age grade
		}
	}
	return len(allAgeGrades) > 1
}

// isTimePBThisYear checks if the target time is a this-year PB
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

// isPositionPBThisYear checks if the target position is a this-year PB
func isPositionPBThisYear(targetPos int, thisYearPositions []int) bool {
	for _, pos := range thisYearPositions {
		if pos > 0 && pos < targetPos {
			return false
		}
	}
	return len(thisYearPositions) > 1
}

// isAgeGradePBThisYear checks if the target age grade is a this-year PB
func isAgeGradePBThisYear(targetAG float64, thisYearAgeGrades []float64) bool {
	for _, ag := range thisYearAgeGrades {
		if ag > targetAG {
			return false
		}
	}
	return len(thisYearAgeGrades) > 1
}

// fetchParkrunResults fetches and parses results from Parkrun website (legacy - uses activity struct)
func fetchParkrunResults(ctx context.Context, client *http.Client, integration *pb.ParkrunIntegration, activity PendingParkrunActivity) (*ParkrunResult, error) {
	// Extract numeric athlete ID from barcode (A12345 -> 12345)
	athleteID := strings.TrimPrefix(integration.AthleteId, "A")

	// Build URL: https://www.parkrun.org.uk/parkrunner/{athlete_id}/all/
	baseURL := integration.CountryUrl
	if baseURL == "" {
		baseURL = "www.parkrun.org.uk"
	}
	url := fmt.Sprintf("https://%s/parkrunner/%s/all/", baseURL, athleteID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Parse the HTML to find matching event/date
	return parseAthleteResults(string(body), activity.EventName, activity.StartTime)
}

// parseAthleteResults parses the athlete's results page HTML to find matching result
// The /all/ page has columns: Event, Run Date, Run Number, Pos, Time, Age Grade, PB?
func parseAthleteResults(html string, eventName string, activityDate time.Time) (*ParkrunResult, error) {
	// Format the date for matching (DD/MM/YYYY format used by Parkrun)
	dateStr := activityDate.Format("02/01/2006")

	// Find all rows in the table
	rowPattern := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	rows := rowPattern.FindAllStringSubmatch(html, -1)

	for _, rowMatch := range rows {
		row := rowMatch[1]

		// Skip header rows
		if strings.Contains(row, "<th") {
			continue
		}

		// Check if row contains both event name (or slug) and date
		eventSlug := strings.ToLower(strings.ReplaceAll(eventName, " ", ""))
		rowLower := strings.ToLower(row)

		hasEvent := strings.Contains(row, eventName) || strings.Contains(rowLower, eventSlug)
		hasDate := strings.Contains(row, dateStr)

		if !hasEvent || !hasDate {
			continue
		}

		// Extract table cells
		cellPattern := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
		cells := cellPattern.FindAllStringSubmatch(row, -1)

		// Expect 7 columns: Event (0), Run Date (1), Run Number (2), Pos (3), Time (4), Age Grade (5), PB? (6)
		if len(cells) >= 7 {
			result := &ParkrunResult{
				EventName: eventName,
				EventDate: dateStr,
			}

			fmt.Sscanf(stripTags(cells[3][1]), "%d", &result.Position)
			result.Time = stripTags(cells[4][1])
			result.AgeGrade = stripTags(cells[5][1])
			pbCell := stripTags(cells[6][1])
			// Legacy: set TimeAllTimePB based on PB column (simplified)
			result.TimeAllTimePB = strings.Contains(strings.ToUpper(pbCell), "PB")

			return result, nil
		}
	}

	return nil, nil // No matching result found yet
}

// formatResultsDescription formats results into a nice description with PB badges
func formatResultsDescription(results *ParkrunResult, eventName string) *string {
	if results == nil {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("🏃♂️ Parkrun Results:\n")

	// Position line with PB badges
	sb.WriteString(fmt.Sprintf("Position: %s", ordinal(results.Position)))
	if results.PosAllTimePB {
		sb.WriteString(" | 🏆 New all-time PB!")
	}
	if results.PosThisYearPB {
		sb.WriteString(" | 🏅 New this-year PB!")
	}
	sb.WriteString("\n")

	// Time line with PB badges
	sb.WriteString(fmt.Sprintf("Time: %s", results.Time))
	if results.TimeAllTimePB {
		sb.WriteString(" | 🏆 New all-time PB!")
	}
	if results.TimeThisYearPB {
		sb.WriteString(" | 🏅 New this-year PB!")
	}
	sb.WriteString("\n")

	// Age Grade line with PB badges
	if results.AgeGrade != "" {
		sb.WriteString(fmt.Sprintf("Age Grade: %s", results.AgeGrade))
		if results.AgeGradeAllTimePB {
			sb.WriteString(" | 🏆 New all-time PB!")
		}
		if results.AgeGradeThisYearPB {
			sb.WriteString(" | 🏅 New this-year PB!")
		}
		sb.WriteString("\n")
	}

	// Location line
	sb.WriteString(fmt.Sprintf("Location: %s, %s Parkrun here (%d total)",
		eventName, ordinal(results.TotalAtLocation), results.TotalAllTime))
	if results.FirstAtLocation {
		sb.WriteString(" | 🌟 First time at this location!")
	}

	desc := sb.String()
	return &desc
}

// ordinal converts an integer to its ordinal string (1st, 2nd, 3rd, 4th, etc.)
func ordinal(n int) string {
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

// Helper regex patterns for parsing Parkrun results HTML
var (
	// Parkrun result tables use specific TD classes
	positionRegex = regexp.MustCompile(`<td[^>]*data-th="Pos"[^>]*>(\d+)</td>`)
	timeRegex     = regexp.MustCompile(`<td[^>]*data-th="Time"[^>]*>(\d+:\d+)</td>`)
	ageGradeRegex = regexp.MustCompile(`<td[^>]*data-th="Age Grade"[^>]*>(\d+\.\d+%)</td>`)
	tagRegex      = regexp.MustCompile(`<[^>]*>`)
)

// stripTags removes HTML tags from a string
func stripTags(s string) string {
	return strings.TrimSpace(tagRegex.ReplaceAllString(s, ""))
}
