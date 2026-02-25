package oura_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/types/pb/models/user"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook/sources/oura"
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
	p := oura.NewProvider()
	assert.Equal(t, "oura", p.ID())
}

func TestProvider_VerifySubscription(t *testing.T) {
	p := oura.NewProvider()
	req := httptest.NewRequest(http.MethodGet, "/webhook/oura", nil)
	w := httptest.NewRecorder()

	p.VerifySubscription(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProvider_ParseEvent(t *testing.T) {
	p := oura.NewProvider()

	t.Run("valid workout create payload", func(t *testing.T) {
		bodyStr := `{
			"event_type": "create",
			"data_type": "workout",
			"object_id": "oura123",
			"user_id": "user456",
			"timestamp": "2026-01-01T00:00:00Z"
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/oura", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "oura", events[0].Provider)
		assert.Equal(t, "user456", events[0].ProviderUID)
		assert.Equal(t, "oura123", events[0].ActivityID)
		assert.Equal(t, "create", events[0].Event)
	})

	t.Run("ignore non-workout data_type", func(t *testing.T) {
		bodyStr := `{
			"event_type": "create",
			"data_type": "sleep",
			"object_id": "oura123",
			"user_id": "user456"
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/oura", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Nil(t, events)
	})

	t.Run("ignore delete and update events", func(t *testing.T) {
		bodyStr := `{
			"event_type": "update",
			"data_type": "workout",
			"object_id": "oura123",
			"user_id": "user456"
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/oura", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Nil(t, events)

		bodyStrDelete := `{
			"event_type": "delete",
			"data_type": "workout",
			"object_id": "oura123",
			"user_id": "user456"
		}`
		reqDelete := httptest.NewRequest(http.MethodPost, "/webhook/oura", bytes.NewBufferString(bodyStrDelete))

		eventsDelete, err := p.ParseEvent(reqDelete)

		assert.NoError(t, err)
		assert.Nil(t, eventsDelete)
	})

	t.Run("missing fields", func(t *testing.T) {
		bodyStr := `{
			"event_type": "create",
			"data_type": "workout",
			"user_id": "user456"
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/oura", bytes.NewBufferString(bodyStr))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "missing object_id or user_id")
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/oura", bytes.NewBufferString(`{invalid`))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "invalid json")
	})
}

func TestFetchActivity(t *testing.T) {
	provider := oura.NewProvider()

	t.Run("missing integration returns error", func(t *testing.T) {
		userSvc := &mockUserServiceClient{
			getIntegrationErr: nil,
			getIntegrationResp: &userpb.GetIntegrationResponse{
				Integrations: &user.UserIntegrations{},
			},
		}

		evt := &webhook.WebhookEvent{
			Provider:   "oura",
			ActivityID: "workout123",
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user1", evt)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "oura integration not found or access token missing")
		assert.Nil(t, payload)
	})
}
