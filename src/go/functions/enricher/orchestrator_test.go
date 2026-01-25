package enricher

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
)

// MockDatabase implements shared.Database
type MockDatabase struct {
	GetUserFunc          func(ctx context.Context, id string) (*pb.UserRecord, error)
	GetUserPipelinesFunc func(ctx context.Context, userId string) ([]*pb.PipelineConfig, error)
}

func (m *MockDatabase) GetUser(ctx context.Context, id string) (*pb.UserRecord, error) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(ctx, id)
	}
	return nil, nil
}
func (m *MockDatabase) SetExecution(ctx context.Context, record *pb.ExecutionRecord) error {
	return nil
}
func (m *MockDatabase) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}
func (m *MockDatabase) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}
func (m *MockDatabase) CreatePendingInput(ctx context.Context, input *pb.PendingInput) error {
	return nil
}
func (m *MockDatabase) GetPendingInput(ctx context.Context, id string) (*pb.PendingInput, error) {
	return nil, nil
}
func (m *MockDatabase) UpdatePendingInput(ctx context.Context, id string, data map[string]interface{}) error {
	return nil
}
func (m *MockDatabase) ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error) {
	return nil, nil
}
func (m *MockDatabase) GetCounter(ctx context.Context, userId string, id string) (*pb.Counter, error) {
	return nil, nil
}
func (m *MockDatabase) SetCounter(ctx context.Context, userId string, counter *pb.Counter) error {
	return nil
}
func (m *MockDatabase) ListCounters(ctx context.Context, userId string) ([]*pb.Counter, error) {
	return nil, nil
}

func (m *MockDatabase) SetSynchronizedActivity(ctx context.Context, userId string, activity *pb.SynchronizedActivity) error {
	return nil
}
func (m *MockDatabase) IncrementSyncCount(ctx context.Context, userID string) error {
	return nil
}
func (m *MockDatabase) IncrementPreventedSyncCount(ctx context.Context, userID string) error {
	return nil
}
func (m *MockDatabase) ResetSyncCount(ctx context.Context, userID string) error {
	return nil
}
func (m *MockDatabase) ListPendingParkrunActivities(ctx context.Context) ([]*pb.SynchronizedActivity, []string, error) {
	return nil, nil, nil
}
func (m *MockDatabase) UpdateSynchronizedActivity(ctx context.Context, userId string, activityId string, data map[string]interface{}) error {
	return nil
}
func (m *MockDatabase) GetSynchronizedActivity(ctx context.Context, userId string, activityId string) (*pb.SynchronizedActivity, error) {
	return nil, nil
}
func (m *MockDatabase) ListPendingInputsByEnricher(ctx context.Context, enricherId string, status pb.PendingInput_Status) ([]*pb.PendingInput, error) {
	return nil, nil
}
func (m *MockDatabase) ShowcaseActivityExists(ctx context.Context, showcaseId string) (bool, error) {
	return false, nil
}
func (m *MockDatabase) SetShowcasedActivity(ctx context.Context, activity *pb.ShowcasedActivity) error {
	return nil
}
func (m *MockDatabase) GetShowcasedActivity(ctx context.Context, showcaseId string) (*pb.ShowcasedActivity, error) {
	return nil, nil
}
func (m *MockDatabase) GetPersonalRecord(ctx context.Context, userId string, recordType string) (*pb.PersonalRecord, error) {
	return nil, nil
}
func (m *MockDatabase) SetPersonalRecord(ctx context.Context, userId string, record *pb.PersonalRecord) error {
	return nil
}
func (m *MockDatabase) GetUserPipelines(ctx context.Context, userId string) ([]*pb.PipelineConfig, error) {
	if m.GetUserPipelinesFunc != nil {
		return m.GetUserPipelinesFunc(ctx, userId)
	}
	return []*pb.PipelineConfig{}, nil
}
func (m *MockDatabase) SetUploadedActivity(ctx context.Context, userId string, record *pb.UploadedActivityRecord) error {
	return nil
}
func (m *MockDatabase) GetUploadedActivity(ctx context.Context, userId string, source pb.ActivitySource, externalId string) (*pb.UploadedActivityRecord, error) {
	return nil, nil
}

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
	NameFunc         func() string
	ProviderTypeFunc func() pb.EnricherProviderType
	EnrichFunc       func(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error)
}

func (m *MockProvider) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-provider"
}

func (m *MockProvider) ProviderType() pb.EnricherProviderType {
	if m.ProviderTypeFunc != nil {
		return m.ProviderTypeFunc()
	}
	return pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK
}

func (m *MockProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	if m.EnrichFunc != nil {
		return m.EnrichFunc(ctx, logger, activity, user, inputConfig, doNotRetry)
	}
	return &providers.EnrichmentResult{}, nil
}

func TestOrchestrator_Process(t *testing.T) {
	ctx := context.Background()

	t.Run("Executes configured pipeline", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
				return &pb.UserRecord{
					UserId: id,
				}, nil
			},
			GetUserPipelinesFunc: func(ctx context.Context, userId string) ([]*pb.PipelineConfig, error) {
				return []*pb.PipelineConfig{
					{
						Id:           "pipeline-1",
						Source:       "SOURCE_HEVY",
						Destinations: []pb.Destination{pb.Destination_DESTINATION_STRAVA},
						Enrichers: []*pb.EnricherConfig{
							{
								ProviderType: pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK,
								TypedConfig:  map[string]string{"key": "val"},
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

		orchestrator := NewOrchestrator(mockDB, mockStorage, "test-bucket", nil)

		mockProvider := &MockProvider{
			NameFunc: func() string { return "mock-enricher" },
			EnrichFunc: func(ctx context.Context, _ *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
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
			Timestamp: timestamppb.New(time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)),
			StandardizedActivity: &pb.StandardizedActivity{
				Name: "Original Run",
				Sessions: []*pb.Session{
					{
						StartTime:        timestamppb.New(time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)),
						TotalElapsedTime: 60,
					},
				},
			},
		}

		// Update calls
		result, err := orchestrator.Process(ctx, slog.Default(), payload, "test-parent-exec-id", "test-pipeline-id", false) // false = doNotRetry

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
		if len(event.Destinations) != 1 || event.Destinations[0] != pb.Destination_DESTINATION_STRAVA {
			t.Errorf("Expected destination 'strava'")
		}
	})

	t.Run("Returns empty events if no pipelines match", func(t *testing.T) {
		// After removing legacy fallback, the orchestrator should return 0 events
		// when no pipelines are configured for the source.
		// The webhook handler now prevents this situation by checking pipelines first.
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
				return &pb.UserRecord{
					UserId: id,
					Integrations: &pb.UserIntegrations{
						Strava: &pb.StravaIntegration{
							Enabled: true,
						},
					},
					// No pipelines configured
				}, nil
			},
		}

		orchestrator := NewOrchestrator(mockDB, &MockBlobStore{}, "test-bucket", nil)

		payload := &pb.ActivityPayload{
			UserId: "user-123",
			Source: pb.ActivitySource_SOURCE_HEVY,
			StandardizedActivity: &pb.StandardizedActivity{
				Name: "Run",
				Sessions: []*pb.Session{
					{
						StartTime:        timestamppb.New(time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)),
						TotalElapsedTime: 60,
					},
				},
			},
			Timestamp: timestamppb.New(time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)),
		}

		result, err := orchestrator.Process(ctx, slog.Default(), payload, "test-parent-exec-id", "test-pipeline-id", false)

		if err != nil {
			t.Fatalf("Process should not error on no pipelines, got: %v", err)
		}

		if len(result.Events) != 0 {
			t.Fatalf("Expected 0 events when no pipelines match, got %d", len(result.Events))
		}
		if result.Status != pb.ExecutionStatus_STATUS_SKIPPED {
			t.Errorf("Expected STATUS_SKIPPED, got %v", result.Status)
		}
	})

	t.Run("Fails if multiple sessions present", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
				return &pb.UserRecord{UserId: id}, nil
			},
		}
		orchestrator := NewOrchestrator(mockDB, &MockBlobStore{}, "test-bucket", nil)
		payload := &pb.ActivityPayload{
			UserId: "user-1",
			StandardizedActivity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{{}, {}}, // Two sessions
			},
		}
		_, err := orchestrator.Process(ctx, slog.Default(), payload, "exec-1", "pipe-1", false)
		if err == nil || err.Error() != "multiple sessions not supported" {
			t.Errorf("Expected 'multiple sessions not supported' error, got %v", err)
		}
	})

	t.Run("Fails if session duration is zero", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
				return &pb.UserRecord{UserId: id}, nil
			},
		}
		orchestrator := NewOrchestrator(mockDB, &MockBlobStore{}, "test-bucket", nil)
		payload := &pb.ActivityPayload{
			UserId: "user-1",
			StandardizedActivity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{
					{TotalElapsedTime: 0},
				},
			},
		}
		_, err := orchestrator.Process(ctx, slog.Default(), payload, "exec-1", "pipe-1", false)
		if err == nil || err.Error() != "session total elapsed time is 0" {
			t.Errorf("Expected 'session total elapsed time is 0' error, got %v", err)
		}
	})

	t.Run("Aggregates HR stream into Records", func(t *testing.T) {
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
				return &pb.UserRecord{
					UserId: id,
				}, nil
			},
			GetUserPipelinesFunc: func(ctx context.Context, userId string) ([]*pb.PipelineConfig, error) {
				return []*pb.PipelineConfig{
					{
						Id:     "p1",
						Source: "SOURCE_HEVY",
						Enrichers: []*pb.EnricherConfig{
							{ProviderType: pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK},
						},
					},
				}, nil
			},
		}
		mockProvider := &MockProvider{
			NameFunc: func() string { return "mock-enricher" },
			EnrichFunc: func(ctx context.Context, _ *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
				return &providers.EnrichmentResult{
					HeartRateStream: []int{100, 110, 120}, // 3 data points
				}, nil
			},
		}
		orchestrator := NewOrchestrator(mockDB, &MockBlobStore{}, "test-bucket", nil)
		orchestrator.Register(mockProvider)

		payload := &pb.ActivityPayload{ // Set source explicitly
			Source: pb.ActivitySource_SOURCE_HEVY,
			UserId: "u1",
			StandardizedActivity: &pb.StandardizedActivity{
				StartTime: timestamppb.New(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
				Sessions: []*pb.Session{
					{
						StartTime:        timestamppb.New(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
						TotalElapsedTime: 3,
						// No initial records
					},
				},
			},
		}

		result, err := orchestrator.Process(ctx, slog.Default(), payload, "exec-1", "pipe-1", false)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		// Verify enriched activity in OUTPUT events (not mutated payload due to Pointer Isolation)
		if len(result.Events) == 0 {
			t.Fatal("Expected at least one event")
		}
		enrichedActivity := result.Events[0].ActivityData
		if len(enrichedActivity.Sessions) == 0 {
			t.Fatal("Session missing in enriched event")
		}
		session := enrichedActivity.Sessions[0]
		if len(session.Laps) == 0 {
			t.Fatal("Lap missing in enriched event") // Orchestrator adds default lap
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

	t.Run("Multi-pipeline isolation - no cross-pipeline state leakage", func(t *testing.T) {
		// Setup: Two pipelines from same source, each with an enricher that adds descriptions
		mockDB := &MockDatabase{
			GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
				return &pb.UserRecord{
					UserId: id,
				}, nil
			},
			GetUserPipelinesFunc: func(ctx context.Context, userId string) ([]*pb.PipelineConfig, error) {
				return []*pb.PipelineConfig{
					{
						Id:           "pipeline-A",
						Source:       "SOURCE_HEVY",
						Destinations: []pb.Destination{pb.Destination_DESTINATION_STRAVA},
						Enrichers: []*pb.EnricherConfig{
							{
								ProviderType: pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK,
								TypedConfig:  map[string]string{"id": "A"},
							},
						},
					},
					{
						Id:           "pipeline-B",
						Source:       "SOURCE_HEVY",
						Destinations: []pb.Destination{pb.Destination_DESTINATION_INTERVALS},
						Enrichers: []*pb.EnricherConfig{
							{
								ProviderType: pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK,
								TypedConfig:  map[string]string{"id": "B"},
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

		orchestrator := NewOrchestrator(mockDB, mockStorage, "test-bucket", nil)

		// Mock provider returns a description based on its config ID
		mockProvider := &MockProvider{
			NameFunc: func() string { return "mock-enricher" },
			EnrichFunc: func(ctx context.Context, _ *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
				id := inputConfig["id"]
				return &providers.EnrichmentResult{
					Name:        "Activity " + id,
					Description: "Description from pipeline " + id,
					Metadata: map[string]string{
						"pipeline_id": id,
					},
				}, nil
			},
		}
		orchestrator.Register(mockProvider)

		payload := &pb.ActivityPayload{
			UserId:    "user-123",
			Source:    pb.ActivitySource_SOURCE_HEVY,
			Timestamp: timestamppb.New(time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)),
			StandardizedActivity: &pb.StandardizedActivity{
				Name:        "Original Run",
				Description: "", // Start with empty description
				Sessions: []*pb.Session{
					{
						StartTime:        timestamppb.New(time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)),
						TotalElapsedTime: 60,
					},
				},
			},
		}

		result, err := orchestrator.Process(ctx, slog.Default(), payload, "parent-exec", "base-pipeline-exec", false)

		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		// Should produce 2 events (one per pipeline)
		if len(result.Events) != 2 {
			t.Fatalf("Expected 2 events, got %d", len(result.Events))
		}

		// Find events by pipeline ID
		var eventA, eventB *pb.EnrichedActivityEvent
		for _, e := range result.Events {
			if e.PipelineId == "pipeline-A" {
				eventA = e
			} else if e.PipelineId == "pipeline-B" {
				eventB = e
			}
		}

		if eventA == nil || eventB == nil {
			t.Fatal("Expected events for both pipeline-A and pipeline-B")
		}

		// Verify Pipeline A contains ONLY its own description
		if eventA.Description != "Description from pipeline A" {
			t.Errorf("Pipeline A: expected 'Description from pipeline A', got '%s'", eventA.Description)
		}

		// Verify Pipeline B contains ONLY its own description (NOT merged with A)
		if eventB.Description != "Description from pipeline B" {
			t.Errorf("Pipeline B: expected 'Description from pipeline B', got '%s'", eventB.Description)
		}

		// Verify each event has a unique pipelineExecutionId
		if eventA.PipelineExecutionId == nil || eventB.PipelineExecutionId == nil {
			t.Fatal("Expected both events to have pipelineExecutionId")
		}
		if *eventA.PipelineExecutionId == *eventB.PipelineExecutionId {
			t.Errorf("Expected unique pipelineExecutionIds, but both are '%s'", *eventA.PipelineExecutionId)
		}

		// Verify execution IDs contain the pipeline ID
		if !strings.Contains(*eventA.PipelineExecutionId, "pipeline-A") {
			t.Errorf("Pipeline A execution ID should contain 'pipeline-A', got '%s'", *eventA.PipelineExecutionId)
		}
		if !strings.Contains(*eventB.PipelineExecutionId, "pipeline-B") {
			t.Errorf("Pipeline B execution ID should contain 'pipeline-B', got '%s'", *eventB.PipelineExecutionId)
		}
	})
}
