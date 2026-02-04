package shared

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/fitglue/server/src/go/pkg/types/pb"
)

// --- Persistence Interfaces ---

type Database interface {
	SetExecution(ctx context.Context, record *pb.ExecutionRecord) error
	UpdateExecution(ctx context.Context, userId string, id string, data map[string]interface{}) error
	GetUser(ctx context.Context, id string) (*pb.UserRecord, error)
	UpdateUser(ctx context.Context, id string, data map[string]interface{}) error

	// Sync Count (for tier limits)
	IncrementSyncCount(ctx context.Context, userID string) error
	IncrementPreventedSyncCount(ctx context.Context, userID string) error
	ResetSyncCount(ctx context.Context, userID string) error

	// Pending Inputs
	GetPendingInput(ctx context.Context, userId string, id string) (*pb.PendingInput, error)
	CreatePendingInput(ctx context.Context, userId string, input *pb.PendingInput) error
	UpdatePendingInput(ctx context.Context, userId string, id string, data map[string]interface{}) error
	DeletePendingInput(ctx context.Context, userId string, id string) error
	ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error)
	ListPendingInputsByEnricher(ctx context.Context, enricherId string, status pb.PendingInput_Status) ([]*pb.PendingInput, error)

	// Counters
	GetCounter(ctx context.Context, userId string, id string) (*pb.Counter, error)
	SetCounter(ctx context.Context, userId string, counter *pb.Counter) error
	ListCounters(ctx context.Context, userId string) ([]*pb.Counter, error)
	DeleteCounter(ctx context.Context, userId string, id string) error

	// Personal Records
	GetPersonalRecord(ctx context.Context, userId string, recordType string) (*pb.PersonalRecord, error)
	SetPersonalRecord(ctx context.Context, userId string, record *pb.PersonalRecord) error
	ListPersonalRecords(ctx context.Context, userId string) ([]*pb.PersonalRecord, error)
	DeletePersonalRecord(ctx context.Context, userId string, recordType string) error

	// Pipelines (Sub-collection)
	GetUserPipelines(ctx context.Context, userId string) ([]*pb.PipelineConfig, error)

	// Showcased Activities (public shareable snapshots)
	ShowcaseActivityExists(ctx context.Context, showcaseId string) (bool, error)
	SetShowcasedActivity(ctx context.Context, activity *pb.ShowcasedActivity) error
	GetShowcasedActivity(ctx context.Context, showcaseId string) (*pb.ShowcasedActivity, error)

	// Uploaded Activities (for loop prevention - tracks what we've posted to destinations)
	SetUploadedActivity(ctx context.Context, userId string, record *pb.UploadedActivityRecord) error
	GetUploadedActivity(ctx context.Context, userId string, destination pb.Destination, destinationId string) (*pb.UploadedActivityRecord, error)

	// Pipeline Runs (lifecycle tracking)
	CreatePipelineRun(ctx context.Context, userId string, run *pb.PipelineRun) error
	GetPipelineRun(ctx context.Context, userId string, id string) (*pb.PipelineRun, error)
	GetPipelineRunByActivityId(ctx context.Context, userId string, activityId string) (*pb.PipelineRun, error)
	UpdatePipelineRun(ctx context.Context, userId string, id string, data map[string]interface{}) error

	// Destination Outcomes (subcollection of Pipeline Runs - avoids race conditions)
	SetDestinationOutcome(ctx context.Context, userId string, pipelineRunId string, outcome *pb.DestinationOutcome) error
	GetDestinationOutcomes(ctx context.Context, userId string, pipelineRunId string) ([]*pb.DestinationOutcome, error)
}

// --- Messaging Interfaces ---

type Publisher interface {
	PublishCloudEvent(ctx context.Context, topic string, e event.Event) (string, error)
}

// --- Storage Interfaces ---

type BlobStore interface {
	Write(ctx context.Context, bucket, object string, data []byte) error
	Read(ctx context.Context, bucket, object string) ([]byte, error)
}

// --- Notification Interfaces ---

type NotificationService interface {
	SendPushNotification(ctx context.Context, userID string, title, body string, tokens []string, data map[string]string) error
}
