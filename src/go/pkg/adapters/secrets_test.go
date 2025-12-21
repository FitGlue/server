package adapters

import (
	"context"
	"os"
	"testing"
)

func TestGetSecret_EnvVar(t *testing.T) {
	// Setup
	os.Setenv("TEST_SECRET", "local_value")
	defer os.Unsetenv("TEST_SECRET")

	adapter := &SecretsAdapter{}

	// Execute
	val, err := adapter.GetSecret(context.Background(), "test-project", "TEST_SECRET")

	// Verify
	if err != nil {
		t.Fatalf("Expected check to succeed, got error: %v", err)
	}
	if val != "local_value" {
		t.Errorf("Expected 'local_value', got '%s'", val)
	}
}

func TestGetSecret_MissingEnv_AttemptsGSM(t *testing.T) {
	// We expect this to fail because we aren't mocking the GSM client here effectively without a lot of boilerplate
	// But we can assert that it *tried* (and failed to connect/auth) or at least didn't return empty string immediately if logic was broken.
	// Actually, without auth, NewClient might fail or AccessSecretVersion will fail.

	// Just ensure it doesn't panic.
	adapter := &SecretsAdapter{}
	_, err := adapter.GetSecret(context.Background(), "test-project", "NON_EXISTENT_SECRET")

	if err == nil {
		t.Error("Expected error from real GSM call (without creds), got nil")
	}
}
