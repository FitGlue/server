package database

import (
	"context"

	"cloud.google.com/go/firestore"
)

// FirestoreAdapter provides database operations using Firestore
type FirestoreAdapter struct {
	Client *firestore.Client
}

func (a *FirestoreAdapter) SetExecution(ctx context.Context, id string, data map[string]interface{}) error {
	var ref *firestore.DocumentRef
	if id == "" {
		ref = a.Client.Collection("executions").NewDoc()
	} else {
		ref = a.Client.Collection("executions").Doc(id)
	}
	_, err := ref.Set(ctx, data, firestore.MergeAll)
	return err
}

func (a *FirestoreAdapter) UpdateExecution(ctx context.Context, id string, data map[string]interface{}) error {
	_, err := a.Client.Collection("executions").Doc(id).Set(ctx, data, firestore.MergeAll)
	return err
}

func (a *FirestoreAdapter) GetUser(ctx context.Context, id string) (map[string]interface{}, error) {
	snap, err := a.Client.Collection("users").Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	return snap.Data(), nil
}

func (a *FirestoreAdapter) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	_, err := a.Client.Collection("users").Doc(id).Set(ctx, data, firestore.MergeAll)
	return err
}
