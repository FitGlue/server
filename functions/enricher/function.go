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
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"fitglue-enricher/pkg/fit"
	"fitglue-enricher/pkg/fitbit"
	"fitglue-enricher/pkg/shared"
	"fitglue-enricher/pkg/shared/adapters"
	"fitglue-enricher/pkg/shared/types"
	pb "fitglue-enricher/pkg/shared/types/pb/proto"
)

var svc *Service

func init() {
	// Initialize real dependencies for Production/Local execution
	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = shared.ProjectID // Fallback to shared constant
	}

	// Clients
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

	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Warning: Storage init failed: %v", err)
	}

	svc = &Service{
		DB:      &adapters.FirestoreAdapter{Client: fsClient},
		Pub:     pubAdapter,
		Store:   &adapters.StorageAdapter{Client: gcsClient},
		Secrets: &adapters.SecretsAdapter{},
	}

	functions.CloudEvent("EnrichActivity", svc.EnrichActivity)
}

// Service holds dependencies
type Service struct {
	DB      shared.Database
	Pub     shared.Publisher
	Store   shared.BlobStore
	Secrets shared.SecretStore
}

// PubSubMessage is imported from shared types

// EnrichActivity is the entry point
func (s *Service) EnrichActivity(ctx context.Context, e event.Event) error {
	var msg types.PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var rawEvent pb.ActivityPayload
	if err := json.Unmarshal(msg.Message.Data, &rawEvent); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// Logging setup (Firestore Executions)
	execID := fmt.Sprintf("%s-%d", rawEvent.UserId, time.Now().UnixNano())

	// Create Execution Doc
	execData := map[string]interface{}{
		"service":   "enricher",
		"status":    "STARTED",
		"inputs":    &rawEvent,
		"startTime": time.Now(),
	}
	if err := s.DB.SetExecution(ctx, execID, execData); err != nil {
		log.Printf("Failed to log start: %v", err)
	}

	// 1. Logic: Merge Data
	startTime, _ := time.Parse(time.RFC3339, rawEvent.Timestamp)
	duration := 3600

	// 1a. Fetch Credentials
	clientID, _ := s.Secrets.GetSecret(ctx, shared.ProjectID, "fitbit-client-id")
	clientSecret, _ := s.Secrets.GetSecret(ctx, shared.ProjectID, "fitbit-client-secret")

	fbClient := fitbit.NewClient(rawEvent.UserId, clientID, clientSecret)
	// Fetch actual HR data (Placeholder)
	_ = fbClient
	hrStream := make([]int, duration)
	powerStream := make([]int, duration)

	// 2. Generate FIT
	fitBytes, err := fit.GenerateFitFile(startTime, duration, powerStream, hrStream)
	if err != nil {
		s.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": err.Error()})
		return err
	}

	// 3. Save to GCS
	bucketName := os.Getenv("GCS_ARTIFACT_BUCKET")
	if bucketName == "" {
		bucketName = "fitglue-artifacts" // Default fallback
	}
	objName := fmt.Sprintf("activities/%s/%d.fit", rawEvent.UserId, startTime.Unix())
	if err := s.Store.Write(ctx, bucketName, objName, fitBytes); err != nil {
		s.DB.UpdateExecution(ctx, execID, map[string]interface{}{"status": "FAILED", "error": err.Error()})
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

	if _, err := s.Pub.Publish(ctx, shared.TopicEnrichedActivity, payload); err != nil {
		return err
	}

	s.DB.UpdateExecution(ctx, execID, map[string]interface{}{
		"status":  "SUCCESS",
		"outputs": enrichedEvent,
		"endTime": time.Now(),
	})

	log.Printf("Enrichment complete for %s", rawEvent.Timestamp)
	return nil
}
