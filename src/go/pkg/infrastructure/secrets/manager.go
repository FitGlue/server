package secrets

import (
	"context"
	"fmt"
	"hash/crc32"
	"log"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// SecretsAdapter provides secret management using Google Secret Manager
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
