package fitbit_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/fitglue/server/src/go/pkg/api/fitbit"
)

func fitbitAPIExtra2Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

func apiDate() openapi_types.Date {
	return openapi_types.Date{Time: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)}
}

// TestFitbitAPIExtra2 covers ALL remaining uncovered method calls in pkg/api/fitbit.
func TestFitbitAPIExtra2(t *testing.T) {
	srv := fitbitAPIExtra2Server()
	defer srv.Close()

	c, err := fitbit.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	d := apiDate()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		// Auth / OAuth
		{"IntrospectWithFormdataBody", func() (*http.Response, error) {
			return c.IntrospectWithFormdataBody(ctx, fitbit.IntrospectFormdataRequestBody{})
		}},
		{"IntrospectWithBody", func() (*http.Response, error) {
			return c.IntrospectWithBody(ctx, "application/x-www-form-urlencoded", strings.NewReader("token=test"))
		}},
		{"OauthToken", func() (*http.Response, error) {
			p := &fitbit.OauthTokenParams{}
			return c.OauthToken(ctx, p)
		}},
		{"RevokeWithBody", func() (*http.Response, error) {
			return c.RevokeWithBody(ctx, "application/x-www-form-urlencoded", strings.NewReader("token=test"))
		}},
		{"RevokeWithFormdataBody", func() (*http.Response, error) {
			return c.RevokeWithFormdataBody(ctx, fitbit.RevokeFormdataRequestBody{})
		}},
		// Sleep
		{"AddSleep", func() (*http.Response, error) {
			return c.AddSleep(ctx, &fitbit.AddSleepParams{})
		}},
		{"UpdateSleepGoal", func() (*http.Response, error) {
			return c.UpdateSleepGoal(ctx, &fitbit.UpdateSleepGoalParams{})
		}},
		// Activities
		{"AddActivitiesLog", func() (*http.Response, error) {
			return c.AddActivitiesLog(ctx, &fitbit.AddActivitiesLogParams{})
		}},
		{"AddUpdateActivitiesGoals", func() (*http.Response, error) {
			return c.AddUpdateActivitiesGoals(ctx, "daily", &fitbit.AddUpdateActivitiesGoalsParams{})
		}},
		// Intraday (api/fitbit uses openapi_types.Date for date params!)
		{"GetActivitiesResourceByDateIntraday", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDateIntraday(ctx, fitbit.GetActivitiesResourceByDateIntradayParamsResourcePath("steps"), d, "1min")
		}},
		// Body Goals
		{"UpdateBodyFatGoal", func() (*http.Response, error) {
			return c.UpdateBodyFatGoal(ctx, &fitbit.UpdateBodyFatGoalParams{})
		}},
		{"UpdateWeightGoal", func() (*http.Response, error) {
			return c.UpdateWeightGoal(ctx, &fitbit.UpdateWeightGoalParams{})
		}},
		// Alarms
		{"AddAlarms", func() (*http.Response, error) {
			return c.AddAlarms(ctx, 12345, &fitbit.AddAlarmsParams{})
		}},
		{"DeleteAlarms", func() (*http.Response, error) {
			return c.DeleteAlarms(ctx, 12345, 99)
		}},
		{"UpdateAlarms", func() (*http.Response, error) {
			return c.UpdateAlarms(ctx, 12345, 99, &fitbit.UpdateAlarmsParams{})
		}},
		// Foods
		{"AddFoods", func() (*http.Response, error) {
			return c.AddFoods(ctx, &fitbit.AddFoodsParams{})
		}},
		{"AddFoodsLog", func() (*http.Response, error) {
			return c.AddFoodsLog(ctx, &fitbit.AddFoodsLogParams{})
		}},
		{"DeleteFavoriteFood", func() (*http.Response, error) {
			return c.DeleteFavoriteFood(ctx, "food-1")
		}},
		{"AddFavoriteFood", func() (*http.Response, error) {
			return c.AddFavoriteFood(ctx, "food-1")
		}},
		{"AddUpdateFoodsGoal", func() (*http.Response, error) {
			return c.AddUpdateFoodsGoal(ctx, &fitbit.AddUpdateFoodsGoalParams{})
		}},
		// Water log (unique to api/fitbit)
		{"AddWaterLog", func() (*http.Response, error) {
			return c.AddWaterLog(ctx, &fitbit.AddWaterLogParams{})
		}},
		{"AddUpdateWaterGoal", func() (*http.Response, error) {
			return c.AddUpdateWaterGoal(ctx, &fitbit.AddUpdateWaterGoalParams{})
		}},
		{"DeleteWaterLog", func() (*http.Response, error) {
			return c.DeleteWaterLog(ctx, "water-1")
		}},
		{"UpdateWaterLog", func() (*http.Response, error) {
			return c.UpdateWaterLog(ctx, "water-1", &fitbit.UpdateWaterLogParams{})
		}},
		{"DeleteFoodsLog", func() (*http.Response, error) {
			return c.DeleteFoodsLog(ctx, "log-1")
		}},
		{"EditFoodsLog", func() (*http.Response, error) {
			return c.EditFoodsLog(ctx, "log-1", &fitbit.EditFoodsLogParams{})
		}},
		{"DeleteFoods", func() (*http.Response, error) {
			return c.DeleteFoods(ctx, "food-1")
		}},
		// Meals
		{"AddMeal", func() (*http.Response, error) {
			return c.AddMeal(ctx, fitbit.AddMealJSONRequestBody{})
		}},
		{"UpdateMealWithBody", func() (*http.Response, error) {
			var r io.Reader = bytes.NewBufferString("{}")
			return c.UpdateMealWithBody(ctx, "meal-1", "application/json", r)
		}},
		{"UpdateMeal", func() (*http.Response, error) {
			return c.UpdateMeal(ctx, "meal-1", fitbit.UpdateMealJSONRequestBody{})
		}},
		// Subscriptions (no params in api/fitbit)
		{"DeleteSubscriptions", func() (*http.Response, error) {
			return c.DeleteSubscriptions(ctx, "activities", "sub-1")
		}},
		{"AddSubscriptions", func() (*http.Response, error) {
			return c.AddSubscriptions(ctx, "activities", "sub-1")
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.fn()
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
				return
			}
			if resp == nil {
				t.Errorf("%s: expected non-nil response", tc.name)
			}
		})
	}
}
