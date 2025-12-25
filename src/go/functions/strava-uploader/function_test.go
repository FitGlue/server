package stravauploader

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/mocks"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
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

	// Inject Mocks into Global Service
	svc = &UploaderService{
		Service: &bootstrap.Service{
			DB:      mockDB,
			Store:   mockStore,
			Secrets: &mocks.MockSecretStore{},
			Config: &bootstrap.Config{
				ProjectID:         "test-project",
				GCSArtifactBucket: "test-bucket",
			},
		},
		HTTPClient: mockHTTP,
	}

	// Prepare Input
	eventPayload := pb.EnrichedActivityEvent{
		UserId:       "user_upload",
		FitFileUri:   "gs://fitglue-artifacts/activities/user_upload/123.fit",
		Description:  "Test Activity",
		ActivityType: "WEIGHT_TRAINING",
		Name:         "Test Workout",
		Source:       pb.ActivitySource_SOURCE_HEVY,
	}
	marshalOpts := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}
	payloadBytes, _ := marshalOpts.Marshal(&eventPayload)

	psMsg := types.PubSubMessage{
		Message: struct {
			Data       []byte            `json:"data"`
			Attributes map[string]string `json:"attributes"`
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
	err := UploadToStrava(context.Background(), e)
	if err != nil {
		t.Fatalf("UploadToStrava failed: %v", err)
	}
}
