package server

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
)

// FirestoreApiKeyStore implements ApiKeyStore using Cloud Firestore
type FirestoreApiKeyStore struct {
	client *firestore.Client
}

// NewFirestoreApiKeyStore creates a new Firestore-backed API key store
func NewFirestoreApiKeyStore(client *firestore.Client) *FirestoreApiKeyStore {
	return &FirestoreApiKeyStore{client: client}
}

// CreateIngressKey stores a hashed API key record in the ingress_api_keys collection
func (s *FirestoreApiKeyStore) CreateIngressKey(ctx context.Context, keyHash, userID, label string, scopes []string, createdAt time.Time) error {
	_, err := s.client.Collection("ingress_api_keys").Doc(keyHash).Set(ctx, map[string]interface{}{
		"user_id":    userID,
		"label":      label,
		"scopes":     scopes,
		"created_at": createdAt,
	})
	return err
}
