package function

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"fitglue-router/pkg/shared"
	"fitglue-router/pkg/shared/adapters"
	"fitglue-router/pkg/shared/types"
	pb "fitglue-router/pkg/shared/types/pb/proto"
)

var svc *Service

func init() {
	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = shared.ProjectID
	}

	fsClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("Warning: Firestore init failed: %v", err)
	}
	// Pub/Sub Client
	var pubAdapter shared.Publisher
	if os.Getenv("ENABLE_PUBLISH") == "true" {
		psClient, err := pubsub.NewClient(ctx, projectID)
		if err != nil {
			log.Printf("Warning: PubSub init failed: %v", err)
		}
		pubAdapter = &adapters.PubSubAdapter{Client: psClient}
		log.Println("Pub/Sub: REAL (ENABLE_PUBLISH=true)")
	} else {
		pubAdapter = &adapters.LogPublisher{}
		log.Println("Pub/Sub: MOCK (LogPublisher)")
	}

	svc = &Service{
		DB:  &adapters.FirestoreAdapter{Client: fsClient},
		Pub: pubAdapter,
	}

	functions.CloudEvent("RouteActivity", svc.RouteActivity)
}

type Service struct {
	DB  shared.Database
	Pub shared.Publisher
}

type UserConfig struct {
	StravaEnabled bool `firestore:"strava_enabled"`
	OtherEnabled  bool `firestore:"other_enabled"`
}

func (s *Service) RouteActivity(ctx context.Context, e event.Event) error {
	var msg types.PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var eventPayload pb.EnrichedActivityEvent
	if err := json.Unmarshal(msg.Message.Data, &eventPayload); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// Logging setup
	execID := fmt.Sprintf("router-%s-%d", eventPayload.UserId, time.Now().UnixNano())
	execRefData := map[string]interface{}{
		"service":   "router",
		"status":    "STARTED",
		"inputs":    eventPayload.UserId,
		"startTime": time.Now(),
	}
	if err := s.DB.SetExecution(ctx, execID, execRefData); err != nil {
		log.Printf("Failed to log start: %v", err)
	}

	// 1. Fetch User Config
	userData, err := s.DB.GetUser(ctx, eventPayload.UserId)
	if err != nil {
		s.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": "User config not found"})
		return err
	}

	// Manual Mapping from Map to Config (Interface Abstraction)
	stravaEnabled, _ := userData["strava_enabled"].(bool)
	// otherEnabled, _ := userData["other_enabled"].(bool)

	// 2. Fan-out
	routings := []string{}

	if stravaEnabled {
		resID, err := s.Pub.Publish(ctx, shared.TopicJobUploadStrava, msg.Message.Data)
		if err != nil {
			log.Printf("Failed to publish to Strava queue: %v", err)
		} else {
			routings = append(routings, "strava:"+resID)
		}
	}

	s.DB.UpdateExecution(ctx, execID, map[string]interface{}{
		"status":  "SUCCESS",
		"outputs": routings,
		"endTime": time.Now(),
	})

	log.Printf("Routed activity for %s to %v", eventPayload.UserId, routings)
	return nil
}
