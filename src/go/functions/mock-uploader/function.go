package mockuploader

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/destination"
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
			// Error returned to caller
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

		// Note: synchronized_activities is deprecated - pipeline_runs is now the source of truth
		// The destination.UpdateStatus call at the end of this function updates pipeline_runs with the externalId

		// Increment sync count for billing (per successful destination sync)
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		fwCtx.Logger.Info("Mock upload complete",
			"activity_id", eventPayload.ActivityId,
			"mock_external_id", mockExternalID,
		)

		// Update PipelineRun destination as synced
		destination.UpdateStatus(ctx, svc.DB, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_MOCK, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, mockExternalID, "", fwCtx.Logger)

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
