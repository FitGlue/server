package polar_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/polar"
)

func polarFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestPolarNewClient(t *testing.T) {
	srv := polarFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := polar.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestPolarListExercisesWithoutTransaction(t *testing.T) {
	srv := polarFakeServer(http.StatusOK, map[string]interface{}{"exercises": []interface{}{}})
	defer srv.Close()
	c, _ := polar.NewClient(srv.URL)
	params := &polar.ListExercisesWithoutTransactionParams{}
	resp, err := c.ListExercisesWithoutTransaction(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPolarGetExerciseWithoutTransaction(t *testing.T) {
	srv := polarFakeServer(http.StatusOK, map[string]interface{}{"id": "ex-1"})
	defer srv.Close()
	c, _ := polar.NewClient(srv.URL)
	params := &polar.GetExerciseWithoutTransactionParams{}
	resp, err := c.GetExerciseWithoutTransaction(context.Background(), "ex-1", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPolarGetExerciseFitWithoutTransaction(t *testing.T) {
	srv := polarFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, _ := polar.NewClient(srv.URL)
	resp, err := c.GetExerciseFitWithoutTransaction(context.Background(), "ex-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPolarGetExerciseGpxWithoutTransaction(t *testing.T) {
	srv := polarFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, _ := polar.NewClient(srv.URL)
	resp, err := c.GetExerciseGpxWithoutTransaction(context.Background(), "ex-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPolarGetExerciseTcxWithoutTransaction(t *testing.T) {
	srv := polarFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, _ := polar.NewClient(srv.URL)
	resp, err := c.GetExerciseTcxWithoutTransaction(context.Background(), "ex-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPolarList(t *testing.T) {
	srv := polarFakeServer(http.StatusOK, map[string]interface{}{"items": []interface{}{}})
	defer srv.Close()
	c, _ := polar.NewClient(srv.URL)
	resp, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPolarWithHTTPClient(t *testing.T) {
	srv := polarFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := polar.NewClient(srv.URL, polar.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient with HTTP client failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
