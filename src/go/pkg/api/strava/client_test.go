package strava_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/api/strava"
)

func stravaAPIFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestStravaAPINewClient(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := strava.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestStravaAPIGetLoggedInAthlete(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, map[string]interface{}{"id": 123})
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

func TestStravaAPIGetLoggedInAthleteActivities(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, []interface{}{})
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

func TestStravaAPIGetActivityById(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, map[string]interface{}{"id": 42})
	defer srv.Close()
	c, _ := strava.NewClient(srv.URL)
	includeAll := true
	params := &strava.GetActivityByIdParams{IncludeAllEfforts: &includeAll}
	resp, err := c.GetActivityById(context.Background(), 42, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStravaAPIGetLapsByActivityId(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, []interface{}{})
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

func TestStravaAPIGetZonesByActivityId(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, map[string]interface{}{})
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

func TestStravaAPIGetCommentsByActivityId(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, []interface{}{})
	defer srv.Close()
	c, _ := strava.NewClient(srv.URL)
	page := 1
	perPage := 10
	params := &strava.GetCommentsByActivityIdParams{Page: &page, PerPage: &perPage}
	resp, err := c.GetCommentsByActivityId(context.Background(), 42, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStravaAPIGetActivityStreams(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, map[string]interface{}{})
	defer srv.Close()
	c, _ := strava.NewClient(srv.URL)
	params := &strava.GetActivityStreamsParams{
		Keys:      []strava.GetActivityStreamsParamsKeys{strava.GetActivityStreamsParamsKeysTime},
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

func TestStravaAPIWithHTTPClient(t *testing.T) {
	srv := stravaAPIFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := strava.NewClient(srv.URL, strava.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
