package fitbit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	fitbit "github.com/fitglue/server/src/go/pkg/integrations/fitbit"
)

func extDate2(year, month, day int) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}
}

func allMethodsServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

func TestAllFitbitMethods(t *testing.T) {
	srv := allMethodsServer()
	defer srv.Close()

	c, err := fitbit.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	date := extDate2(2024, 6, 15)
	date2Var := extDate2(2024, 6, 30)

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		// Heart Rate
		{"GetHeartByDateRange", func() (*http.Response, error) {
			return c.GetHeartByDateRange(ctx, "2024-06-01", date2Var)
		}},
		{"GetHeartByDateIntraday", func() (*http.Response, error) {
			return c.GetHeartByDateIntraday(ctx, "2024-06-15", "1min")
		}},
		{"GetHeartByDatePeriod", func() (*http.Response, error) {
			return c.GetHeartByDatePeriod(ctx, "2024-06-15", "1w")
		}},
		{"GetHeartByDateTimestampIntraday", func() (*http.Response, error) {
			return c.GetHeartByDateTimestampIntraday(ctx, "2024-06-15", "1min", "00:00", "23:59")
		}},
		{"GetHeartByDateRangeIntraday", func() (*http.Response, error) {
			return c.GetHeartByDateRangeIntraday(ctx, "2024-06-01", "2024-06-30", "1min")
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
			return c.GetActivitiesTrackerResourceByDateRange(ctx, "steps", "2024-06-01", date2Var)
		}},
		{"GetActivitiesTrackerResourceByDatePeriod", func() (*http.Response, error) {
			return c.GetActivitiesTrackerResourceByDatePeriod(ctx, "steps", "2024-06-01", "1w")
		}},
		{"DeleteActivitiesLog", func() (*http.Response, error) {
			return c.DeleteActivitiesLog(ctx, 123)
		}},
		{"GetActivitiesResourceByDateRange", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDateRange(ctx, "steps", "2024-06-01", date2Var)
		}},
		{"GetActivitiesResourceByDatePeriod", func() (*http.Response, error) {
			return c.GetActivitiesResourceByDatePeriod(ctx, "steps", "2024-06-01", "1w")
		}},
		{"GetBadges", func() (*http.Response, error) {
			return c.GetBadges(ctx)
		}},
		// AZM (Active Zone Minutes)
		{"GetAZMTimeSeriesByDate", func() (*http.Response, error) {
			return c.GetAZMTimeSeriesByDate(ctx, "2024-06-01", "1w")
		}},
		{"GetAZMTimeSeriesByInterval", func() (*http.Response, error) {
			return c.GetAZMTimeSeriesByInterval(ctx, "2024-06-01", "2024-06-30")
		}},
		{"GetAZMByDateTimeSeriesIntraday", func() (*http.Response, error) {
			return c.GetAZMByDateTimeSeriesIntraday(ctx, "2024-06-15", "1min", "00:00", "23:59")
		}},
		{"GetAZMByIntervalTimeSeriesIntraday", func() (*http.Response, error) {
			return c.GetAZMByIntervalTimeSeriesIntraday(ctx, "2024-06-01", "2024-06-30", "00:00", "23:59", "1min")
		}},
		{"GetAZMByIntervalIntraday", func() (*http.Response, error) {
			return c.GetAZMByIntervalIntraday(ctx, "2024-06-01", "2024-06-30", "1min")
		}},
		// Body
		{"AddBodyFatLog", func() (*http.Response, error) {
			p := &fitbit.AddBodyFatLogParams{Date: date, Fat: 20}
			return c.AddBodyFatLog(ctx, p)
		}},
		{"GetBodyFatByDateRange", func() (*http.Response, error) {
			return c.GetBodyFatByDateRange(ctx, "2024-06-01", date2Var)
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
			return c.GetBodyResourceByDateRange(ctx, "weight", "2024-06-01", date2Var)
		}},
		{"GetBodyResourceByDatePeriod", func() (*http.Response, error) {
			return c.GetBodyResourceByDatePeriod(ctx, "weight", "2024-06-15", "1m")
		}},
		// Weight
		{"AddWeightLog", func() (*http.Response, error) {
			p := &fitbit.AddWeightLogParams{Date: date, Weight: 75}
			return c.AddWeightLog(ctx, p)
		}},
		{"GetWeightByDateRange", func() (*http.Response, error) {
			return c.GetWeightByDateRange(ctx, "2024-06-01", date2Var)
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
			return c.GetBreathingRateSummaryByDate(ctx, "2024-06-15")
		}},
		{"GetBreathingRateIntradayByDate", func() (*http.Response, error) {
			return c.GetBreathingRateIntradayByDate(ctx, "2024-06-15")
		}},
		{"GetBreathingRateSummaryByInterval", func() (*http.Response, error) {
			return c.GetBreathingRateSummaryByInterval(ctx, "2024-06-01", "2024-06-30")
		}},
		{"GetBreathingRateIntradayByInterval", func() (*http.Response, error) {
			return c.GetBreathingRateIntradayByInterval(ctx, "2024-06-01", "2024-06-30")
		}},
		// VO2Max
		{"GetVo2MaxSummaryByDate", func() (*http.Response, error) {
			return c.GetVo2MaxSummaryByDate(ctx, "2024-06-15")
		}},
		{"GetVo2MaxSummaryByInterval", func() (*http.Response, error) {
			return c.GetVo2MaxSummaryByInterval(ctx, "2024-06-01", "2024-06-30")
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
			return c.GetFoodsByDateRange(ctx, "calories", "2024-06-01", date2Var)
		}},
		{"GetFoodsResourceByDatePeriod", func() (*http.Response, error) {
			return c.GetFoodsResourceByDatePeriod(ctx, "calories", "2024-06-01", "1m")
		}},
		// Water
		{"AddWaterLog", func() (*http.Response, error) {
			p := &fitbit.AddWaterLogParams{Amount: 500, Date: date}
			return c.AddWaterLog(ctx, p)
		}},
		{"GetWaterByDate", func() (*http.Response, error) {
			return c.GetWaterByDate(ctx, date)
		}},
		{"GetWaterGoal", func() (*http.Response, error) {
			return c.GetWaterGoal(ctx)
		}},
		// HRV
		{"GetHrvSummaryDate", func() (*http.Response, error) {
			return c.GetHrvSummaryDate(ctx, "2024-06-15")
		}},
		{"GetHrvIntradayByDate", func() (*http.Response, error) {
			return c.GetHrvIntradayByDate(ctx, "2024-06-15")
		}},
		{"GetHrvSummaryInterval", func() (*http.Response, error) {
			return c.GetHrvSummaryInterval(ctx, "2024-06-01", "2024-06-30")
		}},
		{"GetHrvIntradayByInterval", func() (*http.Response, error) {
			return c.GetHrvIntradayByInterval(ctx, "2024-06-01", "2024-06-30")
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
		{"GetMeal", func() (*http.Response, error) {
			return c.GetMeal(ctx, "meal-123")
		}},
		// Profile
		{"GetProfile", func() (*http.Response, error) {
			return c.GetProfile(ctx)
		}},
		// SpO2
		{"GetSpO2SummaryByDate", func() (*http.Response, error) {
			return c.GetSpO2SummaryByDate(ctx, "2024-06-15")
		}},
		{"GetSpO2IntradayByDate", func() (*http.Response, error) {
			return c.GetSpO2IntradayByDate(ctx, "2024-06-15")
		}},
		{"GetSpO2SummaryByInterval", func() (*http.Response, error) {
			return c.GetSpO2SummaryByInterval(ctx, "2024-06-01", "2024-06-30")
		}},
		{"GetSpO2IntradayByInterval", func() (*http.Response, error) {
			return c.GetSpO2IntradayByInterval(ctx, "2024-06-01", "2024-06-30")
		}},
		// Temperature
		{"GetTempCoreSummaryByDate", func() (*http.Response, error) {
			return c.GetTempCoreSummaryByDate(ctx, "2024-06-15")
		}},
		{"GetTempCoreSummaryByInterval", func() (*http.Response, error) {
			return c.GetTempCoreSummaryByInterval(ctx, "2024-06-01", "2024-06-30")
		}},
		{"GetTempSkinSummaryDate", func() (*http.Response, error) {
			return c.GetTempSkinSummaryDate(ctx, "2024-06-15")
		}},
		{"GetTempSkinSummaryByInterval", func() (*http.Response, error) {
			return c.GetTempSkinSummaryByInterval(ctx, "2024-06-01", "2024-06-30")
		}},
		// Subscriptions
		{"GetSubscriptionsList", func() (*http.Response, error) {
			return c.GetSubscriptionsList(ctx, "activities")
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

func TestFitbitNewClientWithResponses(t *testing.T) {
	srv := allMethodsServer()
	defer srv.Close()

	c, err := fitbit.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestFitbitActivitiesTCX(t *testing.T) {
	srv := allMethodsServer()
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	p := &fitbit.GetActivitiesTCXParams{}
	resp, err := c.GetActivitiesTCX(context.Background(), "log-123", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitDeleteFavoriteActivities(t *testing.T) {
	srv := allMethodsServer()
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.DeleteFavoriteActivities(context.Background(), "activity-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFitbitAddFavoriteActivities(t *testing.T) {
	srv := allMethodsServer()
	defer srv.Close()

	c, _ := fitbit.NewClient(srv.URL)
	resp, err := c.AddFavoriteActivities(context.Background(), "activity-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
