package providers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestFitBitHeartRate_Enrich(t *testing.T) {
	// Setup mock HTTP client
	mockHTTPClient := &http.Client{
		Transport: &mockTransport{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				// Return mock heart rate data
				mockResponse := `{
					"activities-heart-intraday": {
						"dataset": [
							{"time": "10:00:00", "value": 120},
							{"time": "10:00:30", "value": 125},
							{"time": "10:01:00", "value": 130}
						]
					}
				}`
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				}, nil
			},
		},
	}

	// Create provider with mock service
	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	// Create test activity
	startTime := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	activity := &pb.StandardizedActivity{
		StartTime: startTime.Format(time.RFC3339),
		Sessions: []*pb.Session{
			{TotalElapsedTime: 3600}, // 1 hour
		},
	}

	// Create test user with Fitbit integration
	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{
				Enabled:     true,
				AccessToken: "test-token",
			},
		},
	}

	// Execute enrichment
	result, err := provider.EnrichWithClient(context.Background(), activity, user, nil, mockHTTPClient)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Metadata["hr_source"] != "fitbit" {
		t.Errorf("Expected hr_source=fitbit, got %s", result.Metadata["hr_source"])
	}

	if len(result.HeartRateStream) != 3600 {
		t.Errorf("Expected heart rate stream of 3600 seconds, got %d", len(result.HeartRateStream))
	}

	// Verify heart rate stream has data
	if len(result.HeartRateStream) > 0 {
		t.Logf("Heart rate stream populated with %d values", len(result.HeartRateStream))
	}
}

func TestFitBitHeartRate_Enrich_IntegrationDisabled(t *testing.T) {
	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	activity := &pb.StandardizedActivity{
		StartTime: time.Now().Format(time.RFC3339),
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{
				Enabled: false,
			},
		},
	}

	_, err := provider.Enrich(context.Background(), activity, user, nil)
	if err == nil {
		t.Error("Expected error when Fitbit integration is disabled")
	}
}

func TestFitBitHeartRate_Enrich_APIError(t *testing.T) {
	mockHTTPClient := &http.Client{
		Transport: &mockTransport{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 401,
					Body:       io.NopCloser(bytes.NewBufferString(`{"errors":[{"errorType":"invalid_token"}]}`)),
				}, nil
			},
		},
	}

	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	activity := &pb.StandardizedActivity{
		StartTime: time.Now().Format(time.RFC3339),
		Sessions:  []*pb.Session{{TotalElapsedTime: 3600}},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{
				Enabled:     true,
				AccessToken: "invalid-token",
			},
		},
	}

	_, err := provider.EnrichWithClient(context.Background(), activity, user, nil, mockHTTPClient)
	if err == nil {
		t.Error("Expected error when API returns 401")
	}
}

// mockTransport implements http.RoundTripper
type mockTransport struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"activities-heart-intraday":{"dataset":[]}}`)),
	}, nil
}
