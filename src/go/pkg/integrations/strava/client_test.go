package strava_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/strava"
)

func stravaFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestStravaNewClient(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, map[string]interface{}{})
	defer srv.Close()

	c, err := strava.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestGetLoggedInAthlete(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, map[string]interface{}{
		"id": 123, "firstname": "John", "lastname": "Doe",
	})
	defer srv.Close()

	c, _ := strava.NewClient(srv.URL)
	resp, err := c.GetLoggedInAthlete(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetLoggedInAthleteActivities(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, []interface{}{})
	defer srv.Close()

	c, _ := strava.NewClient(srv.URL)
	params := &strava.GetLoggedInAthleteActivitiesParams{}
	resp, err := c.GetLoggedInAthleteActivities(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetActivityById(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, map[string]interface{}{"id": 42, "name": "Morning Run"})
	defer srv.Close()

	c, _ := strava.NewClient(srv.URL)
	includeAllEfforts := true
	params := &strava.GetActivityByIdParams{
		IncludeAllEfforts: &includeAllEfforts,
	}
	resp, err := c.GetActivityById(context.Background(), 42, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetActivityStreams(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, map[string]interface{}{"series_type": "distance"})
	defer srv.Close()

	c, _ := strava.NewClient(srv.URL)
	params := &strava.GetActivityStreamsParams{
		Keys:      []strava.GetActivityStreamsParamsKeys{strava.GetActivityStreamsParamsKeysTime, strava.GetActivityStreamsParamsKeysDistance},
		KeyByType: true,
	}
	resp, err := c.GetActivityStreams(context.Background(), 42, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetLapsByActivityId(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, []interface{}{})
	defer srv.Close()

	c, _ := strava.NewClient(srv.URL)
	resp, err := c.GetLapsByActivityId(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetZonesByActivityId(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, map[string]interface{}{"score": 100})
	defer srv.Close()

	c, _ := strava.NewClient(srv.URL)
	resp, err := c.GetZonesByActivityId(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetCommentsByActivityId(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, []interface{}{})
	defer srv.Close()

	c, _ := strava.NewClient(srv.URL)
	page := 1
	perPage := 10
	params := &strava.GetCommentsByActivityIdParams{
		Page:    &page,
		PerPage: &perPage,
	}
	resp, err := c.GetCommentsByActivityId(context.Background(), 42, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetKudoersByActivityId(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, []interface{}{})
	defer srv.Close()

	c, _ := strava.NewClient(srv.URL)
	page := 1
	perPage := 10
	params := &strava.GetKudoersByActivityIdParams{
		Page:    &page,
		PerPage: &perPage,
	}
	resp, err := c.GetKudoersByActivityId(context.Background(), 42, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStravaWithHTTPClient(t *testing.T) {
	srv := stravaFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := strava.NewClient(srv.URL, strava.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
