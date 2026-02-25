package polar_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/types/pb/models/user"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook/sources/polar"
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
	p := polar.NewProvider()
	assert.Equal(t, "polar", p.ID())
}

func TestProvider_VerifySubscription(t *testing.T) {
	p := polar.NewProvider()
	req := httptest.NewRequest(http.MethodGet, "/webhook/polar", nil)
	w := httptest.NewRecorder()

	p.VerifySubscription(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProvider_ParseEvent(t *testing.T) {
	p := polar.NewProvider()

	t.Run("valid exercise event", func(t *testing.T) {
		bodyStr := `{
			"event": "EXERCISE",
			"user_id": 123456,
			"entity_id": "polar789",
			"timestamp": "2026-01-01T00:00:00Z"
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/polar", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "polar", events[0].Provider)
		assert.Equal(t, "123456", events[0].ProviderUID)
		assert.Equal(t, "polar789", events[0].ActivityID)
		assert.Equal(t, "EXERCISE", events[0].Event)
	})

	t.Run("ignore non-exercise events", func(t *testing.T) {
		bodyStr := `{
			"event": "SLEEP",
			"user_id": 123456,
			"entity_id": "polar789"
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/polar", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Nil(t, events)
	})

	t.Run("missing entity_id", func(t *testing.T) {
		bodyStr := `{
			"event": "EXERCISE",
			"user_id": 123456
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/polar", bytes.NewBufferString(bodyStr))

		_, err := p.ParseEvent(req)

		assert.ErrorContains(t, err, "missing entity_id")
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/polar", bytes.NewBufferString(`{invalid`))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "invalid json")
	})
}

func TestFetchActivity(t *testing.T) {
	provider := polar.NewProvider()

	t.Run("missing integration returns error", func(t *testing.T) {
		userSvc := &mockUserServiceClient{
			getIntegrationErr: nil,
			getIntegrationResp: &userpb.GetIntegrationResponse{
				Integrations: &user.UserIntegrations{},
			},
		}

		evt := &webhook.WebhookEvent{
			Provider:   "polar",
			ActivityID: "ex123",
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user1", evt)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "polar integration not found or missing tokens")
		assert.Nil(t, payload)
	})
}
