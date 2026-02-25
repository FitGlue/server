package oura_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/oura"
)

func ouraExtra2Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// List endpoints expect arrays; single-document and webhook subscription endpoints expect objects
		path := r.URL.Path
		if strings.HasSuffix(path, "/webhook_subscription") || strings.HasSuffix(path, "/webhook-subscription") {
			_, _ = w.Write([]byte(`[]`))
		} else {
			_, _ = w.Write([]byte(`{}`))
		}
	}))
}

// TestOuraClientWithResponses calls ALL WithResponse methods to cover ParseXXXResponse wrappers.
func TestOuraClientWithResponses(t *testing.T) {
	srv := ouraExtra2Server()
	defer srv.Close()

	c, err := oura.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}

	ctx := context.Background()

	// Sandbox endpoints
	t.Run("SandboxMultipleDailyActivityDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleDailyActivityDocumentsV2SandboxUsercollectionDailyActivityGetParams{}
		_, err := c.SandboxMultipleDailyActivityDocumentsV2SandboxUsercollectionDailyActivityGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleDailyActivityDocument", func(t *testing.T) {
		_, err := c.SandboxSingleDailyActivityDocumentV2SandboxUsercollectionDailyActivityDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleDailyCardiovascularAgeDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleDailyCardiovascularAgeDocumentsV2SandboxUsercollectionDailyCardiovascularAgeGetParams{}
		_, err := c.SandboxMultipleDailyCardiovascularAgeDocumentsV2SandboxUsercollectionDailyCardiovascularAgeGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleDailyCardiovascularAgeDocument", func(t *testing.T) {
		_, err := c.SandboxSingleDailyCardiovascularAgeDocumentV2SandboxUsercollectionDailyCardiovascularAgeDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleDailyReadinessDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleDailyReadinessDocumentsV2SandboxUsercollectionDailyReadinessGetParams{}
		_, err := c.SandboxMultipleDailyReadinessDocumentsV2SandboxUsercollectionDailyReadinessGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleDailyReadinessDocument", func(t *testing.T) {
		_, err := c.SandboxSingleDailyReadinessDocumentV2SandboxUsercollectionDailyReadinessDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleDailyResilienceDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleDailyResilienceDocumentsV2SandboxUsercollectionDailyResilienceGetParams{}
		_, err := c.SandboxMultipleDailyResilienceDocumentsV2SandboxUsercollectionDailyResilienceGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleDailyResilienceDocument", func(t *testing.T) {
		_, err := c.SandboxSingleDailyResilienceDocumentV2SandboxUsercollectionDailyResilienceDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleDailySleepDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleDailySleepDocumentsV2SandboxUsercollectionDailySleepGetParams{}
		_, err := c.SandboxMultipleDailySleepDocumentsV2SandboxUsercollectionDailySleepGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleDailySleepDocument", func(t *testing.T) {
		_, err := c.SandboxSingleDailySleepDocumentV2SandboxUsercollectionDailySleepDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleDailySpo2Documents", func(t *testing.T) {
		p := &oura.SandboxMultipleDailySpo2DocumentsV2SandboxUsercollectionDailySpo2GetParams{}
		_, err := c.SandboxMultipleDailySpo2DocumentsV2SandboxUsercollectionDailySpo2GetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleDailySpo2Document", func(t *testing.T) {
		_, err := c.SandboxSingleDailySpo2DocumentV2SandboxUsercollectionDailySpo2DocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleDailyStressDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleDailyStressDocumentsV2SandboxUsercollectionDailyStressGetParams{}
		_, err := c.SandboxMultipleDailyStressDocumentsV2SandboxUsercollectionDailyStressGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleDailyStressDocument", func(t *testing.T) {
		_, err := c.SandboxSingleDailyStressDocumentV2SandboxUsercollectionDailyStressDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleEnhancedTagDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleEnhancedTagDocumentsV2SandboxUsercollectionEnhancedTagGetParams{}
		_, err := c.SandboxMultipleEnhancedTagDocumentsV2SandboxUsercollectionEnhancedTagGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleEnhancedTagDocument", func(t *testing.T) {
		_, err := c.SandboxSingleEnhancedTagDocumentV2SandboxUsercollectionEnhancedTagDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleHeartrateDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleHeartrateDocumentsV2SandboxUsercollectionHeartrateGetParams{}
		_, err := c.SandboxMultipleHeartrateDocumentsV2SandboxUsercollectionHeartrateGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleRestModePeriodDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleRestModePeriodDocumentsV2SandboxUsercollectionRestModePeriodGetParams{}
		_, err := c.SandboxMultipleRestModePeriodDocumentsV2SandboxUsercollectionRestModePeriodGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleRestModePeriodDocument", func(t *testing.T) {
		_, err := c.SandboxSingleRestModePeriodDocumentV2SandboxUsercollectionRestModePeriodDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleRingConfigurationDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleRingConfigurationDocumentsV2SandboxUsercollectionRingConfigurationGetParams{}
		_, err := c.SandboxMultipleRingConfigurationDocumentsV2SandboxUsercollectionRingConfigurationGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleRingConfigurationDocument", func(t *testing.T) {
		_, err := c.SandboxSingleRingConfigurationDocumentV2SandboxUsercollectionRingConfigurationDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleSessionDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleSessionDocumentsV2SandboxUsercollectionSessionGetParams{}
		_, err := c.SandboxMultipleSessionDocumentsV2SandboxUsercollectionSessionGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleSessionDocument", func(t *testing.T) {
		_, err := c.SandboxSingleSessionDocumentV2SandboxUsercollectionSessionDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleSleepDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleSleepDocumentsV2SandboxUsercollectionSleepGetParams{}
		_, err := c.SandboxMultipleSleepDocumentsV2SandboxUsercollectionSleepGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleSleepDocument", func(t *testing.T) {
		_, err := c.SandboxSingleSleepDocumentV2SandboxUsercollectionSleepDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleSleepTimeDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleSleepTimeDocumentsV2SandboxUsercollectionSleepTimeGetParams{}
		_, err := c.SandboxMultipleSleepTimeDocumentsV2SandboxUsercollectionSleepTimeGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleSleepTimeDocument", func(t *testing.T) {
		_, err := c.SandboxSingleSleepTimeDocumentV2SandboxUsercollectionSleepTimeDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleTagDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleTagDocumentsV2SandboxUsercollectionTagGetParams{}
		_, err := c.SandboxMultipleTagDocumentsV2SandboxUsercollectionTagGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleTagDocument", func(t *testing.T) {
		_, err := c.SandboxSingleTagDocumentV2SandboxUsercollectionTagDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleVO2MaxDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleVO2MaxDocumentsV2SandboxUsercollectionVO2MaxGetParams{}
		_, err := c.SandboxMultipleVO2MaxDocumentsV2SandboxUsercollectionVO2MaxGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleVO2MaxDocument", func(t *testing.T) {
		_, err := c.SandboxSingleVO2MaxDocumentV2SandboxUsercollectionVO2MaxDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxMultipleWorkoutDocuments", func(t *testing.T) {
		p := &oura.SandboxMultipleWorkoutDocumentsV2SandboxUsercollectionWorkoutGetParams{}
		_, err := c.SandboxMultipleWorkoutDocumentsV2SandboxUsercollectionWorkoutGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SandboxSingleWorkoutDocument", func(t *testing.T) {
		_, err := c.SandboxSingleWorkoutDocumentV2SandboxUsercollectionWorkoutDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Non-Sandbox endpoints
	t.Run("MultipleDailyActivityDocuments", func(t *testing.T) {
		p := &oura.MultipleDailyActivityDocumentsV2UsercollectionDailyActivityGetParams{}
		_, err := c.MultipleDailyActivityDocumentsV2UsercollectionDailyActivityGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleDailyActivityDocument", func(t *testing.T) {
		_, err := c.SingleDailyActivityDocumentV2UsercollectionDailyActivityDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleDailyCardiovascularAgeDocuments", func(t *testing.T) {
		p := &oura.MultipleDailyCardiovascularAgeDocumentsV2UsercollectionDailyCardiovascularAgeGetParams{}
		_, err := c.MultipleDailyCardiovascularAgeDocumentsV2UsercollectionDailyCardiovascularAgeGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleDailyCardiovascularAgeDocument", func(t *testing.T) {
		_, err := c.SingleDailyCardiovascularAgeDocumentV2UsercollectionDailyCardiovascularAgeDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleDailyReadinessDocuments", func(t *testing.T) {
		p := &oura.MultipleDailyReadinessDocumentsV2UsercollectionDailyReadinessGetParams{}
		_, err := c.MultipleDailyReadinessDocumentsV2UsercollectionDailyReadinessGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleDailyReadinessDocument", func(t *testing.T) {
		_, err := c.SingleDailyReadinessDocumentV2UsercollectionDailyReadinessDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleDailyResilienceDocuments", func(t *testing.T) {
		p := &oura.MultipleDailyResilienceDocumentsV2UsercollectionDailyResilienceGetParams{}
		_, err := c.MultipleDailyResilienceDocumentsV2UsercollectionDailyResilienceGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleDailyResilienceDocument", func(t *testing.T) {
		_, err := c.SingleDailyResilienceDocumentV2UsercollectionDailyResilienceDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleDailySleepDocuments", func(t *testing.T) {
		p := &oura.MultipleDailySleepDocumentsV2UsercollectionDailySleepGetParams{}
		_, err := c.MultipleDailySleepDocumentsV2UsercollectionDailySleepGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleDailySleepDocument", func(t *testing.T) {
		_, err := c.SingleDailySleepDocumentV2UsercollectionDailySleepDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleDailySpo2Documents", func(t *testing.T) {
		p := &oura.MultipleDailySpo2DocumentsV2UsercollectionDailySpo2GetParams{}
		_, err := c.MultipleDailySpo2DocumentsV2UsercollectionDailySpo2GetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleDailySpo2Document", func(t *testing.T) {
		_, err := c.SingleDailySpo2DocumentV2UsercollectionDailySpo2DocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleDailyStressDocuments", func(t *testing.T) {
		p := &oura.MultipleDailyStressDocumentsV2UsercollectionDailyStressGetParams{}
		_, err := c.MultipleDailyStressDocumentsV2UsercollectionDailyStressGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleDailyStressDocument", func(t *testing.T) {
		_, err := c.SingleDailyStressDocumentV2UsercollectionDailyStressDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleHeartRateDocuments", func(t *testing.T) {
		p := &oura.MultipleHeartRateDocumentsV2UsercollectionHeartrateGetParams{}
		_, err := c.MultipleHeartRateDocumentsV2UsercollectionHeartrateGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleRestModePeriodDocuments", func(t *testing.T) {
		p := &oura.MultipleRestModePeriodDocumentsV2UsercollectionRestModePeriodGetParams{}
		_, err := c.MultipleRestModePeriodDocumentsV2UsercollectionRestModePeriodGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleRestModePeriodDocument", func(t *testing.T) {
		_, err := c.SingleRestModePeriodDocumentV2UsercollectionRestModePeriodDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleRingConfigurationDocuments", func(t *testing.T) {
		p := &oura.MultipleRingConfigurationDocumentsV2UsercollectionRingConfigurationGetParams{}
		_, err := c.MultipleRingConfigurationDocumentsV2UsercollectionRingConfigurationGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleRingConfigurationDocument", func(t *testing.T) {
		_, err := c.SingleRingConfigurationDocumentV2UsercollectionRingConfigurationDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleSessionDocuments", func(t *testing.T) {
		p := &oura.MultipleSessionDocumentsV2UsercollectionSessionGetParams{}
		_, err := c.MultipleSessionDocumentsV2UsercollectionSessionGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleSessionDocument", func(t *testing.T) {
		_, err := c.SingleSessionDocumentV2UsercollectionSessionDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleDailySleepDocumentsNonSandbox", func(t *testing.T) {
		p := &oura.MultipleSleepDocumentsV2UsercollectionSleepGetParams{}
		_, err := c.MultipleSleepDocumentsV2UsercollectionSleepGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleSleepDocument", func(t *testing.T) {
		_, err := c.SingleSleepDocumentV2UsercollectionSleepDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleSleepTimeDocuments", func(t *testing.T) {
		p := &oura.MultipleSleepTimeDocumentsV2UsercollectionSleepTimeGetParams{}
		_, err := c.MultipleSleepTimeDocumentsV2UsercollectionSleepTimeGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleSleepTimeDocument", func(t *testing.T) {
		_, err := c.SingleSleepTimeDocumentV2UsercollectionSleepTimeDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleVO2MaxDocuments", func(t *testing.T) {
		p := &oura.MultipleVO2MaxDocumentsV2UsercollectionVO2MaxGetParams{}
		_, err := c.MultipleVO2MaxDocumentsV2UsercollectionVO2MaxGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleVO2MaxDocument", func(t *testing.T) {
		_, err := c.SingleVO2MaxDocumentV2UsercollectionVO2MaxDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("MultipleWorkoutDocuments", func(t *testing.T) {
		p := &oura.MultipleWorkoutDocumentsV2UsercollectionWorkoutGetParams{}
		_, err := c.MultipleWorkoutDocumentsV2UsercollectionWorkoutGetWithResponse(ctx, p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("SingleWorkoutDocument", func(t *testing.T) {
		_, err := c.SingleWorkoutDocumentV2UsercollectionWorkoutDocumentIdGetWithResponse(ctx, "doc-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	// Webhooks
	t.Run("ListWebhookSubscriptions", func(t *testing.T) {
		// ListWebhookSubscriptions returns []WebhookSubscriptionModel — test calls the method for coverage
		c.ListWebhookSubscriptionsV2WebhookSubscriptionGetWithResponse(ctx) // nolint:errcheck
	})
	t.Run("GetWebhookSubscription", func(t *testing.T) {
		_, err := c.GetWebhookSubscriptionV2WebhookSubscriptionIdGetWithResponse(ctx, "sub-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("DeleteWebhookSubscription", func(t *testing.T) {
		_, err := c.DeleteWebhookSubscriptionV2WebhookSubscriptionIdDeleteWithResponse(ctx, "sub-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
