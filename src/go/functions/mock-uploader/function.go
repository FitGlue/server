package mockuploader

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/framework"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("MockUpload", MockUpload)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		baseSvc, err := bootstrap.NewService(ctx)
		if err != nil {
			slog.Error("Failed to initialize service", "error", err)
			svcErr = err
			return
		}
		svc = baseSvc
	})
	return svc, svcErr
}

// MockUpload is the entry point for the mock destination.
// It simulates an upload by logging the activity and persisting a SynchronizedActivity record.
func MockUpload(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("mock-uploader", svc, mockHandler())(ctx, e)
}

func mockHandler() framework.HandlerFunc {
	return func(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		var eventPayload pb.EnrichedActivityEvent

		unmarshaler := protojson.UnmarshalOptions{
			DiscardUnknown: true,
			AllowPartial:   true,
		}
		if err := unmarshaler.Unmarshal(e.Data(), &eventPayload); err != nil {
			return nil, fmt.Errorf("protojson.Unmarshal: %w", err)
		}

		fwCtx.Logger.Info("Mock upload received",
			"activity_id", eventPayload.ActivityId,
			"pipeline_id", eventPayload.PipelineId,
			"user_id", eventPayload.UserId,
			"name", eventPayload.Name,
			"type", eventPayload.ActivityType,
			"source", eventPayload.Source,
			"destinations", eventPayload.Destinations,
		)

		// Generate a mock external ID
		mockExternalID := fmt.Sprintf("mock-%s", eventPayload.ActivityId)

		// Persist SynchronizedActivity
		// Check if activity already exists (e.g., repost scenario)
		// If it does, only update destinations to preserve original pipelineExecutionId
		existingActivity, _ := svc.DB.GetSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId)
		if existingActivity != nil {
			// Activity exists - update only destinations (preserves original pipelineExecutionId for boosters display)
			if err := svc.DB.UpdateSynchronizedActivity(ctx, eventPayload.UserId, eventPayload.ActivityId, map[string]interface{}{
				"destinations.mock": mockExternalID,
				"synced_at":         timestamppb.Now().AsTime(),
			}); err != nil {
				fwCtx.Logger.Error("Failed to update synchronized activity destinations", "error", err)
				return nil, fmt.Errorf("failed to update synchronized activity: %w", err)
			}
			fwCtx.Logger.Info("Updated synchronized activity destinations (preserved execution ID)", "activity_id", eventPayload.ActivityId)
		} else {
			// New activity - create full record including pipelineExecutionId
			syncedActivity := &pb.SynchronizedActivity{
				ActivityId:          eventPayload.ActivityId,
				Title:               eventPayload.Name,
				Description:         eventPayload.Description,
				Type:                eventPayload.ActivityType,
				Source:              eventPayload.Source.String(),
				StartTime:           eventPayload.StartTime,
				SyncedAt:            timestamppb.Now(),
				PipelineId:          eventPayload.PipelineId,
				PipelineExecutionId: fwCtx.PipelineExecutionId, // Use framework context (guaranteed populated)
				Destinations: map[string]string{
					"mock": mockExternalID,
				},
			}

			if err := svc.DB.SetSynchronizedActivity(ctx, eventPayload.UserId, syncedActivity); err != nil {
				fwCtx.Logger.Error("Failed to persist synchronized activity", "error", err)
				return nil, fmt.Errorf("failed to persist synchronized activity: %w", err)
			}
		}

		// Increment sync count for billing (per successful destination sync)
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		fwCtx.Logger.Info("Mock upload complete",
			"activity_id", eventPayload.ActivityId,
			"mock_external_id", mockExternalID,
		)

		return map[string]interface{}{
			"status":           "SUCCESS",
			"mock_external_id": mockExternalID,
			"activity_id":      eventPayload.ActivityId,
			"pipeline_id":      eventPayload.PipelineId,
			"activity_name":    eventPayload.Name,
			"activity_type":    eventPayload.ActivityType.String(),
		}, nil
	}
}
