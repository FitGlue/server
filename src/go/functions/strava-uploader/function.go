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

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/framework"
	"github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/oauth"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
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
			slog.Error("Failed to initialize service", "error", err)
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
		if err := e.DataAs(&eventPayload); err != nil {
			return nil, fmt.Errorf("event.DataAs(EnrichedActivityEvent): %v", err)
		}

		fwCtx.Logger.Info("Starting upload", "activity_id", eventPayload.ActivityId, "pipeline_id", eventPayload.PipelineId)

		// Initialize OAuth HTTP Client if not provided (for testing)
		if httpClient == nil {
			tokenSource := oauth.NewFirestoreTokenSource(fwCtx.Service, eventPayload.UserId, "strava")
			httpClient = oauth.NewHTTPClient(tokenSource)
		}

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
		if eventPayload.ActivityType != "" {
			writer.WriteField("sport_type", eventPayload.ActivityType)
			writer.WriteField("activity_type", eventPayload.ActivityType) // Legacy fallback
		}
		writer.Close()

		// Log what we're uploading for debugging
		fwCtx.Logger.Info("Uploading to Strava",
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
			bodyBytes, _ := io.ReadAll(httpResp.Body)
			fwCtx.Logger.Error("Strava upload failed", "status", httpResp.StatusCode, "body", string(bodyBytes))
			return nil, fmt.Errorf("strava upload failed: status %d", httpResp.StatusCode)
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

		status := "SUCCESS"
		if uploadResp.Error != "" {
			status = "FAILED_STRAVA_PROCESSING"
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
			"activity_type":      eventPayload.ActivityType,
			"description":        eventPayload.Description,
		}

		if status != "SUCCESS" {
			return result, fmt.Errorf("strava upload failed: %s", uploadResp.Error)
		}

		return result, nil
	}
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
