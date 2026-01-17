package parkrun_results_source

import (
	"context"
	"fmt"
	"io"
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

		// Default HTTP client for fetching results
		if httpClient == nil {
			httpClient = &http.Client{Timeout: 30 * time.Second}
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

// ParkrunResult represents fetched results
type ParkrunResult struct {
	Position  int
	Time      string // e.g., "24:12"
	AgeGrade  string // e.g., "65.2%"
	IsPB      bool
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

	// Build URL: https://www.parkrun.org.uk/parkrunner/{athlete_id}/
	baseURL := integration.CountryUrl
	if baseURL == "" {
		baseURL = "www.parkrun.org.uk"
	}
	url := fmt.Sprintf("https://%s/parkrunner/%s/", baseURL, athleteID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "FitGlue/1.0 (https://fitglue.com)")
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

	// Parse the HTML to find matching event by slug
	return parseAthleteResultsBySlug(string(body), eventSlug)
}

// parseAthleteResultsBySlug parses the athlete's results page HTML to find result by event slug
func parseAthleteResultsBySlug(html string, eventSlug string) (*ParkrunResult, error) {
	// Similar to parseAthleteResults but matches by event slug instead of name/date
	// The athlete page has a table with recent results
	// Each row contains: Event, Run No., Date, Time, Pos, Age Grade, PB?

	// Find rows in the results table
	rowPattern := regexp.MustCompile(`<tr[^>]*>.*?</tr>`)
	rows := rowPattern.FindAllString(html, -1)

	for _, row := range rows {
		// Check if this row contains the event slug
		if !strings.Contains(strings.ToLower(row), strings.ToLower(eventSlug)) {
			continue
		}

		// Extract position, time, age grade
		cellPattern := regexp.MustCompile(`<td[^>]*>(.*?)</td>`)
		cells := cellPattern.FindAllStringSubmatch(row, -1)

		if len(cells) >= 6 {
			event := stripTags(cells[0][1])
			position := 0
			fmt.Sscanf(stripTags(cells[4][1]), "%d", &position)
			time := stripTags(cells[3][1])
			ageGrade := stripTags(cells[5][1])
			isPB := strings.Contains(row, "PB") || strings.Contains(row, "pb")

			return &ParkrunResult{
				Position:  position,
				Time:      time,
				AgeGrade:  ageGrade,
				IsPB:      isPB,
				EventName: event,
			}, nil
		}
	}

	return nil, nil // Results not found yet
}

// fetchParkrunResults fetches and parses results from Parkrun website (legacy - uses activity struct)
func fetchParkrunResults(ctx context.Context, client *http.Client, integration *pb.ParkrunIntegration, activity PendingParkrunActivity) (*ParkrunResult, error) {
	// Extract numeric athlete ID from barcode (A12345 -> 12345)
	athleteID := strings.TrimPrefix(integration.AthleteId, "A")

	// Build URL: https://www.parkrun.org.uk/parkrunner/{athlete_id}/
	baseURL := integration.CountryUrl
	if baseURL == "" {
		baseURL = "www.parkrun.org.uk"
	}
	url := fmt.Sprintf("https://%s/parkrunner/%s/", baseURL, athleteID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "FitGlue/1.0 (https://fitglue.com)")
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
func parseAthleteResults(html string, eventName string, activityDate time.Time) (*ParkrunResult, error) {
	// The athlete page has a table with recent results
	// Each row contains: Event, Run No., Date, Time, Pos, Age Grade, PB?
	// We need to find the row matching eventName + date

	// Format the date for matching (DD/MM/YYYY format used by Parkrun)
	dateStr := activityDate.Format("02/01/2006")

	// Find all result rows - looking for the pattern in the table
	// Match rows containing the event name and date
	rowPattern := regexp.MustCompile(`(?s)<tr[^>]*>.*?` + regexp.QuoteMeta(eventName) + `.*?` + regexp.QuoteMeta(dateStr) + `.*?</tr>`)
	rowMatch := rowPattern.FindString(html)

	if rowMatch == "" {
		// Try alternative: search for event slug in lowercase
		eventSlug := strings.ToLower(strings.ReplaceAll(eventName, " ", ""))
		rowPattern = regexp.MustCompile(`(?s)<tr[^>]*>.*?` + eventSlug + `.*?` + regexp.QuoteMeta(dateStr) + `.*?</tr>`)
		rowMatch = rowPattern.FindString(html)
	}

	if rowMatch == "" {
		return nil, nil // No matching result found yet
	}

	// Extract position, time, age grade from the matched row
	result := &ParkrunResult{
		EventName: eventName,
		EventDate: dateStr,
	}

	// Extract time (MM:SS format)
	timeMatch := timeRegex.FindStringSubmatch(rowMatch)
	if len(timeMatch) >= 2 {
		result.Time = timeMatch[1]
	}

	// Extract position (number)
	posMatch := positionRegex.FindStringSubmatch(rowMatch)
	if len(posMatch) >= 2 {
		fmt.Sscanf(posMatch[1], "%d", &result.Position)
	}

	// Extract age grade (XX.XX% format)
	ageMatch := ageGradeRegex.FindStringSubmatch(rowMatch)
	if len(ageMatch) >= 2 {
		result.AgeGrade = ageMatch[1]
	}

	// Check for PB indicator
	result.IsPB = strings.Contains(rowMatch, "PB") || strings.Contains(rowMatch, "New PB")

	return result, nil
}

// formatResultsDescription formats results into a nice description
func formatResultsDescription(results *ParkrunResult, eventName string) *string {
	if results == nil {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("🏃 **Official Parkrun Results**\n\n")
	sb.WriteString(fmt.Sprintf("📍 %s\n", eventName))
	sb.WriteString(fmt.Sprintf("🏁 Position: %d\n", results.Position))
	sb.WriteString(fmt.Sprintf("⏱️ Official Time: %s\n", results.Time))

	if results.AgeGrade != "" {
		sb.WriteString(fmt.Sprintf("📊 Age Grade: %s\n", results.AgeGrade))
	}

	if results.IsPB {
		sb.WriteString("🎉 **New PB!** 🎉\n")
	}

	desc := sb.String()
	return &desc
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
