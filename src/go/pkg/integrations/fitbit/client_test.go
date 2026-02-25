package fitbit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	fitbit "github.com/fitglue/server/src/go/pkg/integrations/fitbit"
)

func fitbitIntFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func fitbitIntDate(year, month, day int) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}
}

func TestFitbitIntegratonNewClient(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := fitbit.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestFitbitIntegrationWithHTTPClient(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := fitbit.NewClient(srv.URL, fitbit.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestFitbitIntegrationGetFriends(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, map[string]interface{}{"friends": []interface{}{}})
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetFriends(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationGetSleepGoal(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, map[string]interface{}{"goal": 8})
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetSleepGoal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationGetSleepByDate(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, map[string]interface{}{"sleep": []interface{}{}})
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetSleepByDate(context.Background(), fitbitIntDate(2024, 1, 15))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationGetSleepByDateRange(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, map[string]interface{}{"sleep": []interface{}{}})
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetSleepByDateRange(context.Background(), fitbitIntDate(2024, 1, 1), fitbitIntDate(2024, 1, 31))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationDeleteSleep(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusNoContent, nil)
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.DeleteSleep(context.Background(), "sleep-id-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationGetActivitiesTypes(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, map[string]interface{}{"categories": []interface{}{}})
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetActivitiesTypes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationGetActivitiesLog(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, map[string]interface{}{"activities": []interface{}{}})
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetActivitiesLog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationGetFoodsLocales(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, []interface{}{})
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetFoodsLocales(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationGetFoodsUnits(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, []interface{}{})
	defer srv.Close()
	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetFoodsUnits(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitIntegrationWithRequestEditorFn(t *testing.T) {
	srv := fitbitIntFakeServer(http.StatusOK, nil)
	defer srv.Close()

	called := false
	editor := fitbit.RequestEditorFn(func(ctx context.Context, req *http.Request) error {
		called = true
		req.Header.Set("Authorization", "Bearer test-token")
		return nil
	})

	c, _ := fitbit.NewClient(srv.URL, fitbit.WithRequestEditorFn(editor))
	_, _ = c.GetSleepGoal(context.Background())
	if !called {
		t.Error("expected request editor to be called")
	}
}
