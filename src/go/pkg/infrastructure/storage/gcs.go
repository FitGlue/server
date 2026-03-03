package storage

import (
	"context"
	"io"
	"strings"
	"time"

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

// SignedURL generates a V4 signed URL for uploading or downloading an object.
// On Cloud Run with a service account, credentials are auto-detected.
func (a *StorageAdapter) SignedURL(ctx context.Context, bucketName, objectName, contentType string, expiry time.Duration) (string, error) {
	bucketName, objectName = parseURI(bucketName, objectName)

	method := "GET"
	var headers []string
	if contentType != "" {
		method = "PUT"
		headers = []string{"Content-Type:" + contentType}
	}

	url, err := a.Client.Bucket(bucketName).SignedURL(objectName, &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  method,
		Expires: time.Now().Add(expiry),
		Headers: headers,
	})
	if err != nil {
		return "", err
	}
	return url, nil
}
