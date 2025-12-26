package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
)

// Token represents the OAuth token structure we care about
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

// TokenSource returns a valid token.
// It is safe for concurrent use by multiple goroutines.
type TokenSource interface {
	Token(context.Context) (*Token, error)
	ForceRefresh(context.Context) (*Token, error)
}

// FirestoreTokenSource reads from Firestore and refreshes if necessary.
type FirestoreTokenSource struct {
	db       *bootstrap.Service
	userID   string
	provider string
	mu       sync.Mutex
}

func NewFirestoreTokenSource(svc *bootstrap.Service, userID, provider string) *FirestoreTokenSource {
	return &FirestoreTokenSource{
		db:       svc,
		userID:   userID,
		provider: provider,
	}
}

// ForceRefresh forcibly refreshes the token regardless of expiry.
func (s *FirestoreTokenSource) ForceRefresh(ctx context.Context) (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Fetch refresh token explicitly from DB again to be safe
	userData, err := s.db.DB.GetUser(ctx, s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	integrations, ok := userData["integrations"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid integrations format")
	}

	providerData, ok := integrations[s.provider].(map[string]interface{})
	if !ok || providerData == nil {
		return nil, fmt.Errorf("provider %s not linked", s.provider)
	}

	refreshToken, _ := providerData["refresh_token"].(string)
	if refreshToken == "" {
		return nil, fmt.Errorf("missing refresh token for %s", s.provider)
	}

	return s.refreshToken(ctx, refreshToken)
}

// Token returns a token, refreshing it if necessary.
func (s *FirestoreTokenSource) Token(ctx context.Context) (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Fetch current token from Firestore
	userData, err := s.db.DB.GetUser(ctx, s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	integrations, ok := userData["integrations"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid integrations format")
	}

	providerData, ok := integrations[s.provider].(map[string]interface{})
	if !ok || providerData == nil {
		return nil, fmt.Errorf("provider %s not linked", s.provider)
	}

	accessToken, _ := providerData["access_token"].(string)
	refreshToken, _ := providerData["refresh_token"].(string)

	// Handle expiry
	var expiry time.Time
	if t, ok := providerData["expires_at"].(time.Time); ok {
		expiry = t
	} else if tStr, ok := providerData["expires_at"].(string); ok {
		expiry, _ = time.Parse(time.RFC3339, tStr)
	}

	if accessToken == "" || refreshToken == "" {
		return nil, fmt.Errorf("missing tokens for %s", s.provider)
	}

	// 2. Check Expiry (Proactive Refresh)
	// Refresh if expired or expiring in the next minute
	if time.Now().Add(1 * time.Minute).After(expiry) {
		return s.refreshToken(ctx, refreshToken)
	}

	return &Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       expiry,
	}, nil
}

// refreshToken performs the HTTP exchange to get a new token & updates Firestore
func (s *FirestoreTokenSource) refreshToken(ctx context.Context, refreshToken string) (*Token, error) {
	clientID, err := s.getSecret("client_id")
	if err != nil {
		return nil, err
	}
	clientSecret, err := s.getSecret("client_secret")
	if err != nil {
		return nil, err
	}

	var tokenURL string
	if s.provider == "strava" {
		tokenURL = "https://www.strava.com/oauth/token"
	} else if s.provider == "fitbit" {
		tokenURL = "https://api.fitbit.com/oauth2/token"
	} else {
		return nil, fmt.Errorf("unsupported provider for refresh: %s", s.provider)
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if s.provider == "fitbit" {
		req.SetBasicAuth(clientID, clientSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("refresh failed with status: %d", resp.StatusCode)
	}

	// Parse Response
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		ExpiresAt    int64  `json:"expires_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	newExpiry := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	if result.ExpiresAt != 0 {
		newExpiry = time.Unix(result.ExpiresAt, 0)
	}

	// Update Firestore
	updateData := map[string]interface{}{}
	baseKey := fmt.Sprintf("integrations.%s", s.provider)
	updateData[baseKey+".access_token"] = result.AccessToken
	updateData[baseKey+".refresh_token"] = result.RefreshToken
	updateData[baseKey+".expires_at"] = newExpiry

	if err := s.db.DB.UpdateUser(ctx, s.userID, updateData); err != nil {
		return nil, fmt.Errorf("failed to persist new tokens: %w", err)
	}

	return &Token{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		Expiry:       newExpiry,
	}, nil
}

func (s *FirestoreTokenSource) getSecret(keyType string) (string, error) {
	name := fmt.Sprintf("%s_%s", s.provider, keyType)
	return s.db.Secrets.GetSecret(context.Background(), s.db.Config.ProjectID, name)
}
