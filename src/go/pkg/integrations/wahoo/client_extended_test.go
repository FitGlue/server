package wahoo_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/wahoo"
)

func wahooServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

func TestAllWahooMethods(t *testing.T) {
	srv := wahooServer()
	defer srv.Close()

	c, err := wahoo.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		{"GetUser", func() (*http.Response, error) {
			return c.GetUser(ctx)
		}},
		{"GetWorkout", func() (*http.Response, error) {
			return c.GetWorkout(ctx, "workout-id-1")
		}},
		{"GetWorkoutFile", func() (*http.Response, error) {
			return c.GetWorkoutFile(ctx, "workout-id-1")
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.fn()
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
				return
			}
			if resp == nil {
				t.Errorf("%s: expected non-nil response", tc.name)
			}
		})
	}
}

func TestWahooNewClientWithResponsesExtended(t *testing.T) {
	srv := wahooServer()
	defer srv.Close()

	c, err := wahoo.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
