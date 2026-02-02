package router

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

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
			// Error returned to caller
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

	// Store enriched_event_uri for repost functionality
	// If enricher offloaded to GCS (activity_data_uri set), reuse that URI
	// The GCS blob contains the full EnrichedActivityEvent, so it works for repost
	if eventPayload.UserId != "" && pipelineExecID != "" {
		updateData := map[string]interface{}{
			"updated_at": protojson.Format(timestamppb.Now()),
		}

		// Reuse activity_data_uri as enriched_event_uri (same blob contains full event)
		if eventPayload.ActivityDataUri != "" {
			updateData["enriched_event_uri"] = eventPayload.ActivityDataUri
			fwCtx.Logger.Debug("Reusing activity_data_uri for enriched_event_uri", "uri", eventPayload.ActivityDataUri)
		} else {
			// Fallback: upload full event to GCS if enricher didn't offload
			// (for small events that didn't exceed threshold)
			bucketName := fwCtx.Service.Config.GCSArtifactBucket
			if bucketName != "" {
				gcsPath := fmt.Sprintf("enriched_events/%s/%s.json", eventPayload.UserId, pipelineExecID)
				jsonBytes, err := protojson.Marshal(&eventPayload)
				if err != nil {
					fwCtx.Logger.Warn("Failed to marshal enriched event for GCS", "error", err)
				} else if err := fwCtx.Service.Store.Write(ctx, bucketName, gcsPath, jsonBytes); err != nil {
					fwCtx.Logger.Warn("Failed to upload enriched event to GCS", "error", err)
				} else {
					updateData["enriched_event_uri"] = fmt.Sprintf("gs://%s/%s", bucketName, gcsPath)
					fwCtx.Logger.Debug("Uploaded enriched event to GCS", "uri", updateData["enriched_event_uri"])
				}
			}
		}

		if err := fwCtx.Service.DB.UpdatePipelineRun(ctx, eventPayload.UserId, pipelineExecID, updateData); err != nil {
			fwCtx.Logger.Error("Failed to update pipeline run with enriched event URI", "error", err, "pipeline_run_id", pipelineExecID)
		}
	}

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
