package storage

import (
	"context"
	"io"

	"strings"

	"cloud.google.com/go/storage"
)

// StorageAdapter provides blob storage operations using Google Cloud Storage
type StorageAdapter struct {
	Client *storage.Client
}

func parseURI(bucketName, objectName string) (string, string) {
	if bucketName == "" && strings.HasPrefix(objectName, "gs://") {
		withoutProtocol := strings.TrimPrefix(objectName, "gs://")
		parts := strings.SplitN(withoutProtocol, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	return bucketName, objectName
}

func (a *StorageAdapter) Write(ctx context.Context, bucketName, objectName string, data []byte) error {
	bucketName, objectName = parseURI(bucketName, objectName)
	wc := a.Client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	if _, err := wc.Write(data); err != nil {
		return err
	}
	return wc.Close()
}

func (a *StorageAdapter) Get(ctx context.Context, bucketName, objectName string) ([]byte, error) {
	bucketName, objectName = parseURI(bucketName, objectName)
	rc, err := a.Client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (a *StorageAdapter) Delete(ctx context.Context, bucketName, objectName string) error {
	bucketName, objectName = parseURI(bucketName, objectName)
	return a.Client.Bucket(bucketName).Object(objectName).Delete(ctx)
}
