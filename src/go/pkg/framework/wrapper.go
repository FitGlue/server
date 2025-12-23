package framework

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/ripixel/fitglue-server/src/go/pkg/execution"
	"github.com/ripixel/fitglue-server/src/go/pkg/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
)

// FrameworkContext contains dependencies injected by the framework
// Similar to TypeScript's FrameworkContext
type FrameworkContext struct {
	Service     *bootstrap.Service
	Logger      *slog.Logger
	ExecutionID string
}

// HandlerFunc is the signature for a cloud function handler
// Similar to TypeScript's FrameworkHandler
type HandlerFunc func(ctx context.Context, e event.Event, fwCtx *FrameworkContext) (interface{}, error)

// WrapCloudEvent wraps a handler with automatic execution logging
// Handles both HTTP and Pub/Sub triggers
// Similar to TypeScript's createCloudFunction
func WrapCloudEvent(serviceName string, svc *bootstrap.Service, handler HandlerFunc) func(context.Context, event.Event) error {
	return func(ctx context.Context, e event.Event) error {
		// Extract metadata from event
		userID, testRunID := extractEventMetadata(e)

		// Determine trigger type
		triggerType := "pubsub"
		if e.Type() == "google.cloud.functions.http" {
			triggerType = "http"
		}

		// Determine log level from env
		logLevelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
		var logLevel slog.Level
		switch logLevelStr {
		case "debug":
			logLevel = slog.LevelDebug
		case "info":
			logLevel = slog.LevelInfo
		case "warn":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		default:
			logLevel = slog.LevelInfo
		}

		// Create base logger with configured level
		opts := &slog.HandlerOptions{
			Level: logLevel,
		}
		logger := slog.New(slog.NewJSONHandler(os.Stderr, opts)).With("service", serviceName)
		if userID != "" {
			logger = logger.With("user_id", userID)
		}

		// Log execution start
		execID, err := execution.LogStart(ctx, svc.DB, serviceName, execution.ExecutionOptions{
			UserID:      userID,
			TestRunID:   testRunID,
			TriggerType: triggerType,
		})
		if err != nil {
			logger.Error("Failed to log execution start", "error", err)
			// Continue anyway - don't fail the function just because logging failed
		}

		// Add execution ID to logger
		logger = logger.With("execution_id", execID)
		logger.Info("Function started")

		// Create framework context
		fwCtx := &FrameworkContext{
			Service:     svc,
			Logger:      logger,
			ExecutionID: execID,
		}

		// Execute handler
		outputs, handlerErr := handler(ctx, e, fwCtx)

		// Log execution result
		if handlerErr != nil {
			logger.Error("Function failed", "error", handlerErr)
			if logErr := execution.LogFailure(ctx, svc.DB, execID, handlerErr); logErr != nil {
				logger.Warn("Failed to log execution failure", "error", logErr)
			}
			return handlerErr
		}

		logger.Info("Function completed successfully")
		if logErr := execution.LogSuccess(ctx, svc.DB, execID, outputs); logErr != nil {
			logger.Warn("Failed to log execution success", "error", logErr)
		}

		return nil
	}
}

// extractEventMetadata extracts user_id and test_run_id from the event
// Handles both Pub/Sub messages and HTTP requests
func extractEventMetadata(e event.Event) (userID string, testRunID string) {
	// Try to extract from Pub/Sub message
	var msg types.PubSubMessage
	if err := e.DataAs(&msg); err == nil {
		// Try to parse the message data to extract user_id
		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Message.Data, &payload); err == nil {
			if uid, ok := payload["user_id"].(string); ok {
				userID = uid
			}
			if uid, ok := payload["userId"].(string); ok {
				userID = uid
			}
		}

		// Extract test_run_id from Pub/Sub message attributes
		if msg.Message.Attributes != nil {
			if trid, ok := msg.Message.Attributes["test_run_id"]; ok {
				testRunID = trid
			}
		}
	}

	// For HTTP requests, check CloudEvent extensions
	// (HTTP headers are mapped to extensions by Functions Framework)
	if testRunID == "" {
		extensions := e.Extensions()
		if trid, ok := extensions["test_run_id"].(string); ok {
			testRunID = trid
		}
		if trid, ok := extensions["testrunid"].(string); ok {
			testRunID = trid
		}
	}

	return userID, testRunID
}
