package pipeline

import "context"

// BlobStore defines the contract for object storage operations (e.g. GCS).
type BlobStore interface {
	// Get retrieves a blob's contents by its URI (e.g. "gs://bucket/path/to/object.json").
	Get(ctx context.Context, uri string) ([]byte, error)
	// Write uploads a blob's contents to the given URI.
	Write(ctx context.Context, bucket, path string, data []byte) error
}
