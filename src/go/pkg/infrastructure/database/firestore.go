package database

import (
	"context"

	"cloud.google.com/go/firestore"
	storage "github.com/ripixel/fitglue-server/src/go/pkg/storage/firestore"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
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
