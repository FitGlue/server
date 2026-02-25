package fitbit_test

import (
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

func fitbitAPIExtra3Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

func fitbitAPIExtra3Date() openapi_types.Date {
	return openapi_types.Date{Time: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)}
}

// TestFitbitAPIClientWithResponses calls ALL pkg/api/fitbit ClientWithResponses methods for code coverage.
// No error assertions — focused on exercising ParseXXXResponse functions for coverage.
// Note: pkg/api/fitbit uses openapi_types.Date for ALL date params (unlike pkg/integrations/fitbit which uses strings).
func TestFitbitAPIClientWithResponses(t *testing.T) {
	srv := fitbitAPIExtra3Server()
	defer srv.Close()

	c, err := fitbit.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}

	ctx := context.Background()
	d := fitbitAPIExtra3Date()

	// Auth
	c.IntrospectWithBodyWithResponse(ctx, "application/x-www-form-urlencoded", strings.NewReader("token=test")) //nolint:errcheck
	c.IntrospectWithFormdataBodyWithResponse(ctx, fitbit.IntrospectFormdataRequestBody{})                       //nolint:errcheck
	c.RevokeWithBodyWithResponse(ctx, "application/x-www-form-urlencoded", strings.NewReader("token=test"))     //nolint:errcheck
	c.RevokeWithFormdataBodyWithResponse(ctx, fitbit.RevokeFormdataRequestBody{})                               //nolint:errcheck
	c.OauthTokenWithResponse(ctx, &fitbit.OauthTokenParams{})                                                   //nolint:errcheck

	// Friends
	c.GetFriendsWithResponse(ctx)            //nolint:errcheck
	c.GetFriendsLeaderboardWithResponse(ctx) //nolint:errcheck

	// Sleep
	c.AddSleepWithResponse(ctx, &fitbit.AddSleepParams{})               //nolint:errcheck
	c.GetSleepByDateRangeWithResponse(ctx, d, d)                        //nolint:errcheck
	c.GetSleepByDateWithResponse(ctx, d)                                //nolint:errcheck
	c.GetSleepGoalWithResponse(ctx)                                     //nolint:errcheck
	c.UpdateSleepGoalWithResponse(ctx, &fitbit.UpdateSleepGoalParams{}) //nolint:errcheck
	c.GetSleepListWithResponse(ctx, &fitbit.GetSleepListParams{})       //nolint:errcheck
	c.DeleteSleepWithResponse(ctx, "log-1")                             //nolint:errcheck

	// Activities Types
	c.GetActivitiesTypesWithResponse(ctx)               //nolint:errcheck
	c.GetActivitiesTypeDetailWithResponse(ctx, "90009") //nolint:errcheck

	// Foods lookup
	c.GetFoodsLocalesWithResponse(ctx)                            //nolint:errcheck
	c.GetFoodsListWithResponse(ctx, &fitbit.GetFoodsListParams{}) //nolint:errcheck
	c.GetFoodsUnitsWithResponse(ctx)                              //nolint:errcheck
	c.GetFoodsInfoWithResponse(ctx, "12345")                      //nolint:errcheck

	// Activities Log
	c.GetActivitiesLogWithResponse(ctx)                                            //nolint:errcheck
	c.AddActivitiesLogWithResponse(ctx, &fitbit.AddActivitiesLogParams{})          //nolint:errcheck
	c.GetActivitiesLogListWithResponse(ctx, &fitbit.GetActivitiesLogListParams{})  //nolint:errcheck
	c.DeleteActivitiesLogWithResponse(ctx, 12345)                                  //nolint:errcheck
	c.GetActivitiesTCXWithResponse(ctx, "log-1", &fitbit.GetActivitiesTCXParams{}) //nolint:errcheck

	// AZM and Intraday
	c.GetAZMByDateIntradayWithResponse(ctx, d, fitbit.GetAZMByDateIntradayParamsDetailLevel("1min"))                                                                                            //nolint:errcheck
	c.GetActivitiesResourceByDateRangeIntradayWithResponse(ctx, fitbit.GetActivitiesResourceByDateRangeIntradayParamsResourcePath("steps"), d, d, "1min")                                       //nolint:errcheck
	c.GetActivitiesResourceByDateIntradayWithResponse(ctx, fitbit.GetActivitiesResourceByDateIntradayParamsResourcePath("steps"), d, "1min")                                                    //nolint:errcheck
	c.GetActivitiesResourceByDateTimeSeriesIntradayWithResponse(ctx, fitbit.GetActivitiesResourceByDateTimeSeriesIntradayParamsResourcePath("steps"), d, "1min", "00:00", "23:59")              //nolint:errcheck
	c.GetActivitiesResourceByDateRangeTimeSeriesIntradayWithResponse(ctx, fitbit.GetActivitiesResourceByDateRangeTimeSeriesIntradayParamsResourcePath("steps"), d, d, "1min", "00:00", "23:59") //nolint:errcheck

	// Favorite/Recent Activities
	c.DeleteFavoriteActivitiesWithResponse(ctx, "90009") //nolint:errcheck
	c.AddFavoriteActivitiesWithResponse(ctx, "90009")    //nolint:errcheck
	c.GetFrequentActivitiesWithResponse(ctx)             //nolint:errcheck
	c.GetRecentActivitiesWithResponse(ctx)               //nolint:errcheck

	// Activity Goals
	c.GetActivitiesGoalsWithResponse(ctx, "daily")                                                 //nolint:errcheck
	c.AddUpdateActivitiesGoalsWithResponse(ctx, "daily", &fitbit.AddUpdateActivitiesGoalsParams{}) //nolint:errcheck

	// Heart Rate
	c.GetHeartByDateRangeWithResponse(ctx, "2024-06-15", d)                                 //nolint:errcheck
	c.GetHeartByDateIntradayWithResponse(ctx, d, "1min")                                    //nolint:errcheck
	c.GetHeartByDateTimestampIntradayWithResponse(ctx, d, "1min", "00:00", "23:59")         //nolint:errcheck
	c.GetHeartByDateRangeIntradayWithResponse(ctx, d, d, "1min")                            //nolint:errcheck
	c.GetHeartByDateRangeTimestampIntradayWithResponse(ctx, d, d, "1min", "00:00", "23:59") //nolint:errcheck
	c.GetHeartByDatePeriodWithResponse(ctx, d, "7d")                                        //nolint:errcheck

	// Activities Tracker Resource
	c.GetActivitiesTrackerResourceByDateRangeWithResponse(ctx, fitbit.GetActivitiesTrackerResourceByDateRangeParamsResourcePath("steps"), d, d)      //nolint:errcheck
	c.GetActivitiesTrackerResourceByDatePeriodWithResponse(ctx, fitbit.GetActivitiesTrackerResourceByDatePeriodParamsResourcePath("steps"), d, "7d") //nolint:errcheck

	// Activities Resource
	c.GetActivitiesResourceByDateRangeWithResponse(ctx, fitbit.GetActivitiesResourceByDateRangeParamsResourcePath("steps"), d, d)      //nolint:errcheck
	c.GetActivitiesResourceByDatePeriodWithResponse(ctx, fitbit.GetActivitiesResourceByDatePeriodParamsResourcePath("steps"), d, "7d") //nolint:errcheck

	// Badges
	c.GetBadgesWithResponse(ctx) //nolint:errcheck

	// Body Fat
	c.AddBodyFatLogWithResponse(ctx, &fitbit.AddBodyFatLogParams{})         //nolint:errcheck
	c.GetBodyFatByDateRangeWithResponse(ctx, d, d)                          //nolint:errcheck
	c.GetBodyFatByDateWithResponse(ctx, d)                                  //nolint:errcheck
	c.GetBodyFatByDatePeriodWithResponse(ctx, d, "7d")                      //nolint:errcheck
	c.UpdateBodyFatGoalWithResponse(ctx, &fitbit.UpdateBodyFatGoalParams{}) //nolint:errcheck
	c.DeleteBodyFatLogWithResponse(ctx, 12345)                              //nolint:errcheck

	// Weight
	c.AddWeightLogWithResponse(ctx, &fitbit.AddWeightLogParams{})                                                   //nolint:errcheck
	c.GetWeightByDateRangeWithResponse(ctx, d, d)                                                                   //nolint:errcheck
	c.GetWeightByDateWithResponse(ctx, d)                                                                           //nolint:errcheck
	c.GetWeightByDatePeriodWithResponse(ctx, d, "7d")                                                               //nolint:errcheck
	c.UpdateWeightGoalWithResponse(ctx, &fitbit.UpdateWeightGoalParams{})                                           //nolint:errcheck
	c.DeleteWeightLogWithResponse(ctx, 12345)                                                                       //nolint:errcheck
	c.GetBodyGoalsWithResponse(ctx, "weight")                                                                       //nolint:errcheck
	c.GetBodyResourceByDateRangeWithResponse(ctx, fitbit.GetBodyResourceByDateRangeParamsResourcePath("bmi"), d, d) //nolint:errcheck

	// Alarms
	c.AddAlarmsWithResponse(ctx, 12345, &fitbit.AddAlarmsParams{})           //nolint:errcheck
	c.DeleteAlarmsWithResponse(ctx, 12345, 99)                               //nolint:errcheck
	c.UpdateAlarmsWithResponse(ctx, 12345, 99, &fitbit.UpdateAlarmsParams{}) //nolint:errcheck

	// Foods CRUD
	c.AddFoodsWithResponse(ctx, &fitbit.AddFoodsParams{})                                                                         //nolint:errcheck
	c.AddFoodsLogWithResponse(ctx, &fitbit.AddFoodsLogParams{})                                                                   //nolint:errcheck
	c.DeleteFavoriteFoodWithResponse(ctx, "food-1")                                                                               //nolint:errcheck
	c.AddFavoriteFoodWithResponse(ctx, "food-1")                                                                                  //nolint:errcheck
	c.GetFrequentFoodsWithResponse(ctx)                                                                                           //nolint:errcheck
	c.GetFoodsGoalWithResponse(ctx)                                                                                               //nolint:errcheck
	c.AddUpdateFoodsGoalWithResponse(ctx, &fitbit.AddUpdateFoodsGoalParams{})                                                     //nolint:errcheck
	c.GetRecentFoodsWithResponse(ctx)                                                                                             //nolint:errcheck
	c.DeleteFoodsLogWithResponse(ctx, "log-1")                                                                                    //nolint:errcheck
	c.EditFoodsLogWithResponse(ctx, "log-1", &fitbit.EditFoodsLogParams{})                                                        //nolint:errcheck
	c.GetFoodsByDateRangeWithResponse(ctx, fitbit.GetFoodsByDateRangeParamsResourcePath("caloriesIn"), d, d)                      //nolint:errcheck
	c.GetFoodsResourceByDatePeriodWithResponse(ctx, fitbit.GetFoodsResourceByDatePeriodParamsResourcePath("caloriesIn"), d, "7d") //nolint:errcheck
	c.DeleteFoodsWithResponse(ctx, "food-1")                                                                                      //nolint:errcheck

	// Water
	c.AddWaterLogWithResponse(ctx, &fitbit.AddWaterLogParams{})                  //nolint:errcheck
	c.GetWaterByDateWithResponse(ctx, d)                                         //nolint:errcheck
	c.GetWaterGoalWithResponse(ctx)                                              //nolint:errcheck
	c.AddUpdateWaterGoalWithResponse(ctx, &fitbit.AddUpdateWaterGoalParams{})    //nolint:errcheck
	c.DeleteWaterLogWithResponse(ctx, "water-1")                                 //nolint:errcheck
	c.UpdateWaterLogWithResponse(ctx, "water-1", &fitbit.UpdateWaterLogParams{}) //nolint:errcheck

	// HRV
	c.GetHrvSummaryDateWithResponse(ctx, d)           //nolint:errcheck
	c.GetHrvIntradayByDateWithResponse(ctx, d)        //nolint:errcheck
	c.GetHrvSummaryIntervalWithResponse(ctx, d, d)    //nolint:errcheck
	c.GetHrvIntradayByIntervalWithResponse(ctx, d, d) //nolint:errcheck

	// IRN
	c.GetIrnAlertsListWithResponse(ctx, &fitbit.GetIrnAlertsListParams{}) //nolint:errcheck
	c.GetIrnProfileWithResponse(ctx)                                      //nolint:errcheck

	// Meals
	c.GetMealsWithResponse(ctx)                                                                                //nolint:errcheck
	c.AddMealWithBodyWithResponse(ctx, "application/json", io.NopCloser(strings.NewReader("{}")))              //nolint:errcheck
	c.AddMealWithResponse(ctx, fitbit.AddMealJSONRequestBody{})                                                //nolint:errcheck
	c.DeleteMealWithResponse(ctx, "meal-1")                                                                    //nolint:errcheck
	c.UpdateMealWithBodyWithResponse(ctx, "meal-1", "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.UpdateMealWithResponse(ctx, "meal-1", fitbit.UpdateMealJSONRequestBody{})                                //nolint:errcheck

	// Profile
	c.GetProfileWithResponse(ctx) //nolint:errcheck

	// SpO2
	c.GetSpO2SummaryByDateWithResponse(ctx, d)         //nolint:errcheck
	c.GetSpO2IntradayByDateWithResponse(ctx, d)        //nolint:errcheck
	c.GetSpO2SummaryByIntervalWithResponse(ctx, d, d)  //nolint:errcheck
	c.GetSpO2IntradayByIntervalWithResponse(ctx, d, d) //nolint:errcheck

	// Temperature
	c.GetTempCoreSummaryByDateWithResponse(ctx, d)        //nolint:errcheck
	c.GetTempCoreSummaryByIntervalWithResponse(ctx, d, d) //nolint:errcheck
	c.GetTempSkinSummaryDateWithResponse(ctx, d)          //nolint:errcheck
	c.GetTempSkinSummaryByIntervalWithResponse(ctx, d, d) //nolint:errcheck

	// Subscriptions
	c.GetSubscriptionsListWithResponse(ctx, "activities") //nolint:errcheck

	t.Logf("All fitbit api ClientWithResponses methods exercised for coverage")
}
