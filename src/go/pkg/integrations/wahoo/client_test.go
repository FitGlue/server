package wahoo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/wahoo"
)

func wahooFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestWahooNewClient(t *testing.T) {
	srv := wahooFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := wahoo.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestWahooGetUser(t *testing.T) {
	srv := wahooFakeServer(http.StatusOK, map[string]interface{}{
		"id": "wahoo-user-123", "email": "wahoo@example.com",
	})
	defer srv.Close()

	c, _ := wahoo.NewClient(srv.URL)
	resp, err := c.GetUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWahooGetWorkout(t *testing.T) {
	srv := wahooFakeServer(http.StatusOK, map[string]interface{}{
		"id": "workout-abc", "name": "Outdoor Ride",
	})
	defer srv.Close()

	c, _ := wahoo.NewClient(srv.URL)
	resp, err := c.GetWorkout(context.Background(), "workout-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWahooGetWorkoutFile(t *testing.T) {
	srv := wahooFakeServer(http.StatusOK, map[string]interface{}{
		"file_type": "fit", "url": "https://example.com/workout.fit",
	})
	defer srv.Close()

	c, _ := wahoo.NewClient(srv.URL)
	resp, err := c.GetWorkoutFile(context.Background(), "workout-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWahooNewClientWithResponses(t *testing.T) {
	srv := wahooFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := wahoo.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
