package router

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

func RouteActivity(ctx context.Context, e event.Event) error {
	_, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}

	var msg types.PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var eventPayload pb.EnrichedActivityEvent
	if err := json.Unmarshal(msg.Message.Data, &eventPayload); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// Structured Logging
	execID := fmt.Sprintf("router-%s-%d", eventPayload.UserId, time.Now().UnixNano())
	logger := slog.With("execution_id", execID, "user_id", eventPayload.UserId, "service", "router")

	logger.Info("Starting routing")

	// Log execution start
	execID, err = execution.LogStart(ctx, svc.DB, "router", execution.ExecutionOptions{
		UserID:      eventPayload.UserId,
		TriggerType: "pubsub",
		Inputs:      eventPayload.UserId,
	})
	if err != nil {
		logger.Error("Failed to log execution start", "error", err)
		return err
	}

	// 1. Fetch User Config
	userData, err := svc.DB.GetUser(ctx, eventPayload.UserId)
	if err != nil {
		svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": "User config not found"})
		return err
	}

	stravaEnabled, _ := userData["strava_enabled"].(bool)
	// otherEnabled, _ := userData["other_enabled"].(bool)

	// 2. Fan-out
	routings := []string{}

	if stravaEnabled {
		resID, err := svc.Pub.Publish(ctx, shared.TopicJobUploadStrava, msg.Message.Data)
		if err != nil {
			logger.Error("Failed to publish to Strava queue", "error", err)
		} else {
			routings = append(routings, "strava:"+resID)
		}
	}

	svc.DB.UpdateExecution(ctx, execID, map[string]interface{}{
		"status":    "SUCCESS",
		"outputs":   routings,
		"timestamp": time.Now(),
		"endTime":   time.Now(),
	})

	logger.Info("Routed activity", "routes", routings)
	return nil
}
