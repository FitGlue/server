package pipelinesplitter

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	shared "github.com/fitglue/server/src/go/pkg"
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
	functions.CloudEvent("SplitByPipeline", SplitByPipeline)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		svc, svcErr = bootstrap.NewService(ctx)
	})
	return svc, svcErr
}

// SplitByPipeline is the entry point - receives raw activities and fans out to per-pipeline messages
func SplitByPipeline(ctx context.Context, e cloudevents.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %w", err)
	}
	return framework.WrapCloudEvent("pipeline-splitter", svc, splitHandler)(ctx, e)
}

// splitHandler contains the business logic
func splitHandler(ctx context.Context, e cloudevents.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Parse ActivityPayload
	rawData := e.Data()

	var payload pb.ActivityPayload
	unmarshalOpts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOpts.Unmarshal(rawData, &payload); err != nil {
		return nil, fmt.Errorf("protojson unmarshal: %w", err)
	}

	logger := fwCtx.Logger

	// If pipelineId is already set, pass through unchanged (resume/targeted repost)
	if payload.PipelineId != nil && *payload.PipelineId != "" {
		logger.Info("PipelineId already set, passing through", "pipelineId", *payload.PipelineId)
		return passThrough(ctx, fwCtx, &payload, logger)
	}

	// Resolve matching pipelines for this source
	pipelines, err := resolvePipelinesForSource(ctx, fwCtx.Service.DB, payload.UserId, payload.Source, logger)
	if err != nil {
		return nil, fmt.Errorf("resolve pipelines: %w", err)
	}

	if len(pipelines) == 0 {
		logger.Info("No pipelines configured for source", "source", payload.Source.String())
		return map[string]interface{}{
			"status":  "SKIPPED",
			"reason":  "no_pipelines",
			"source":  payload.Source.String(),
			"user_id": payload.UserId,
		}, nil
	}

	// Fan out: publish one message per pipeline
	basePipelineExecId := ""
	if payload.PipelineExecutionId != nil {
		basePipelineExecId = *payload.PipelineExecutionId
	}
	if basePipelineExecId == "" {
		basePipelineExecId = fwCtx.ExecutionID
	}

	logger.Info("Fanning out to pipelines", "count", len(pipelines), "source", payload.Source.String())

	publishedCount := 0
	for _, p := range pipelines {
		pipelineId := p.Id
		pipelineExecId := fmt.Sprintf("%s-%s", basePipelineExecId, pipelineId)

		// Clone the payload for this pipeline
		clonedPayload := proto.Clone(&payload).(*pb.ActivityPayload)
		clonedPayload.PipelineId = &pipelineId
		clonedPayload.PipelineExecutionId = &pipelineExecId

		// Publish to pipeline-activity topic
		if err := publishToPipelineActivity(ctx, fwCtx, clonedPayload, logger); err != nil {
			logger.Error("Failed to publish pipeline message", "pipelineId", pipelineId, "error", err)
			continue
		}

		publishedCount++
		logger.Info("Published pipeline message", "pipelineId", pipelineId, "pipelineExecId", pipelineExecId)
	}

	return map[string]interface{}{
		"status":          "FAN_OUT",
		"pipelines_found": len(pipelines),
		"published":       publishedCount,
		"source":          payload.Source.String(),
	}, nil
}

// passThrough publishes the payload directly to pipeline-activity topic without modification
func passThrough(ctx context.Context, fwCtx *framework.FrameworkContext, payload *pb.ActivityPayload, logger *slog.Logger) (interface{}, error) {
	if err := publishToPipelineActivity(ctx, fwCtx, payload, logger); err != nil {
		return nil, fmt.Errorf("pass-through publish: %w", err)
	}

	return map[string]interface{}{
		"status":     "PASS_THROUGH",
		"pipelineId": *payload.PipelineId,
	}, nil
}

// resolvePipelinesForSource finds all pipelines matching the given source
func resolvePipelinesForSource(ctx context.Context, db shared.Database, userId string, source pb.ActivitySource, logger *slog.Logger) ([]*pb.PipelineConfig, error) {
	userPipelines, err := db.GetUserPipelines(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("get user pipelines: %w", err)
	}

	sourceName := source.String()
	var matching []*pb.PipelineConfig

	for _, p := range userPipelines {
		// Skip disabled pipelines
		if p.Disabled {
			logger.Info("Skipping disabled pipeline", "id", p.Id, "name", p.Name)
			continue
		}

		// Match by source
		if p.Source == sourceName {
			matching = append(matching, p)
		}
	}

	return matching, nil
}

// publishToPipelineActivity publishes an ActivityPayload to the pipeline-activity topic
func publishToPipelineActivity(ctx context.Context, fwCtx *framework.FrameworkContext, payload *pb.ActivityPayload, logger *slog.Logger) error {
	// Serialize to JSON
	marshalOpts := protojson.MarshalOptions{UseProtoNames: true}
	data, err := marshalOpts.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Create CloudEvent
	ce, err := infrapubsub.NewCloudEvent(
		infrapubsub.GetCloudEventSource(pb.CloudEventSource_CLOUD_EVENT_SOURCE_PIPELINE_SPLITTER),
		"com.fitglue.activity.pipeline",
		data,
	)
	if err != nil {
		return fmt.Errorf("create cloud event: %w", err)
	}

	// Set extensions for tracing
	if payload.PipelineExecutionId != nil {
		ce.SetExtension("pipeline_execution_id", *payload.PipelineExecutionId)
	}

	// Publish
	_, err = fwCtx.Service.Pub.PublishCloudEvent(ctx, shared.TopicPipelineActivity, ce)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	return nil
}
