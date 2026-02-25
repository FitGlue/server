package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/github"
)

func ghFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestGitHubNewClient(t *testing.T) {
	srv := ghFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := github.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestGitHubReposgetCommit(t *testing.T) {
	srv := ghFakeServer(http.StatusOK, map[string]interface{}{
		"sha": "abc123", "commit": map[string]interface{}{"message": "Initial commit"},
	})
	defer srv.Close()

	c, _ := github.NewClient(srv.URL)
	resp, err := c.ReposgetCommit(context.Background(), "owner", "repo", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGitHubReposgetContent(t *testing.T) {
	srv := ghFakeServer(http.StatusOK, map[string]interface{}{
		"type": "file", "name": "README.md",
	})
	defer srv.Close()

	c, _ := github.NewClient(srv.URL)
	params := &github.ReposgetContentParams{}
	resp, err := c.ReposgetContent(context.Background(), "owner", "repo", "README.md", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGitHubReposlistWebhooks(t *testing.T) {
	srv := ghFakeServer(http.StatusOK, []interface{}{})
	defer srv.Close()

	c, _ := github.NewClient(srv.URL)
	resp, err := c.ReposlistWebhooks(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGitHubReposdeleteWebhook(t *testing.T) {
	srv := ghFakeServer(http.StatusNoContent, nil)
	defer srv.Close()

	c, _ := github.NewClient(srv.URL)
	resp, err := c.ReposdeleteWebhook(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestGitHubWithHTTPClient(t *testing.T) {
	srv := ghFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := github.NewClient(srv.URL, github.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
