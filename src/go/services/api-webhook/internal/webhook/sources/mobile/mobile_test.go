package mobile_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	activitypb "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook/sources/mobile"
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

func TestProvider_ID(t *testing.T) {
	p := mobile.NewProvider()
	assert.Equal(t, "mobile", p.ID())
}

func TestProvider_VerifySubscription(t *testing.T) {
	p := mobile.NewProvider()
	req := httptest.NewRequest(http.MethodGet, "/webhook/mobile", nil)
	w := httptest.NewRecorder()

	p.VerifySubscription(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProvider_ParseEvent(t *testing.T) {
	p := mobile.NewProvider()

	t.Run("valid payload with auth token", func(t *testing.T) {
		bodyStr := `{
			"activities": [
				{
					"source": "healthkit",
					"externalId": "hk123",
					"activityName": "Running",
					"startTime": "2026-01-01T00:00:00Z"
				}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/mobile", bytes.NewBufferString(bodyStr))
		req.Header.Set("Authorization", "Bearer valid.jwt.token")

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "mobile", events[0].Provider)
		assert.Equal(t, "valid.jwt.token", events[0].ProviderUID)
		assert.Equal(t, "hk123", events[0].ActivityID)
		assert.Equal(t, "sync", events[0].Event)
	})

	t.Run("valid payload no auth token", func(t *testing.T) {
		bodyStr := `{
			"activities": [
				{
					"source": "healthconnect",
					"externalId": "hc456"
				}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/mobile", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "mobile", events[0].Provider)
		assert.Equal(t, "", events[0].ProviderUID)
		assert.Equal(t, "hc456", events[0].ActivityID)
		assert.Equal(t, "sync", events[0].Event)
	})

	t.Run("missing externalId uses fallback", func(t *testing.T) {
		bodyStr := `{
			"activities": [
				{
					"source": "healthconnect",
					"startTime": "2026-02-01T00:00:00Z"
				}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/mobile", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "healthconnect-2026-02-01T00:00:00Z", events[0].ActivityID)
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/mobile", bytes.NewBufferString(`{invalid`))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "invalid json")
	})
}

func TestFetchActivity(t *testing.T) {
	provider := mobile.NewProvider()

	t.Run("valid apple health payload", func(t *testing.T) {
		userSvc := &mockUserServiceClient{}

		evt := &webhook.WebhookEvent{
			Provider:   "mobile",
			ActivityID: "apple-123",
			RawPayload: []byte(`{"source": "SOURCE_APPLE_HEALTH", "durationSeconds": 3600}`),
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user1", evt)

		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.Equal(t, activitypb.ActivitySource_SOURCE_APPLE_HEALTH, payload.Source)
		assert.Equal(t, "user1", payload.UserId)
		assert.Equal(t, string(evt.RawPayload), payload.OriginalPayloadJson)
	})

	t.Run("valid health connect payload", func(t *testing.T) {
		userSvc := &mockUserServiceClient{}

		evt := &webhook.WebhookEvent{
			Provider:   "mobile",
			ActivityID: "hc-456",
			RawPayload: []byte(`{"source": "SOURCE_HEALTH_CONNECT", "durationSeconds": 1800}`),
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user2", evt)

		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.Equal(t, activitypb.ActivitySource_SOURCE_HEALTH_CONNECT, payload.Source)
		assert.Equal(t, "user2", payload.UserId)
	})

	t.Run("missing raw payload", func(t *testing.T) {
		userSvc := &mockUserServiceClient{}

		evt := &webhook.WebhookEvent{
			Provider:   "mobile",
			ActivityID: "no-payload",
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user3", evt)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing raw payload for mobile activity sync")
		assert.Nil(t, payload)
	})
}
