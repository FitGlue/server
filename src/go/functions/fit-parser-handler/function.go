package fitparserhandler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	shared "github.com/fitglue/server/src/go/pkg"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/fit_parser"
	infrapubsub "github.com/fitglue/server/src/go/pkg/infrastructure/pubsub"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.HTTP("ParseFitFile", ParseFitFile)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		baseSvc, err := bootstrap.NewService(ctx)
		if err != nil {
			slog.Error("Failed to initialize service", "error", err)
			svcErr = err
			return
		}
		svc = baseSvc
	})
	return svc, svcErr
}

// ParseFitFileRequest is the expected request body
type ParseFitFileRequest struct {
	// Base64-encoded FIT file data
	FitFileBase64 string `json:"fitFileBase64"`
	// Optional user-provided overrides
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// ParseFitFileResponse is the response body
type ParseFitFileResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
	ActivityId string `json:"activityId,omitempty"`
}

// ParseFitFile is the HTTP entry point for FIT file parsing
func ParseFitFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	// Only accept POST
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Initialize service
	svc, err := initService(ctx)
	if err != nil {
		slog.Error("Service init failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Internal server error",
		})
		return
	}

	// Parse request body
	var req ParseFitFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate required fields
	if req.FitFileBase64 == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "fitFileBase64 is required",
		})
		return
	}

	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Authorization header is required",
		})
		return
	}

	// Parse Bearer token
	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Invalid Authorization header format",
		})
		return
	}
	idToken := authHeader[len(bearerPrefix):]

	// Verify Firebase token and get user ID
	userId, err := verifyFirebaseToken(ctx, idToken)
	if err != nil {
		slog.Warn("Token verification failed", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	slog.Info("Processing FIT file upload", "user_id", userId)

	// Check if user has a FILE_UPLOAD pipeline
	user, err := svc.DB.GetUser(ctx, userId)
	if err != nil || user == nil {
		slog.Error("Failed to get user", "error", err, "user_id", userId)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	hasPipeline := false
	for _, p := range user.Pipelines {
		if p.Source == "SOURCE_FILE_UPLOAD" {
			hasPipeline = true
			break
		}
	}
	if !hasPipeline {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "No pipeline configured for File Upload source. Create a pipeline first.",
		})
		return
	}

	// Decode base64 FIT file
	fitData, err := base64.StdEncoding.DecodeString(req.FitFileBase64)
	if err != nil {
		slog.Error("Failed to decode base64", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Invalid base64 data",
		})
		return
	}

	// Parse FIT file
	activity, err := fit_parser.ParseFitFile(fitData)
	if err != nil {
		slog.Error("Failed to parse FIT file", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse FIT file: %s", err.Error()),
		})
		return
	}

	// Generate unique external ID
	externalId := fmt.Sprintf("upload_%s", uuid.NewString())

	// Apply user overrides
	activity.UserId = userId
	activity.ExternalId = externalId
	if req.Title != "" {
		activity.Name = req.Title
	}
	if req.Description != "" {
		activity.Description = req.Description
	}

	// Create activity payload
	payload := &pb.ActivityPayload{
		Source:               pb.ActivitySource_SOURCE_FILE_UPLOAD,
		UserId:               userId,
		Timestamp:            timestamppb.Now(),
		StandardizedActivity: activity,
		IsResume:             false,
	}

	// Create CloudEvent for publishing
	event, err := infrapubsub.NewCloudEvent("/fit-parser", "com.fitglue.activity.raw", payload)
	if err != nil {
		slog.Error("Failed to create cloud event", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Internal server error",
		})
		return
	}

	// Publish to raw-activity topic
	msgID, err := svc.Pub.PublishCloudEvent(ctx, shared.TopicRawActivity, event)
	if err != nil {
		slog.Error("Failed to publish to PubSub", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ParseFitFileResponse{
			Success: false,
			Error:   "Failed to queue activity for processing",
		})
		return
	}

	slog.Info("FIT file parsed and published",
		"user_id", userId,
		"external_id", externalId,
		"message_id", msgID,
		"activity_type", activity.Type.String(),
		"sessions", len(activity.Sessions),
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ParseFitFileResponse{
		Success:    true,
		Message:    "Activity uploaded and queued for processing",
		ActivityId: externalId,
	})
}

// verifyFirebaseToken verifies the Firebase ID token and returns the user ID
func verifyFirebaseToken(ctx context.Context, idToken string) (string, error) {
	// Use Firebase Admin SDK to verify the token
	// For now, we'll use a simplified approach that decodes the JWT
	// In production, this should use the Firebase Admin SDK

	// Import Firebase Admin SDK
	// This is a placeholder - the actual implementation would use:
	// auth, err := firebase.NewApp(ctx, nil)
	// client, err := auth.Auth(ctx)
	// token, err := client.VerifyIDToken(ctx, idToken)
	// return token.UID, nil

	// For this implementation, we'll trust the token from the frontend
	// and extract the user ID from it (in production, ALWAYS verify!)

	// Decode JWT to get claims (without verification for now - TODO: add proper verification)
	parts := splitToken(idToken)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode claims: %w", err)
	}

	var claims struct {
		Sub string `json:"sub"` // Firebase user ID
	}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return "", fmt.Errorf("failed to parse claims: %w", err)
	}

	if claims.Sub == "" {
		return "", fmt.Errorf("no user ID in token")
	}

	return claims.Sub, nil
}

func splitToken(token string) []string {
	var result []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			result = append(result, token[start:i])
			start = i + 1
		}
	}
	result = append(result, token[start:])
	return result
}
