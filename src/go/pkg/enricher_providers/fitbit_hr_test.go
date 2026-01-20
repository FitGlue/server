package enricher_providers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		StartTime: timestamppb.New(startTime),
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
	result, err := provider.EnrichWithClient(context.Background(), activity, user, nil, mockHTTPClient, false)

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
	if result.Metadata["status_detail"] != "Success" {
		t.Errorf("Expected status_detail=Success, got %s", result.Metadata["status_detail"])
	}
	if result.Metadata["query_start"] != "10:00" {
		t.Errorf("Expected query_start=10:00, got %s", result.Metadata["query_start"])
	}

	if len(result.HeartRateStream) != 3600 {
		t.Errorf("Expected heart rate stream of 3600 seconds, got %d", len(result.HeartRateStream))
	}

	// Verify heart rate stream has data
	foundData := false
	for _, val := range result.HeartRateStream {
		if val > 0 {
			foundData = true
			break
		}
	}
	if !foundData {
		t.Error("Heart rate stream contains only zeros, expected populated data")
	}
}

func TestFitBitHeartRate_Enrich_IntegrationDisabled(t *testing.T) {
	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{
				Enabled: false,
			},
		},
	}

	_, err := provider.Enrich(context.Background(), activity, user, nil, false)
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
		StartTime: timestamppb.New(time.Now()),
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

	_, err := provider.EnrichWithClient(context.Background(), activity, user, nil, mockHTTPClient, false)
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

func TestFitBitHeartRate_Enrich_LagDetected(t *testing.T) {
	mockHTTPClient := &http.Client{
		Transport: &mockTransport{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				// Return EMPTY mock heart rate data
				mockResponse := `{"activities-heart-intraday":{"dataset":[]}}`
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				}, nil
			},
		},
	}

	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	// Activity ended 5 minutes ago (Recent -> Should Retry)
	endTime := time.Now().Add(-5 * time.Minute)
	startTime := endTime.Add(-1 * time.Hour)

	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(startTime),
		Sessions:  []*pb.Session{{TotalElapsedTime: 3600}},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{Enabled: true, AccessToken: "t"},
		},
	}

	_, err := provider.EnrichWithClient(context.Background(), activity, user, nil, mockHTTPClient, false)
	if err == nil {
		t.Fatal("Expected error for recent missing data")
	}

	if retryErr, ok := err.(*RetryableError); !ok {
		t.Errorf("Expected RetryableError, got %T: %v", err, err)
	} else {
		if retryErr.RetryAfter == 0 {
			t.Error("Expected non-zero RetryAfter")
		}
	}
}

func TestFitBitHeartRate_Enrich_LagExpired(t *testing.T) {
	mockHTTPClient := &http.Client{
		Transport: &mockTransport{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`{"activities-heart-intraday":{"dataset":[]}}`)),
				}, nil
			},
		},
	}

	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	// Activity ended 2 hours ago (Old -> Should Accept Empty)
	endTime := time.Now().Add(-2 * time.Hour)
	startTime := endTime.Add(-1 * time.Hour)

	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(startTime),
		Sessions:  []*pb.Session{{TotalElapsedTime: 3600}},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{Enabled: true, AccessToken: "t"},
		},
	}

	res, err := provider.EnrichWithClient(context.Background(), activity, user, nil, mockHTTPClient, false)
	if err != nil {
		t.Fatalf("Expected success for old missing data, got: %v", err)
	}
	if len(res.HeartRateStream) != 3600 {
		t.Errorf("Expected stream length 3600, got %d", len(res.HeartRateStream))
	}
}

func TestFitBitHeartRate_Name(t *testing.T) {
	provider := NewFitBitHeartRate()
	expected := "fitbit-heart-rate"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestFitBitHeartRate_Enrich_LagExhausted(t *testing.T) {
	mockHTTPClient := &http.Client{
		Transport: &mockTransport{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				// Return EMPTY mock heart rate data
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`{"activities-heart-intraday":{"dataset":[]}}`)),
				}, nil
			},
		},
	}

	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	// Activity ended 5 minutes ago (Recent -> Should Normally Retry)
	endTime := time.Now().Add(-5 * time.Minute)
	startTime := endTime.Add(-1 * time.Hour)

	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(startTime),
		Sessions:  []*pb.Session{{TotalElapsedTime: 3600}},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{Enabled: true, AccessToken: "t"},
		},
	}

	// Should not return error despite missing data (doNotRetry=true)
	_, err := provider.EnrichWithClient(context.Background(), activity, user, nil, mockHTTPClient, true)
	if err != nil {
		t.Fatalf("Expected success when doNotRetry is set, got error: %v", err)
	}

}

func TestFitBitHeartRate_Enrich_SkipIfExistingHRData(t *testing.T) {
	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	// Create activity WITH existing heart rate data
	startTime := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(startTime),
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 120}, // Existing HR data
							{HeartRate: 130},
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{
				Enabled:     true,
				AccessToken: "test-token",
			},
		},
	}

	// Without force=true, should skip
	result, err := provider.Enrich(context.Background(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Metadata["hr_source"] != "skipped" {
		t.Errorf("Expected hr_source=skipped, got %s", result.Metadata["hr_source"])
	}
	if result.Metadata["force"] != "false" {
		t.Errorf("Expected force=false in metadata, got %s", result.Metadata["force"])
	}
}

func TestFitBitHeartRate_Enrich_ForceOverwrite(t *testing.T) {
	mockHTTPClient := &http.Client{
		Transport: &mockTransport{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				mockResponse := `{
					"activities-heart-intraday": {
						"dataset": [
							{"time": "10:00:00", "value": 120}
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

	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	// Create activity WITH existing heart rate data
	startTime := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(startTime),
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{HeartRate: 120}, // Existing HR data
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Fitbit: &pb.FitbitIntegration{
				Enabled:     true,
				AccessToken: "test-token",
			},
		},
	}

	// With force=true, should proceed to fetch from Fitbit
	result, err := provider.EnrichWithClient(context.Background(), activity, user, map[string]string{"force": "true"}, mockHTTPClient, false)
	if err != nil {
		t.Fatalf("Expected no error with force=true, got: %v", err)
	}

	if result.Metadata["hr_source"] != "fitbit" {
		t.Errorf("Expected hr_source=fitbit with force=true, got %s", result.Metadata["hr_source"])
	}
}
