package function

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"fitglue-enricher/pkg/fit"
	"fitglue-enricher/pkg/fitbit"
	"fitglue-enricher/pkg/shared" // Injected by Makefile
)

func init() {
	functions.CloudEvent("EnrichActivity", EnrichActivity)
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

// ActivityPayload and EnrichedActivityEvent are in pkg/shared but for now
// we might need to reference them directly or use aliases.
// Wait, shared/payload.go defines ActivityPayload.
// We need to ensure EnrichedActivityEvent is also shared if Router uses it.
// For now, assume EnrichedActivityEvent is specific to this hand-off, or we move it to shared.
// Given the user request "unified payload", EnrichedActivityEvent IS a payload for the next stage.
// Let's assume we should move EnrichedActivityEvent to shared as well or keep it local if Router imports shared.
// The Router reads "EnrichedActivityEvent".
// Let's check shared/payload.go content. It only has "ActivityPayload" (Ingestion->Enricher).
// I should probably add EnrichedActivityEvent to shared/payload.go?
// The user said "Hevy and Keiser topic publish data to be identical". That's ActivityPayload.
// The Enricher OUTPUT is internal.
// I'll keep EnrichedActivityEvent local for now to minimize scope creep unless needed.

// However, I MUST replace RawActivityEvent with shared.ActivityPayload.

// EnrichActivity is the entry point
func EnrichActivity(ctx context.Context, e event.Event) error {
	var msg PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var rawEvent shared.ActivityPayload
	if err := json.Unmarshal(msg.Data, &rawEvent); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// Logging setup (Firestore Executions)
	client, _ := firestore.NewClient(ctx, shared.ProjectID)
	defer client.Close()
	execRef := client.Collection("executions").NewDoc()
	execRef.Set(ctx, map[string]interface{}{
		"service":   "enricher",
		"status":    "STARTED",
		"inputs":    rawEvent,
		"startTime": time.Now(),
	})

	// 1. Logic: Merge Data
	// For Hevy/Keiser, we extract start/end time, fetch Fitbit HR.
	// Assume we extracted start/end from OriginalPayload.
	startTime, _ := time.Parse(time.RFC3339, rawEvent.Timestamp)
	duration := 3600 // Mock 1 hour

	fbClient := fitbit.NewClient(rawEvent.UserId)
	// hrData, _ := fbClient.GetHeartRateSeries(...)
	hrStream := make([]int, duration) // Populated from FB

	// Mock Power from Raw Payload
	powerStream := make([]int, duration)

	// 2. Generate FIT
	fitBytes, err := fit.GenerateFitFile(startTime, duration, powerStream, hrStream)
	if err != nil {
		execRef.Set(ctx, map[string]interface{}{"status": "FAILED", "error": err.Error()}, firestore.MergeAll)
		return err
	}

	// 3. Save to GCS
	gcsClient, _ := storage.NewClient(ctx)
	defer gcsClient.Close()
	bucket := gcsClient.Bucket("fitglue-artifacts") // Should correspond to bucket resource
	objName := fmt.Sprintf("activities/%s/%d.fit", rawEvent.UserId, startTime.Unix())
	wc := bucket.Object(objName).NewWriter(ctx)
	wc.Write(fitBytes)
	wc.Close()

	// 4. Generate Description (Stats/Hashtags)
	desc := "Enhanced Activity\n\n#PowerMap #HeartrateMap"
	// Parkrun logic here...

	// 5. Publish to Router
	psClient, _ := pubsub.NewClient(ctx, shared.ProjectID)
	topic := psClient.Topic(shared.TopicEnrichedActivity)
	enrichedEvent := EnrichedActivityEvent{
		UserId:      rawEvent.UserId,
		GcsURI:      fmt.Sprintf("gs://fitglue-artifacts/%s", objName),
		Description: desc,
	}
	payload, _ := json.Marshal(enrichedEvent)
	topic.Publish(ctx, &pubsub.Message{Data: payload})

	execRef.Set(ctx, map[string]interface{}{
		"status":  "SUCCESS",
		"outputs": enrichedEvent,
		"endTime": time.Now(),
	}, firestore.MergeAll)

	log.Printf("Enrichment complete for %s", rawEvent.Timestamp)
	return nil
}
