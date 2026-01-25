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
		bucketName = "fitglue-artifacts"
	}
	objectName := strings.TrimPrefix(eventPayload.FitFileUri, "gs://"+bucketName+"/")

	fileData, err := fwCtx.Service.Store.Read(ctx, bucketName, objectName)
	if err != nil {
		fwCtx.Logger.Error("GCS Read Error", "error", err)
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
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute with OAuth transport (handles auth + token refresh)
	httpResp, err := httpClient.Do(req)
	if err != nil {
		fwCtx.Logger.Error("Strava API Error", "error", err)
		return nil, fmt.Errorf("Strava API Error: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		err := httputil.WrapResponseError(httpResp, "Strava upload failed")
		fwCtx.Logger.Error("Strava upload failed", "status", httpResp.StatusCode, "error", err)
		return nil, err
	}

	var uploadResp stravaUploadResponse
	json.NewDecoder(httpResp.Body).Decode(&uploadResp)

	fwCtx.Logger.Info("Upload initiated", "upload_id", uploadResp.ID, "status", uploadResp.Status)

	// Soft Poll: Wait up to 15 seconds for completion
	// This covers 95% of use cases without needing complex async infrastructure
	if uploadResp.ActivityID == 0 {
		finalResp, err := waitForUploadCompletion(ctx, httpClient, uploadResp.ID, fwCtx.Logger)
		if err != nil {
			// Log warning but return SUCCESS with partial data so pipeline continues
			fwCtx.Logger.Warn("Soft polling finished without final ID (async processing continues)", "error", err)
		} else {
			uploadResp = *finalResp
		}
	}

	fwCtx.Logger.Info("Upload complete", "upload_id", uploadResp.ID, "activity_id", uploadResp.ActivityID, "status", uploadResp.Status)

	// Persist SynchronizedActivity if successful
	if uploadResp.ActivityID != 0 {
		stravaDestID := fmt.Sprintf("%d", uploadResp.ActivityID)

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

		// Check if activity already exists (e.g., repost scenario)
		// If it does, only update destinations to preserve original pipelineExecutionId
		existingActivity, _ := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
		if existingActivity != nil {
			// Activity exists - update only destinations (preserves original pipelineExecutionId for boosters display)
			// Use nested map structure so MergeAll properly merges into destinations
			if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
				"destinations": map[string]interface{}{
					"strava": stravaDestID,
				},
				"synced_at": timestamppb.Now().AsTime(),
			}); err != nil {
				fwCtx.Logger.Error("Failed to update synchronized activity destinations", "error", err)
			} else {
				fwCtx.Logger.Info("Updated synchronized activity destinations (preserved execution ID)", "activity_id", eventPayload.ActivityId)
			}
		} else {
			// New activity - create full record including pipelineExecutionId
			syncedActivity := &pb.SynchronizedActivity{
				ActivityId:          eventPayload.ActivityId,
				Title:               eventPayload.Name,
				Description:         eventPayload.Description,
				Type:                eventPayload.ActivityType,
				Source:              eventPayload.Source.String(), // Use original event source (FROM webhook trigger)
				StartTime:           eventPayload.StartTime,
				SyncedAt:            timestamppb.Now(),
				PipelineId:          eventPayload.PipelineId,
				PipelineExecutionId: fwCtx.PipelineExecutionId, // Link to execution trace
				Destinations: map[string]string{
					"strava": stravaDestID,
				},
			}
			if err := svc.DB.SetSynchronizedActivity(ctx, eventPayload.UserId, syncedActivity); err != nil {
				fwCtx.Logger.Error("Failed to persist synchronized activity", "error", err)
				// Don't fail the function, this is just recording history
			} else {
				fwCtx.Logger.Info("Persisted synchronized activity", "activity_id", eventPayload.ActivityId)
			}
		}

		// Increment sync count for billing (per successful destination sync)
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		// Upload enrichment assets as photos (Athlete tier feature)
		if eventPayload.EnrichmentMetadata != nil {
			if err := uploadPhotosToStrava(ctx, httpClient, uploadResp.ActivityID, eventPayload.EnrichmentMetadata, fwCtx); err != nil {
				fwCtx.Logger.Warn("Failed to upload photos to Strava", "error", err)
				// Don't fail the overall upload - photos are a nice-to-have enhancement
			}
		}
	}

	status := "SUCCESS"
	if uploadResp.Error != "" {
		status = "FAILED_STRAVA_PROCESSING"
	} else if uploadResp.ActivityID == 0 {
		// ActivityID is still 0 after soft polling - Strava is still processing async
		// Return PENDING status so this shows up in failed/stalled list
		status = "PENDING_STRAVA_PROCESSING"
	}

	result := map[string]interface{}{
		"status":             status,
		"strava_upload_id":   uploadResp.ID,
		"strava_activity_id": uploadResp.ActivityID,
		"upload_status":      uploadResp.Status,
		"upload_error":       uploadResp.Error,
		"activity_id":        eventPayload.ActivityId,
		"pipeline_id":        eventPayload.PipelineId,
		"fit_file_uri":       eventPayload.FitFileUri,
		"activity_name":      eventPayload.Name,
		"activity_type":      activity.GetStravaActivityType(eventPayload.ActivityType),
		"description":        eventPayload.Description,
	}

	if status != "SUCCESS" {
		errMsg := uploadResp.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("status=%s, activity_id=%d", status, uploadResp.ActivityID)
		}
		return result, fmt.Errorf("strava upload incomplete: %s", errMsg)
	}

	return result, nil
}

// handleStravaUpdate modifies an existing Strava activity (PUT /activities/{id})
// Used in resume mode for delayed enrichment (e.g., Parkrun results)
func handleStravaUpdate(ctx context.Context, httpClient *http.Client, eventPayload *pb.EnrichedActivityEvent, fwCtx *framework.FrameworkContext) (interface{}, error) {
	fwCtx.Logger.Info("Starting Strava UPDATE",
		"activity_id", eventPayload.ActivityId,
		"user_id", eventPayload.UserId)

	// 1. Lookup SynchronizedActivity to get Strava activity ID
	syncActivity, err := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
	if err != nil {
		return nil, fmt.Errorf("failed to get synchronized activity: %w", err)
	}
	if syncActivity == nil {
		return nil, fmt.Errorf("synchronized activity not found for activity_id: %s", eventPayload.ActivityId)
	}

	stravaIDStr, ok := syncActivity.Destinations["strava"]
	if !ok || stravaIDStr == "" {
		return nil, fmt.Errorf("no Strava destination found in synchronized activity")
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
			// Replace the existing section with the new content
			mergedDescription = description.ReplaceSection(mergedDescription, sectionHeader, eventPayload.Description)
			fwCtx.Logger.Info("Replaced description section", "header", sectionHeader)
		} else if mergedDescription != "" {
			// Fallback to append
			mergedDescription += "\n\n" + eventPayload.Description
		} else {
			mergedDescription = eventPayload.Description
		}
	}

	// 4. Build update payload
	updateBody := map[string]interface{}{}
	if eventPayload.Name != "" && eventPayload.Name != existingActivity.Name {
		updateBody["name"] = eventPayload.Name
	}
	if mergedDescription != existingActivity.Description {
		updateBody["description"] = mergedDescription
	}

	if len(updateBody) == 0 {
		fwCtx.Logger.Info("No changes to update, skipping PUT")
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

	// 6. Update SynchronizedActivity with new description
	if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
		"description": mergedDescription,
	}); err != nil {
		fwCtx.Logger.Warn("Failed to update synchronized activity description", "error", err)
	}

	// Increment sync count for billing (per successful destination sync)
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
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

type stravaUploadResponse struct {
	ID         int64  `json:"id"`
	ExternalID string `json:"external_id"`
	ActivityID int64  `json:"activity_id"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

func waitForUploadCompletion(ctx context.Context, client *http.Client, uploadID int64, logger *slog.Logger) (*stravaUploadResponse, error) {
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

			var status stravaUploadResponse
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				return nil, fmt.Errorf("failed to decode poll response: %w", err)
			}

			logger.Info("Polled upload status", "status", status.Status, "activity_id", status.ActivityID, "error", status.Error)

			if status.ActivityID != 0 || status.Error != "" {
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

// uploadPhotosToStrava uploads enrichment assets (images) to a Strava activity
// It iterates through enrichment_metadata looking for keys prefixed with "asset_"
// and POSTs each image to Strava's photo API
func uploadPhotosToStrava(ctx context.Context, httpClient *http.Client, activityID int64, metadata map[string]string, fwCtx *framework.FrameworkContext) error {
	bucketName := fwCtx.Service.Config.GCSArtifactBucket
	if bucketName == "" {
		bucketName = "fitglue-artifacts"
	}

	var uploadErrors []string

	for key, gcsURI := range metadata {
		// Only process asset_* keys (e.g., asset_muscle_heatmap, asset_route_thumbnail)
		if !strings.HasPrefix(key, "asset_") {
			continue
		}

		fwCtx.Logger.Info("Uploading photo to Strava", "asset_key", key, "gcs_uri", gcsURI, "activity_id", activityID)

		// Extract object name from GCS URI (gs://bucket/path/to/file.png)
		objectName := strings.TrimPrefix(gcsURI, "gs://"+bucketName+"/")
		if objectName == gcsURI {
			// URI didn't match expected bucket format, try to extract path after any bucket
			parts := strings.SplitN(gcsURI, "/", 4)
			if len(parts) >= 4 {
				objectName = parts[3]
			} else {
				fwCtx.Logger.Warn("Invalid GCS URI format", "uri", gcsURI)
				uploadErrors = append(uploadErrors, fmt.Sprintf("%s: invalid URI", key))
				continue
			}
		}

		// Download image from GCS
		imageData, err := fwCtx.Service.Store.Read(ctx, bucketName, objectName)
		if err != nil {
			fwCtx.Logger.Warn("Failed to download image from GCS", "error", err, "object", objectName)
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: GCS read failed", key))
			continue
		}

		// Determine filename and content type from object name
		filename := objectName
		if idx := strings.LastIndex(objectName, "/"); idx >= 0 {
			filename = objectName[idx+1:]
		}

		// Build multipart form data
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			fwCtx.Logger.Warn("Failed to create form file", "error", err)
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: form creation failed", key))
			continue
		}
		part.Write(imageData)
		writer.Close()

		// POST to Strava photos API
		photoURL := fmt.Sprintf("https://www.strava.com/api/v3/activities/%d/photos", activityID)
		req, err := http.NewRequestWithContext(ctx, "POST", photoURL, body)
		if err != nil {
			fwCtx.Logger.Warn("Failed to create photo upload request", "error", err)
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: request creation failed", key))
			continue
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := httpClient.Do(req)
		if err != nil {
			fwCtx.Logger.Warn("Failed to upload photo to Strava", "error", err)
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: upload failed", key))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			fwCtx.Logger.Warn("Strava photo upload failed",
				"status", resp.StatusCode,
				"response", string(bodyBytes),
				"asset_key", key)
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: status %d", key, resp.StatusCode))
			continue
		}

		fwCtx.Logger.Info("Successfully uploaded photo to Strava", "asset_key", key, "activity_id", activityID)
	}

	if len(uploadErrors) > 0 {
		return fmt.Errorf("photo uploads had errors: %s", strings.Join(uploadErrors, "; "))
	}

	return nil
}
