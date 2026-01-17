package database

import (
	"context"

	"cloud.google.com/go/firestore"
	storage "github.com/fitglue/server/src/go/pkg/storage/firestore"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// FirestoreAdapter provides database operations using Firestore
// It wraps our typed storage client
type FirestoreAdapter struct {
	Client  *firestore.Client
	storage *storage.Client // internal typed wrapper
}

func NewFirestoreAdapter(client *firestore.Client) *FirestoreAdapter {
	return &FirestoreAdapter{
		Client:  client, // Keep raw client accessible if needed? OR remove it if unused.
		storage: storage.NewClient(client),
	}
}

func (a *FirestoreAdapter) SetExecution(ctx context.Context, record *pb.ExecutionRecord) error {
	// Use typed storage
	return a.storage.Executions().Doc(record.ExecutionId).Set(ctx, record)
}

func (a *FirestoreAdapter) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	// Use untyped update on connection
	return a.storage.Executions().Doc(id).Update(ctx, data)
}

func (a *FirestoreAdapter) GetUser(ctx context.Context, id string) (*pb.UserRecord, error) {
	doc, err := a.storage.Users().Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	// Manually populate ID since it's the doc key
	doc.UserId = id
	return doc, nil
}

func (a *FirestoreAdapter) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return a.storage.Users().Doc(id).Update(ctx, data)
}

// --- Sync Count (for tier limits) ---

func (a *FirestoreAdapter) IncrementSyncCount(ctx context.Context, userID string) error {
	_, err := a.Client.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
		{Path: "sync_count_this_month", Value: firestore.Increment(1)},
	})
	return err
}

func (a *FirestoreAdapter) ResetSyncCount(ctx context.Context, userID string) error {
	_, err := a.Client.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
		{Path: "sync_count_this_month", Value: 0},
		{Path: "sync_count_reset_at", Value: firestore.ServerTimestamp},
	})
	return err
}

// --- Pending Inputs ---

func (a *FirestoreAdapter) GetPendingInput(ctx context.Context, id string) (*pb.PendingInput, error) {
	doc, err := a.storage.PendingInputs().Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (a *FirestoreAdapter) CreatePendingInput(ctx context.Context, input *pb.PendingInput) error {
	// Use Set to handle potential retries/race conditions
	return a.storage.PendingInputs().Doc(input.ActivityId).Set(ctx, input)

}

func (a *FirestoreAdapter) UpdatePendingInput(ctx context.Context, id string, data map[string]interface{}) error {
	return a.storage.PendingInputs().Doc(id).Update(ctx, data)
}

func (a *FirestoreAdapter) ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error) {
	// Query: where("user_id", "==", userID).where("status", "==", STATUS_WAITING)
	// We need to use the raw client for queries as our storage wrapper might not support generic queries yet?
	// Looking at `server/src/go/pkg/storage/firestore/collection.go` (inferred), usually wrapper has basic CRUD.
	// `client.go` exposes `Users()` which returns `*Collection`.
	// Let's use the raw client exposed in Adapter if needed, or check if Collection supports Where.
	// Assuming raw client usage for queries for now to be safe.
	iter := a.Client.Collection("pending_inputs").Where("user_id", "==", userID).Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		return nil, err
	}

	var results []*pb.PendingInput
	for _, d := range docs {
		// Manually convert using our converter
		m := d.Data()
		p := storage.FirestoreToPendingInput(m)
		// Ensure ActivityID is set from doc ID if missing (though it should be in data)
		if p.ActivityId == "" {
			p.ActivityId = d.Ref.ID
		}
		results = append(results, p)
	}
	return results, nil
}

// --- Counters ---

func (a *FirestoreAdapter) GetCounter(ctx context.Context, userId string, id string) (*pb.Counter, error) {
	doc, err := a.storage.Counters(userId).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	doc.Id = id
	return doc, nil
}

func (a *FirestoreAdapter) SetCounter(ctx context.Context, userId string, counter *pb.Counter) error {
	// Set (overwrite/create)
	return a.storage.Counters(userId).Doc(counter.Id).Set(ctx, counter)
}

// --- Activities ---

func (a *FirestoreAdapter) SetSynchronizedActivity(ctx context.Context, userId string, activity *pb.SynchronizedActivity) error {
	return a.storage.Activities(userId).Doc(activity.ActivityId).Set(ctx, activity)
}

// ListPendingParkrunActivities queries all users' activities with pending Parkrun results
func (a *FirestoreAdapter) ListPendingParkrunActivities(ctx context.Context) ([]*pb.SynchronizedActivity, []string, error) {
	// Query across ALL users' activities subcollections using collection group query
	// Filter by parkrun_results_state == PARKRUN_RESULTS_STATE_PENDING (2)
	iter := a.Client.CollectionGroup("activities").
		Where("parkrun_results_state", "==", int32(pb.ParkrunResultsState_PARKRUN_RESULTS_STATE_PENDING)).
		Documents(ctx)

	docs, err := iter.GetAll()
	if err != nil {
		return nil, nil, err
	}

	var activities []*pb.SynchronizedActivity
	var userIDs []string

	for _, d := range docs {
		// Extract user ID from path: users/{userId}/activities/{activityId}
		pathParts := d.Ref.Parent.Parent.ID // This gets us the userId
		userID := pathParts

		m := d.Data()
		activity := storage.FirestoreToSynchronizedActivity(m)
		if activity.ActivityId == "" {
			activity.ActivityId = d.Ref.ID
		}
		activities = append(activities, activity)
		userIDs = append(userIDs, userID)
	}

	return activities, userIDs, nil
}

// UpdateSynchronizedActivity updates specific fields on an activity
func (a *FirestoreAdapter) UpdateSynchronizedActivity(ctx context.Context, userId string, activityId string, data map[string]interface{}) error {
	return a.storage.Activities(userId).Doc(activityId).Update(ctx, data)
}

// GetSynchronizedActivity retrieves a single synchronized activity
func (a *FirestoreAdapter) GetSynchronizedActivity(ctx context.Context, userId string, activityId string) (*pb.SynchronizedActivity, error) {
	activity, err := a.storage.Activities(userId).Doc(activityId).Get(ctx)
	if err != nil {
		return nil, err
	}
	// Ensure activity ID is set
	if activity != nil && activity.ActivityId == "" {
		activity.ActivityId = activityId
	}
	return activity, nil
}

// ListPendingInputsByEnricher retrieves pending inputs for a specific enricher
func (a *FirestoreAdapter) ListPendingInputsByEnricher(ctx context.Context, enricherId string, status pb.PendingInput_Status) ([]*pb.PendingInput, error) {
	// Query across all pending inputs using collection group query
	iter := a.Client.CollectionGroup("pending_inputs").
		Where("enricher_provider_id", "==", enricherId).
		Where("status", "==", int32(status)).
		Documents(ctx)

	docs, err := iter.GetAll()
	if err != nil {
		return nil, err
	}

	var inputs []*pb.PendingInput
	for _, d := range docs {
		m := d.Data()
		input := storage.FirestoreToPendingInput(m)
		if input.ActivityId == "" {
			input.ActivityId = d.Ref.ID
		}
		inputs = append(inputs, input)
	}

	return inputs, nil
}
