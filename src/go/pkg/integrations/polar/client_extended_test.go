package polar_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/fitglue/server/src/go/pkg/integrations/polar"
)

func polarAllServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

func mkPolarDate() openapi_types.Date {
	return openapi_types.Date{Time: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)}
}

// TestAllPolarMethods calls ALL remaining polar client methods not covered in client_test.go.
func TestAllPolarMethods(t *testing.T) {
	srv := polarAllServer()
	defer srv.Close()

	c, err := polar.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	d := mkPolarDate()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		// Activities
		{"ListActivitiesWithoutTransaction", func() (*http.Response, error) {
			p := &polar.ListActivitiesWithoutTransactionParams{}
			return c.ListActivitiesWithoutTransaction(ctx, p)
		}},
		{"ListActivitiesWithoutTransactionByDateRange", func() (*http.Response, error) {
			p := &polar.ListActivitiesWithoutTransactionByDateRangeParams{}
			return c.ListActivitiesWithoutTransactionByDateRange(ctx, p)
		}},
		{"GetActivityWithoutTransaction", func() (*http.Response, error) {
			p := &polar.GetActivityWithoutTransactionParams{}
			return c.GetActivityWithoutTransaction(ctx, d, p)
		}},
		// Activity Samples (without transaction)
		{"ListActivitySamplesWithoutTransaction", func() (*http.Response, error) {
			return c.ListActivitySamplesWithoutTransaction(ctx)
		}},
		{"ListActivitySamplesWithoutTransactionByDateRange", func() (*http.Response, error) {
			p := &polar.ListActivitySamplesWithoutTransactionByDateRangeParams{}
			return c.ListActivitySamplesWithoutTransactionByDateRange(ctx, p)
		}},
		{"GetActivitySamplesWithoutTransaction", func() (*http.Response, error) {
			return c.GetActivitySamplesWithoutTransaction(ctx, d)
		}},
		// User
		{"RegisterUser", func() (*http.Response, error) {
			body := polar.RegisterUserJSONRequestBody{MemberId: "member-1"}
			return c.RegisterUser(ctx, body)
		}},
		{"DeleteUser", func() (*http.Response, error) {
			return c.DeleteUser(ctx, 12345)
		}},
		{"GetUserInformation", func() (*http.Response, error) {
			return c.GetUserInformation(ctx, 99999)
		}},
		// Biosensing (no user ID param — takes params struct only)
		{"GetV3UsersBiosensingBodytemperature", func() (*http.Response, error) {
			p := &polar.GetV3UsersBiosensingBodytemperatureParams{}
			return c.GetV3UsersBiosensingBodytemperature(ctx, p)
		}},
		{"GetV3UsersBiosensingEcg", func() (*http.Response, error) {
			p := &polar.GetV3UsersBiosensingEcgParams{}
			return c.GetV3UsersBiosensingEcg(ctx, p)
		}},
		{"GetV3UsersBiosensingSkincontacts", func() (*http.Response, error) {
			p := &polar.GetV3UsersBiosensingSkincontactsParams{}
			return c.GetV3UsersBiosensingSkincontacts(ctx, p)
		}},
		{"GetV3UsersBiosensingSkintemperature", func() (*http.Response, error) {
			p := &polar.GetV3UsersBiosensingSkintemperatureParams{}
			return c.GetV3UsersBiosensingSkintemperature(ctx, p)
		}},
		{"GetV3UsersBiosensingSpo2", func() (*http.Response, error) {
			p := &polar.GetV3UsersBiosensingSpo2Params{}
			return c.GetV3UsersBiosensingSpo2(ctx, p)
		}},
		// Cardio Load
		{"GetV3UsersCardioLoad", func() (*http.Response, error) {
			return c.GetV3UsersCardioLoad(ctx)
		}},
		{"GetCardioLoadByDateRange", func() (*http.Response, error) {
			p := &polar.GetCardioLoadByDateRangeParams{}
			return c.GetCardioLoadByDateRange(ctx, p)
		}},
		{"GetV3UsersCardioLoadDate", func() (*http.Response, error) {
			return c.GetV3UsersCardioLoadDate(ctx, d)
		}},
		// Continuous Heart Rate
		{"GetV3UsersContinuousHeartRate", func() (*http.Response, error) {
			p := &polar.GetV3UsersContinuousHeartRateParams{}
			return c.GetV3UsersContinuousHeartRate(ctx, p)
		}},
		{"GetV3UsersContinuousHeartRateDate", func() (*http.Response, error) {
			return c.GetV3UsersContinuousHeartRateDate(ctx, d)
		}},
		// Nightly Recharge
		{"ListNightlyRecharge", func() (*http.Response, error) {
			return c.ListNightlyRecharge(ctx)
		}},
		{"GetV3UsersNightlyRechargeDate", func() (*http.Response, error) {
			return c.GetV3UsersNightlyRechargeDate(ctx, "2024-06-15")
		}},
		// Sleep
		{"ListNights", func() (*http.Response, error) {
			return c.ListNights(ctx)
		}},
		{"GetV3UsersSleepAvailable", func() (*http.Response, error) {
			return c.GetV3UsersSleepAvailable(ctx)
		}},
		{"GetV3UsersSleepDate", func() (*http.Response, error) {
			return c.GetV3UsersSleepDate(ctx, "2024-06-15")
		}},
		// Sleepwise
		{"GetV3UsersSleepwiseAlertness", func() (*http.Response, error) {
			return c.GetV3UsersSleepwiseAlertness(ctx)
		}},
		{"GetV3UsersSleepwiseAlertnessDate", func() (*http.Response, error) {
			p := &polar.GetV3UsersSleepwiseAlertnessDateParams{}
			return c.GetV3UsersSleepwiseAlertnessDate(ctx, p)
		}},
		{"GetV3UsersSleepwiseCircadianBedtime", func() (*http.Response, error) {
			return c.GetV3UsersSleepwiseCircadianBedtime(ctx)
		}},
		{"GetV3UsersSleepwiseCircadianBedtimeDate", func() (*http.Response, error) {
			p := &polar.GetV3UsersSleepwiseCircadianBedtimeDateParams{}
			return c.GetV3UsersSleepwiseCircadianBedtimeDate(ctx, p)
		}},
		// Activity Transaction API
		{"CreateActivityTransaction", func() (*http.Response, error) {
			return c.CreateActivityTransaction(ctx, 1001)
		}},
		{"ListActivities", func() (*http.Response, error) {
			return c.ListActivities(ctx, 1001, 12345)
		}},
		{"CommitActivityTransaction", func() (*http.Response, error) {
			return c.CommitActivityTransaction(ctx, 1001, 12345)
		}},
		{"GetActivitySummary", func() (*http.Response, error) {
			return c.GetActivitySummary(ctx, 1001, 12345, 9001)
		}},
		{"GetStepSamples", func() (*http.Response, error) {
			return c.GetStepSamples(ctx, 1001, 12345, 9001)
		}},
		{"GetZoneSamples", func() (*http.Response, error) {
			return c.GetZoneSamples(ctx, 1001, 12345, 9001)
		}},
		// Exercise Transaction API
		{"CreateExerciseTransaction", func() (*http.Response, error) {
			return c.CreateExerciseTransaction(ctx, 1001)
		}},
		{"ListExercises", func() (*http.Response, error) {
			return c.ListExercises(ctx, 1001, 12345)
		}},
		{"CommitExerciseTransaction", func() (*http.Response, error) {
			return c.CommitExerciseTransaction(ctx, 1001, 12345)
		}},
		{"GetExerciseSummary", func() (*http.Response, error) {
			return c.GetExerciseSummary(ctx, 1001, 12345, 8001)
		}},
		{"GetFit", func() (*http.Response, error) {
			return c.GetFit(ctx, 1001, 12345, 8001)
		}},
		{"GetGpx", func() (*http.Response, error) {
			p := &polar.GetGpxParams{}
			return c.GetGpx(ctx, 1001, 12345, 8001, p)
		}},
		{"GetHeartRateZones", func() (*http.Response, error) {
			return c.GetHeartRateZones(ctx, 1001, 12345, 8001)
		}},
		{"GetAvailableSamples", func() (*http.Response, error) {
			return c.GetAvailableSamples(ctx, 1001, 12345, 8001)
		}},
		{"GetTcx", func() (*http.Response, error) {
			return c.GetTcx(ctx, 1001, 12345, 8001)
		}},
		// Physical Info Transaction
		{"CreatePhysicalInfoTransaction", func() (*http.Response, error) {
			return c.CreatePhysicalInfoTransaction(ctx, 1001)
		}},
		{"ListPhysicalInfos", func() (*http.Response, error) {
			return c.ListPhysicalInfos(ctx, 1001, 12345)
		}},
		{"CommitPhysicalInfoTransaction", func() (*http.Response, error) {
			return c.CommitPhysicalInfoTransaction(ctx, 1001, 12345)
		}},
		{"GetPhysicalInfo", func() (*http.Response, error) {
			return c.GetPhysicalInfo(ctx, 1001, 12345, 7001)
		}},
		// Webhooks
		{"GetWebhook", func() (*http.Response, error) {
			return c.GetWebhook(ctx)
		}},
		{"PostV3WebhooksActivate", func() (*http.Response, error) {
			return c.PostV3WebhooksActivate(ctx)
		}},
		{"PostV3WebhooksDeactivate", func() (*http.Response, error) {
			return c.PostV3WebhooksDeactivate(ctx)
		}},
		{"DeleteWebhook", func() (*http.Response, error) {
			return c.DeleteWebhook(ctx, "subscription-id")
		}},
		// Cardio Load Period
		{"GetV3UsersCardioLoadPeriodDaysDays", func() (*http.Response, error) {
			return c.GetV3UsersCardioLoadPeriodDaysDays(ctx, 7)
		}},
		{"GetV3UsersCardioLoadPeriodMonthsMonths", func() (*http.Response, error) {
			return c.GetV3UsersCardioLoadPeriodMonthsMonths(ctx, 3)
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

func TestPolarNewClientWithResponses(t *testing.T) {
	srv := polarAllServer()
	defer srv.Close()

	c, err := polar.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestPolarGetSamples(t *testing.T) {
	srv := polarAllServer()
	defer srv.Close()

	c, _ := polar.NewClient(srv.URL)
	resp, err := c.GetSamples(context.Background(), 1001, 12345, 8001, []byte("heartrate"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
