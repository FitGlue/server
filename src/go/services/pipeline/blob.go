package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
)

// GCSBlobStore implements pipeline.BlobStore using the official GCS client
type GCSBlobStore struct {
	client *storage.Client
}

func NewGCSBlobStore(client *storage.Client) *GCSBlobStore {
	return &GCSBlobStore{client: client}
}

// Get retrieves a blob's contents by its URI (e.g., "gs://bucket/path")
func (s *GCSBlobStore) Get(ctx context.Context, uri string) ([]byte, error) {
	// Not needed for pipeline execution yet, just a stub to satisfy the interface if no reads occur
	// If routing needs it, we will parse the gs:// URI later.
	return nil, fmt.Errorf("GCSBlobStore.Get not implemented for URI: %s", uri)
}

// Write uploads a blob's contents
func (s *GCSBlobStore) Write(ctx context.Context, bucket, path string, data []byte) error {
	obj := s.client.Bucket(bucket).Object(path)
	writer := obj.NewWriter(ctx)
	defer writer.Close()

	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("write to GCS: %w", err)
	}
	return nil
}
