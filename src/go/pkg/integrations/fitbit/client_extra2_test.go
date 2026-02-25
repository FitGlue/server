package fitbit_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/fitbit"
)

func fitbitExtra2Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestFitbitIntegrationsExtra2 covers ALL remaining uncovered method calls in pkg/integrations/fitbit.
func TestFitbitIntegrationsExtra2(t *testing.T) {
	srv := fitbitExtra2Server()
	defer srv.Close()

	c, err := fitbit.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		// Auth / OAuth
		{"IntrospectWithFormdataBody", func() (*http.Response, error) {
			body := fitbit.IntrospectFormdataRequestBody{}
			return c.IntrospectWithFormdataBody(ctx, body)
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
			body := fitbit.RevokeFormdataRequestBody{}
			return c.RevokeWithFormdataBody(ctx, body)
		}},
		// Social
		{"GetFriendsLeaderboard", func() (*http.Response, error) {
			return c.GetFriendsLeaderboard(ctx)
		}},
		// Sleep
		{"AddSleep", func() (*http.Response, error) {
			p := &fitbit.AddSleepParams{}
			return c.AddSleep(ctx, p)
		}},
		{"UpdateSleepGoal", func() (*http.Response, error) {
			p := &fitbit.UpdateSleepGoalParams{}
			return c.UpdateSleepGoal(ctx, p)
		}},
		{"GetSleepList", func() (*http.Response, error) {
			p := &fitbit.GetSleepListParams{}
			return c.GetSleepList(ctx, p)
		}},
		// Activities
		{"GetActivitiesTypeDetail", func() (*http.Response, error) {
			return c.GetActivitiesTypeDetail(ctx, "90009")
		}},
		{"AddActivitiesLog", func() (*http.Response, error) {
			p := &fitbit.AddActivitiesLogParams{}
			return c.AddActivitiesLog(ctx, p)
		}},
		{"DeleteFavoriteActivities", func() (*http.Response, error) {
			return c.DeleteFavoriteActivities(ctx, "90009")
		}},
		{"AddFavoriteActivities", func() (*http.Response, error) {
			return c.AddFavoriteActivities(ctx, "90009")
		}},
		{"AddUpdateActivitiesGoals", func() (*http.Response, error) {
			p := &fitbit.AddUpdateActivitiesGoalsParams{}
			return c.AddUpdateActivitiesGoals(ctx, "daily", p)
		}},
		{"GetActivitiesTCX", func() (*http.Response, error) {
			p := &fitbit.GetActivitiesTCXParams{}
			return c.GetActivitiesTCX(ctx, "12345", p)
		}},
		// Intraday
		{"GetAZMByDateIntraday", func() (*http.Response, error) {
			return c.GetAZMByDateIntraday(ctx, "2024-06-15", fitbit.GetAZMByDateIntradayParamsDetailLevel("1min"))
		}},
		{"GetHeartByDateRangeTimestampIntraday", func() (*http.Response, error) {
			return c.GetHeartByDateRangeTimestampIntraday(ctx, "2024-06-15", "2024-06-16", "1min", "00:00", "23:59")
		}},
		{"GetActivitiesResourceByDateRangeIntraday", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDateRangeIntraday(ctx, fitbit.GetActivitiesResourceByDateRangeIntradayParamsResourcePath("steps"), "2024-06-15", "2024-06-16", "1min")
		}},
		{"GetActivitiesResourceByDateIntraday", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDateIntraday(ctx, fitbit.GetActivitiesResourceByDateIntradayParamsResourcePath("steps"), "2024-06-15", "1min")
		}},
		{"GetActivitiesResourceByDateTimeSeriesIntraday", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDateTimeSeriesIntraday(ctx, fitbit.GetActivitiesResourceByDateTimeSeriesIntradayParamsResourcePath("steps"), "2024-06-15", "1min", "00:00", "23:59")
		}},
		{"GetActivitiesResourceByDateRangeTimeSeriesIntraday", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDateRangeTimeSeriesIntraday(ctx, fitbit.GetActivitiesResourceByDateRangeTimeSeriesIntradayParamsResourcePath("steps"), "2024-06-15", "2024-06-16", "1min", "00:00", "23:59")
		}},
		// Body Goals
		{"UpdateBodyFatGoal", func() (*http.Response, error) {
			p := &fitbit.UpdateBodyFatGoalParams{}
			return c.UpdateBodyFatGoal(ctx, p)
		}},
		{"UpdateWeightGoal", func() (*http.Response, error) {
			p := &fitbit.UpdateWeightGoalParams{}
			return c.UpdateWeightGoal(ctx, p)
		}},
		// Alarms
		{"AddAlarms", func() (*http.Response, error) {
			p := &fitbit.AddAlarmsParams{}
			return c.AddAlarms(ctx, 12345, p)
		}},
		{"DeleteAlarms", func() (*http.Response, error) {
			return c.DeleteAlarms(ctx, 12345, 99)
		}},
		{"UpdateAlarms", func() (*http.Response, error) {
			p := &fitbit.UpdateAlarmsParams{}
			return c.UpdateAlarms(ctx, 12345, 99, p)
		}},
		// Foods
		{"GetFoodsList", func() (*http.Response, error) {
			p := &fitbit.GetFoodsListParams{}
			return c.GetFoodsList(ctx, p)
		}},
		{"GetFoodsInfo", func() (*http.Response, error) {
			return c.GetFoodsInfo(ctx, "12345")
		}},
		{"AddFoods", func() (*http.Response, error) {
			p := &fitbit.AddFoodsParams{}
			return c.AddFoods(ctx, p)
		}},
		{"AddFoodsLog", func() (*http.Response, error) {
			p := &fitbit.AddFoodsLogParams{}
			return c.AddFoodsLog(ctx, p)
		}},
		{"DeleteFavoriteFood", func() (*http.Response, error) {
			return c.DeleteFavoriteFood(ctx, "food-1")
		}},
		{"AddFavoriteFood", func() (*http.Response, error) {
			return c.AddFavoriteFood(ctx, "food-1")
		}},
		{"AddUpdateFoodsGoal", func() (*http.Response, error) {
			p := &fitbit.AddUpdateFoodsGoalParams{}
			return c.AddUpdateFoodsGoal(ctx, p)
		}},
		{"AddUpdateWaterGoal", func() (*http.Response, error) {
			p := &fitbit.AddUpdateWaterGoalParams{}
			return c.AddUpdateWaterGoal(ctx, p)
		}},
		{"DeleteWaterLog", func() (*http.Response, error) {
			return c.DeleteWaterLog(ctx, "water-1")
		}},
		{"UpdateWaterLog", func() (*http.Response, error) {
			p := &fitbit.UpdateWaterLogParams{}
			return c.UpdateWaterLog(ctx, "water-1", p)
		}},
		{"DeleteFoodsLog", func() (*http.Response, error) {
			return c.DeleteFoodsLog(ctx, "log-1")
		}},
		{"EditFoodsLog", func() (*http.Response, error) {
			p := &fitbit.EditFoodsLogParams{}
			return c.EditFoodsLog(ctx, "log-1", p)
		}},
		{"DeleteFoods", func() (*http.Response, error) {
			return c.DeleteFoods(ctx, "food-1")
		}},
		// Meals
		{"AddMeal", func() (*http.Response, error) {
			return c.AddMeal(ctx, fitbit.AddMealJSONRequestBody{})
		}},
		{"AddMealWithBody", func() (*http.Response, error) {
			return c.AddMealWithBody(ctx, "application/json", bytes.NewBufferString("{}"))
		}},
		{"UpdateMeal", func() (*http.Response, error) {
			return c.UpdateMeal(ctx, "meal-1", fitbit.UpdateMealJSONRequestBody{})
		}},
		{"UpdateMealWithBody", func() (*http.Response, error) {
			return c.UpdateMealWithBody(ctx, "meal-1", "application/json", bytes.NewBufferString("{}"))
		}},
		// User Profile
		{"Post1UserProfileJson", func() (*http.Response, error) {
			return c.Post1UserProfileJson(ctx)
		}},
		// Subscriptions
		{"DeleteSubscriptions", func() (*http.Response, error) {
			p := &fitbit.DeleteSubscriptionsParams{}
			return c.DeleteSubscriptions(ctx, "activities", "sub-1", p)
		}},
		{"AddSubscriptions", func() (*http.Response, error) {
			p := &fitbit.AddSubscriptionsParams{}
			return c.AddSubscriptions(ctx, "activities", "sub-1", p)
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
