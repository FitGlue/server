package oura_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/oura"
)

func ouraAllServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestAllOuraMethods calls ALL remaining oura client methods not covered in client_test.go.
func TestAllOuraMethods(t *testing.T) {
	srv := ouraAllServer()
	defer srv.Close()

	c, err := oura.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		// Sandbox: Daily Sleep
		{"SandboxMultipleDailySleepDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleDailySleepDocumentsV2SandboxUsercollectionDailySleepGetParams{}
			return c.SandboxMultipleDailySleepDocumentsV2SandboxUsercollectionDailySleepGet(ctx, p)
		}},
		{"SandboxSingleDailySleepDocument", func() (*http.Response, error) {
			return c.SandboxSingleDailySleepDocumentV2SandboxUsercollectionDailySleepDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: SpO2
		{"SandboxMultipleDailySpo2Documents", func() (*http.Response, error) {
			p := &oura.SandboxMultipleDailySpo2DocumentsV2SandboxUsercollectionDailySpo2GetParams{}
			return c.SandboxMultipleDailySpo2DocumentsV2SandboxUsercollectionDailySpo2Get(ctx, p)
		}},
		{"SandboxSingleDailySpo2Document", func() (*http.Response, error) {
			return c.SandboxSingleDailySpo2DocumentV2SandboxUsercollectionDailySpo2DocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Daily Stress
		{"SandboxMultipleDailyStressDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleDailyStressDocumentsV2SandboxUsercollectionDailyStressGetParams{}
			return c.SandboxMultipleDailyStressDocumentsV2SandboxUsercollectionDailyStressGet(ctx, p)
		}},
		{"SandboxSingleDailyStressDocument", func() (*http.Response, error) {
			return c.SandboxSingleDailyStressDocumentV2SandboxUsercollectionDailyStressDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Enhanced Tag
		{"SandboxMultipleEnhancedTagDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleEnhancedTagDocumentsV2SandboxUsercollectionEnhancedTagGetParams{}
			return c.SandboxMultipleEnhancedTagDocumentsV2SandboxUsercollectionEnhancedTagGet(ctx, p)
		}},
		{"SandboxSingleEnhancedTagDocument", func() (*http.Response, error) {
			return c.SandboxSingleEnhancedTagDocumentV2SandboxUsercollectionEnhancedTagDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Heartrate
		{"SandboxMultipleHeartrateDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleHeartrateDocumentsV2SandboxUsercollectionHeartrateGetParams{}
			return c.SandboxMultipleHeartrateDocumentsV2SandboxUsercollectionHeartrateGet(ctx, p)
		}},
		// Sandbox: Rest Mode Period
		{"SandboxMultipleRestModePeriodDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleRestModePeriodDocumentsV2SandboxUsercollectionRestModePeriodGetParams{}
			return c.SandboxMultipleRestModePeriodDocumentsV2SandboxUsercollectionRestModePeriodGet(ctx, p)
		}},
		{"SandboxSingleRestModePeriodDocument", func() (*http.Response, error) {
			return c.SandboxSingleRestModePeriodDocumentV2SandboxUsercollectionRestModePeriodDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Ring Configuration
		{"SandboxMultipleRingConfigurationDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleRingConfigurationDocumentsV2SandboxUsercollectionRingConfigurationGetParams{}
			return c.SandboxMultipleRingConfigurationDocumentsV2SandboxUsercollectionRingConfigurationGet(ctx, p)
		}},
		{"SandboxSingleRingConfigurationDocument", func() (*http.Response, error) {
			return c.SandboxSingleRingConfigurationDocumentV2SandboxUsercollectionRingConfigurationDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Session
		{"SandboxMultipleSessionDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleSessionDocumentsV2SandboxUsercollectionSessionGetParams{}
			return c.SandboxMultipleSessionDocumentsV2SandboxUsercollectionSessionGet(ctx, p)
		}},
		{"SandboxSingleSessionDocument", func() (*http.Response, error) {
			return c.SandboxSingleSessionDocumentV2SandboxUsercollectionSessionDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Sleep
		{"SandboxMultipleSleepDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleSleepDocumentsV2SandboxUsercollectionSleepGetParams{}
			return c.SandboxMultipleSleepDocumentsV2SandboxUsercollectionSleepGet(ctx, p)
		}},
		{"SandboxSingleSleepDocument", func() (*http.Response, error) {
			return c.SandboxSingleSleepDocumentV2SandboxUsercollectionSleepDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Sleep Time
		{"SandboxMultipleSleepTimeDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleSleepTimeDocumentsV2SandboxUsercollectionSleepTimeGetParams{}
			return c.SandboxMultipleSleepTimeDocumentsV2SandboxUsercollectionSleepTimeGet(ctx, p)
		}},
		{"SandboxSingleSleepTimeDocument", func() (*http.Response, error) {
			return c.SandboxSingleSleepTimeDocumentV2SandboxUsercollectionSleepTimeDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Tag
		{"SandboxMultipleTagDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleTagDocumentsV2SandboxUsercollectionTagGetParams{}
			return c.SandboxMultipleTagDocumentsV2SandboxUsercollectionTagGet(ctx, p)
		}},
		{"SandboxSingleTagDocument", func() (*http.Response, error) {
			return c.SandboxSingleTagDocumentV2SandboxUsercollectionTagDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: VO2Max
		{"SandboxMultipleVO2MaxDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleVO2MaxDocumentsV2SandboxUsercollectionVO2MaxGetParams{}
			return c.SandboxMultipleVO2MaxDocumentsV2SandboxUsercollectionVO2MaxGet(ctx, p)
		}},
		{"SandboxSingleVO2MaxDocument", func() (*http.Response, error) {
			return c.SandboxSingleVO2MaxDocumentV2SandboxUsercollectionVO2MaxDocumentIdGet(ctx, "doc-1")
		}},
		// Sandbox: Workout
		{"SandboxMultipleWorkoutDocuments", func() (*http.Response, error) {
			p := &oura.SandboxMultipleWorkoutDocumentsV2SandboxUsercollectionWorkoutGetParams{}
			return c.SandboxMultipleWorkoutDocumentsV2SandboxUsercollectionWorkoutGet(ctx, p)
		}},
		{"SandboxSingleWorkoutDocument", func() (*http.Response, error) {
			return c.SandboxSingleWorkoutDocumentV2SandboxUsercollectionWorkoutDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Daily Activity
		{"MultipleDailyActivityDocuments", func() (*http.Response, error) {
			p := &oura.MultipleDailyActivityDocumentsV2UsercollectionDailyActivityGetParams{}
			return c.MultipleDailyActivityDocumentsV2UsercollectionDailyActivityGet(ctx, p)
		}},
		{"SingleDailyActivityDocument", func() (*http.Response, error) {
			return c.SingleDailyActivityDocumentV2UsercollectionDailyActivityDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Cardiovascular Age
		{"MultipleDailyCardiovascularAgeDocuments", func() (*http.Response, error) {
			p := &oura.MultipleDailyCardiovascularAgeDocumentsV2UsercollectionDailyCardiovascularAgeGetParams{}
			return c.MultipleDailyCardiovascularAgeDocumentsV2UsercollectionDailyCardiovascularAgeGet(ctx, p)
		}},
		{"SingleDailyCardiovascularAgeDocument", func() (*http.Response, error) {
			return c.SingleDailyCardiovascularAgeDocumentV2UsercollectionDailyCardiovascularAgeDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Readiness
		{"MultipleDailyReadinessDocuments", func() (*http.Response, error) {
			p := &oura.MultipleDailyReadinessDocumentsV2UsercollectionDailyReadinessGetParams{}
			return c.MultipleDailyReadinessDocumentsV2UsercollectionDailyReadinessGet(ctx, p)
		}},
		{"SingleDailyReadinessDocument", func() (*http.Response, error) {
			return c.SingleDailyReadinessDocumentV2UsercollectionDailyReadinessDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Resilience
		{"MultipleDailyResilienceDocuments", func() (*http.Response, error) {
			p := &oura.MultipleDailyResilienceDocumentsV2UsercollectionDailyResilienceGetParams{}
			return c.MultipleDailyResilienceDocumentsV2UsercollectionDailyResilienceGet(ctx, p)
		}},
		{"SingleDailyResilienceDocument", func() (*http.Response, error) {
			return c.SingleDailyResilienceDocumentV2UsercollectionDailyResilienceDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Daily Sleep
		{"MultipleDailySleepDocuments", func() (*http.Response, error) {
			p := &oura.MultipleDailySleepDocumentsV2UsercollectionDailySleepGetParams{}
			return c.MultipleDailySleepDocumentsV2UsercollectionDailySleepGet(ctx, p)
		}},
		{"SingleDailySleepDocument", func() (*http.Response, error) {
			return c.SingleDailySleepDocumentV2UsercollectionDailySleepDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: SpO2
		{"MultipleDailySpo2Documents", func() (*http.Response, error) {
			p := &oura.MultipleDailySpo2DocumentsV2UsercollectionDailySpo2GetParams{}
			return c.MultipleDailySpo2DocumentsV2UsercollectionDailySpo2Get(ctx, p)
		}},
		{"SingleDailySpo2Document", func() (*http.Response, error) {
			return c.SingleDailySpo2DocumentV2UsercollectionDailySpo2DocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Daily Stress
		{"MultipleDailyStressDocuments", func() (*http.Response, error) {
			p := &oura.MultipleDailyStressDocumentsV2UsercollectionDailyStressGetParams{}
			return c.MultipleDailyStressDocumentsV2UsercollectionDailyStressGet(ctx, p)
		}},
		{"SingleDailyStressDocument", func() (*http.Response, error) {
			return c.SingleDailyStressDocumentV2UsercollectionDailyStressDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Enhanced Tag
		{"MultipleEnhancedTagDocuments", func() (*http.Response, error) {
			p := &oura.MultipleEnhancedTagDocumentsV2UsercollectionEnhancedTagGetParams{}
			return c.MultipleEnhancedTagDocumentsV2UsercollectionEnhancedTagGet(ctx, p)
		}},
		{"SingleEnhancedTagDocument", func() (*http.Response, error) {
			return c.SingleEnhancedTagDocumentV2UsercollectionEnhancedTagDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Heart Rate
		{"MultipleHeartRateDocuments", func() (*http.Response, error) {
			p := &oura.MultipleHeartRateDocumentsV2UsercollectionHeartrateGetParams{}
			return c.MultipleHeartRateDocumentsV2UsercollectionHeartrateGet(ctx, p)
		}},
		// Non-Sandbox: Personal Info
		{"SinglePersonalInfoDocument", func() (*http.Response, error) {
			return c.SinglePersonalInfoDocumentV2UsercollectionPersonalInfoGet(ctx)
		}},
		// Non-Sandbox: Rest Mode Period
		{"MultipleRestModePeriodDocuments", func() (*http.Response, error) {
			p := &oura.MultipleRestModePeriodDocumentsV2UsercollectionRestModePeriodGetParams{}
			return c.MultipleRestModePeriodDocumentsV2UsercollectionRestModePeriodGet(ctx, p)
		}},
		{"SingleRestModePeriodDocument", func() (*http.Response, error) {
			return c.SingleRestModePeriodDocumentV2UsercollectionRestModePeriodDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Ring Configuration
		{"MultipleRingConfigurationDocuments", func() (*http.Response, error) {
			p := &oura.MultipleRingConfigurationDocumentsV2UsercollectionRingConfigurationGetParams{}
			return c.MultipleRingConfigurationDocumentsV2UsercollectionRingConfigurationGet(ctx, p)
		}},
		{"SingleRingConfigurationDocument", func() (*http.Response, error) {
			return c.SingleRingConfigurationDocumentV2UsercollectionRingConfigurationDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Session
		{"MultipleSessionDocuments", func() (*http.Response, error) {
			p := &oura.MultipleSessionDocumentsV2UsercollectionSessionGetParams{}
			return c.MultipleSessionDocumentsV2UsercollectionSessionGet(ctx, p)
		}},
		{"SingleSessionDocument", func() (*http.Response, error) {
			return c.SingleSessionDocumentV2UsercollectionSessionDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Sleep
		{"MultipleSleepDocuments", func() (*http.Response, error) {
			p := &oura.MultipleSleepDocumentsV2UsercollectionSleepGetParams{}
			return c.MultipleSleepDocumentsV2UsercollectionSleepGet(ctx, p)
		}},
		{"SingleSleepDocument", func() (*http.Response, error) {
			return c.SingleSleepDocumentV2UsercollectionSleepDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Sleep Time
		{"MultipleSleepTimeDocuments", func() (*http.Response, error) {
			p := &oura.MultipleSleepTimeDocumentsV2UsercollectionSleepTimeGetParams{}
			return c.MultipleSleepTimeDocumentsV2UsercollectionSleepTimeGet(ctx, p)
		}},
		{"SingleSleepTimeDocument", func() (*http.Response, error) {
			return c.SingleSleepTimeDocumentV2UsercollectionSleepTimeDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Tag
		{"MultipleTagDocuments", func() (*http.Response, error) {
			p := &oura.MultipleTagDocumentsV2UsercollectionTagGetParams{}
			return c.MultipleTagDocumentsV2UsercollectionTagGet(ctx, p)
		}},
		{"SingleTagDocument", func() (*http.Response, error) {
			return c.SingleTagDocumentV2UsercollectionTagDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: VO2Max
		{"MultipleVO2MaxDocuments", func() (*http.Response, error) {
			p := &oura.MultipleVO2MaxDocumentsV2UsercollectionVO2MaxGetParams{}
			return c.MultipleVO2MaxDocumentsV2UsercollectionVO2MaxGet(ctx, p)
		}},
		{"SingleVO2MaxDocument", func() (*http.Response, error) {
			return c.SingleVO2MaxDocumentV2UsercollectionVO2MaxDocumentIdGet(ctx, "doc-1")
		}},
		// Non-Sandbox: Workout
		{"MultipleWorkoutDocuments", func() (*http.Response, error) {
			p := &oura.MultipleWorkoutDocumentsV2UsercollectionWorkoutGetParams{}
			return c.MultipleWorkoutDocumentsV2UsercollectionWorkoutGet(ctx, p)
		}},
		{"SingleWorkoutDocument", func() (*http.Response, error) {
			return c.SingleWorkoutDocumentV2UsercollectionWorkoutDocumentIdGet(ctx, "doc-1")
		}},
		// Webhooks
		{"ListWebhookSubscriptions", func() (*http.Response, error) {
			return c.ListWebhookSubscriptionsV2WebhookSubscriptionGet(ctx)
		}},
		{"GetWebhookSubscription", func() (*http.Response, error) {
			return c.GetWebhookSubscriptionV2WebhookSubscriptionIdGet(ctx, "sub-1")
		}},
		{"DeleteWebhookSubscription", func() (*http.Response, error) {
			return c.DeleteWebhookSubscriptionV2WebhookSubscriptionIdDelete(ctx, "sub-1")
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

func TestOuraNewClientWithResponses(t *testing.T) {
	srv := ouraAllServer()
	defer srv.Close()

	c, err := oura.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
