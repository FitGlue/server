package router

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
func RouteActivity(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("router", svc, routeHandler)(ctx, e)
}

// routeHandler contains the business logic
func routeHandler(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Parse Pub/Sub message
	var msg types.PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return nil, fmt.Errorf("event.DataAs: %v", err)
	}

	var eventPayload pb.EnrichedActivityEvent
	if err := json.Unmarshal(msg.Message.Data, &eventPayload); err != nil {
		return nil, fmt.Errorf("json unmarshal: %v", err)
	}

	fwCtx.Logger.Info("Starting routing")

	// Fetch User Config
	userData, err := fwCtx.Service.DB.GetUser(ctx, eventPayload.UserId)
	if err != nil {
		fwCtx.Logger.Error("User config not found")
		return nil, fmt.Errorf("user config not found")
	}

	stravaEnabled, _ := userData["strava_enabled"].(bool)

	// Fan-out
	routings := []string{}

	if stravaEnabled {
		resID, err := fwCtx.Service.Pub.Publish(ctx, shared.TopicJobUploadStrava, msg.Message.Data)
		if err != nil {
			fwCtx.Logger.Error("Failed to publish to Strava queue", "error", err)
		} else {
			routings = append(routings, "strava:"+resID)
		}
	}

	fwCtx.Logger.Info("Routed activity", "routes", routings)
	return map[string]interface{}{
		"strava_enabled": stravaEnabled,
		"routings":       routings,
	}, nil
}
