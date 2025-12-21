package fit

import (
	"testing"
	"time"
)

func TestGenerateFitFile(t *testing.T) {
	// Identify inputs
	start := time.Now()
	duration := 3600
	powerParams := []int{100, 110, 120}
	hrParams := []int{140, 145, 150}

	// Exec
	result, err := GenerateFitFile(start, duration, powerParams, hrParams)

	// Verify (Current Stub Behavior)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "MOCK_FIT_FILE"
	if string(result) != expected {
		t.Errorf("Expected %s, got %s", expected, string(result))
	}
}
