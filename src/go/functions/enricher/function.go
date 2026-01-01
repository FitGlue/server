package enricher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

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

			// Check if this is already a lag-retry (from attributes)
			isLagRetry := false
			if msg.Message.Attributes != nil && msg.Message.Attributes["origin"] == "lag-queue" {
				isLagRetry = true
			}

			if isLagRetry {
				fwCtx.Logger.Warn("Lag Retry failed (will retry with backoff)", "error", err)
				// Returning error triggers the Lag Subscription's retry policy (60s+ backoff)
				return map[string]interface{}{
					"status": "STATUS_LAGGED_RETRY",
					"error":  err.Error(),
				}, err
			} else {
				fwCtx.Logger.Info("Activity data lagging, offloading to lag queue", "error", err)

				// Publish to Lag Topic with "origin=lag-queue" to break infinite loop on next consumption
				lagAttributes := map[string]string{"origin": "lag-queue"}

				// Republish the exact same raw payload
				_, pubErr := fwCtx.Service.Pub.PublishWithAttrs(ctx, shared.TopicEnrichmentLag, msg.Message.Data, lagAttributes)
				if pubErr != nil {
					fwCtx.Logger.Error("Failed to publish to lag topic", "error", pubErr)
					return nil, pubErr // Fail execution to trigger retry of this offload attempt
				}

				return map[string]interface{}{
					"status": "STATUS_LAGGED",
					"reason": err.Error(),
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

func isRetryable(err error) bool {
	_, ok := err.(*providers.RetryableError)
	return ok
}
