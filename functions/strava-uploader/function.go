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
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
)

func init() {
	functions.CloudEvent("UploadToStrava", UploadToStrava)
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type EnrichedActivityEvent struct {
	UserId      string `json:"userId"`
	GcsURI      string `json:"gcsUri"`
	Description string `json:"description"`
}

type UserTokens struct {
	AccessToken  string    `firestore:"strava_access_token"`
	RefreshToken string    `firestore:"strava_refresh_token"`
	ExpiresAt    time.Time `firestore:"strava_expires_at"`
}

func UploadToStrava(ctx context.Context, e event.Event) error {
	var msg PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var eventPayload EnrichedActivityEvent
	if err := json.Unmarshal(msg.Data, &eventPayload); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	client, _ := firestore.NewClient(ctx, "fitglue-project")
	defer client.Close()
	execRef := client.Collection("executions").NewDoc()
	execRef.Set(ctx, map[string]interface{}{
		"service":   "strava-uploader",
		"status":    "STARTED",
		"inputs":    eventPayload.UserId,
		"startTime": time.Now(),
	})

	// 1. Get Tokens & Rotate if needed
	userRef := client.Collection("users").Doc(eventPayload.UserId)
	docSnap, err := userRef.Get(ctx)
	if err != nil {
		return err
	}

	var tokens UserTokens
	docSnap.DataTo(&tokens)

	token := tokens.AccessToken
	if time.Now().After(tokens.ExpiresAt.Add(-5 * time.Minute)) {
		// Mock Rotation Logic
		// token = RefreshStravaToken(tokens.RefreshToken)
		// userRef.Update(ctx, ...)
		log.Println("Token refresh mock executed")
	}

	// 2. Download FIT from GCS
	// Parse GCS URI gs://bucket/path
	bucketName := "fitglue-artifacts" // simplified
	objectName := eventPayload.GcsURI[len("gs://fitglue-artifacts/"):]

	gcsClient, _ := storage.NewClient(ctx)
	defer gcsClient.Close()
	rc, err := gcsClient.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return err
	}
	defer rc.Close()

	fileData, _ := io.ReadAll(rc)

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

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	// 4. Handle Response & Poll (Mocked)
	// var uploadResp UploadResponse
	// json.NewDecoder(httpResp.Body).Decode(&uploadResp)
	// activityId := PollUploadStatus(uploadResp.Id, token)

	// 5. Update Description
	// reqUpdate, _ := http.NewRequest("PUT", fmt.Sprintf(".../activities/%d", activityId), ...)
	// ...

	execRef.Set(ctx, map[string]interface{}{
		"status":  "SUCCESS",
		"endTime": time.Now(),
	}, firestore.MergeAll)

	return nil
}
