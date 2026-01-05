package shared

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// --- Persistence Interfaces ---

type Database interface {
	SetExecution(ctx context.Context, record *pb.ExecutionRecord) error
	UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error
	GetUser(ctx context.Context, id string) (*pb.UserRecord, error)
	UpdateUser(ctx context.Context, id string, data map[string]interface{}) error

	// Pending Inputs
	GetPendingInput(ctx context.Context, id string) (*pb.PendingInput, error)
	CreatePendingInput(ctx context.Context, input *pb.PendingInput) error
	UpdatePendingInput(ctx context.Context, id string, data map[string]interface{}) error
	ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error) // Optional: for web list

	// Counters
	GetCounter(ctx context.Context, userId string, id string) (*pb.Counter, error)
	SetCounter(ctx context.Context, userId string, counter *pb.Counter) error

	// Activities
	SetSynchronizedActivity(ctx context.Context, userId string, activity *pb.SynchronizedActivity) error
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

// --- Secrets Interface ---

type SecretStore interface {
	GetSecret(ctx context.Context, projectID, name string) (string, error)
}

// --- Notification Interfaces ---

type NotificationService interface {
	SendPushNotification(ctx context.Context, userID string, title, body string, tokens []string, data map[string]string) error
}
