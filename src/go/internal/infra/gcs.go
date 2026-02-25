package infra

import "context"

// BlobStore defines an interface for interacting with Google Cloud Storage or similar blob storage systems.
type BlobStore interface {
	Upload(ctx context.Context, bucket string, path string, data []byte, contentType string) (uri string, err error)
	Download(ctx context.Context, bucket string, path string) ([]byte, error)
	Delete(ctx context.Context, bucket string, path string) error
	GetSignedURL(ctx context.Context, bucket string, path string, expiryMinutes int) (string, error)
}
