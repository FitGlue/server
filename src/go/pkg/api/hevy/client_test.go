package hevy_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/api/hevy"
)

func hevyAPIFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestHevyAPINewClient(t *testing.T) {
	srv := hevyAPIFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := hevy.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestHevyAPIGetV1ExerciseTemplates(t *testing.T) {
	srv := hevyAPIFakeServer(http.StatusOK, map[string]interface{}{"exercise_templates": []interface{}{}})
	defer srv.Close()
	c, _ := hevy.NewClient(srv.URL)
	page := 1
	pageSize := 10
	params := &hevy.GetV1ExerciseTemplatesParams{Page: &page, PageSize: &pageSize}
	resp, err := c.GetV1ExerciseTemplates(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHevyAPIGetV1Workouts(t *testing.T) {
	srv := hevyAPIFakeServer(http.StatusOK, map[string]interface{}{"workouts": []interface{}{}})
	defer srv.Close()
	c, _ := hevy.NewClient(srv.URL)
	page := 1
	pageSize := 5
	params := &hevy.GetV1WorkoutsParams{Page: &page, PageSize: &pageSize}
	resp, err := c.GetV1Workouts(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHevyAPIGetV1WorkoutsCount(t *testing.T) {
	srv := hevyAPIFakeServer(http.StatusOK, map[string]interface{}{"workout_count": 5})
	defer srv.Close()
	c, _ := hevy.NewClient(srv.URL)
	params := &hevy.GetV1WorkoutsCountParams{}
	resp, err := c.GetV1WorkoutsCount(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHevyAPIGetV1RoutineFolders(t *testing.T) {
	srv := hevyAPIFakeServer(http.StatusOK, map[string]interface{}{"routine_folders": []interface{}{}})
	defer srv.Close()
	c, _ := hevy.NewClient(srv.URL)
	page := 1
	params := &hevy.GetV1RoutineFoldersParams{Page: &page}
	resp, err := c.GetV1RoutineFolders(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHevyAPIGetV1Routines(t *testing.T) {
	srv := hevyAPIFakeServer(http.StatusOK, map[string]interface{}{"routines": []interface{}{}})
	defer srv.Close()
	c, _ := hevy.NewClient(srv.URL)
	page := 1
	params := &hevy.GetV1RoutinesParams{Page: &page}
	resp, err := c.GetV1Routines(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHevyAPIWithHTTPClient(t *testing.T) {
	srv := hevyAPIFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := hevy.NewClient(srv.URL, hevy.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
