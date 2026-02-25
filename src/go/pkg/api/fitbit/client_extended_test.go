package fitbit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	fitbit "github.com/fitglue/server/src/go/pkg/api/fitbit"
)

func apifitbitDate(year, month, day int) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}
}

func apiFitbitServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestAllApiFitbitMethods calls ALL remaining client methods using openapi_types.Date which
// this package version uses (unlike integrations/fitbit which uses string).
func TestAllApiFitbitMethods(t *testing.T) {
	srv := apiFitbitServer()
	defer srv.Close()

	c, err := fitbit.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	date := apifitbitDate(2024, 6, 15)
	date2 := apifitbitDate(2024, 6, 30)

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		// Heart Rate
		{"GetHeartByDateRange", func() (*http.Response, error) {
			return c.GetHeartByDateRange(ctx, "2024-06-01", date2)
		}},
		{"GetHeartByDateIntraday", func() (*http.Response, error) {
			return c.GetHeartByDateIntraday(ctx, date, "1min")
		}},
		{"GetHeartByDatePeriod", func() (*http.Response, error) {
			return c.GetHeartByDatePeriod(ctx, date, "1w")
		}},
		{"GetHeartByDateTimestampIntraday", func() (*http.Response, error) {
			return c.GetHeartByDateTimestampIntraday(ctx, date, "1min", "00:00", "23:59")
		}},
		{"GetHeartByDateRangeIntraday", func() (*http.Response, error) {
			return c.GetHeartByDateRangeIntraday(ctx, date, date2, "1min")
		}},
		// Activity
		{"GetActivitiesByDate", func() (*http.Response, error) {
			return c.GetActivitiesByDate(ctx, date)
		}},
		{"GetFavoriteActivities", func() (*http.Response, error) {
			return c.GetFavoriteActivities(ctx)
		}},
		{"GetFrequentActivities", func() (*http.Response, error) {
			return c.GetFrequentActivities(ctx)
		}},
		{"GetActivitiesGoals", func() (*http.Response, error) {
			return c.GetActivitiesGoals(ctx, "daily")
		}},
		{"GetActivitiesLogList", func() (*http.Response, error) {
			p := &fitbit.GetActivitiesLogListParams{}
			return c.GetActivitiesLogList(ctx, p)
		}},
		{"GetRecentActivities", func() (*http.Response, error) {
			return c.GetRecentActivities(ctx)
		}},
		{"GetActivitiesTrackerResourceByDateRange", func() (*http.Response, error) {
			return c.GetActivitiesTrackerResourceByDateRange(ctx, "steps", date, date2)
		}},
		{"GetActivitiesTrackerResourceByDatePeriod", func() (*http.Response, error) {
			return c.GetActivitiesTrackerResourceByDatePeriod(ctx, "steps", date, "1w")
		}},
		{"DeleteActivitiesLog", func() (*http.Response, error) {
			return c.DeleteActivitiesLog(ctx, 123)
		}},
		{"GetActivitiesResourceByDateRange", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDateRange(ctx, "steps", date, date2)
		}},
		{"GetActivitiesResourceByDatePeriod", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDatePeriod(ctx, "steps", date, "1w")
		}},
		{"GetBadges", func() (*http.Response, error) {
			return c.GetBadges(ctx)
		}},
		// AZM (Active Zone Minutes)
		{"GetAZMByDateIntraday", func() (*http.Response, error) {
			return c.GetAZMByDateIntraday(ctx, date, fitbit.GetAZMByDateIntradayParamsDetailLevel("1min"))
		}},
		{"GetAZMTimeSeriesByDate", func() (*http.Response, error) {
			return c.GetAZMTimeSeriesByDate(ctx, date, "1w")
		}},
		{"GetAZMTimeSeriesByInterval", func() (*http.Response, error) {
			return c.GetAZMTimeSeriesByInterval(ctx, date, date2)
		}},
		{"GetAZMByDateTimeSeriesIntraday", func() (*http.Response, error) {
			return c.GetAZMByDateTimeSeriesIntraday(ctx, date, "1min", "00:00", "23:59")
		}},
		{"GetAZMByIntervalTimeSeriesIntraday", func() (*http.Response, error) {
			return c.GetAZMByIntervalTimeSeriesIntraday(ctx, date, date2, "00:00", "23:59")
		}},
		{"GetAZMByIntervalIntraday", func() (*http.Response, error) {
			return c.GetAZMByIntervalIntraday(ctx, date, date2, "1min")
		}},
		// Body
		{"AddBodyFatLog", func() (*http.Response, error) {
			p := &fitbit.AddBodyFatLogParams{Date: date, Fat: 20}
			return c.AddBodyFatLog(ctx, p)
		}},
		{"GetBodyFatByDateRange", func() (*http.Response, error) {
			return c.GetBodyFatByDateRange(ctx, date, date2)
		}},
		{"GetBodyFatByDate", func() (*http.Response, error) {
			return c.GetBodyFatByDate(ctx, date)
		}},
		{"GetBodyFatByDatePeriod", func() (*http.Response, error) {
			return c.GetBodyFatByDatePeriod(ctx, date, "1m")
		}},
		{"DeleteBodyFatLog", func() (*http.Response, error) {
			return c.DeleteBodyFatLog(ctx, 123)
		}},
		{"GetBodyGoals", func() (*http.Response, error) {
			return c.GetBodyGoals(ctx, "weight")
		}},
		{"GetBodyResourceByDateRange", func() (*http.Response, error) {
			return c.GetBodyResourceByDateRange(ctx, "weight", date, date2)
		}},
		{"GetBodyResourceByDatePeriod", func() (*http.Response, error) {
			return c.GetBodyResourceByDatePeriod(ctx, "weight", date, "1m")
		}},
		// Weight
		{"AddWeightLog", func() (*http.Response, error) {
			p := &fitbit.AddWeightLogParams{Date: date, Weight: 75}
			return c.AddWeightLog(ctx, p)
		}},
		{"GetWeightByDateRange", func() (*http.Response, error) {
			return c.GetWeightByDateRange(ctx, date, date2)
		}},
		{"GetWeightByDate", func() (*http.Response, error) {
			return c.GetWeightByDate(ctx, date)
		}},
		{"GetWeightByDatePeriod", func() (*http.Response, error) {
			return c.GetWeightByDatePeriod(ctx, date, "1m")
		}},
		{"DeleteWeightLog", func() (*http.Response, error) {
			return c.DeleteWeightLog(ctx, 456)
		}},
		// Breathing Rate
		{"GetBreathingRateSummaryByDate", func() (*http.Response, error) {
			return c.GetBreathingRateSummaryByDate(ctx, date)
		}},
		{"GetBreathingRateIntradayByDate", func() (*http.Response, error) {
			return c.GetBreathingRateIntradayByDate(ctx, date)
		}},
		{"GetBreathingRateSummaryByInterval", func() (*http.Response, error) {
			return c.GetBreathingRateSummaryByInterval(ctx, date, date2)
		}},
		{"GetBreathingRateIntradayByInterval", func() (*http.Response, error) {
			return c.GetBreathingRateIntradayByInterval(ctx, date, date2)
		}},
		// VO2Max
		{"GetVo2MaxSummaryByDate", func() (*http.Response, error) {
			return c.GetVo2MaxSummaryByDate(ctx, date)
		}},
		{"GetVo2MaxSummaryByInterval", func() (*http.Response, error) {
			return c.GetVo2MaxSummaryByInterval(ctx, date, date2)
		}},
		// Devices
		{"GetDevices", func() (*http.Response, error) {
			return c.GetDevices(ctx)
		}},
		{"GetAlarms", func() (*http.Response, error) {
			return c.GetAlarms(ctx, 12345)
		}},
		// ECG
		{"GetEcgLogList", func() (*http.Response, error) {
			p := &fitbit.GetEcgLogListParams{}
			return c.GetEcgLogList(ctx, p)
		}},
		// Foods
		{"GetFoodsByDate", func() (*http.Response, error) {
			return c.GetFoodsByDate(ctx, date)
		}},
		{"GetFavoriteFoods", func() (*http.Response, error) {
			return c.GetFavoriteFoods(ctx)
		}},
		{"GetFrequentFoods", func() (*http.Response, error) {
			return c.GetFrequentFoods(ctx)
		}},
		{"GetFoodsGoal", func() (*http.Response, error) {
			return c.GetFoodsGoal(ctx)
		}},
		{"GetRecentFoods", func() (*http.Response, error) {
			return c.GetRecentFoods(ctx)
		}},
		{"GetFoodsByDateRange", func() (*http.Response, error) {
			return c.GetFoodsByDateRange(ctx, "calories", date, date2)
		}},
		{"GetFoodsResourceByDatePeriod", func() (*http.Response, error) {
			return c.GetFoodsResourceByDatePeriod(ctx, "calories", date, "1m")
		}},
		// Water
		{"GetWaterByDate", func() (*http.Response, error) {
			return c.GetWaterByDate(ctx, date)
		}},
		{"GetWaterGoal", func() (*http.Response, error) {
			return c.GetWaterGoal(ctx)
		}},
		// HRV
		{"GetHrvSummaryDate", func() (*http.Response, error) {
			return c.GetHrvSummaryDate(ctx, date)
		}},
		{"GetHrvIntradayByDate", func() (*http.Response, error) {
			return c.GetHrvIntradayByDate(ctx, date)
		}},
		{"GetHrvSummaryInterval", func() (*http.Response, error) {
			return c.GetHrvSummaryInterval(ctx, date, date2)
		}},
		{"GetHrvIntradayByInterval", func() (*http.Response, error) {
			return c.GetHrvIntradayByInterval(ctx, date, date2)
		}},
		// IRN
		{"GetIrnAlertsList", func() (*http.Response, error) {
			p := &fitbit.GetIrnAlertsListParams{}
			return c.GetIrnAlertsList(ctx, p)
		}},
		{"GetIrnProfile", func() (*http.Response, error) {
			return c.GetIrnProfile(ctx)
		}},
		// Meals
		{"GetMeals", func() (*http.Response, error) {
			return c.GetMeals(ctx)
		}},
		{"DeleteMeal", func() (*http.Response, error) {
			return c.DeleteMeal(ctx, "meal-123")
		}},
		// Profile
		{"GetProfile", func() (*http.Response, error) {
			return c.GetProfile(ctx)
		}},
		// SpO2
		{"GetSpO2SummaryByDate", func() (*http.Response, error) {
			return c.GetSpO2SummaryByDate(ctx, date)
		}},
		{"GetSpO2IntradayByDate", func() (*http.Response, error) {
			return c.GetSpO2IntradayByDate(ctx, date)
		}},
		{"GetSpO2SummaryByInterval", func() (*http.Response, error) {
			return c.GetSpO2SummaryByInterval(ctx, date, date2)
		}},
		{"GetSpO2IntradayByInterval", func() (*http.Response, error) {
			return c.GetSpO2IntradayByInterval(ctx, date, date2)
		}},
		// Temperature
		{"GetTempCoreSummaryByDate", func() (*http.Response, error) {
			return c.GetTempCoreSummaryByDate(ctx, date)
		}},
		{"GetTempCoreSummaryByInterval", func() (*http.Response, error) {
			return c.GetTempCoreSummaryByInterval(ctx, date, date2)
		}},
		{"GetTempSkinSummaryDate", func() (*http.Response, error) {
			return c.GetTempSkinSummaryDate(ctx, date)
		}},
		{"GetTempSkinSummaryByInterval", func() (*http.Response, error) {
			return c.GetTempSkinSummaryByInterval(ctx, date, date2)
		}},
		// Subscriptions
		{"GetSubscriptionsList", func() (*http.Response, error) {
			return c.GetSubscriptionsList(ctx, "activities")
		}},
		// Sleep
		{"GetSleepGoal", func() (*http.Response, error) {
			return c.GetSleepGoal(ctx)
		}},
		{"GetSleepByDate", func() (*http.Response, error) {
			return c.GetSleepByDate(ctx, date)
		}},
		{"GetSleepByDateRange", func() (*http.Response, error) {
			return c.GetSleepByDateRange(ctx, date, date2)
		}},
		{"GetSleepList", func() (*http.Response, error) {
			beforeDate := apifitbitDate(2024, 12, 1)
			p := &fitbit.GetSleepListParams{BeforeDate: &beforeDate}
			return c.GetSleepList(ctx, p)
		}},
		{"DeleteSleep", func() (*http.Response, error) {
			return c.DeleteSleep(ctx, "sleep-id")
		}},
		{"DeleteFavoriteActivities", func() (*http.Response, error) {
			return c.DeleteFavoriteActivities(ctx, "activity-id")
		}},
		{"AddFavoriteActivities", func() (*http.Response, error) {
			return c.AddFavoriteActivities(ctx, "activity-id")
		}},
		{"GetActivitiesTCX", func() (*http.Response, error) {
			p := &fitbit.GetActivitiesTCXParams{}
			return c.GetActivitiesTCX(ctx, "log-123", p)
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

func TestApiFitbitNewClientWithResponses(t *testing.T) {
	srv := apiFitbitServer()
	defer srv.Close()

	c, err := fitbit.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
