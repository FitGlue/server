package shared

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/fitglue/server/src/go/pkg/types/pb"
)

// --- Persistence Interfaces ---

type Database interface {
	SetExecution(ctx context.Context, record *pb.ExecutionRecord) error
	UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error
	GetUser(ctx context.Context, id string) (*pb.UserRecord, error)
	UpdateUser(ctx context.Context, id string, data map[string]interface{}) error

	// Sync Count (for tier limits)
	IncrementSyncCount(ctx context.Context, userID string) error
	IncrementPreventedSyncCount(ctx context.Context, userID string) error
	ResetSyncCount(ctx context.Context, userID string) error

	// Pending Inputs
	GetPendingInput(ctx context.Context, id string) (*pb.PendingInput, error)
	CreatePendingInput(ctx context.Context, input *pb.PendingInput) error
	UpdatePendingInput(ctx context.Context, id string, data map[string]interface{}) error
	ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error) // Optional: for web list
	ListPendingInputsByEnricher(ctx context.Context, enricherId string, status pb.PendingInput_Status) ([]*pb.PendingInput, error)

	// Counters
	GetCounter(ctx context.Context, userId string, id string) (*pb.Counter, error)
	SetCounter(ctx context.Context, userId string, counter *pb.Counter) error
	ListCounters(ctx context.Context, userId string) ([]*pb.Counter, error)

	// Personal Records
	GetPersonalRecord(ctx context.Context, userId string, recordType string) (*pb.PersonalRecord, error)
	SetPersonalRecord(ctx context.Context, userId string, record *pb.PersonalRecord) error

	// Activities
	SetSynchronizedActivity(ctx context.Context, userId string, activity *pb.SynchronizedActivity) error
	GetSynchronizedActivity(ctx context.Context, userId string, activityId string) (*pb.SynchronizedActivity, error)
	ListPendingParkrunActivities(ctx context.Context) ([]*pb.SynchronizedActivity, []string, error)
	UpdateSynchronizedActivity(ctx context.Context, userId string, activityId string, data map[string]interface{}) error

	// Pipelines (Sub-collection)
	GetUserPipelines(ctx context.Context, userId string) ([]*pb.PipelineConfig, error)

	// Showcased Activities (public shareable snapshots)
	ShowcaseActivityExists(ctx context.Context, showcaseId string) (bool, error)
	SetShowcasedActivity(ctx context.Context, activity *pb.ShowcasedActivity) error
	GetShowcasedActivity(ctx context.Context, showcaseId string) (*pb.ShowcasedActivity, error)

	// Uploaded Activities (for loop prevention - tracks what we've posted to destinations)
	SetUploadedActivity(ctx context.Context, userId string, record *pb.UploadedActivityRecord) error
	GetUploadedActivity(ctx context.Context, userId string, source pb.ActivitySource, externalId string) (*pb.UploadedActivityRecord, error)
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
