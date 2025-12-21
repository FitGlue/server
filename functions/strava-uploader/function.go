package function

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"fitglue-strava-uploader/pkg/shared"
	"fitglue-strava-uploader/pkg/shared/adapters"
	"fitglue-strava-uploader/pkg/shared/types"
	pb "fitglue-strava-uploader/pkg/shared/types/pb/proto"
)

var svc *Service

func init() {
	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = shared.ProjectID
	}

	fsClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("Warning: Firestore init failed: %v", err)
	}
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Warning: Storage init failed: %v", err)
	}

	svc = &Service{
		DB:         &adapters.FirestoreAdapter{Client: fsClient},
		Store:      &adapters.StorageAdapter{Client: gcsClient},
		Secrets:    &adapters.SecretsAdapter{},
		HTTPClient: http.DefaultClient,
	}

	functions.CloudEvent("UploadToStrava", svc.UploadToStrava)
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Service struct {
	DB         shared.Database
	Store      shared.BlobStore
	Secrets    shared.SecretStore
	HTTPClient HTTPClient
}

type UserTokens struct {
	AccessToken  string    `firestore:"strava_access_token"`
	RefreshToken string    `firestore:"strava_refresh_token"`
	ExpiresAt    time.Time `firestore:"strava_expires_at"`
}

func (s *Service) UploadToStrava(ctx context.Context, e event.Event) error {
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
	execData := map[string]interface{}{
		"service":   "strava-uploader",
		"status":    "STARTED",
		"inputs":    eventPayload.UserId,
		"startTime": time.Now(),
	}
	if err := s.DB.SetExecution(ctx, execID, execData); err != nil {
		log.Printf("Failed to log start: %v", err)
	}

	// 1. Get Tokens & Rotate if needed
	userData, err := s.DB.GetUser(ctx, eventPayload.UserId)
	if err != nil {
		s.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": "User tokens not found"})
		return err
	}

	// Extract Tokens manually from map
	token, _ := userData["strava_access_token"].(string)
	// refreshToken, _ := userData["strava_refresh_token"].(string)
	expiresAt, _ := userData["strava_expires_at"].(time.Time)

	if time.Now().After(expiresAt.Add(-5 * time.Minute)) {
		// TODO: Implement Token Rotation using RefreshToken
		log.Println("Token expiring soon - rotation required")
	}

	// 2. Download FIT from GCS
	bucketName := os.Getenv("GCS_ARTIFACT_BUCKET")
	if bucketName == "" {
		bucketName = "fitglue-artifacts"
	}
	// Parse GCS URI: assume "gs://bucket/path" format
	objectName := strings.TrimPrefix(eventPayload.GcsUri, "gs://"+bucketName+"/")

	fileData, err := s.Store.Read(ctx, bucketName, objectName)
	if err != nil {
		s.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": "GCS Read Error: " + err.Error()})
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

	httpResp, err := s.HTTPClient.Do(req)
	if err != nil {
		s.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": "Strava API Error: " + err.Error()})
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		errStr := fmt.Sprintf("Strava Error %d: %s", httpResp.StatusCode, string(bodyBytes))
		s.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": errStr})
		return fmt.Errorf("%s", errStr)
	}

	// 4. Handle Response & Poll (Mocked for now)

	// 5. Update Description

	s.DB.UpdateExecution(ctx, execID, map[string]interface{}{
		"status":  "SUCCESS",
		"endTime": time.Now(),
	})

	return nil
}
