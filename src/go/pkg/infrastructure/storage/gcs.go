package storage

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
)

// StorageAdapter provides blob storage operations using Google Cloud Storage
type StorageAdapter struct {
	Client *storage.Client
}

func (a *StorageAdapter) Write(ctx context.Context, bucketName, objectName string, data []byte) error {
	wc := a.Client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	if _, err := wc.Write(data); err != nil {
		return err
	}
	return wc.Close()
}

func (a *StorageAdapter) Read(ctx context.Context, bucketName, objectName string) ([]byte, error) {
	rc, err := a.Client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
