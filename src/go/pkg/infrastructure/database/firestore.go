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
	return doc, nil
}

func (a *FirestoreAdapter) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return a.storage.Users().Doc(id).Update(ctx, data)
}
