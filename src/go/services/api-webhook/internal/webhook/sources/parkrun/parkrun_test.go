package parkrun_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	activitypb "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook"
	"github.com/fitglue/server/src/go/services/api-webhook/internal/webhook/sources/parkrun"
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
	p := parkrun.NewProvider()
	assert.Equal(t, "parkrun", p.ID())
}

func TestProvider_VerifySubscription(t *testing.T) {
	p := parkrun.NewProvider()
	req := httptest.NewRequest(http.MethodGet, "/webhook/parkrun", nil)
	w := httptest.NewRecorder()

	p.VerifySubscription(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProvider_ParseEvent(t *testing.T) {
	p := parkrun.NewProvider()

	t.Run("valid payload", func(t *testing.T) {
		bodyStr := `{"athleteId":"A1234", "runNumber":"100", "event":"run"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/parkrun", bytes.NewBufferString(bodyStr))

		events, err := p.ParseEvent(req)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "parkrun", events[0].Provider)
		assert.Equal(t, "A1234", events[0].ProviderUID)
		assert.Equal(t, "100", events[0].ActivityID)
		assert.Equal(t, "run", events[0].Event)
	})

	t.Run("missing athleteId", func(t *testing.T) {
		bodyStr := `{"runNumber":"100", "event":"run"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook/parkrun", bytes.NewBufferString(bodyStr))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "missing athleteId or runNumber")
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/parkrun", bytes.NewBufferString(`{invalid`))

		_, err := p.ParseEvent(req)
		assert.ErrorContains(t, err, "invalid json")
	})
}

func TestFetchActivity(t *testing.T) {
	provider := parkrun.NewProvider()

	t.Run("valid payload", func(t *testing.T) {
		userSvc := &mockUserServiceClient{}

		evt := &webhook.WebhookEvent{
			Provider:   "parkrun",
			ActivityID: "100",
			RawPayload: []byte(`{"athleteId": "A1234", "runNumber": "100"}`),
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user1", evt)

		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.Equal(t, activitypb.ActivitySource_SOURCE_PARKRUN_RESULTS, payload.Source)
		assert.Equal(t, "user1", payload.UserId)
		assert.Equal(t, string(evt.RawPayload), payload.OriginalPayloadJson)
	})

	t.Run("missing raw payload", func(t *testing.T) {
		userSvc := &mockUserServiceClient{}

		evt := &webhook.WebhookEvent{
			Provider:   "parkrun",
			ActivityID: "100",
		}

		payload, err := provider.FetchActivity(context.Background(), userSvc, "user2", evt)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing raw payload for parkrun results")
		assert.Nil(t, payload)
	})
}
