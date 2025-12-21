package function

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
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/ripixel/fitglue-server/src/go/pkg/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb/proto"
)

// HTTPClient interface for mocking
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Extending Bootstrap Service with HTTP Client
type UploaderService struct {
	*bootstrap.Service
	HTTPClient HTTPClient
}

var svc *UploaderService

func init() {
	var err error
	ctx := context.Background()

	baseSvc, err := bootstrap.NewService(ctx)
	if err != nil {
		slog.Error("Failed to initialize base service", "error", err)
	}

	svc = &UploaderService{
		Service:    baseSvc,
		HTTPClient: http.DefaultClient,
	}

	functions.CloudEvent("UploadToStrava", UploadToStrava)
}

func UploadToStrava(ctx context.Context, e event.Event) error {
	if svc == nil || svc.Service == nil {
		return fmt.Errorf("service not initialized")
	}

	var msg types.PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var eventPayload pb.EnrichedActivityEvent
	if err := json.Unmarshal(msg.Message.Data, &eventPayload); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// Logging setup
	execID := fmt.Sprintf("uploader-%s-%d", eventPayload.UserId, time.Now().UnixNano())
	logger := slog.With("execution_id", execID, "user_id", eventPayload.UserId, "service", "strava-uploader")

	logger.Info("Starting upload")

	execData := map[string]interface{}{
		"service":   "strava-uploader",
		"status":    "STARTED",
		"inputs":    eventPayload.UserId,
		"startTime": time.Now(),
	}
	if err := svc.DB.SetExecution(ctx, execID, execData); err != nil {
		logger.Error("Failed to log start", "error", err)
	}

	// 1. Get Tokens & Rotate if needed
	userData, err := svc.DB.GetUser(ctx, eventPayload.UserId)
	if err != nil {
		svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": "User tokens not found"})
		return err
	}

	token, _ := userData["strava_access_token"].(string)
	expiresAt, _ := userData["strava_expires_at"].(time.Time)

	if time.Now().After(expiresAt.Add(-5 * time.Minute)) {
		logger.Info("Token expiring soon - rotation required")
	}

	// 2. Download FIT from GCS
	bucketName := svc.Config.GCSArtifactBucket
	if bucketName == "" {
		bucketName = "fitglue-artifacts"
	}
	// Parse GCS URI: assume "gs://bucket/path"
	objectName := strings.TrimPrefix(eventPayload.GcsUri, "gs://"+bucketName+"/")

	fileData, err := svc.Store.Read(ctx, bucketName, objectName)
	if err != nil {
		svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": "GCS Read Error: " + err.Error()})
		return err
	}

	// 3. Upload to Strava
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
		svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": "Strava API Error: " + err.Error()})
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		errStr := fmt.Sprintf("Strava Error %d: %s", httpResp.StatusCode, string(bodyBytes))
		svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": errStr})
		return fmt.Errorf("%s", errStr)
	}

	svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{
		"status":  "SUCCESS",
		"endTime": time.Now(),
	})

	logger.Info("Upload success")
	return nil
}
