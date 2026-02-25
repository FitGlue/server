package infra

import "context"

// DocumentStore defines an interface for interacting with NoSQL document databases like Firestore.
type DocumentStore interface {
	Get(ctx context.Context, collection, id string, dest interface{}) error
	Set(ctx context.Context, collection, id string, data interface{}) error
	Update(ctx context.Context, collection, id string, updates map[string]interface{}) error
	Delete(ctx context.Context, collection, id string) error

	// Transaction runs a set of operations in a transaction
	Transaction(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error
}

// Transaction defines the interface for transactional operations on a DocumentStore.
type Transaction interface {
	Get(collection, id string, dest interface{}) error
	Set(collection, id string, data interface{}) error
	Update(collection, id string, updates map[string]interface{}) error
	Delete(collection, id string) error
}
