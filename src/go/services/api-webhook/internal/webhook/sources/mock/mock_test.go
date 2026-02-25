package mock_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	activitypb "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook/sources/mock"
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
	p := mock.NewProvider()
	assert.Equal(t, "mock", p.ID())
}

func TestProvider_VerifySubscription(t *testing.T) {
	p := mock.NewProvider()
	req := httptest.NewRequest(http.MethodGet, "/webhook/mock", nil)
	w := httptest.NewRecorder()

	p.VerifySubscription(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProvider_ParseEvent(t *testing.T) {
	p := mock.NewProvider()

	t.Run("valid payload", func(t *testing.T) {
		bodyStr := `{"user_id":"u123", "activity_id":"act456", "event":"create"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/mock", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "mock", events[0].Provider)
		assert.Equal(t, "u123", events[0].ProviderUID)
		assert.Equal(t, "act456", events[0].ActivityID)
		assert.Equal(t, "create", events[0].Event)
	})

	t.Run("missing user_id", func(t *testing.T) {
		bodyStr := `{"activity_id":"act456", "event":"create"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/mock", bytes.NewBufferString(bodyStr))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "missing activity_id or user_id")
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/mock", bytes.NewBufferString(`{invalid`))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "invalid json")
	})
}

func TestFetchActivity(t *testing.T) {
	provider := mock.NewProvider()

	t.Run("returns generic mock response", func(t *testing.T) {
		userSvc := &mockUserServiceClient{}

		evt := &webhook.WebhookEvent{
			Provider:   "mock",
			ActivityID: "mock-123",
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user1", evt)

		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.Equal(t, activitypb.ActivitySource_SOURCE_TEST, payload.Source)
		assert.Equal(t, "user1", payload.UserId)
		assert.Equal(t, `{"id": "mock-123"}`, payload.OriginalPayloadJson)
	})
}
