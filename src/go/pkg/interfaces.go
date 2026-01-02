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
