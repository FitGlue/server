package adapters

import (
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"cloud.google.com/go/storage"
)

// --- Firestore Adapter ---
type FirestoreAdapter struct {
	Client *firestore.Client
}

func (a *FirestoreAdapter) SetExecution(ctx context.Context, id string, data map[string]interface{}) error {
	var ref *firestore.DocumentRef
	if id == "" {
		ref = a.Client.Collection("executions").NewDoc()
	} else {
		ref = a.Client.Collection("executions").Doc(id)
	}
	_, err := ref.Set(ctx, data, firestore.MergeAll)
	return err
}

func (a *FirestoreAdapter) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	_, err := a.Client.Collection("executions").Doc(id).Set(ctx, data, firestore.MergeAll)
	return err
}

func (a *FirestoreAdapter) GetUser(ctx context.Context, id string) (map[string]interface{}, error) {
	snap, err := a.Client.Collection("users").Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	return snap.Data(), nil
}

// --- PubSub Adapter ---
type PubSubAdapter struct {
	Client *pubsub.Client
}

func (a *PubSubAdapter) Publish(ctx context.Context, topicID string, data []byte) (string, error) {
	topic := a.Client.Topic(topicID)
	res := topic.Publish(ctx, &pubsub.Message{Data: data})
	return res.Get(ctx)
}

// --- LogPublisher Adapter ---
type LogPublisher struct{}

func (p *LogPublisher) Publish(ctx context.Context, topicID string, data []byte) (string, error) {
	log.Printf("[LogPublisher] MOCK PUBLISH to %s: %s", topicID, string(data))
	return "mock-msg-id", nil
}

// --- Storage Adapter ---
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

// --- Secrets Adapter ---
type SecretsAdapter struct{}

func (a *SecretsAdapter) GetSecret(ctx context.Context, projectID, secretName string) (string, error) {
	// 1. Local Fallback
	if val := os.Getenv(secretName); val != "" {
		log.Printf("[SecretManager] Using local env var for: %s", secretName)
		return val, nil
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create secretmanager client: %v", err)
	}
	defer client.Close()

	// Build the request.
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, secretName)
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	// Call the API.
	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret version: %v", err)
	}

	// Verify the data checksum.
	crc32c := crc32.MakeTable(crc32.Castagnoli)
	checksum := int64(crc32.Checksum(result.Payload.Data, crc32c))
	if result.Payload.DataCrc32C != nil && *result.Payload.DataCrc32C != checksum {
		return "", fmt.Errorf("datas corruption detected")
	}

	return string(result.Payload.Data), nil
}
