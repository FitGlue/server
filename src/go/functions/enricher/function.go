package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	shared "github.com/ripixel/fitglue-server/src/go/pkg"
	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/enricher"
	"github.com/ripixel/fitglue-server/src/go/pkg/enricher/providers"
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
	if err := json.Unmarshal(msg.Message.Data, &rawEvent); err != nil {
		return nil, fmt.Errorf("json unmarshal: %v", err)
	}

	fwCtx.Logger.Info("Starting enrichment", "timestamp", rawEvent.Timestamp, "source", rawEvent.Source)

	// Initialize Orchestrator
	bucketName := fwCtx.Service.Config.GCSArtifactBucket
	if bucketName == "" {
		bucketName = "fitglue-artifacts"
	}

	orchestrator := enricher.NewOrchestrator(fwCtx.Service.DB, fwCtx.Service.Store, bucketName)

	// Register Providers
	// Use FitBit HR Provider
	orchestrator.Register(providers.NewFitBitHeartRate())

	// Process
	enrichedEvent, err := orchestrator.Process(ctx, &rawEvent)
	if err != nil {
		fwCtx.Logger.Error("Orchestrator failed", "error", err)
		return nil, err
	}

	// Publish to Router
	payload, _ := json.Marshal(enrichedEvent)

	if _, err := fwCtx.Service.Pub.Publish(ctx, shared.TopicEnrichedActivity, payload); err != nil {
		fwCtx.Logger.Error("Failed to publish", "error", err)
		return nil, err
	}

	fwCtx.Logger.Info("Enrichment complete", "id", enrichedEvent.ActivityId)
	return enrichedEvent, nil
}
