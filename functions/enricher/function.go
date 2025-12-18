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
)

func init() {
	functions.CloudEvent("EnrichActivity", EnrichActivity)
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type RawActivityEvent struct {
	Source          string    `json:"source"`
	UserId          string    `json:"userId"`
	Timestamp       string    `json:"timestamp"` // ISO
	OriginalPayload interface{} `json:"originalPayload"`
}

type EnrichedActivityEvent struct {
	UserId      string `json:"userId"`
	ActivityId  string `json:"activityId"` // GCS object name or similar
	Metadata    string `json:"metadata"`   // JSON string of stats
	GcsURI      string `json:"gcsUri"`
	Description string `json:"description"`
}

// EnrichActivity is the entry point
func EnrichActivity(ctx context.Context, e event.Event) error {
	var msg PubSubMessage
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("failed to get data: %v", err)
	}

	var rawEvent RawActivityEvent
	if err := json.Unmarshal(msg.Data, &rawEvent); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// Logging setup (Firestore Executions)
	client, _ := firestore.NewClient(ctx, "fitglue-project") // Use real project ID
	defer client.Close()
	execRef := client.Collection("executions").NewDoc()
	execRef.Set(ctx, map[string]interface{}{
		"service": "enricher",
		"status":  "STARTED",
		"inputs":  rawEvent,
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
	psClient, _ := pubsub.NewClient(ctx, "fitglue-project")
	topic := psClient.Topic("topic-enriched-activity")
	enrichedEvent := EnrichedActivityEvent{
		UserId: rawEvent.UserId,
		GcsURI: fmt.Sprintf("gs://fitglue-artifacts/%s", objName),
		Description: desc,
	}
	payload, _ := json.Marshal(enrichedEvent)
	topic.Publish(ctx, &pubsub.Message{Data: payload})

	execRef.Set(ctx, map[string]interface{}{
		"status": "SUCCESS",
		"outputs": enrichedEvent,
		"endTime": time.Now(),
	}, firestore.MergeAll)

	log.Printf("Enrichment complete for %s", rawEvent.Timestamp)
	return nil
}
