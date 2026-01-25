package intervalsuploader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/fitglue/server/src/go/pkg/loopprevention"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

const (
	baseURL = "https://intervals.icu/api/v1"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("UploadToIntervals", UploadToIntervals)
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

// UploadToIntervals is the entry point
func UploadToIntervals(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("intervals-uploader", svc, uploadHandler(nil))(ctx, e)
}

// uploadHandler contains the business logic
// httpClient can be injected for testing; if nil, creates Basic Auth client
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

		fwCtx.Logger.Info("Starting upload", "activity_id", eventPayload.ActivityId, "pipeline_id", eventPayload.PipelineId)

		// Note: Loop prevention is handled at source-handler level (isBounceback check)
		// The source handler checks uploaded_activities before publishing to the enricher

		// 1. Get user's Intervals integration credentials
		user, err := svc.DB.GetUser(ctx, eventPayload.UserId)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
		if user.Integrations == nil || user.Integrations.Intervals == nil || !user.Integrations.Intervals.Enabled {
			return nil, fmt.Errorf("Intervals integration not enabled for user")
		}

		intervalsIntegration := user.Integrations.Intervals
		if intervalsIntegration.ApiKey == "" || intervalsIntegration.AthleteId == "" {
			return nil, fmt.Errorf("Intervals credentials incomplete: missing API key or athlete ID")
		}

		// Initialize HTTP Client with Basic Auth if not provided (for testing)
		if httpClient == nil {
			httpClient = &http.Client{
				Timeout: 30 * time.Second,
			}
		}

		// Check if this is an UPDATE operation
		if useUpdate, ok := eventPayload.EnrichmentMetadata["use_update_method"]; ok && useUpdate == "true" {
			return handleIntervalsUpdate(ctx, httpClient, intervalsIntegration, &eventPayload, fwCtx)
		}

		// --- CREATE MODE ---
		return handleIntervalsCreate(ctx, httpClient, intervalsIntegration, &eventPayload, fwCtx)
	}
}

// handleIntervalsCreate uploads a new activity to Intervals.icu
func handleIntervalsCreate(ctx context.Context, httpClient *http.Client, integration *pb.IntervalsIntegration, eventPayload *pb.EnrichedActivityEvent, fwCtx *framework.FrameworkContext) (interface{}, error) {
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

	fwCtx.Logger.Info("Uploading to Intervals.icu (CREATE)",
		"title", eventPayload.Name,
		"type", eventPayload.ActivityType,
		"description_length", len(eventPayload.Description),
		"athlete_id", integration.AthleteId,
	)

	// Upload FIT file to Intervals.icu
	uploadURL := fmt.Sprintf("%s/athlete/%s/activities", baseURL, integration.AthleteId)
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewBuffer(fileData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Basic Auth with API key as username, no password
	req.SetBasicAuth(integration.ApiKey, "")
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := httpClient.Do(req)
	if err != nil {
		fwCtx.Logger.Error("Intervals API Error", "error", err)
		return nil, fmt.Errorf("Intervals API Error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		err := httputil.WrapResponseError(resp, "Intervals upload failed")
		fwCtx.Logger.Error("Intervals upload failed", "status", resp.StatusCode, "error", err)
		return nil, err
	}

	var uploadResp intervalsActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	fwCtx.Logger.Info("Upload complete", "activity_id", uploadResp.ID)

	// Update activity with title and description
	if eventPayload.Name != "" || eventPayload.Description != "" {
		updateResp, err := updateIntervalsActivity(ctx, httpClient, integration, uploadResp.ID, eventPayload, fwCtx)
		if err != nil {
			fwCtx.Logger.Warn("Failed to update activity metadata", "error", err)
		} else {
			uploadResp = *updateResp
		}
	}

	// Persist SynchronizedActivity
	intervalsDestID := fmt.Sprintf("%d", uploadResp.ID)

	// Record upload for loop prevention
	// Key is destination:destinationId so when Intervals sends a webhook,
	// we can look it up and detect the bounceback
	if intervalsDestID != "" {
		uploadRecord := &pb.UploadedActivityRecord{
			Id:            loopprevention.BuildUploadedActivityID(pb.Destination_DESTINATION_INTERVALS, intervalsDestID),
			UserId:        eventPayload.UserId,
			Source:        eventPayload.Source,
			ExternalId:    eventPayload.ActivityData.GetExternalId(),
			StartTime:     eventPayload.StartTime,
			Destination:   pb.Destination_DESTINATION_INTERVALS,
			DestinationId: intervalsDestID,
			UploadedAt:    timestamppb.Now(),
		}
		if err := svc.DB.SetUploadedActivity(ctx, eventPayload.UserId, uploadRecord); err != nil {
			fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
		} else {
			fwCtx.Logger.Debug("Recorded upload for loop prevention", "id", uploadRecord.Id)
		}
	}

	existingActivity, _ := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
	if existingActivity != nil {
		if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
			"destinations": map[string]interface{}{
				"intervals": intervalsDestID,
			},
			"synced_at": timestamppb.Now().AsTime(),
		}); err != nil {
			fwCtx.Logger.Error("Failed to update synchronized activity destinations", "error", err)
		} else {
			fwCtx.Logger.Info("Updated synchronized activity destinations (preserved execution ID)", "activity_id", eventPayload.ActivityId)
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
				"intervals": intervalsDestID,
			},
		}
		if err := svc.DB.SetSynchronizedActivity(ctx, eventPayload.UserId, syncedActivity); err != nil {
			fwCtx.Logger.Error("Failed to persist synchronized activity", "error", err)
		} else {
			fwCtx.Logger.Info("Persisted synchronized activity", "activity_id", eventPayload.ActivityId)
		}
	}

	// Increment sync count for billing
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
	}

	return map[string]interface{}{
		"status":                "SUCCESS",
		"intervals_activity_id": uploadResp.ID,
		"activity_id":           eventPayload.ActivityId,
		"pipeline_id":           eventPayload.PipelineId,
		"fit_file_uri":          eventPayload.FitFileUri,
		"activity_name":         eventPayload.Name,
		"activity_type":         activity.GetIntervalsActivityType(eventPayload.ActivityType),
		"description":           eventPayload.Description,
	}, nil
}

// handleIntervalsUpdate modifies an existing Intervals activity
func handleIntervalsUpdate(ctx context.Context, httpClient *http.Client, integration *pb.IntervalsIntegration, eventPayload *pb.EnrichedActivityEvent, fwCtx *framework.FrameworkContext) (interface{}, error) {
	fwCtx.Logger.Info("Starting Intervals UPDATE",
		"activity_id", eventPayload.ActivityId,
		"user_id", eventPayload.UserId)

	// Lookup SynchronizedActivity to get Intervals activity ID
	syncActivity, err := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
	if err != nil {
		return nil, fmt.Errorf("failed to get synchronized activity: %w", err)
	}
	if syncActivity == nil {
		return nil, fmt.Errorf("synchronized activity not found for activity_id: %s", eventPayload.ActivityId)
	}

	intervalsIDStr, ok := syncActivity.Destinations["intervals"]
	if !ok || intervalsIDStr == "" {
		return nil, fmt.Errorf("no Intervals destination found in synchronized activity")
	}

	fwCtx.Logger.Info("Found existing Intervals activity", "intervals_activity_id", intervalsIDStr)

	// GET current activity from Intervals
	getURL := fmt.Sprintf("%s/athlete/%s/activities/%s", baseURL, integration.AthleteId, intervalsIDStr)
	getReq, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}
	getReq.SetBasicAuth(integration.ApiKey, "")

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

	// Merge description: use DESTINATION's description as base (fetched via GET above)
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

	// Build update payload
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
			"status":                "SUCCESS",
			"intervals_activity_id": intervalsIDStr,
			"update_skipped":        true,
			"reason":                "no_changes",
			"activity_name":         eventPayload.Name,
			"activity_type":         eventPayload.ActivityType.String(),
			"description":           eventPayload.Description,
		}, nil
	}

	bodyJSON, err := json.Marshal(updateBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update body: %w", err)
	}

	// PUT to Intervals
	putURL := fmt.Sprintf("%s/athlete/%s/activities/%s", baseURL, integration.AthleteId, intervalsIDStr)
	putReq, err := http.NewRequestWithContext(ctx, "PUT", putURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	putReq.SetBasicAuth(integration.ApiKey, "")
	putReq.Header.Set("Content-Type", "application/json")

	fwCtx.Logger.Info("Updating Intervals activity (PUT)",
		"intervals_activity_id", intervalsIDStr,
		"updated_fields", updateBody,
		"description_length", len(mergedDescription),
	)

	putResp, err := httpClient.Do(putReq)
	if err != nil {
		return nil, fmt.Errorf("failed to PUT activity: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode >= 400 {
		err := httputil.WrapResponseError(putResp, "Intervals PUT failed")
		fwCtx.Logger.Error("Intervals PUT failed", "status", putResp.StatusCode, "error", err)
		return nil, err
	}

	fwCtx.Logger.Info("Successfully updated Intervals activity",
		"intervals_activity_id", intervalsIDStr,
		"updated_fields", updateBody)

	// Update SynchronizedActivity with new description
	if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
		"description": mergedDescription,
	}); err != nil {
		fwCtx.Logger.Warn("Failed to update synchronized activity description", "error", err)
	}

	// Increment sync count for billing
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
	}

	return map[string]interface{}{
		"status":                "SUCCESS",
		"intervals_activity_id": intervalsIDStr,
		"updated_fields":        updateBody,
		"mode":                  "UPDATE",
		"activity_name":         eventPayload.Name,
		"activity_type":         eventPayload.ActivityType.String(),
		"description":           mergedDescription,
	}, nil
}

// updateIntervalsActivity updates activity name and description after FIT upload
func updateIntervalsActivity(ctx context.Context, httpClient *http.Client, integration *pb.IntervalsIntegration, activityID int64, eventPayload *pb.EnrichedActivityEvent, fwCtx *framework.FrameworkContext) (*intervalsActivityResponse, error) {
	updateBody := map[string]interface{}{}
	if eventPayload.Name != "" {
		updateBody["name"] = eventPayload.Name
	}
	if eventPayload.Description != "" {
		updateBody["description"] = eventPayload.Description
	}
	if eventPayload.ActivityType != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
		updateBody["type"] = activity.GetIntervalsActivityType(eventPayload.ActivityType)
	}

	bodyJSON, err := json.Marshal(updateBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update body: %w", err)
	}

	putURL := fmt.Sprintf("%s/athlete/%s/activities/%d", baseURL, integration.AthleteId, activityID)
	putReq, err := http.NewRequestWithContext(ctx, "PUT", putURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	putReq.SetBasicAuth(integration.ApiKey, "")
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := httpClient.Do(putReq)
	if err != nil {
		return nil, fmt.Errorf("failed to PUT activity: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode >= 400 {
		return nil, httputil.WrapResponseError(putResp, "Intervals PUT failed")
	}

	var updatedActivity intervalsActivityResponse
	if err := json.NewDecoder(putResp.Body).Decode(&updatedActivity); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &updatedActivity, nil
}

type intervalsActivityResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Type           string `json:"type"`
	StartDateLocal string `json:"start_date_local"`
}
