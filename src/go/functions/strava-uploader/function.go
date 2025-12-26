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

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/framework"
	"github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/oauth"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
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
		// Parse Pub/Sub message
		var msg types.PubSubMessage
		if err := e.DataAs(&msg); err != nil {
			return nil, fmt.Errorf("event.DataAs: %v", err)
		}

		var eventPayload pb.EnrichedActivityEvent
		unmarshalOpts := protojson.UnmarshalOptions{DiscardUnknown: true}
		if err := unmarshalOpts.Unmarshal(msg.Message.Data, &eventPayload); err != nil {
			return nil, fmt.Errorf("protojson unmarshal: %v", err)
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
		writer.Close()

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

		var uploadResp struct {
			ID         int64  `json:"id"`
			ExternalID string `json:"external_id"`
			ActivityID int64  `json:"activity_id"`
			Status     string `json:"status"`
		}
		json.NewDecoder(httpResp.Body).Decode(&uploadResp)

		fwCtx.Logger.Info("Upload success", "upload_id", uploadResp.ID, "status", uploadResp.Status)
		return map[string]interface{}{
			"status":             "SUCCESS",
			"strava_upload_id":   uploadResp.ID,
			"strava_activity_id": uploadResp.ActivityID,
			"upload_status":      uploadResp.Status,
			"activity_id":        eventPayload.ActivityId,
			"pipeline_id":        eventPayload.PipelineId,
			"fit_file_uri":       eventPayload.FitFileUri,
			"activity_name":      eventPayload.Name,
			"activity_type":      eventPayload.ActivityType,
		}, nil
	}
}
