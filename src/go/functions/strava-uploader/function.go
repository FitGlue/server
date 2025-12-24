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

	"github.com/ripixel/fitglue-server/src/go/pkg/framework"
	"github.com/ripixel/fitglue-server/src/go/pkg/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// HTTPClient interface for mocking
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// UploaderService wraps bootstrap.Service with HTTP client
type UploaderService struct {
	*bootstrap.Service
	HTTPClient HTTPClient
}

var (
	svc     *UploaderService
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("UploadToStrava", UploadToStrava)
}

func initService(ctx context.Context) (*UploaderService, error) {
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
		svc = &UploaderService{
			Service:    baseSvc,
			HTTPClient: &http.Client{Timeout: 30 * time.Second},
		}
	})
	return svc, svcErr
}

// UploadToStrava is the entry point
func UploadToStrava(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("strava-uploader", svc.Service, uploadHandler(svc))(ctx, e)
}

// uploadHandler contains the business logic
func uploadHandler(svc *UploaderService) framework.HandlerFunc {
	return func(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		// Parse Pub/Sub message
		var msg types.PubSubMessage
		if err := e.DataAs(&msg); err != nil {
			return nil, fmt.Errorf("event.DataAs: %v", err)
		}

		var eventPayload pb.EnrichedActivityEvent
		if err := json.Unmarshal(msg.Message.Data, &eventPayload); err != nil {
			return nil, fmt.Errorf("json unmarshal: %v", err)
		}

		fwCtx.Logger.Info("Starting upload")

		// Get Tokens & Rotate if needed
		userData, err := fwCtx.Service.DB.GetUser(ctx, eventPayload.UserId)
		if err != nil {
			fwCtx.Logger.Error("User tokens not found")
			return nil, fmt.Errorf("user tokens not found")
		}

		token, _ := userData["strava_access_token"].(string)
		expiresAt, _ := userData["strava_expires_at"].(time.Time)

		if time.Now().After(expiresAt.Add(-5 * time.Minute)) {
			fwCtx.Logger.Info("Token expiring soon - rotation required")
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

		// Upload to Strava
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "activity.fit")
		part.Write(fileData)
		writer.WriteField("data_type", "fit")
		writer.Close()

		req, _ := http.NewRequest("POST", "https://www.strava.com/api/v3/uploads", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		httpResp, err := svc.HTTPClient.Do(req)
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
			ID int64 `json:"id"`
		}
		json.NewDecoder(httpResp.Body).Decode(&uploadResp)

		fwCtx.Logger.Info("Upload success")
		return map[string]interface{}{
			"strava_activity_id": uploadResp.ID,
		}, nil
	}
}
