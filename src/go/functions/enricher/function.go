package enricher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"

	shared "github.com/ripixel/fitglue-server/src/go/pkg"
	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	providers "github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	"github.com/ripixel/fitglue-server/src/go/pkg/framework"
	infrapubsub "github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/pubsub"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"

	// Register providers
	_ "github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers/auto_increment"
	_ "github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers/condition_matcher"
	_ "github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers/parkrun"
	_ "github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers/user_input"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("EnrichActivity", EnrichActivity)
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
func EnrichActivity(ctx context.Context, e cloudevents.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("enricher", svc, enrichHandler)(ctx, e)
}

// enrichHandler contains the business logic
func enrichHandler(ctx context.Context, e cloudevents.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Extract payload and attributes
	// We assume strict CloudEvent input (legacy Pub/Sub messages are no longer supported)
	rawData := e.Data()
	isLagRetry := false
	if val, ok := e.Extensions()["origin"].(string); ok && val == "lag-queue" {
		isLagRetry = true
	}

	var rawEvent pb.ActivityPayload
	// Use protojson to unmarshal, which supports both camelCase (canonical) and snake_case field names
	unmarshalOpts := protojson.UnmarshalOptions{
		DiscardUnknown: true, // Be resilient to future schema changes
	}
	if err := unmarshalOpts.Unmarshal(rawData, &rawEvent); err != nil {
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
	for _, provider := range providers.GetAll() {
		// Set service if the provider supports it
		if sp, ok := provider.(interface{ SetService(*bootstrap.Service) }); ok {
			sp.SetService(fwCtx.Service)
		}
		orchestrator.Register(provider)
	}

	// Calculate lag exhaustion (Force mode / Do Not Retry)
	doNotRetry := false
	// For Pub/Sub events, e.Time() is the publish time.
	// We want to force if the message is older than our max backoff (20 mins + buffer)
	// Note: For unwrapped events, e.Time() is the original event time, which is what we want.
	if !e.Time().IsZero() {
		lagDuration := time.Since(e.Time())
		if lagDuration > 15*time.Minute {
			fwCtx.Logger.Warn("Activity lag exhausted, forcing partial enrichment", "age", lagDuration)
			doNotRetry = true
		}
	}

	// Process
	processResult, err := orchestrator.Process(ctx, &rawEvent, fwCtx.ExecutionID, doNotRetry)

	if err != nil {
		// Check if the error is retryable (e.g. data lag)
		if ok := isRetryable(err); ok {

			if isLagRetry {
				fwCtx.Logger.Warn("Lag Retry failed (will retry with backoff)", "error", err)
				// ACK the message and let Pub/Sub retry policy handle backoff
				return map[string]interface{}{
					"status": "STATUS_LAGGED_RETRY",
					"error":  err.Error(),
				}, nil // ACK message to allow Pub/Sub retry policy to handle backoff
			} else {
				// Preserve the original error before it gets shadowed
				originalErr := err
				fwCtx.Logger.Info("Activity data lagging, offloading to lag queue", "error", originalErr)

				// Publish to Lag Topic with "origin=lag-queue" to break infinite loop on next consumption
				// Create CloudEvent
				lagEvent, err := infrapubsub.NewCloudEvent("/enricher", "com.fitglue.enrichment.lag", rawData)
				if err != nil {
					fwCtx.Logger.Error("Failed to create lag event", "error", err)
					return nil, err
				}
				lagEvent.SetExtension("origin", "lag-queue")

				_, pubErr := fwCtx.Service.Pub.PublishCloudEvent(ctx, shared.TopicEnrichmentLag, lagEvent)
				if pubErr != nil {
					fwCtx.Logger.Error("Failed to publish to lag topic", "error", pubErr)
					return nil, pubErr // Fail execution to trigger retry of this offload attempt
				}

				return map[string]interface{}{
					"status": "STATUS_LAGGED",
					"reason": originalErr.Error(),
				}, nil // ACK original message since we've successfully moved it to the delay queue
			}
		}

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
		resultEvent, err := infrapubsub.NewCloudEvent("/enricher", "com.fitglue.activity.enriched", event)
		if err != nil {
			fwCtx.Logger.Error("Failed to create result event", "error", err)
			continue
		}

		msgID, err := fwCtx.Service.Pub.PublishCloudEvent(ctx, shared.TopicEnrichedActivity, resultEvent)
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

	finalStatus := "SUCCESS"
	if processResult.Status == pb.ExecutionStatus_STATUS_WAITING {
		finalStatus = "STATUS_WAITING"
	}

	return map[string]interface{}{
		"status":              finalStatus,
		"published_count":     publishedCount,
		"total_events":        len(processResult.Events),
		"published_events":    publishedEvents,
		"provider_executions": processResult.ProviderExecutions,
	}, nil
}

func isRetryable(err error) bool {
	_, ok := err.(*providers.RetryableError)
	return ok
}
