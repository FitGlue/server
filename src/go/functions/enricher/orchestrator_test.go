package enricher

import (
	"context"
	"testing"

	providers "github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// MockDatabase implements shared.Database
type MockDatabase struct {
	GetUserFunc func(ctx context.Context, id string) (map[string]interface{}, error)
}

func (m *MockDatabase) GetUser(ctx context.Context, id string) (map[string]interface{}, error) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(ctx, id)
	}
	return nil, nil
}
func (m *MockDatabase) SetExecution(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}
func (m *MockDatabase) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}
func (m *MockDatabase) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}

// MockBlobStore implements shared.BlobStore
type MockBlobStore struct {
	WriteFunc func(ctx context.Context, bucket, object string, data []byte) error
}

func (m *MockBlobStore) Write(ctx context.Context, bucket, object string, data []byte) error {
	if m.WriteFunc != nil {
		return m.WriteFunc(ctx, bucket, object, data)
	}
	return nil
}
func (m *MockBlobStore) Read(ctx context.Context, bucket, object string) ([]byte, error) {
	return nil, nil
}

// MockProvider implements providers.Provider
type MockProvider struct {
	NameFunc   func() string
	EnrichFunc func(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string) (*providers.EnrichmentResult, error)
}

func (m *MockProvider) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-provider"
}

func (m *MockProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string) (*providers.EnrichmentResult, error) {
	if m.EnrichFunc != nil {
		return m.EnrichFunc(ctx, activity, user, inputConfig)
	}
	return &providers.EnrichmentResult{}, nil
}

func TestOrchestrator_Process(t *testing.T) {
	ctx := context.Background()

	t.Run("Executes configured pipeline", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"user_id": id,
					"pipelines": []interface{}{
						map[string]interface{}{
							"id":     "pipeline-1",
							"source": "SOURCE_HEVY",
							"destinations": []interface{}{
								"strava",
							},
							"enrichers": []interface{}{
								map[string]interface{}{
									"providerType": 99, // ENRICHER_PROVIDER_MOCK
									"inputs": map[string]interface{}{
										"key": "val",
									},
								},
							},
						},
					},
				}, nil
			},
		}

		mockStorage := &MockBlobStore{
			WriteFunc: func(ctx context.Context, bucket, object string, data []byte) error {
				return nil
			},
		}

		orchestrator := NewOrchestrator(mockDB, mockStorage, "test-bucket")

		mockProvider := &MockProvider{
			NameFunc: func() string { return "mock-enricher" },
			EnrichFunc: func(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string) (*providers.EnrichmentResult, error) {
				return &providers.EnrichmentResult{
					Name:        "Enriched Activity",
					Description: "Added by mock",
					Metadata: map[string]string{
						"processed_by": "mock",
					},
				}, nil
			},
		}
		orchestrator.Register(mockProvider)

		payload := &pb.ActivityPayload{
			UserId:    "user-123",
			Source:    pb.ActivitySource_SOURCE_HEVY,
			Timestamp: "2023-01-01T10:00:00Z",
			StandardizedActivity: &pb.StandardizedActivity{
				Name: "Original Run",
				Sessions: []*pb.Session{
					{
						StartTime:        "2023-01-01T10:00:00Z",
						TotalElapsedTime: 60,
					},
				},
			},
		}

		result, err := orchestrator.Process(ctx, payload, "test-parent-exec-id")
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		if len(result.Events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(result.Events))
		}

		event := result.Events[0]
		if event.Name != "Enriched Activity" {
			t.Errorf("Expected name 'Enriched Activity', got '%s'", event.Name)
		}
		if event.Description != "Added by mock" {
			t.Errorf("Expected description 'Added by mock', got '%s'", event.Description)
		}
		if event.EnrichmentMetadata["processed_by"] != "mock" {
			t.Errorf("Expected metadata 'processed_by'='mock'")
		}
		if len(event.Destinations) != 1 || event.Destinations[0] != "strava" {
			t.Errorf("Expected destination 'strava'")
		}
	})

	t.Run("Falls back to default if no pipelines match", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"user_id": id,
					"integrations": map[string]interface{}{
						"strava": map[string]interface{}{
							"enabled": true,
						},
					},
				}, nil
			},
		}

		orchestrator := NewOrchestrator(mockDB, &MockBlobStore{}, "test-bucket")

		payload := &pb.ActivityPayload{
			UserId: "user-123",
			Source: pb.ActivitySource_SOURCE_HEVY,
			StandardizedActivity: &pb.StandardizedActivity{
				Name: "Run",
				Sessions: []*pb.Session{
					{
						StartTime:        "2023-01-01T10:00:00Z",
						TotalElapsedTime: 60,
					},
				},
			},
			Timestamp: "2023-01-01T10:00:00Z",
		}

		result, err := orchestrator.Process(ctx, payload, "test-parent-exec-id")
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		if len(result.Events) != 1 {
			t.Fatalf("Expected 1 default event, got %d", len(result.Events))
		}
		if result.Events[0].PipelineId != "default-legacy" {
			t.Errorf("Expected default-legacy pipeline, got %s", result.Events[0].PipelineId)
		}
	})

	t.Run("Fails if multiple sessions present", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (map[string]interface{}, error) {
				return map[string]interface{}{"user_id": id}, nil
			},
		}
		orchestrator := NewOrchestrator(mockDB, &MockBlobStore{}, "test-bucket")
		payload := &pb.ActivityPayload{
			UserId: "user-1",
			StandardizedActivity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{{}, {}}, // Two sessions
			},
		}
		_, err := orchestrator.Process(ctx, payload, "exec-1")
		if err == nil || err.Error() != "multiple sessions not supported" {
			t.Errorf("Expected 'multiple sessions not supported' error, got %v", err)
		}
	})

	t.Run("Fails if session duration is zero", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (map[string]interface{}, error) {
				return map[string]interface{}{"user_id": id}, nil
			},
		}
		orchestrator := NewOrchestrator(mockDB, &MockBlobStore{}, "test-bucket")
		payload := &pb.ActivityPayload{
			UserId: "user-1",
			StandardizedActivity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{
					{TotalElapsedTime: 0},
				},
			},
		}
		_, err := orchestrator.Process(ctx, payload, "exec-1")
		if err == nil || err.Error() != "session total elapsed time is 0" {
			t.Errorf("Expected 'session total elapsed time is 0' error, got %v", err)
		}
	})

	t.Run("Aggregates HR stream into Records", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"user_id": id,
					"pipelines": []interface{}{
						map[string]interface{}{
							"id":        "p1",
							"source":    "SOURCE_HEVY",
							"enrichers": []interface{}{map[string]interface{}{"providerType": 99}},
						},
					},
				}, nil
			},
		}
		mockProvider := &MockProvider{
			NameFunc: func() string { return "mock-enricher" },
			EnrichFunc: func(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string) (*providers.EnrichmentResult, error) {
				return &providers.EnrichmentResult{
					HeartRateStream: []int{100, 110, 120}, // 3 data points
				}, nil
			},
		}
		orchestrator := NewOrchestrator(mockDB, &MockBlobStore{}, "test-bucket")
		orchestrator.Register(mockProvider)

		payload := &pb.ActivityPayload{ // Set source explicitly
			Source: pb.ActivitySource_SOURCE_HEVY,
			UserId: "u1",
			StandardizedActivity: &pb.StandardizedActivity{
				StartTime: "2024-01-01T10:00:00Z",
				Sessions: []*pb.Session{
					{
						StartTime:        "2024-01-01T10:00:00Z",
						TotalElapsedTime: 3,
						// No initial records
					},
				},
			},
		}

		_, err := orchestrator.Process(ctx, payload, "exec-1")
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		// Verify records were populated
		if len(payload.StandardizedActivity.Sessions) == 0 {
			t.Fatal("Session missing")
		}
		session := payload.StandardizedActivity.Sessions[0]
		if len(session.Laps) == 0 {
			t.Fatal("Lap missing") // Orchestrator adds default lap
		}
		records := session.Laps[0].Records
		if len(records) != 3 {
			t.Errorf("Expected 3 records, got %d", len(records))
		} else {
			if records[0].HeartRate != 100 {
				t.Errorf("Expected HR 100, got %d", records[0].HeartRate)
			}
			if records[1].HeartRate != 110 {
				t.Errorf("Expected HR 110, got %d", records[1].HeartRate)
			}
			if records[2].HeartRate != 120 {
				t.Errorf("Expected HR 120, got %d", records[2].HeartRate)
			}
		}
	})
}
