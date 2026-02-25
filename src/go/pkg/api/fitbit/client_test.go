package fitbit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	fitbit "github.com/fitglue/server/src/go/pkg/api/fitbit"
)

func fitbitFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func fitbitDate(year, month, day int) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}
}

func TestFitbitNewClient(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := fitbit.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestFitbitWithHTTPClient(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, nil)
	defer srv.Close()

	c, err := fitbit.NewClient(srv.URL, fitbit.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient with custom client failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestFitbitGetFriends(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"friends": []interface{}{}})
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

func TestFitbitGetFriendsLeaderboard(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{})
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetFriendsLeaderboard(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitGetSleepGoal(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"goal": 8})
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

func TestFitbitGetSleepByDate(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"sleep": []interface{}{}})
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetSleepByDate(context.Background(), fitbitDate(2024, 1, 15))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitGetSleepByDateRange(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"sleep": []interface{}{}})
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetSleepByDateRange(context.Background(), fitbitDate(2024, 1, 1), fitbitDate(2024, 1, 31))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitGetSleepList(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"sleep": []interface{}{}})
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	beforeDate := fitbitDate(2024, 12, 1)
	params := &fitbit.GetSleepListParams{BeforeDate: &beforeDate}
	resp, err := c.GetSleepList(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitDeleteSleep(t *testing.T) {
	srv := fitbitFakeServer(http.StatusNoContent, nil)
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.DeleteSleep(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestFitbitGetActivitiesTypes(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"categories": []interface{}{}})
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

func TestFitbitGetActivitiesTypeDetail(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"activity": map[string]interface{}{}})
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetActivitiesTypeDetail(context.Background(), "90013")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitGetActivitiesLog(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"activities": []interface{}{}})
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

func TestFitbitGetFoodsList(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"foods": []interface{}{}})
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	params := &fitbit.GetFoodsListParams{Query: "banana"}
	resp, err := c.GetFoodsList(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitGetFoodsUnits(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, []interface{}{})
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

func TestFitbitGetFoodsLocales(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, []interface{}{})
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

func TestFitbitGetFoodsInfo(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{"food": map[string]interface{}{}})
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetFoodsInfo(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitGetAZMByDateIntraday(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, map[string]interface{}{})
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.GetAZMByDateIntraday(context.Background(), fitbitDate(2024, 6, 1), fitbit.GetAZMByDateIntradayParamsDetailLevel("1min"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitWithRequestEditorFn(t *testing.T) {
	srv := fitbitFakeServer(http.StatusOK, nil)
	defer srv.Close()

	called := false
	editor := fitbit.RequestEditorFn(func(ctx context.Context, req *http.Request) error {
		called = true
		req.Header.Set("Authorization", "Bearer fake-token")
		return nil
	})

	c, _ := fitbit.NewClient(srv.URL, fitbit.WithRequestEditorFn(editor))
	_, _ = c.GetFriends(context.Background())

	if !called {
		t.Error("expected request editor to be called")
	}
}
