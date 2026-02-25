package fitbit_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/types/pb/models/user"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook/sources/fitbit"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// mockUserServiceClient implements userpb.UserServiceClient
type mockUserServiceClient struct {
	userpb.UserServiceClient
	getIntegrationResp *userpb.GetIntegrationResponse
	getIntegrationErr  error
}

func (m *mockUserServiceClient) GetIntegration(ctx context.Context, in *userpb.GetIntegrationRequest, opts ...grpc.CallOption) (*userpb.GetIntegrationResponse, error) {
	if m.getIntegrationErr != nil {
		return nil, m.getIntegrationErr
	}
	return m.getIntegrationResp, nil
}

func TestVerifySubscription(t *testing.T) {
	provider := fitbit.NewProvider("secret-code")

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?verify=secret-code", nil)
		rec := httptest.NewRecorder()

		provider.VerifySubscription(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?verify=wrong-code", nil)
		rec := httptest.NewRecorder()

		provider.VerifySubscription(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestParseEvent(t *testing.T) {
	provider := fitbit.NewProvider("secret-code")

	t.Run("valid activity collection", func(t *testing.T) {
		// Fitbit sends an array
		payload := `[
			{
				"collectionType": "activities",
				"date": "2023-10-25",
				"ownerId": "fitbitUser1",
				"ownerType": "user",
				"subscriptionId": "fitglue-activities"
			},
			{
				"collectionType": "sleep",
				"date": "2023-10-25",
				"ownerId": "fitbitUser1",
				"ownerType": "user",
				"subscriptionId": "fitglue-sleep"
			}
		]`

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(payload))

		events, err := provider.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1) // Only activities should be parsed
		assert.Equal(t, "fitbit", events[0].Provider)
		assert.Equal(t, "fitbitUser1", events[0].ProviderUID)
		assert.Equal(t, "2023-10-25", events[0].ActivityID) // Fitbit uses date
		assert.Equal(t, "update", events[0].Event)
	})
}

func TestFetchActivity(t *testing.T) {
	provider := fitbit.NewProvider("secret")

	t.Run("missing integration returns error", func(t *testing.T) {
		userSvc := &mockUserServiceClient{
			getIntegrationErr: nil,
			getIntegrationResp: &userpb.GetIntegrationResponse{
				Integrations: &user.UserIntegrations{},
			},
		}

		evt := &webhook.WebhookEvent{
			Provider:   "fitbit",
			ActivityID: "2023-10-25",
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user1", evt)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fitbit integration not found or access token missing")
		assert.Nil(t, payload)
	})
}
