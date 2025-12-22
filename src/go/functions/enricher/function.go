package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	shared "github.com/ripixel/fitglue-server/src/go/pkg"
	"github.com/ripixel/fitglue-server/src/go/pkg/execution"
	"github.com/ripixel/fitglue-server/src/go/pkg/fit"
	"github.com/ripixel/fitglue-server/src/go/pkg/fitbit"
	"github.com/ripixel/fitglue-server/src/go/pkg/pkg/bootstrap"
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
	_, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}

	var msg types.PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var rawEvent pb.ActivityPayload
	if err := json.Unmarshal(msg.Message.Data, &rawEvent); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// Structured Logging
	logger := slog.With("user_id", rawEvent.UserId, "service", "enricher")

	logger.Info("Starting enrichment", "timestamp", rawEvent.Timestamp)

	// Log execution start
	execID, err := execution.LogStart(ctx, svc.DB, "enricher", execution.ExecutionOptions{
		UserID:      rawEvent.UserId,
		TriggerType: "pubsub",
		Inputs:      &rawEvent,
	})
	if err != nil {
		logger.Error("Failed to log execution start", "error", err)
		return err
	}

	// 1. Logic: Merge Data
	startTime, _ := time.Parse(time.RFC3339, rawEvent.Timestamp)
	duration := 3600

	// 1a. Fetch Credentials
	clientID, _ := svc.Secrets.GetSecret(ctx, svc.Config.ProjectID, "fitbit-client-id")
	clientSecret, _ := svc.Secrets.GetSecret(ctx, svc.Config.ProjectID, "fitbit-client-secret")

	fbClient := fitbit.NewClient(rawEvent.UserId, clientID, clientSecret)
	_ = fbClient // Placeholder

	// 2. Generate FIT
	hrStream := make([]int, duration)
	powerStream := make([]int, duration)
	fitBytes, err := fit.GenerateFitFile(startTime, duration, powerStream, hrStream)
	if err != nil {
		svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": err.Error()})
		return err
	}

	// 3. Save to GCS
	bucketName := svc.Config.GCSArtifactBucket
	if bucketName == "" {
		bucketName = "fitglue-artifacts"
	}
	objName := fmt.Sprintf("activities/%s/%d.fit", rawEvent.UserId, startTime.Unix())
	if err := svc.Store.Write(ctx, bucketName, objName, fitBytes); err != nil {
		svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": err.Error()})
		return err
	}

	// 4. Generate Description
	desc := "Enhanced Activity\n\n#PowerMap #HeartrateMap"

	// 5. Publish to Router
	enrichedEvent := &pb.EnrichedActivityEvent{
		UserId:      rawEvent.UserId,
		GcsUri:      fmt.Sprintf("gs://%s/%s", bucketName, objName),
		Description: desc,
	}
	payload, _ := json.Marshal(enrichedEvent)

	if _, err := svc.Pub.Publish(ctx, shared.TopicEnrichedActivity, payload); err != nil {
		logger.Error("Failed to publish", "error", err)
		return err
	}

	// Log execution success
	if err := execution.LogSuccess(ctx, svc.DB, execID, enrichedEvent); err != nil {
		logger.Warn("Failed to log execution success", "error", err)
	}

	logger.Info("Enrichment complete")
	return nil
}
