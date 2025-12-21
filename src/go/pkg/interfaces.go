package shared

import (
	"context"
)

// --- Persistence Interfaces ---

type Database interface {
	SetExecution(ctx context.Context, id string, data map[string]interface{}) error
	UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error
	GetUser(ctx context.Context, id string) (map[string]interface{}, error)
}

// --- Messaging Interfaces ---

type Publisher interface {
	Publish(ctx context.Context, topic string, data []byte) (string, error)
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
