package stravauploader

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/framework"
	"github.com/ripixel/fitglue-server/src/go/pkg/testing/mocks"
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
	// Setup Mock HTTP Client
	// Setup Mock HTTP Client
	mockHTTPClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			// 1. Handle POST Upload
			if req.Method == "POST" && req.URL.Path == "/api/v3/uploads" {
				if req.Header.Get("Content-Type") == "" {
					t.Error("Expected Content-Type header")
				}
				// Return response indicating processing (no activity_id)
				return &http.Response{
					StatusCode: 201,
					Body:       io.NopCloser(bytes.NewBufferString(`{"id": 999, "status": "Your activity is still being processed."}`)),
				}, nil
			}

			// 2. Handle GET Poll (Soft Polling)
			if req.Method == "GET" && req.URL.Path == "/api/v3/uploads/999" {
				// Simulate successful completion with activity ID
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`{"id": 999, "activity_id": 888, "status": "Your activity is ready."}`)),
				}, nil
			}

			t.Errorf("Unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		},
	}

	mockDB := &mocks.MockDatabase{
		SetExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			return nil
		},
		UpdateExecutionFunc: func(ctx context.Context, id string, data map[string]interface{}) error {
			// Verify rich output structure
			if outputsJSON, ok := data["outputs"].(string); ok {
				var outputs map[string]interface{}
				if err := json.Unmarshal([]byte(outputsJSON), &outputs); err != nil {
					t.Errorf("Failed to unmarshal outputs: %v", err)
					return nil
				}

				// Verify expected fields
				if status, ok := outputs["status"].(string); !ok || status == "" {
					t.Error("Expected 'status' field in outputs")
				}
				if _, ok := outputs["strava_upload_id"]; !ok {
					t.Error("Expected 'strava_upload_id' field in outputs")
				}
				if _, ok := outputs["activity_id"]; !ok {
					t.Error("Expected 'activity_id' field in outputs")
				}
				if _, ok := outputs["fit_file_uri"]; !ok {
					t.Error("Expected 'fit_file_uri' field in outputs")
				}
			}
			return nil
		},
	}

	mockStore := &mocks.MockBlobStore{
		ReadFunc: func(ctx context.Context, bucket, object string) ([]byte, error) {
			return []byte("MOCK_FIT_DATA"), nil
		},
	}

	// Inject Mocks into Global Service
	svc = &bootstrap.Service{
		DB:      mockDB,
		Store:   mockStore,
		Secrets: &mocks.MockSecretStore{},
		Config: &bootstrap.Config{
			ProjectID:         "test-project",
			GCSArtifactBucket: "test-bucket",
		},
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

	// Execute with injected mock HTTP client
	mockClient := &http.Client{Transport: &mockTransport{mockHTTPClient}}
	handler := uploadHandler(mockClient)
	err := framework.WrapCloudEvent("strava-uploader", svc, handler)(context.Background(), e)
	if err != nil {
		t.Fatalf("UploadToStrava failed: %v", err)
	}
}

// mockTransport wraps MockHTTPClient to implement http.RoundTripper
type mockTransport struct {
	client *MockHTTPClient
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.client.Do(req)
}
