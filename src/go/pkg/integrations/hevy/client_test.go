package hevy_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/hevy"
)

// fakeServer returns a httptest.Server that always replies with the given status and body.
func fakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestNewClient(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{"count": 0})
	defer srv.Close()

	c, err := hevy.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	srv := fakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := hevy.NewClient(srv.URL, hevy.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestGetV1ExerciseTemplates(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{
		"exercise_templates": []interface{}{},
		"page":               1,
		"page_count":         1,
	})
	defer srv.Close()

	c, _ := hevy.NewClient(srv.URL)
	page := 1
	pageSize := 10
	params := &hevy.GetV1ExerciseTemplatesParams{
		Page:     &page,
		PageSize: &pageSize,
	}
	resp, err := c.GetV1ExerciseTemplates(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetV1Workouts(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{
		"workouts":   []interface{}{},
		"page":       1,
		"page_count": 1,
	})
	defer srv.Close()

	c, _ := hevy.NewClient(srv.URL)
	page := 1
	pageSize := 5
	params := &hevy.GetV1WorkoutsParams{
		Page:     &page,
		PageSize: &pageSize,
	}
	resp, err := c.GetV1Workouts(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetV1WorkoutsCount(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{"workout_count": 42})
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

func TestGetV1WorkoutsEvents(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{
		"events": []interface{}{},
		"page":   1,
	})
	defer srv.Close()

	c, _ := hevy.NewClient(srv.URL)
	page := 1
	params := &hevy.GetV1WorkoutsEventsParams{Page: &page}
	resp, err := c.GetV1WorkoutsEvents(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetV1Routines(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{"routines": []interface{}{}})
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

func TestGetV1RoutineFolders(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{"routine_folders": []interface{}{}})
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

func TestGetV1ExerciseHistoryExerciseTemplateId(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{"sets": []interface{}{}})
	defer srv.Close()

	c, _ := hevy.NewClient(srv.URL)
	params := &hevy.GetV1ExerciseHistoryExerciseTemplateIdParams{}
	resp, err := c.GetV1ExerciseHistoryExerciseTemplateId(context.Background(), 12345, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPostV1Workouts_WithBody(t *testing.T) {
	srv := fakeServer(http.StatusCreated, map[string]interface{}{"id": "w1"})
	defer srv.Close()

	c, _ := hevy.NewClient(srv.URL)
	params := &hevy.PostV1WorkoutsParams{}
	body := hevy.PostV1WorkoutsJSONRequestBody{}

	resp, err := c.PostV1Workouts(context.Background(), params, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestGetV1RoutineFoldersFolderId(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{"id": 1, "name": "Test"})
	defer srv.Close()

	c, _ := hevy.NewClient(srv.URL)
	params := &hevy.GetV1RoutineFoldersFolderIdParams{}
	resp, err := c.GetV1RoutineFoldersFolderId(context.Background(), 1, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetV1RoutinesRoutineId(t *testing.T) {
	srv := fakeServer(http.StatusOK, map[string]interface{}{"id": 1, "name": "My Routine"})
	defer srv.Close()

	c, _ := hevy.NewClient(srv.URL)
	params := &hevy.GetV1RoutinesRoutineIdParams{}
	resp, err := c.GetV1RoutinesRoutineId(context.Background(), 1, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWithRequestEditorFn(t *testing.T) {
	srv := fakeServer(http.StatusOK, nil)
	defer srv.Close()

	called := false
	editor := hevy.RequestEditorFn(func(ctx context.Context, req *http.Request) error {
		called = true
		req.Header.Set("X-Test", "value")
		return nil
	})

	c, err := hevy.NewClient(srv.URL, hevy.WithRequestEditorFn(editor))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	params := &hevy.GetV1WorkoutsCountParams{}
	_, _ = c.GetV1WorkoutsCount(context.Background(), params)

	if !called {
		t.Error("expected request editor to be called")
	}
}
