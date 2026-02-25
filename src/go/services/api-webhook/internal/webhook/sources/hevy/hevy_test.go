package hevy_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/types/pb/models/user"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook/sources/hevy"
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
	p := hevy.NewProvider()
	assert.Equal(t, "hevy", p.ID())
}

func TestProvider_VerifySubscription(t *testing.T) {
	p := hevy.NewProvider()
	req := httptest.NewRequest(http.MethodGet, "/webhook/hevy", nil)
	w := httptest.NewRecorder()

	p.VerifySubscription(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProvider_ParseEvent(t *testing.T) {
	p := hevy.NewProvider()

	t.Run("valid flat workout payload with header", func(t *testing.T) {
		bodyStr := `{"workoutId": "hevy123"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/hevy", bytes.NewBufferString(bodyStr))
		req.Header.Set("x-api-key", "test-api-key")

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "hevy", events[0].Provider)
		assert.Equal(t, "test-api-key", events[0].ProviderUID)
		assert.Equal(t, "hevy123", events[0].ActivityID)
	})

	t.Run("valid nested workout payload with auth header", func(t *testing.T) {
		bodyStr := `{
			"payload": {
				"workoutId": "hevy456"
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/hevy", bytes.NewBufferString(bodyStr))
		req.Header.Set("Authorization", "Bearer token-123")

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "hevy", events[0].Provider)
		assert.Equal(t, "token-123", events[0].ProviderUID)
		assert.Equal(t, "hevy456", events[0].ActivityID)
	})

	t.Run("valid nested workout payload with raw auth header", func(t *testing.T) {
		bodyStr := `{
			"payload": {
				"workoutId": "hevy456"
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/hevy", bytes.NewBufferString(bodyStr))
		req.Header.Set("Authorization", "raw-token-123")

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "hevy", events[0].Provider)
		assert.Equal(t, "raw-token-123", events[0].ProviderUID)
		assert.Equal(t, "hevy456", events[0].ActivityID)
	})

	t.Run("valid flat workout payload with query param", func(t *testing.T) {
		bodyStr := `{"workoutId": "hevy789"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/hevy?key=query-key", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "hevy", events[0].Provider)
		assert.Equal(t, "query-key", events[0].ProviderUID)
		assert.Equal(t, "hevy789", events[0].ActivityID)
	})

	t.Run("missing api key", func(t *testing.T) {
		bodyStr := `{"workoutId": "hevy789"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/hevy", bytes.NewBufferString(bodyStr))

		_, err := p.ParseEvent(req)

		assert.ErrorContains(t, err, "missing api key")
	})

	t.Run("ignore non workout payload", func(t *testing.T) {
		bodyStr := `{"other": "data"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/hevy", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Nil(t, events)
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/hevy", bytes.NewBufferString(`{invalid`))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "invalid json")
	})
}

func TestFetchActivity(t *testing.T) {
	provider := hevy.NewProvider()

	t.Run("missing integration returns error", func(t *testing.T) {
		userSvc := &mockUserServiceClient{
			getIntegrationErr: nil,
			getIntegrationResp: &userpb.GetIntegrationResponse{
				Integrations: &user.UserIntegrations{},
			},
		}

		evt := &webhook.WebhookEvent{
			Provider:   "hevy",
			ActivityID: "workout123",
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user1", evt)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hevy integration not found or api key missing")
		assert.Nil(t, payload)
	})
}
