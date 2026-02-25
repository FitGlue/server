package activity

import "context"

// BlobStore defines the contract for object storage operations (e.g. GCS).
type BlobStore interface {
	// Get retrieves a blob's contents
	Get(ctx context.Context, bucket, path string) ([]byte, error)
	// Write uploads a blob's contents
	Write(ctx context.Context, bucket, path string, data []byte) error
	// Delete removes a blob
	Delete(ctx context.Context, bucket, path string) error
}
