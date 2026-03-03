package main

import (
	"context"
	"time"

	gcsstorage "github.com/fitglue/server/src/go/pkg/infrastructure/storage"
)

// GCSBlobStore wraps the shared StorageAdapter to implement activity.BlobStore
type GCSBlobStore struct {
	adapter *gcsstorage.StorageAdapter
}

func (s *GCSBlobStore) Get(ctx context.Context, bucket, path string) ([]byte, error) {
	return s.adapter.Get(ctx, bucket, path)
}

func (s *GCSBlobStore) Write(ctx context.Context, bucket, path string, data []byte) error {
	return s.adapter.Write(ctx, bucket, path, data)
}

func (s *GCSBlobStore) Delete(ctx context.Context, bucket, path string) error {
	return s.adapter.Delete(ctx, bucket, path)
}

func (s *GCSBlobStore) SignedURL(ctx context.Context, bucket, path, contentType string, expiry time.Duration) (string, error) {
	return s.adapter.SignedURL(ctx, bucket, path, contentType, expiry)
}
