package googlesheetsuploader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/destination"
	"github.com/fitglue/server/src/go/pkg/domain/activity"
	"github.com/fitglue/server/src/go/pkg/framework"
	httputil "github.com/fitglue/server/src/go/pkg/infrastructure/http"
	"github.com/fitglue/server/src/go/pkg/infrastructure/oauth"
	"github.com/fitglue/server/src/go/pkg/loopprevention"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("UploadToGoogleSheets", UploadToGoogleSheets)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		baseSvc, err := bootstrap.NewService(ctx)
		if err != nil {
			// Error returned to caller
			svcErr = err
			return
		}
		svc = baseSvc
	})
	return svc, svcErr
}

// UploadToGoogleSheets is the Cloud Function entry point
func UploadToGoogleSheets(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("googlesheets-uploader", svc, uploadHandler(nil))(ctx, e)
}

// uploadHandler contains the business logic
// httpClient can be injected for testing; if nil, creates OAuth client
func uploadHandler(httpClient *http.Client) framework.HandlerFunc {
	return func(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		var eventPayload pb.EnrichedActivityEvent

		unmarshaler := protojson.UnmarshalOptions{
			DiscardUnknown: true,
			AllowPartial:   true,
		}
		if err := unmarshaler.Unmarshal(e.Data(), &eventPayload); err != nil {
			return nil, fmt.Errorf("protojson.Unmarshal: %w", err)
		}

		// Resolve activity data from GCS if needed (for large payloads offloaded by enricher)
		if err := activity.ResolveEnrichedEvent(ctx, &eventPayload, fwCtx.Service.Store); err != nil {
			fwCtx.Logger.Warn("Failed to resolve activity data from GCS", "error", err)
		}

		fwCtx.Logger.Info("Starting Google Sheets upload",
			"activity_id", eventPayload.ActivityId,
			"pipeline_id", eventPayload.PipelineId,
			"user_id", eventPayload.UserId,
		)

		// Note: Loop prevention is handled at source-handler level (isBounceback check)
		// The source handler checks uploaded_activities before publishing to the enricher

		// 1. Get user's Google Sheets integration
		user, err := svc.DB.GetUser(ctx, eventPayload.UserId)
		if err != nil {
			destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_GOOGLESHEETS, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("failed to get user: %s", err), fwCtx.Logger)
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		if user.Integrations == nil || user.Integrations.Google == nil || !user.Integrations.Google.Enabled {
			fwCtx.Logger.Warn("User has no Google integration configured", "userId", eventPayload.UserId)
			destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_GOOGLESHEETS, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", "Google integration not configured", fwCtx.Logger)
			return map[string]interface{}{
				"status": "FAILED",
				"reason": "no_google_integration",
			}, fmt.Errorf("user has no Google integration configured")
		}

		// 3. Get configuration from enrichment metadata (set by router)
		spreadsheetID := ""
		sheetName := "Activities"
		includeShowcaseLink := true
		includeVisuals := true

		if eventPayload.EnrichmentMetadata != nil {
			if id, ok := eventPayload.EnrichmentMetadata["googlesheets_spreadsheet_id"]; ok {
				spreadsheetID = id
			}
			if name, ok := eventPayload.EnrichmentMetadata["googlesheets_sheet_name"]; ok && name != "" {
				sheetName = name
			}
			if link, ok := eventPayload.EnrichmentMetadata["googlesheets_include_showcase_link"]; ok && link == "false" {
				includeShowcaseLink = false
			}
			if visuals, ok := eventPayload.EnrichmentMetadata["googlesheets_include_visuals"]; ok && visuals == "false" {
				includeVisuals = false
			}
		}

		if spreadsheetID == "" {
			destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_GOOGLESHEETS, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", "spreadsheet_id not configured", fwCtx.Logger)
			return nil, fmt.Errorf("spreadsheet_id not configured in metadata")
		}

		// Initialize OAuth HTTP Client if not provided (for testing)
		if httpClient == nil {
			tokenSource := oauth.NewFirestoreTokenSource(fwCtx.Service, eventPayload.UserId, "google")
			httpClient = oauth.NewClientWithUsageTracking(tokenSource, fwCtx.Service, eventPayload.UserId, "google")
		}

		// Check for UPDATE mode
		if eventPayload.EnrichmentMetadata != nil {
			if mode, ok := eventPayload.EnrichmentMetadata["operation_mode"]; ok && mode == "UPDATE" {
				return handleGooglesheetsUpdate(ctx, httpClient, &eventPayload, spreadsheetID, sheetName, includeShowcaseLink, includeVisuals, fwCtx)
			}
		}

		return handleGoogleSheetsCreate(ctx, httpClient, &eventPayload, spreadsheetID, sheetName, includeShowcaseLink, includeVisuals, fwCtx)
	}
}

// handleGoogleSheetsCreate appends a new row to the Google Sheet
func handleGoogleSheetsCreate(ctx context.Context, httpClient *http.Client, eventPayload *pb.EnrichedActivityEvent, spreadsheetID, sheetName string, includeShowcaseLink, includeVisuals bool, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Build row data
	row := buildSheetRow(eventPayload, includeShowcaseLink, includeVisuals)

	// Append to Google Sheets
	rowNumber, err := appendToSheet(ctx, httpClient, spreadsheetID, sheetName, row, fwCtx)
	if err != nil {
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_GOOGLESHEETS, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("API error: %s", err), fwCtx.Logger)
		return nil, fmt.Errorf("failed to append to Google Sheets: %w", err)
	}

	fwCtx.Logger.Info("Successfully appended to Google Sheets",
		"spreadsheetId", spreadsheetID,
		"sheetName", sheetName,
		"rowNumber", rowNumber,
		"activityId", eventPayload.ActivityId)

	// Record upload for loop prevention
	googlesheetsDestID := fmt.Sprintf("%d", rowNumber)
	// Key is destination:destinationId - though Google Sheets doesn't send webhooks,
	// this maintains consistency with other uploaders
	uploadRecord := &pb.UploadedActivityRecord{
		Id:            loopprevention.BuildUploadedActivityID(pb.Destination_DESTINATION_GOOGLESHEETS, googlesheetsDestID),
		UserId:        eventPayload.UserId,
		Source:        eventPayload.Source,
		ExternalId:    eventPayload.ActivityData.GetExternalId(),
		StartTime:     eventPayload.StartTime,
		Destination:   pb.Destination_DESTINATION_GOOGLESHEETS,
		DestinationId: googlesheetsDestID,
		UploadedAt:    timestamppb.Now(),
	}
	if err := svc.DB.SetUploadedActivity(ctx, eventPayload.UserId, uploadRecord); err != nil {
		fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
	} else {
		fwCtx.Logger.Debug("Recorded upload for loop prevention", "id", uploadRecord.Id)
	}

	// Persist SynchronizedActivity
	existingActivity, _ := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
	if existingActivity != nil {
		if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
			"destinations": map[string]interface{}{
				"googlesheets": fmt.Sprintf("%d", rowNumber),
			},
			"synced_at": timestamppb.Now().AsTime(),
		}); err != nil {
			fwCtx.Logger.Error("Failed to update synchronized activity destinations", "error", err)
		} else {
			fwCtx.Logger.Info("Updated synchronized activity destinations", "activity_id", eventPayload.ActivityId)
		}
	} else {
		syncedActivity := &pb.SynchronizedActivity{
			ActivityId:          eventPayload.ActivityId,
			Title:               eventPayload.Name,
			Description:         eventPayload.Description,
			Type:                eventPayload.ActivityType,
			Source:              eventPayload.Source.String(),
			StartTime:           eventPayload.StartTime,
			SyncedAt:            timestamppb.Now(),
			PipelineId:          eventPayload.PipelineId,
			PipelineExecutionId: fwCtx.PipelineExecutionId,
			Destinations: map[string]string{
				"googlesheets": fmt.Sprintf("%d", rowNumber),
			},
		}

		if err := svc.DB.SetSynchronizedActivity(ctx, eventPayload.UserId, syncedActivity); err != nil {
			fwCtx.Logger.Error("Failed to persist synchronized activity", "error", err)
		}
	}

	// Increment sync count for billing
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
	}

	// Update PipelineRun destination as synced
	destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_GOOGLESHEETS, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, googlesheetsDestID, "", fwCtx.Logger)

	return map[string]interface{}{
		"status":         "SUCCESS",
		"spreadsheet_id": spreadsheetID,
		"sheet_name":     sheetName,
		"row_number":     rowNumber,
		"activity_id":    eventPayload.ActivityId,
		"pipeline_id":    eventPayload.PipelineId,
		"activity_name":  eventPayload.Name,
		"activity_type":  eventPayload.ActivityType.String(),
		"description":    eventPayload.Description,
	}, nil
}

// handleGooglesheetsUpdate handles UPDATE operations.
// Google Sheets is append-only, so UPDATE appends a new row with updated data.
// This is intentional - spreadsheet history is preserved rather than modified.
func handleGooglesheetsUpdate(ctx context.Context, httpClient *http.Client, eventPayload *pb.EnrichedActivityEvent, spreadsheetID, sheetName string, includeShowcaseLink, includeVisuals bool, fwCtx *framework.FrameworkContext) (interface{}, error) {
	fwCtx.Logger.Info("Handling Google Sheets UPDATE (append-only mode)",
		"activity_id", eventPayload.ActivityId)

	// For append-only destinations, UPDATE is the same as CREATE
	return handleGoogleSheetsCreate(ctx, httpClient, eventPayload, spreadsheetID, sheetName, includeShowcaseLink, includeVisuals, fwCtx)
}

// buildSheetRow constructs the row data for Google Sheets
func buildSheetRow(event *pb.EnrichedActivityEvent, includeShowcaseLink, includeVisuals bool) []interface{} {
	row := []interface{}{}

	// Date (activity start)
	date := ""
	if event.StartTime != nil {
		date = event.StartTime.AsTime().Format("2006-01-02")
	}
	row = append(row, date)

	// Activity Type
	activityType := event.ActivityType.String()
	// Remove ACTIVITY_TYPE_ prefix for cleaner display
	activityType = strings.TrimPrefix(activityType, "ACTIVITY_TYPE_")
	activityType = strings.ReplaceAll(activityType, "_", " ")
	row = append(row, activityType)

	// Title
	row = append(row, event.Name)

	// Duration (formatted as HH:MM:SS)
	duration := ""
	if event.ActivityData != nil && len(event.ActivityData.Sessions) > 0 {
		totalSeconds := int(event.ActivityData.Sessions[0].TotalElapsedTime)
		hours := totalSeconds / 3600
		minutes := (totalSeconds % 3600) / 60
		seconds := totalSeconds % 60
		duration = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	row = append(row, duration)

	// Distance (km)
	distance := ""
	if event.ActivityData != nil && len(event.ActivityData.Sessions) > 0 {
		distanceMeters := event.ActivityData.Sessions[0].TotalDistance
		if distanceMeters > 0 {
			distance = fmt.Sprintf("%.2f", distanceMeters/1000.0)
		}
	}
	row = append(row, distance)

	// Calories (not available in Session, skip for now)
	calories := ""
	row = append(row, calories)

	// Average HR (calculate from records if available)
	avgHR := ""
	if event.ActivityData != nil && len(event.ActivityData.Sessions) > 0 {
		var hrSum, hrCount int
		for _, lap := range event.ActivityData.Sessions[0].Laps {
			for _, record := range lap.Records {
				if record.HeartRate > 0 {
					hrSum += int(record.HeartRate)
					hrCount++
				}
			}
		}
		if hrCount > 0 {
			avgHR = fmt.Sprintf("%.0f", float64(hrSum)/float64(hrCount))
		}
	}
	row = append(row, avgHR)

	// Muscle Heatmap (IMAGE formula if available and enabled)
	muscleHeatmap := ""
	if includeVisuals && event.EnrichmentMetadata != nil {
		if url, ok := event.EnrichmentMetadata["asset_muscle_heatmap"]; ok && url != "" {
			muscleHeatmap = fmt.Sprintf("=IMAGE(\"%s\")", url)
		}
	}
	row = append(row, muscleHeatmap)

	// Route Thumbnail (IMAGE formula if available and enabled)
	routeThumbnail := ""
	if includeVisuals && event.EnrichmentMetadata != nil {
		if url, ok := event.EnrichmentMetadata["asset_route_thumbnail"]; ok && url != "" {
			routeThumbnail = fmt.Sprintf("=IMAGE(\"%s\")", url)
		}
	}
	row = append(row, routeThumbnail)

	// Description (truncated to 500 chars)
	description := event.Description
	if len(description) > 500 {
		description = description[:500] + "..."
	}
	row = append(row, description)

	// Showcase Link (if enabled)
	showcaseLink := ""
	if includeShowcaseLink && event.EnrichmentMetadata != nil {
		if url, ok := event.EnrichmentMetadata["showcase_url"]; ok && url != "" {
			showcaseLink = url
		}
	}
	row = append(row, showcaseLink)

	return row
}

// appendToSheet appends a row to the specified Google Sheet
func appendToSheet(ctx context.Context, httpClient *http.Client, spreadsheetID, sheetName string, row []interface{}, fwCtx *framework.FrameworkContext) (int, error) {
	// Build request payload
	payload := map[string]interface{}{
		"values": [][]interface{}{row},
	}

	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Google Sheets API endpoint for appending
	url := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s:append?valueInputOption=USER_ENTERED", spreadsheetID, sheetName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	fwCtx.Logger.Debug("Sending POST to Google Sheets API",
		"url", url,
		"bodyLength", len(bodyJSON))

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Google Sheets API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		err := httputil.WrapResponseError(resp, "Google Sheets API error")
		fwCtx.Logger.Error("Google Sheets API error", "status", resp.StatusCode, "error", err)
		return 0, err
	}

	// Parse response to get updated range (includes row number)
	var respBody struct {
		Updates struct {
			UpdatedRange string `json:"updatedRange"`
		} `json:"updates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return 0, fmt.Errorf("failed to decode Google Sheets response: %w", err)
	}

	// Extract row number from updatedRange (e.g., "Activities!A2:K2" -> 2)
	rowNumber := 0
	if respBody.Updates.UpdatedRange != "" {
		parts := strings.Split(respBody.Updates.UpdatedRange, "!")
		if len(parts) == 2 {
			rangePart := parts[1]
			// Extract row number from range like "A2:K2"
			if idx := strings.Index(rangePart, ":"); idx > 0 {
				rowStr := rangePart[1:idx] // "2" from "A2"
				fmt.Sscanf(rowStr, "%d", &rowNumber)
			}
		}
	}

	return rowNumber, nil
}
