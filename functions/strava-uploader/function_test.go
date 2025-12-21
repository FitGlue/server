package function

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"

	"fitglue-strava-uploader/pkg/shared/mocks"
	"fitglue-strava-uploader/pkg/shared/types"
	pb "fitglue-strava-uploader/pkg/shared/types/pb/proto"
)

// MockHTTPClient
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return &http.Response{
		StatusCode: 201,
		Body:       io.NopCloser(bytes.NewBufferString(`{"id": 12345}`)),
	}, nil
}

func TestUploadToStrava(t *testing.T) {
	// Setup Mocks
	mockDB := &mocks.MockDatabase{
		GetUserFunc: func(ctx context.Context, id string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"strava_access_token": "token-123",
				"strava_expires_at":   time.Now().Add(1 * time.Hour),
			}, nil
		},
		SetExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			return nil
		},
	}

	mockStore := &mocks.MockBlobStore{
		ReadFunc: func(ctx context.Context, bucket, object string) ([]byte, error) {
			// Mock FIT file
			return []byte("MOCK_FIT_DATA"), nil
		},
	}

	mockHTTP := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			// Verify Headers
			if req.Header.Get("Authorization") != "Bearer token-123" {
				t.Errorf("Wrong Token")
			}
			return &http.Response{
				StatusCode: 201,
				Body:       io.NopCloser(bytes.NewBufferString(`{"id": 999}`)),
			}, nil
		},
	}

	svc := &Service{
		DB:         mockDB,
		Store:      mockStore,
		Secrets:    &mocks.MockSecretStore{},
		HTTPClient: mockHTTP,
	}

	// Prepare Input
	eventPayload := pb.EnrichedActivityEvent{
		UserId: "user_upload",
		GcsUri: "gs://fitglue-artifacts/activities/user_upload/123.fit",
	}
	payloadBytes, _ := json.Marshal(&eventPayload)

	psMsg := types.PubSubMessage{
		Message: struct {
			Data []byte `json:"data"`
		}{
			Data: payloadBytes,
		},
	}

	e := event.New()
	e.SetID("evt-upload")
	e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
	e.SetSource("//pubsub")
	e.SetData(event.ApplicationJSON, psMsg)

	// Execute
	err := svc.UploadToStrava(context.Background(), e)
	if err != nil {
		t.Fatalf("UploadToStrava failed: %v", err)
	}
}
