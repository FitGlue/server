// nolint:proto-json
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/fitglue/server/src/go/internal/infra"
	"github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
	"github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	pbsvc "github.com/fitglue/server/src/go/pkg/types/pb/services/pipeline"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MockStore
type MockPipelineStore struct {
	Pipelines     map[string]*pipeline.PipelineConfig
	PendingInputs map[string]*pipeline.PendingInput
	Runs          map[string]*pipeline.PipelineRun
}

func NewMockStore() *MockPipelineStore {
	return &MockPipelineStore{
		Pipelines:     make(map[string]*pipeline.PipelineConfig),
		PendingInputs: make(map[string]*pipeline.PendingInput),
		Runs:          make(map[string]*pipeline.PipelineRun),
	}
}

func (m *MockPipelineStore) key(userID, id string) string {
	return userID + "_" + id
}

func (m *MockPipelineStore) ListPipelines(ctx context.Context, userID string) ([]*pipeline.PipelineConfig, error) {
	var results []*pipeline.PipelineConfig
	for _, p := range m.Pipelines {
		// Just a mock, we don't strictly filter by user here since tests will isolate
		results = append(results, p)
	}
	return results, nil
}

func (m *MockPipelineStore) GetPipeline(ctx context.Context, userID, pipelineID string) (*pipeline.PipelineConfig, error) {
	return m.Pipelines[m.key(userID, pipelineID)], nil
}

func (m *MockPipelineStore) CreatePipeline(ctx context.Context, userID string, cfg *pipeline.PipelineConfig) (*pipeline.PipelineConfig, error) {
	m.Pipelines[m.key(userID, cfg.Id)] = cfg
	return cfg, nil
}

func (m *MockPipelineStore) UpdatePipeline(ctx context.Context, userID string, cfg *pipeline.PipelineConfig) (*pipeline.PipelineConfig, error) {
	m.Pipelines[m.key(userID, cfg.Id)] = cfg
	return cfg, nil
}

func (m *MockPipelineStore) DeletePipeline(ctx context.Context, userID, pipelineID string) error {
	delete(m.Pipelines, m.key(userID, pipelineID))
	return nil
}

func (m *MockPipelineStore) ListPendingInputs(ctx context.Context, userID string) ([]*pipeline.PendingInput, error) {
	var results []*pipeline.PendingInput
	for _, p := range m.PendingInputs {
		results = append(results, p)
	}
	return results, nil
}

func (m *MockPipelineStore) GetPendingInput(ctx context.Context, userID, inputID string) (*pipeline.PendingInput, error) {
	return m.PendingInputs[m.key(userID, inputID)], nil
}

func (m *MockPipelineStore) UpdatePendingInput(ctx context.Context, userID string, input *pipeline.PendingInput) error {
	m.PendingInputs[m.key(userID, input.ActivityId)] = input
	return nil
}

func (m *MockPipelineStore) GetPipelineRun(ctx context.Context, userID, runID string) (*pipeline.PipelineRun, error) {
	return m.Runs[m.key(userID, runID)], nil
}

func (m *MockPipelineStore) ListPipelineRuns(ctx context.Context, userID, pipelineID string, limit int32, pageToken string) ([]*pipeline.PipelineRun, string, error) {
	var results []*pipeline.PipelineRun
	for _, r := range m.Runs {
		if pipelineID == "" || r.PipelineId == pipelineID {
			results = append(results, r)
		}
	}
	return results, "", nil
}

func (m *MockPipelineStore) UpdatePipelineRun(ctx context.Context, userID, runID string, updateData map[string]interface{}) error {
	// For a mock, we can just update the internal map or do nothing.
	return nil
}

// MockPublisher
type MockPublisher struct {
	PublishedEvents []cloudevents.Event
}

func (m *MockPublisher) PublishCloudEvent(ctx context.Context, topic string, ce cloudevents.Event) (string, error) {
	m.PublishedEvents = append(m.PublishedEvents, ce)
	return fmt.Sprintf("msg_%d", len(m.PublishedEvents)), nil
}

// MockBlobStore
type MockBlobStore struct {
	Blobs   map[string][]byte
	GetFn   func(ctx context.Context, uri string) ([]byte, error)
	WriteFn func(ctx context.Context, bucket, path string, data []byte) error
}

func (m *MockBlobStore) Get(ctx context.Context, uri string) ([]byte, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, uri)
	}
	b, ok := m.Blobs[uri]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return b, nil
}

func (m *MockBlobStore) Write(ctx context.Context, bucket, path string, data []byte) error {
	if m.WriteFn != nil {
		return m.WriteFn(ctx, bucket, path, data)
	}
	// Default mock behavior: store in Blobs map
	m.Blobs[fmt.Sprintf("gs://%s/%s", bucket, path)] = data
	return nil
}

// mockLogger
type mockLogger struct{}

func (m mockLogger) Debug(ctx context.Context, msg string, args ...any) {}
func (m mockLogger) Info(ctx context.Context, msg string, args ...any)  {}
func (m mockLogger) Warn(ctx context.Context, msg string, args ...any)  {}
func (m mockLogger) Error(ctx context.Context, msg string, args ...any) {}
func (m mockLogger) With(args ...any) infra.Logger                      { return m }

func TestPipelineCRUD(t *testing.T) {
	store := NewMockStore()
	publisher := &MockPublisher{}
	blobStore := &MockBlobStore{}
	svc := NewService(store, publisher, blobStore, mockLogger{})
	ctx := context.Background()

	// Create
	req := &pbsvc.CreatePipelineRequest{
		UserId: "user1",
		Pipeline: &pipeline.PipelineConfig{
			Name:         "My Pipeline",
			Source:       "SOURCE_STRAVA",
			Destinations: []plugin.DestinationType{1}, // Using int value for enum
		},
	}
	res, err := svc.CreatePipeline(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Id == "" {
		t.Errorf("expected pipeline ID to be generated")
	}

	pipelineID := res.Id

	// Get
	getReq := &pbsvc.GetPipelineRequest{
		UserId:     "user1",
		PipelineId: pipelineID,
	}
	getRes, err := svc.GetPipeline(ctx, getReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if getRes.Name != "My Pipeline" {
		t.Errorf("expected name 'My Pipeline', got %v", getRes.Name)
	}

	// Update
	updReq := &pbsvc.UpdatePipelineRequest{
		UserId:     "user1",
		PipelineId: pipelineID,
		Pipeline: &pipeline.PipelineConfig{
			Id:           pipelineID,
			Name:         "Updated Pipeline",
			Source:       "SOURCE_STRAVA",
			Destinations: []plugin.DestinationType{1, 2}, // Using int values
		},
	}
	updRes, err := svc.UpdatePipeline(ctx, updReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updRes.Name != "Updated Pipeline" {
		t.Errorf("expected name 'Updated Pipeline', got %v", updRes.Name)
	}

	// Delete
	delReq := &pbsvc.DeletePipelineRequest{
		UserId:     "user1",
		PipelineId: pipelineID,
	}
	_, err = svc.DeletePipeline(ctx, delReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Delete
	_, err = svc.GetPipeline(ctx, getReq)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestSubmitInput(t *testing.T) {
	store := NewMockStore()
	publisher := &MockPublisher{}

	payload := map[string]interface{}{"foo": "bar"}
	payloadBytes, _ := json.Marshal(payload)

	blobStore := &MockBlobStore{
		Blobs: map[string][]byte{
			"gs://bucket/path.json": payloadBytes,
		},
	}

	svc := NewService(store, publisher, blobStore, mockLogger{})
	ctx := context.Background()

	// Setup pending input
	store.PendingInputs["user1_input1"] = &pipeline.PendingInput{
		ActivityId:         "input1",
		Status:             pipeline.PendingInput_STATUS_WAITING,
		OriginalPayloadUri: "gs://bucket/path.json",
		LinkedActivityId:   "activity1",
	}

	req := &pbsvc.SubmitInputRequest{
		UserId:         "user1",
		PendingInputId: "input1",
		InputData:      map[string]string{"answer": "42"},
	}

	_, err := svc.SubmitInput(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify DB state
	input, _ := store.GetPendingInput(ctx, "user1", "input1")
	if input.Status != pipeline.PendingInput_STATUS_COMPLETED {
		t.Errorf("expected status COMPLETED, got %v", input.Status)
	}
	if input.InputData["answer"] != "42" {
		t.Errorf("expected input data saved")
	}

	// Verify Pub/Sub
	if len(publisher.PublishedEvents) != 1 {
		t.Fatalf("expected 1 event published, got %d", len(publisher.PublishedEvents))
	}

	eventData := publisher.PublishedEvents[0].Data()
	var publishedPayload map[string]interface{}
	json.Unmarshal(eventData, &publishedPayload)

	if publishedPayload["isResume"] != true {
		t.Errorf("expected isResume=true in published payload")
	}
	if publishedPayload["resumePendingInputId"] != "input1" {
		t.Errorf("expected resumePendingInputId=input1")
	}
	if publishedPayload["activityId"] != "activity1" {
		t.Errorf("expected activityId=activity1")
	}
}
