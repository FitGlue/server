package fitbit_heart_rate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/internal/pipeline/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/user"
	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// newRetryTestFixtures returns a provider, activity and user for transient-error tests.
func newRetryTestFixtures() (*FitBitHeartRate, *pbactivity.StandardizedActivity, *user.Record) {
	provider := NewFitBitHeartRate()
	provider.Service = &bootstrap.Service{}

	activity := &pbactivity.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
		Sessions:  []*pbactivity.Session{{TotalElapsedTime: 3600}},
	}
	userRec := &user.Record{
		UserProfile: &pbuser.UserProfile{UserId: "test-user"},
		Integrations: &pbuser.UserIntegrations{
			Fitbit: &pbuser.FitbitIntegration{Enabled: true, AccessToken: "token"},
		},
	}
	return provider, activity, userRec
}

// hrEndpointClient serves the profile endpoint normally and lets the test
// control the intraday HR endpoint's response.
func hrEndpointClient(hr func(req *http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{
		Transport: &mockTransport{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/profile.json") {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(mockProfileUTC)),
					}, nil
				}
				return hr(req)
			},
		},
	}
}

func TestFitBitHeartRate_Enrich_429IsRetryable(t *testing.T) {
	client := hrEndpointClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"3600"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"errors":[{"errorType":"request"}]}`)),
		}, nil
	})

	provider, activity, userRec := newRetryTestFixtures()
	_, err := provider.EnrichWithClient(context.Background(), slog.Default(), activity, userRec, nil, client, false)

	var re *providers.RetryableError
	if !errors.As(err, &re) {
		t.Fatalf("expected RetryableError for 429, got %v", err)
	}
	if re.RetryAfter != time.Hour {
		t.Errorf("expected RetryAfter=1h from Retry-After header, got %v", re.RetryAfter)
	}
}

func TestFitBitHeartRate_Enrich_5xxIsRetryable(t *testing.T) {
	client := hrEndpointClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(bytes.NewBufferString(`upstream error`)),
		}, nil
	})

	provider, activity, userRec := newRetryTestFixtures()
	_, err := provider.EnrichWithClient(context.Background(), slog.Default(), activity, userRec, nil, client, false)

	var re *providers.RetryableError
	if !errors.As(err, &re) {
		t.Fatalf("expected RetryableError for 502, got %v", err)
	}
	if re.RetryAfter != 0 {
		t.Errorf("expected no RetryAfter without header, got %v", re.RetryAfter)
	}
}

func TestFitBitHeartRate_Enrich_TransientLagExhaustedSkips(t *testing.T) {
	client := hrEndpointClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
		}, nil
	})

	provider, activity, userRec := newRetryTestFixtures()
	res, err := provider.EnrichWithClient(context.Background(), slog.Default(), activity, userRec, nil, client, true /* doNotRetry */)
	if err != nil {
		t.Fatalf("expected graceful skip when lag exhausted, got error: %v", err)
	}
	if res == nil || res.Metadata["hr_source"] != "skipped" {
		t.Errorf("expected hr_source=skipped result, got %+v", res)
	}
}

func TestFitBitHeartRate_Enrich_TransportErrorIsRetryable(t *testing.T) {
	client := hrEndpointClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("connection reset")
	})

	provider, activity, userRec := newRetryTestFixtures()
	_, err := provider.EnrichWithClient(context.Background(), slog.Default(), activity, userRec, nil, client, false)

	var re *providers.RetryableError
	if !errors.As(err, &re) {
		t.Fatalf("expected RetryableError for transport failure, got %v", err)
	}
}

func TestFitBitHeartRate_Enrich_ClientErrorStaysTerminal(t *testing.T) {
	client := hrEndpointClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(bytes.NewBufferString(`{"errors":[{"errorType":"insufficient_scope"}]}`)),
		}, nil
	})

	provider, activity, userRec := newRetryTestFixtures()
	_, err := provider.EnrichWithClient(context.Background(), slog.Default(), activity, userRec, nil, client, false)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	var re *providers.RetryableError
	if errors.As(err, &re) {
		t.Errorf("403 must stay terminal, got RetryableError: %v", err)
	}
}
