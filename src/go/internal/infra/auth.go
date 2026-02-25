package infra

import "context"

// AuthVerifier defines an interface for verifying authentication tokens and custom claims.
type AuthVerifier interface {
	VerifyToken(ctx context.Context, idToken string) (*TokenInfo, error)
	GetCustomClaims(ctx context.Context, userID string) (map[string]interface{}, error)
	DeleteUser(ctx context.Context, userID string) error
}

// TokenInfo contains the resolved metadata from a verified ID token.
type TokenInfo struct {
	UserID string
	Email  string
	Claims map[string]interface{}
}
