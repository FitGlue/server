package enricher

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	shared "github.com/fitglue/server/src/go/pkg"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/user"
	"github.com/fitglue/server/src/go/pkg/testing/mocks"

	"github.com/fitglue/server/src/go/internal/pipeline/enricher/providers"
	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"
)

// installEnricherSvcCapturingEvents mirrors installEnricherSvcWithProvider but
// captures the full published CloudEvents (not just topic names), so tests can
// assert on lag-event extensions.
func installEnricherSvcCapturingEvents(t *testing.T, enrich func(ctx context.Context, l *slog.Logger, a *pbactivity.StandardizedActivity, u *user.Record, cfg map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error)) *[]publishedEvent {
	t.Helper()
	mockDB := &mocks.MockDatabase{
		GetUserFunc: func(ctx context.Context, id string) (*user.Record, error) {
			return &user.Record{UserProfile: &pbuser.UserProfile{UserId: id}}, nil
		},
		GetUserPipelinesFunc: func(ctx context.Context, userId string) ([]*pbpipeline.PipelineConfig, error) {
			return []*pbpipeline.PipelineConfig{{
				Id:           "test-pipeline-1",
				Source:       "SOURCE_HEVY",
				Destinations: []pbplugin.DestinationType{pbplugin.DestinationType_DESTINATION_STRAVA},
				Enrichers: []*pbpipeline.EnricherConfig{
					{ProviderType: pbplugin.EnricherProviderType_ENRICHER_PROVIDER_MOCK},
				},
			}}, nil
		},
	}
	events := &[]publishedEvent{}
	mockPub := &mocks.MockPublisher{
		PublishCloudEventFunc: func(ctx context.Context, topic string, e cloudevents.Event) (string, error) {
			*events = append(*events, publishedEvent{Topic: topic, Event: e})
			return "msg-123", nil
		},
	}
	mockStore := &mocks.MockBlobStore{
		WriteFunc: func(ctx context.Context, bucket, object string, data []byte) error { return nil },
	}

	providers.ClearRegistry()
	providers.Register(&MockProvider{
		NameFunc:         func() string { return "mock-enricher" },
		ProviderTypeFunc: func() pbplugin.EnricherProviderType { return pbplugin.EnricherProviderType_ENRICHER_PROVIDER_MOCK },
		EnrichFunc:       enrich,
	})

	prev := svc
	svc = &bootstrap.Service{
		DB:     mockDB,
		Pub:    mockPub,
		Store:  mockStore,
		Config: &bootstrap.Config{ProjectID: "test-project"},
	}
	t.Cleanup(func() {
		providers.ClearRegistry()
		svc = prev
	})
	return events
}

type publishedEvent struct {
	Topic string
	Event cloudevents.Event
}

// A RetryableError with a RetryAfter must stamp retrynotbefore/lagdeadline
// extensions on the lag event so the requested backoff is honoured downstream.
func TestEnrichActivity_LagOffloadStampsRetryWindow(t *testing.T) {
	events := installEnricherSvcCapturingEvents(t, func(ctx context.Context, _ *slog.Logger, _ *pbactivity.StandardizedActivity, _ *user.Record, _ map[string]string, _ bool) (*providers.EnrichmentResult, error) {
		return nil, providers.NewRetryableError(errors.New("rate limited"), time.Hour, "upstream 429")
	})

	e := cloudevents.NewEvent()
	e.SetID("e-window")
	e.SetType("com.fitglue.activity.created")
	e.SetSource("/test")
	e.SetTime(time.Now())
	_ = e.SetData(cloudevents.ApplicationJSON, validPayloadBytes(t))

	before := time.Now()
	if err := EnrichActivity(context.Background(), e); err != nil {
		t.Fatalf("expected ACK after lag offload, got %v", err)
	}

	var lagEvent *cloudevents.Event
	for i := range *events {
		if (*events)[i].Topic == shared.TopicEnrichmentLag {
			lagEvent = &(*events)[i].Event
		}
	}
	if lagEvent == nil {
		t.Fatal("expected a publish to the lag topic")
	}

	notBefore, ok := parseTimeExtension(*lagEvent, extRetryNotBefore)
	if !ok {
		t.Fatalf("lag event missing %s extension: %v", extRetryNotBefore, lagEvent.Extensions())
	}
	if got := notBefore.Sub(before); got < 55*time.Minute || got > 65*time.Minute {
		t.Errorf("retrynotbefore should be ~1h out, got %v", got)
	}

	deadline, ok := parseTimeExtension(*lagEvent, extLagDeadline)
	if !ok {
		t.Fatalf("lag event missing %s extension", extLagDeadline)
	}
	if got := deadline.Sub(notBefore); got != lagExhaustionWindow {
		t.Errorf("lagdeadline should be retrynotbefore+%v, got +%v", lagExhaustionWindow, got)
	}
}

// A lag message delivered before its retrynotbefore must NACK without invoking
// any provider (so the rate-limited upstream isn't hammered).
func TestEnrichActivity_LagRetryWaitsForWindow(t *testing.T) {
	providerCalled := false
	installEnricherSvcCapturingEvents(t, func(ctx context.Context, _ *slog.Logger, _ *pbactivity.StandardizedActivity, _ *user.Record, _ map[string]string, _ bool) (*providers.EnrichmentResult, error) {
		providerCalled = true
		return &providers.EnrichmentResult{}, nil
	})

	e := cloudevents.NewEvent()
	e.SetID("e-wait")
	e.SetType("com.fitglue.enrichment.lag")
	e.SetSource("/enricher")
	e.SetTime(time.Now())
	e.SetExtension("origin", "lag-queue")
	e.SetExtension(extRetryNotBefore, time.Now().Add(30*time.Minute).Format(time.RFC3339))
	e.SetExtension(extLagDeadline, time.Now().Add(time.Hour).Format(time.RFC3339))
	_ = e.SetData(cloudevents.ApplicationJSON, validPayloadBytes(t))

	err := EnrichActivity(context.Background(), e)
	if err == nil {
		t.Fatal("expected NACK (error) while retry window has not elapsed")
	}
	if providerCalled {
		t.Error("provider must not run before the retry window elapses")
	}
}

// A lag message past its lagdeadline must run providers with doNotRetry=true so
// they complete partially instead of retrying forever.
func TestEnrichActivity_LagDeadlineForcesDoNotRetry(t *testing.T) {
	var sawDoNotRetry bool
	installEnricherSvcCapturingEvents(t, func(ctx context.Context, _ *slog.Logger, _ *pbactivity.StandardizedActivity, _ *user.Record, _ map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
		sawDoNotRetry = doNotRetry
		return &providers.EnrichmentResult{}, nil
	})

	e := cloudevents.NewEvent()
	e.SetID("e-deadline")
	e.SetType("com.fitglue.enrichment.lag")
	e.SetSource("/enricher")
	e.SetTime(time.Now().Add(-2 * time.Hour))
	e.SetExtension("origin", "lag-queue")
	e.SetExtension(extRetryNotBefore, time.Now().Add(-90*time.Minute).Format(time.RFC3339))
	e.SetExtension(extLagDeadline, time.Now().Add(-time.Hour).Format(time.RFC3339))
	_ = e.SetData(cloudevents.ApplicationJSON, validPayloadBytes(t))

	if err := EnrichActivity(context.Background(), e); err != nil {
		t.Fatalf("expected success past lag deadline, got %v", err)
	}
	if !sawDoNotRetry {
		t.Error("expected provider to be called with doNotRetry=true past lagdeadline")
	}
}
