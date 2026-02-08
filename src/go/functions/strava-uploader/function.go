package stravauploader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/description"
	"github.com/fitglue/server/src/go/pkg/destination"
	"github.com/fitglue/server/src/go/pkg/domain/activity"
	"github.com/fitglue/server/src/go/pkg/framework"
	httputil "github.com/fitglue/server/src/go/pkg/infrastructure/http"
	"github.com/fitglue/server/src/go/pkg/infrastructure/oauth"
	"github.com/fitglue/server/src/go/pkg/loopprevention"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"

	strava "github.com/fitglue/server/src/go/pkg/api/strava"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("UploadToStrava", UploadToStrava)
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

// UploadToStrava is the entry point
func UploadToStrava(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("strava-uploader", svc, uploadHandler(nil))(ctx, e)
}

// uploadHandler contains the business logic
// httpClient can be injected for testing; if nil, creates OAuth client
func uploadHandler(httpClient *http.Client) framework.HandlerFunc {
	return func(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		var eventPayload pb.EnrichedActivityEvent

		// Use protojson to unmarshal the event data to handle enum strings correctly
		// Standard json.Unmarshal (used by DataAs) fails on enum strings for int32 fields
		unmarshaler := protojson.UnmarshalOptions{
			DiscardUnknown: true,
			AllowPartial:   true,
		}
		if err := unmarshaler.Unmarshal(e.Data(), &eventPayload); err != nil {
			// Fallback to DataAs if protojson fails (e.g. if data is not JSON object but simple string?)
			// But for our use case, it should be JSON.
			return nil, fmt.Errorf("protojson.Unmarshal: %w", err)
		}

		// Resolve activity data from GCS if needed (for large payloads offloaded by enricher)
		if err := activity.ResolveEnrichedEvent(ctx, &eventPayload, fwCtx.Service.Store); err != nil {
			fwCtx.Logger.Warn("Failed to resolve activity data from GCS", "error", err)
			// Continue anyway - activity_data may not be needed for all operations
		}

		fwCtx.Logger.Info("Starting upload", "activity_id", eventPayload.ActivityId, "pipeline_id", eventPayload.PipelineId)

		// Note: Loop prevention is handled at source-handler level (isBounceback check)
		// The source handler checks uploaded_activities before publishing to the enricher

		// Initialize OAuth HTTP Client if not provided (for testing)
		if httpClient == nil {
			tokenSource := oauth.NewFirestoreTokenSource(fwCtx.Service, eventPayload.UserId, "strava")
			httpClient = oauth.NewClientWithUsageTracking(tokenSource, fwCtx.Service, eventPayload.UserId, "strava")
		}

		// Check if this is an UPDATE operation (resume mode with existing activity)
		// The ActivityPayload metadata contains the use_update_method flag
		if useUpdate, ok := eventPayload.EnrichmentMetadata["use_update_method"]; ok && useUpdate == "true" {
			return handleStravaUpdate(ctx, httpClient, &eventPayload, fwCtx)
		}

		// --- CREATE MODE ---
		return handleStravaCreate(ctx, httpClient, &eventPayload, fwCtx)
	}
}

// handleStravaCreate uploads a new activity to Strava (POST /uploads)
func handleStravaCreate(ctx context.Context, httpClient *http.Client, eventPayload *pb.EnrichedActivityEvent, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Download FIT from GCS
	bucketName := fwCtx.Service.Config.GCSArtifactBucket
	if bucketName == "" {
		bucketName = "fitglue-server-dev-artifacts" // Fallback for local development
	}
	objectName := strings.TrimPrefix(eventPayload.FitFileUri, "gs://"+bucketName+"/")

	fileData, err := fwCtx.Service.Store.Read(ctx, bucketName, objectName)
	if err != nil {
		fwCtx.Logger.Error("GCS Read Error", "error", err)
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("GCS error: %s", err), fwCtx.Logger)
		return nil, fmt.Errorf("GCS Read Error: %w", err)
	}

	// Build multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "activity.fit")
	part.Write(fileData)
	writer.WriteField("data_type", "fit")
	if eventPayload.Name != "" {
		writer.WriteField("name", eventPayload.Name)
	}
	if eventPayload.Description != "" {
		writer.WriteField("description", eventPayload.Description)
	}
	if eventPayload.ActivityType != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
		stravaType := activity.GetStravaActivityType(eventPayload.ActivityType)
		writer.WriteField("sport_type", stravaType)
		writer.WriteField("activity_type", stravaType) // Legacy fallback
	}
	writer.Close()

	// Log what we're uploading for debugging
	fwCtx.Logger.Info("Uploading to Strava (CREATE)",
		"title", eventPayload.Name,
		"type", eventPayload.ActivityType,
		"description_length", len(eventPayload.Description),
		"description_preview", truncateString(eventPayload.Description, 200),
	)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.strava.com/api/v3/uploads", body)
	if err != nil {
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("failed to create request: %s", err), fwCtx.Logger)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute with OAuth transport (handles auth + token refresh)
	httpResp, err := httpClient.Do(req)
	if err != nil {
		fwCtx.Logger.Error("Strava API Error", "error", err)
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("API error: %s", err), fwCtx.Logger)
		return nil, fmt.Errorf("Strava API Error: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		err := httputil.WrapResponseError(httpResp, "Strava upload failed")
		fwCtx.Logger.Error("Strava upload failed", "status", httpResp.StatusCode, "error", err)
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, err), fwCtx.Logger)
		return nil, err
	}

	var uploadResp strava.Upload
	json.NewDecoder(httpResp.Body).Decode(&uploadResp)

	fwCtx.Logger.Info("Upload initiated", "upload_id", derefInt64(uploadResp.Id), "status", derefStr(uploadResp.Status))

	// Soft Poll: Wait up to 15 seconds for completion
	// This covers 95% of use cases without needing complex async infrastructure
	if derefInt64(uploadResp.ActivityId) == 0 {
		finalResp, err := waitForUploadCompletion(ctx, httpClient, derefInt64(uploadResp.Id), fwCtx.Logger)
		if err != nil {
			// Log warning but return SUCCESS with partial data so pipeline continues
			fwCtx.Logger.Warn("Soft polling finished without final ID (async processing continues)", "error", err)
		} else {
			uploadResp = *finalResp
		}
	}

	fwCtx.Logger.Info("Upload complete", "upload_id", derefInt64(uploadResp.Id), "activity_id", derefInt64(uploadResp.ActivityId), "status", derefStr(uploadResp.Status))

	// Check for duplicate activity (common during testing - not a true error)
	// Strava returns this in the error field when the FIT file was already uploaded
	if derefStr(uploadResp.Error) != "" && strings.Contains(strings.ToLower(derefStr(uploadResp.Error)), "duplicate") {
		fwCtx.Logger.Info("Strava duplicate activity detected - skipping",
			"upload_error", derefStr(uploadResp.Error),
			"activity_id", eventPayload.ActivityId,
			"pipeline_id", eventPayload.PipelineId,
		)
		// Update PipelineRun destination as skipped (duplicate)
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SKIPPED, "", "duplicate_activity", fwCtx.Logger)
		// Return SKIPPED status without error - this prevents Sentry capture
		// and shows as "skipped" in the UI rather than "failed"
		return map[string]interface{}{
			"status":           "SKIPPED",
			"skip_reason":      "duplicate_activity",
			"strava_error":     derefStr(uploadResp.Error),
			"strava_upload_id": derefInt64(uploadResp.Id),
			"activity_id":      eventPayload.ActivityId,
			"pipeline_id":      eventPayload.PipelineId,
			"fit_file_uri":     eventPayload.FitFileUri,
			"activity_name":    eventPayload.Name,
			"activity_type":    activity.GetStravaActivityType(eventPayload.ActivityType),
		}, nil
	}

	// Record upload for loop prevention if successful
	if derefInt64(uploadResp.ActivityId) != 0 {
		stravaDestID := fmt.Sprintf("%d", derefInt64(uploadResp.ActivityId))

		// Record upload for loop prevention
		// Key is destination:destinationId so when Strava sends a webhook with activityId,
		// we can look it up by STRAVA:{activityId} and detect the bounceback
		uploadRecord := &pb.UploadedActivityRecord{
			Id:            loopprevention.BuildUploadedActivityID(pb.Destination_DESTINATION_STRAVA, stravaDestID),
			UserId:        eventPayload.UserId,
			Source:        eventPayload.Source,
			ExternalId:    eventPayload.ActivityData.GetExternalId(),
			StartTime:     eventPayload.StartTime,
			Destination:   pb.Destination_DESTINATION_STRAVA,
			DestinationId: stravaDestID,
			UploadedAt:    timestamppb.Now(),
		}
		if err := svc.DB.SetUploadedActivity(ctx, eventPayload.UserId, uploadRecord); err != nil {
			fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
			// Don't fail the upload - this is just for loop prevention
		} else {
			fwCtx.Logger.Debug("Recorded upload for loop prevention", "id", uploadRecord.Id)
		}

		// Note: synchronized_activities is deprecated - pipeline_runs is now the source of truth
		// The destination.UpdateStatus call at the end of this function updates pipeline_runs with the externalId

		// Increment sync count for billing (per successful destination sync)
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		// Note: Photo uploads to Strava require partnership status, which we don't have
		// The Strava API returns 403 for non-partner apps attempting to upload photos
	}

	status := "SUCCESS"
	if derefStr(uploadResp.Error) != "" {
		status = "FAILED_STRAVA_PROCESSING"
	} else if derefInt64(uploadResp.ActivityId) == 0 {
		// ActivityID is still 0 after soft polling - Strava is still processing async
		// Return PENDING status so this shows up in failed/stalled list
		status = "PENDING_STRAVA_PROCESSING"
	}

	result := map[string]interface{}{
		"status":             status,
		"strava_upload_id":   derefInt64(uploadResp.Id),
		"strava_activity_id": derefInt64(uploadResp.ActivityId),
		"upload_status":      derefStr(uploadResp.Status),
		"upload_error":       derefStr(uploadResp.Error),
		"activity_id":        eventPayload.ActivityId,
		"pipeline_id":        eventPayload.PipelineId,
		"fit_file_uri":       eventPayload.FitFileUri,
		"activity_name":      eventPayload.Name,
		"activity_type":      activity.GetStravaActivityType(eventPayload.ActivityType),
		"description":        eventPayload.Description,
	}

	if status != "SUCCESS" {
		errMsg := derefStr(uploadResp.Error)
		if errMsg == "" {
			errMsg = fmt.Sprintf("status=%s, activity_id=%d", status, derefInt64(uploadResp.ActivityId))
		}
		// Update PipelineRun destination as failed
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", errMsg, fwCtx.Logger)
		return result, fmt.Errorf("strava upload incomplete: %s", errMsg)
	}

	// Update PipelineRun destination as synced
	stravaDestID := fmt.Sprintf("%d", derefInt64(uploadResp.ActivityId))
	destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, stravaDestID, "", fwCtx.Logger)

	return result, nil
}

// handleStravaUpdate modifies an existing Strava activity (PUT /activities/{id})
// Used in resume mode for delayed enrichment (e.g., Parkrun results)
func handleStravaUpdate(ctx context.Context, httpClient *http.Client, eventPayload *pb.EnrichedActivityEvent, fwCtx *framework.FrameworkContext) (interface{}, error) {
	fwCtx.Logger.Info("Starting Strava UPDATE",
		"activity_id", eventPayload.ActivityId,
		"user_id", eventPayload.UserId)

	// 1. Lookup PipelineRun to get Strava activity ID from destinations
	pipelineRun, err := svc.DB.GetPipelineRunByActivityId(ctx, eventPayload.UserId, eventPayload.ActivityId)
	if err != nil {
		// Activity not found is expected in resume mode if original upload failed before storing
		// Return SKIPPED (not error) to prevent Sentry noise
		fwCtx.Logger.Info("Pipeline run not found for UPDATE - skipping",
			"activity_id", eventPayload.ActivityId,
			"error", err,
		)
		// Update PipelineRun destination as skipped
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SKIPPED, "", "activity_not_found", fwCtx.Logger)
		return map[string]interface{}{
			"status":      "SKIPPED",
			"skip_reason": "activity_not_found",
			"activity_id": eventPayload.ActivityId,
			"pipeline_id": eventPayload.PipelineId,
			"mode":        "UPDATE",
			"details":     "Original upload may have failed before storing pipeline run",
		}, nil
	}
	if pipelineRun == nil {
		fwCtx.Logger.Info("Pipeline run is nil for UPDATE - skipping",
			"activity_id", eventPayload.ActivityId,
		)
		// Update PipelineRun destination as skipped
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SKIPPED, "", "activity_not_found", fwCtx.Logger)
		return map[string]interface{}{
			"status":      "SKIPPED",
			"skip_reason": "activity_not_found",
			"activity_id": eventPayload.ActivityId,
			"pipeline_id": eventPayload.PipelineId,
			"mode":        "UPDATE",
		}, nil
	}

	// Extract Strava external ID from destinations array
	var stravaIDStr string
	for _, dest := range pipelineRun.Destinations {
		if dest.Destination == pb.Destination_DESTINATION_STRAVA && dest.ExternalId != nil && *dest.ExternalId != "" {
			stravaIDStr = *dest.ExternalId
			break
		}
	}

	if stravaIDStr == "" {
		fwCtx.Logger.Info("No Strava destination in pipeline run for UPDATE - skipping",
			"activity_id", eventPayload.ActivityId,
			"destinations", pipelineRun.Destinations,
		)
		// Update PipelineRun destination as skipped
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SKIPPED, "", "no_strava_destination", fwCtx.Logger)
		return map[string]interface{}{
			"status":      "SKIPPED",
			"skip_reason": "no_strava_destination",
			"activity_id": eventPayload.ActivityId,
			"pipeline_id": eventPayload.PipelineId,
			"mode":        "UPDATE",
			"details":     "Activity was synced to other destinations but not Strava",
		}, nil
	}

	fwCtx.Logger.Info("Found existing Strava activity", "strava_activity_id", stravaIDStr)

	// 2. GET current activity from Strava to get existing description
	getURL := fmt.Sprintf("https://www.strava.com/api/v3/activities/%s", stravaIDStr)
	getReq, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	getResp, err := httpClient.Do(getReq)
	if err != nil {
		return nil, fmt.Errorf("failed to GET existing activity: %w", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(getResp.Body)
		return nil, fmt.Errorf("GET activity failed: status %d, body: %s", getResp.StatusCode, string(bodyBytes))
	}

	var existingActivity struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&existingActivity); err != nil {
		return nil, fmt.Errorf("failed to decode existing activity: %w", err)
	}

	// 3. Merge description: use DESTINATION's description as base (fetched via GET above)
	// then apply section replacement with the enricher's new content
	mergedDescription := existingActivity.Description // From destination API, NOT eventPayload
	if eventPayload.Description != "" {
		// Check for section header in metadata (signals replaceable section)
		sectionHeader := ""
		for key, val := range eventPayload.EnrichmentMetadata {
			if strings.HasPrefix(key, "section_header_") {
				sectionHeader = val
				break
			}
		}

		if sectionHeader != "" && description.HasSection(mergedDescription, sectionHeader) {
			// Extract just the section content from eventPayload.Description
			// The payload contains the full description, but we only want the specific section
			newSectionContent := description.ExtractSection(eventPayload.Description, sectionHeader)
			if newSectionContent != "" {
				mergedDescription = description.ReplaceSection(mergedDescription, sectionHeader, newSectionContent)
				fwCtx.Logger.Info("Replaced description section", "header", sectionHeader)
			} else {
				fwCtx.Logger.Warn("Section header found in metadata but content not found in payload", "header", sectionHeader)
			}
		} else if mergedDescription != "" {
			// Fallback to append
			mergedDescription += "\n\n" + eventPayload.Description
		} else {
			mergedDescription = eventPayload.Description
		}
	}

	// 4. Build update payload
	// In UPDATE mode, we intentionally do NOT update the title.
	// The activity already exists on Strava, and the user may have customized the title there.
	// We only update the description with new enrichment content.
	updateBody := map[string]interface{}{}
	if mergedDescription != existingActivity.Description {
		updateBody["description"] = mergedDescription
	}

	if len(updateBody) == 0 {
		fwCtx.Logger.Info("No changes to update, skipping PUT")
		// Update PipelineRun destination as success (no changes needed, but activity is already synced)
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, stravaIDStr, "", fwCtx.Logger)
		return map[string]interface{}{
			"status":             "SUCCESS",
			"strava_activity_id": stravaIDStr,
			"update_skipped":     true,
			"reason":             "no_changes",
			"activity_name":      eventPayload.Name,
			"activity_type":      eventPayload.ActivityType.String(),
			"description":        eventPayload.Description,
		}, nil
	}

	bodyJSON, err := json.Marshal(updateBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update body: %w", err)
	}

	// 5. PUT to Strava
	putURL := fmt.Sprintf("https://www.strava.com/api/v3/activities/%s", stravaIDStr)
	putReq, err := http.NewRequestWithContext(ctx, "PUT", putURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	putReq.Header.Set("Content-Type", "application/json")

	fwCtx.Logger.Info("Updating Strava activity (PUT)",
		"strava_activity_id", stravaIDStr,
		"updated_fields", updateBody,
		"description_length", len(mergedDescription),
	)

	putResp, err := httpClient.Do(putReq)
	if err != nil {
		return nil, fmt.Errorf("failed to PUT activity: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode >= 400 {
		err := httputil.WrapResponseError(putResp, "Strava PUT failed")
		fwCtx.Logger.Error("Strava PUT failed", "status", putResp.StatusCode, "error", err)
		return nil, err
	}

	fwCtx.Logger.Info("Successfully updated Strava activity",
		"strava_activity_id", stravaIDStr,
		"updated_fields", updateBody)

	// Note: synchronized_activities is deprecated - pipeline_runs is now the source of truth
	// We no longer update synchronized_activities here

	// Increment sync count for billing (per successful destination sync)
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
	}

	// Record upload for loop prevention (same as create path)
	uploadRecord := &pb.UploadedActivityRecord{
		Id:            loopprevention.BuildUploadedActivityID(pb.Destination_DESTINATION_STRAVA, stravaIDStr),
		UserId:        eventPayload.UserId,
		Source:        eventPayload.Source,
		ExternalId:    eventPayload.ActivityData.GetExternalId(),
		StartTime:     eventPayload.StartTime,
		Destination:   pb.Destination_DESTINATION_STRAVA,
		DestinationId: stravaIDStr,
		UploadedAt:    timestamppb.Now(),
	}
	if err := svc.DB.SetUploadedActivity(ctx, eventPayload.UserId, uploadRecord); err != nil {
		fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
	}

	return map[string]interface{}{
		"status":             "SUCCESS",
		"strava_activity_id": stravaIDStr,
		"updated_fields":     updateBody,
		"mode":               "UPDATE",
		"activity_name":      eventPayload.Name,
		"activity_type":      eventPayload.ActivityType.String(),
		"description":        mergedDescription,
	}, nil
}

// derefInt64 safely dereferences a *int64 pointer, returning 0 if nil.
func derefInt64(p *int64) int64 {
	if p != nil {
		return *p
	}
	return 0
}

// derefStr safely dereferences a *string pointer, returning "" if nil.
func derefStr(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

func waitForUploadCompletion(ctx context.Context, client *http.Client, uploadID int64, logger *slog.Logger) (*strava.Upload, error) {
	// Check every 2 seconds, give up after 15 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for upload processing")
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://www.strava.com/api/v3/uploads/%d", uploadID), nil)
			if err != nil {
				return nil, err
			}

			resp, err := client.Do(req)
			if err != nil {
				logger.Warn("Failed to poll upload status", "error", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logger.Warn("Poll returned non-200 status", "status", resp.StatusCode)
				continue
			}

			var status strava.Upload
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				return nil, fmt.Errorf("failed to decode poll response: %w", err)
			}

			logger.Info("Polled upload status", "status", derefStr(status.Status), "activity_id", derefInt64(status.ActivityId), "error", derefStr(status.Error))

			if derefInt64(status.ActivityId) != 0 || derefStr(status.Error) != "" {
				return &status, nil
			}
			// Continue polling if still processing (activity_id == 0 and no error)
		}
	}
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
