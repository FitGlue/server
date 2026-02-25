package trainingpeaks_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/trainingpeaks"
)

func tpFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestTrainingPeaksNewClient(t *testing.T) {
	srv := tpFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := trainingpeaks.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestTrainingPeaksGetAthleteProfile(t *testing.T) {
	srv := tpFakeServer(http.StatusOK, map[string]interface{}{
		"userId": "tp-user-42",
		"email":  "athlete@example.com",
	})
	defer srv.Close()

	c, _ := trainingpeaks.NewClient(srv.URL)
	resp, err := c.GetAthleteProfile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestTrainingPeaksWithHTTPClient(t *testing.T) {
	srv := tpFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := trainingpeaks.NewClient(srv.URL, trainingpeaks.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestTrainingPeaksNewClientWithResponses(t *testing.T) {
	srv := tpFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := trainingpeaks.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
