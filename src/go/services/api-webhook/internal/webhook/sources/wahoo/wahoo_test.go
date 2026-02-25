package wahoo_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/types/pb/models/user"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook/sources/wahoo"
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
	p := wahoo.NewProvider()
	assert.Equal(t, "wahoo", p.ID())
}

func TestProvider_VerifySubscription(t *testing.T) {
	p := wahoo.NewProvider()
	req := httptest.NewRequest(http.MethodGet, "/webhook/wahoo", nil)
	w := httptest.NewRecorder()

	p.VerifySubscription(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProvider_ParseEvent(t *testing.T) {
	p := wahoo.NewProvider()

	t.Run("valid workout summary event", func(t *testing.T) {
		bodyStr := `{
			"event_type": "workout_summary",
			"user": {
				"id": 12345
			},
			"workout_summary": {
				"id": 67890
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/wahoo", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "wahoo", events[0].Provider)
		assert.Equal(t, "12345", events[0].ProviderUID)
		assert.Equal(t, "67890", events[0].ActivityID)
		assert.Equal(t, "workout_summary", events[0].Event)
	})

	t.Run("ignore non-workout summary events", func(t *testing.T) {
		bodyStr := `{
			"event_type": "workout_file",
			"user": {
				"id": 12345
			},
			"workout_summary": {
				"id": 67890
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/wahoo", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Nil(t, events)
	})

	t.Run("missing workout summary info", func(t *testing.T) {
		bodyStr := `{
			"event_type": "workout_summary",
			"user": {
				"id": 12345
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/wahoo", bytes.NewBufferString(bodyStr))

		_, err := p.ParseEvent(req)

		assert.ErrorContains(t, err, "missing workout summary ID")
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/wahoo", bytes.NewBufferString(`{invalid`))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "invalid json")
	})
}

func TestFetchActivity(t *testing.T) {
	provider := wahoo.NewProvider()

	t.Run("missing integration returns error", func(t *testing.T) {
		userSvc := &mockUserServiceClient{
			getIntegrationErr: nil,
			getIntegrationResp: &userpb.GetIntegrationResponse{
				Integrations: &user.UserIntegrations{},
			},
		}

		evt := &webhook.WebhookEvent{
			Provider:   "wahoo",
			ActivityID: "ex123",
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user1", evt)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wahoo integration not found or access token missing")
		assert.Nil(t, payload)
	})
}
