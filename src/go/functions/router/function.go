package router

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/framework"
	infrapubsub "github.com/fitglue/server/src/go/pkg/infrastructure/pubsub"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("RouteActivity", RouteActivity)
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

// RouteActivity is the entry point
func RouteActivity(ctx context.Context, e cloudevents.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("router", svc, routeHandler)(ctx, e)
}

// routeHandler contains the business logic
func routeHandler(ctx context.Context, e cloudevents.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Extract payload
	// We assume strict CloudEvent input
	rawData := e.Data()

	var eventPayload pb.EnrichedActivityEvent
	// Use protojson to unmarshal (supports standard Proto JSON format)
	unmarshalOpts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOpts.Unmarshal(rawData, &eventPayload); err != nil {
		return nil, fmt.Errorf("protojson unmarshal: %v", err)
	}

	fwCtx.Logger.Info("Starting routing", "source", eventPayload.Source, "pipeline", eventPayload.PipelineId)

	// Extract pipeline_execution_id from payload or use current execution ID
	pipelineExecID := ""
	if eventPayload.PipelineExecutionId != nil {
		pipelineExecID = *eventPayload.PipelineExecutionId
	}
	if pipelineExecID == "" {
		pipelineExecID = fwCtx.ExecutionID
	}

	// Since we moved routing logic to the Enricher/Pipeline configuration,
	// the event already carries the list of intended destinations.
	destinations := eventPayload.Destinations

	// Fan-out
	type RoutedDestination struct {
		Destination     string `json:"destination"`
		Topic           string `json:"topic"`
		PubSubMessageID string `json:"pubsub_message_id"`
		Status          string `json:"status"`
		Error           string `json:"error,omitempty"`
	}
	routedDestinations := []RoutedDestination{}

	fwCtx.Logger.Info("Resolved destinations from payload", "dests", destinations)

	for _, dest := range destinations {
		destName := dest.String()
		topic := infrapubsub.GetDestinationTopic(dest)

		if topic == "" {
			fwCtx.Logger.Warn("Unknown or unsupported destination", "dest", destName)
			routedDestinations = append(routedDestinations, RoutedDestination{
				Destination: destName,
				Status:      "SKIPPED",
				Error:       "unknown destination or missing topic configuration",
			})
			continue
		}

		// Construct routing event
		routeEvent, err := infrapubsub.NewCloudEvent(
			infrapubsub.GetCloudEventSource(pb.CloudEventSource_CLOUD_EVENT_SOURCE_ROUTER),
			fmt.Sprintf("com.fitglue.job.%s", destName),
			rawData,
		)
		if err != nil {
			fwCtx.Logger.Error("Failed to create routing event", "error", err)
			continue
		}

		// Propagate pipeline execution ID
		routeEvent.SetExtension("pipeline_execution_id", pipelineExecID)

		resID, err := fwCtx.Service.Pub.PublishCloudEvent(ctx, topic, routeEvent)
		if err != nil {
			fwCtx.Logger.Error("Failed to publish to queue", "dest", destName, "topic", topic, "error", err)
			routedDestinations = append(routedDestinations, RoutedDestination{
				Destination: destName,
				Topic:       topic,
				Status:      "FAILED",
				Error:       err.Error(),
			})
		} else {
			fwCtx.Logger.Info("Routed to destination", "dest", destName, "topic", topic, "message_id", resID)
			routedDestinations = append(routedDestinations, RoutedDestination{
				Destination:     destName,
				Topic:           topic,
				PubSubMessageID: resID,
				Status:          "SUCCESS",
			})
		}
	}

	fwCtx.Logger.Info("Routing complete", "routed_count", len(routedDestinations))
	return map[string]interface{}{
		"status":              "SUCCESS",
		"activity_id":         eventPayload.ActivityId,
		"pipeline_id":         eventPayload.PipelineId,
		"source":              eventPayload.Source.String(),
		"activity_name":       eventPayload.Name,
		"activity_type":       eventPayload.ActivityType,
		"applied_enrichments": eventPayload.AppliedEnrichments,
		"routed_destinations": routedDestinations,
	}, nil
}
