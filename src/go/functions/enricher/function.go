package enricher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	shared "github.com/ripixel/fitglue-server/src/go/pkg"
	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	providers "github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	"github.com/ripixel/fitglue-server/src/go/pkg/framework"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error

	// providerRegistry holds the list of providers to register
	// Can be overridden in tests
	providerRegistry []providers.Provider
)

func init() {
	functions.CloudEvent("EnrichActivity", EnrichActivity)
	// Register default providers
	providerRegistry = []providers.Provider{
		providers.NewFitBitHeartRate(),
		providers.NewWorkoutSummaryProvider(),
		providers.NewMuscleHeatmapProvider(),
		providers.NewSourceLinkProvider(),
		providers.NewVirtualGPSProvider(),
		providers.NewTypeMapperProvider(),
		providers.NewBrandingProvider(), // Always runs last to add footer
	}
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		svc, svcErr = bootstrap.NewService(ctx)
		if svcErr != nil {
			slog.Error("Failed to initialize service", "error", svcErr)
		}
	})
	return svc, svcErr
}

// EnrichActivity is the entry point
func EnrichActivity(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("enricher", svc, enrichHandler)(ctx, e)
}

// enrichHandler contains the business logic
func enrichHandler(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Parse Pub/Sub message
	var msg types.PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return nil, fmt.Errorf("event.DataAs: %v", err)
	}

	var rawEvent pb.ActivityPayload
	// Use protojson to unmarshal, which supports both camelCase (canonical) and snake_case field names
	unmarshalOpts := protojson.UnmarshalOptions{
		DiscardUnknown: true, // Be resilient to future schema changes
	}
	if err := unmarshalOpts.Unmarshal(msg.Message.Data, &rawEvent); err != nil {
		return nil, fmt.Errorf("protojson unmarshal: %v", err)
	}

	if rawEvent.UserId == "" {
		return nil, fmt.Errorf("missing userId in payload")
	}

	fwCtx.Logger.Info("Starting enrichment", "timestamp", rawEvent.Timestamp, "source", rawEvent.Source)

	// Initialize Orchestrator
	bucketName := fwCtx.Service.Config.GCSArtifactBucket
	if bucketName == "" {
		bucketName = "fitglue-artifacts"
	}

	orchestrator := NewOrchestrator(fwCtx.Service.DB, fwCtx.Service.Store, bucketName)

	// Register Providers from registry
	for _, provider := range providerRegistry {
		// Set service if the provider supports it
		if sp, ok := provider.(interface{ SetService(*bootstrap.Service) }); ok {
			sp.SetService(fwCtx.Service)
		}
		orchestrator.Register(provider)
	}

	// Process
	processResult, err := orchestrator.Process(ctx, &rawEvent, fwCtx.ExecutionID)
	if err != nil {
		fwCtx.Logger.Error("Orchestrator failed", "error", err)
		return nil, err
	}

	if len(processResult.Events) == 0 {
		fwCtx.Logger.Info("No pipelines matched, skipping enrichment")
		return map[string]interface{}{
			"status":              "NO_PIPELINES",
			"provider_executions": processResult.ProviderExecutions,
		}, nil
	}

	// Publish Results to Router
	var publishedCount int
	marshalOpts := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}

	// Track published events for rich output
	type PublishedEvent struct {
		ActivityID         string   `json:"activity_id"`
		PipelineID         string   `json:"pipeline_id"`
		Destinations       []string `json:"destinations"`
		AppliedEnrichments []string `json:"applied_enrichments"`
		FitFileURI         string   `json:"fit_file_uri,omitempty"`
		PubSubMessageID    string   `json:"pubsub_message_id"`
	}
	publishedEvents := []PublishedEvent{}

	for _, event := range processResult.Events {
		payload, err := marshalOpts.Marshal(event)
		if err != nil {
			fwCtx.Logger.Error("Failed to marshal enriched event", "error", err)
			continue
		}

		msgID, err := fwCtx.Service.Pub.Publish(ctx, shared.TopicEnrichedActivity, payload)
		if err != nil {
			fwCtx.Logger.Error("Failed to publish result", "error", err, "pipeline_id", event.PipelineId)
		} else {
			publishedCount++
			fwCtx.Logger.Info("Published enriched event",
				"activity_id", event.ActivityId,
				"pipeline_id", event.PipelineId,
				"destinations", event.Destinations,
				"message_id", msgID)

			publishedEvents = append(publishedEvents, PublishedEvent{
				ActivityID:         event.ActivityId,
				PipelineID:         event.PipelineId,
				Destinations:       event.Destinations,
				AppliedEnrichments: event.AppliedEnrichments,
				FitFileURI:         event.FitFileUri,
				PubSubMessageID:    msgID,
			})
		}
	}

	fwCtx.Logger.Info("Enrichment complete", "published_count", publishedCount)
	return map[string]interface{}{
		"status":              "SUCCESS",
		"published_count":     publishedCount,
		"total_events":        len(processResult.Events),
		"published_events":    publishedEvents,
		"provider_executions": processResult.ProviderExecutions,
	}, nil
}
