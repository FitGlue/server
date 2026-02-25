package oura_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/oura"
)

func ouraFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestOuraNewClient(t *testing.T) {
	srv := ouraFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := oura.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestOuraSandboxMultipleDailyActivity(t *testing.T) {
	srv := ouraFakeServer(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	defer srv.Close()
	c, _ := oura.NewClient(srv.URL)
	params := &oura.SandboxMultipleDailyActivityDocumentsV2SandboxUsercollectionDailyActivityGetParams{}
	resp, err := c.SandboxMultipleDailyActivityDocumentsV2SandboxUsercollectionDailyActivityGet(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOuraSandboxSingleDailyActivity(t *testing.T) {
	srv := ouraFakeServer(http.StatusOK, map[string]interface{}{"id": "doc-1"})
	defer srv.Close()
	c, _ := oura.NewClient(srv.URL)
	resp, err := c.SandboxSingleDailyActivityDocumentV2SandboxUsercollectionDailyActivityDocumentIdGet(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOuraSandboxMultipleDailyReadiness(t *testing.T) {
	srv := ouraFakeServer(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	defer srv.Close()
	c, _ := oura.NewClient(srv.URL)
	params := &oura.SandboxMultipleDailyReadinessDocumentsV2SandboxUsercollectionDailyReadinessGetParams{}
	resp, err := c.SandboxMultipleDailyReadinessDocumentsV2SandboxUsercollectionDailyReadinessGet(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOuraSandboxSingleDailyReadiness(t *testing.T) {
	srv := ouraFakeServer(http.StatusOK, map[string]interface{}{"id": "doc-r1"})
	defer srv.Close()
	c, _ := oura.NewClient(srv.URL)
	resp, err := c.SandboxSingleDailyReadinessDocumentV2SandboxUsercollectionDailyReadinessDocumentIdGet(context.Background(), "doc-r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOuraSandboxMultipleDailyResilience(t *testing.T) {
	srv := ouraFakeServer(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	defer srv.Close()
	c, _ := oura.NewClient(srv.URL)
	params := &oura.SandboxMultipleDailyResilienceDocumentsV2SandboxUsercollectionDailyResilienceGetParams{}
	resp, err := c.SandboxMultipleDailyResilienceDocumentsV2SandboxUsercollectionDailyResilienceGet(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOuraSandboxSingleDailyCardiovascularAge(t *testing.T) {
	srv := ouraFakeServer(http.StatusOK, map[string]interface{}{"id": "cv-1"})
	defer srv.Close()
	c, _ := oura.NewClient(srv.URL)
	resp, err := c.SandboxSingleDailyCardiovascularAgeDocumentV2SandboxUsercollectionDailyCardiovascularAgeDocumentIdGet(context.Background(), "cv-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOuraWithHTTPClient(t *testing.T) {
	srv := ouraFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := oura.NewClient(srv.URL, oura.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient with HTTP client failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
