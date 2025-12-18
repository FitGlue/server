package function

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
)

func init() {
	functions.CloudEvent("RouteActivity", RouteActivity)
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

// EnrichedActivityEvent is now imported from pb

type UserConfig struct {
	StravaEnabled bool `firestore:"strava_enabled"`
	OtherEnabled  bool `firestore:"other_enabled"`
}

func RouteActivity(ctx context.Context, e event.Event) error {
	var msg PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var eventPayload pb.EnrichedActivityEvent
	if err := json.Unmarshal(msg.Data, &eventPayload); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	client, _ := firestore.NewClient(ctx, shared.ProjectID)
	defer client.Close()
	execRef := client.Collection("executions").NewDoc()
	execRef.Set(ctx, map[string]interface{}{
		"service":   "router",
		"status":    "STARTED",
		"inputs":    eventPayload.UserId,
		"startTime": time.Now(),
	})

	// 1. Fetch User Config
	docSnap, err := client.Collection("users").Doc(eventPayload.UserId).Get(ctx)
	if err != nil {
		execRef.Set(ctx, map[string]interface{}{"status": "FAILED", "error": "User config not found"}, firestore.MergeAll)
		return err
	}

	var config UserConfig
	docSnap.DataTo(&config)

	// 2. Fan-out
	// 2. Fan-out
	psClient, _ := pubsub.NewClient(ctx, shared.ProjectID)
	defer psClient.Close()

	routings := []string{}

	if config.StravaEnabled {
		topic := psClient.Topic(shared.TopicJobUploadStrava)
		res := topic.Publish(ctx, &pubsub.Message{Data: msg.Data})
		id, _ := res.Get(ctx)
		routings = append(routings, "strava:"+id)
	}

	// Future: if config.OtherEnabled ...

	execRef.Set(ctx, map[string]interface{}{
		"status":  "SUCCESS",
		"outputs": routings,
		"endTime": time.Now(),
	}, firestore.MergeAll)

	log.Printf("Routed activity for %s to %v", eventPayload.UserId, routings)
	return nil
}
