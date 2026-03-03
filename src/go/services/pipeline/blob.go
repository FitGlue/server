package main

import (
	"context"
	"fmt"
	"io"
	"strings"

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
	if !strings.HasPrefix(uri, "gs://") {
		return nil, fmt.Errorf("invalid GCS URI: %s", uri)
	}
	withoutProtocol := strings.TrimPrefix(uri, "gs://")
	parts := strings.SplitN(withoutProtocol, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid GCS URI format (expected gs://bucket/path): %s", uri)
	}
	bucket, object := parts[0], parts[1]

	rc, err := s.client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("open GCS object %s: %w", uri, err)
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// Write uploads a blob's contents
func (s *GCSBlobStore) Write(ctx context.Context, bucket, path string, data []byte) error {
	obj := s.client.Bucket(bucket).Object(path)
	writer := obj.NewWriter(ctx)
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return fmt.Errorf("write to GCS: %w", err)
	}
	return writer.Close()
}
