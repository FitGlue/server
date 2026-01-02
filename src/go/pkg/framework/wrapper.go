package framework

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/execution"
	"github.com/ripixel/fitglue-server/src/go/pkg/types"
	"github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
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
		// 0. Log execution pending (IMMEDIATELY)
		// We don't have metadata yet, so pass empty options.
		// Note: We use a basic logger or fmt if needed, but LogPending handles DB interaction.
		execID, err := execution.LogPending(ctx, svc.DB, serviceName, execution.ExecutionOptions{})
		if err != nil {
			// We can't log nicely yet as we haven't set up the logger context fully,
			// but we proceed.
			// Ideally fmt.Printf or similar to stderr
		}

		// --- 1. CloudEvent Unwrapping (Pub/Sub Envelope) ---
		// Check if this is a Pub/Sub event wrapping a structured CloudEvent
		if e.Type() == "google.cloud.pubsub.topic.v1.messagePublished" {
			var msg types.PubSubMessage
			if err := e.DataAs(&msg); err == nil && len(msg.Message.Data) > 0 {
				// Try to unmarshal the inner data as a CloudEvent
				var innerEvent event.Event
				if err := json.Unmarshal(msg.Message.Data, &innerEvent); err == nil {
					// Check if it looks valid
					if innerEvent.Type() != "" && innerEvent.Source() != "" {
						e = innerEvent
					}
				}
			}
		}

		// Extract metadata
		var userID string
		var testRunID string
		var triggerType = e.Type()

		// Try to parse data to find user_id/test_run_id (best effort)
		var rawData map[string]interface{}
		if err := json.Unmarshal(e.Data(), &rawData); err == nil {
			if uid, ok := rawData["user_id"].(string); ok {
				userID = uid
			}
			if uid, ok := rawData["userId"].(string); ok {
				userID = uid
			}
			if tid, ok := rawData["test_run_id"].(string); ok {
				testRunID = tid
			}
			if tid, ok := rawData["testRunId"].(string); ok {
				testRunID = tid
			}
		}

		// For HTTP requests, or extensions on any event type
		if testRunID == "" {
			extensions := e.Extensions()
			if trid, ok := extensions["test_run_id"].(string); ok {
				testRunID = trid
			}
			if trid, ok := extensions["testrunid"].(string); ok {
				testRunID = trid
			}
		}

		// Setup Logger
		logger := bootstrap.NewLogger(serviceName, false)
		if testRunID != "" {
			logger = logger.With("test_run_id", testRunID)
		}
		if userID != "" {
			logger = logger.With("user_id", userID)
		}

		// If pending log failed earlier, log it now
		if err != nil {
			logger.Error("Failed to log execution pending", "error", err)
		} else {
			logger.Info("Execution pending logged", "execution_id", execID)
			logger = logger.With("execution_id", execID)
		}

		// Extract inputs for logging
		var inputs interface{}
		var rawInputs map[string]interface{}
		if err := e.DataAs(&rawInputs); err == nil {
			inputs = rawInputs
		} else {
			inputs = string(e.Data())
		}

		// Log execution start (handler ready) -> Now includes metadata updates
		startOpts := &execution.ExecutionOptions{
			UserID:      userID,
			TestRunID:   testRunID,
			TriggerType: triggerType,
		}
		if err := execution.LogStart(ctx, svc.DB, execID, inputs, startOpts); err != nil {
			logger.Warn("Failed to log execution start", "error", err)
		}
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
			if logErr := execution.LogFailure(ctx, svc.DB, execID, handlerErr, outputs); logErr != nil {
				logger.Warn("Failed to log execution failure", "error", logErr)
			}
			return handlerErr
		}

		logger.Info("Function completed successfully")

		// Check if outputs has a "status" field we should use
		customStatus := ""
		if outputsMap, ok := outputs.(map[string]interface{}); ok {
			if s, ok := outputsMap["status"].(string); ok {
				customStatus = s
			}
		}

		if customStatus != "" {
			var statusEnum pb.ExecutionStatus
			if val, ok := pb.ExecutionStatus_value[customStatus]; ok {
				statusEnum = pb.ExecutionStatus(val)
			} else if val, ok := pb.ExecutionStatus_value["STATUS_"+strings.ToUpper(customStatus)]; ok {
				statusEnum = pb.ExecutionStatus(val)
			} else {
				statusEnum = pb.ExecutionStatus_STATUS_UNKNOWN
				logger.Warn("Unknown custom status returned", "status", customStatus)
			}

			if logErr := execution.LogExecutionStatus(ctx, svc.DB, execID, statusEnum, outputs); logErr != nil {
				logger.Warn("Failed to log execution status", "error", logErr)
			}
		} else {
			if logErr := execution.LogSuccess(ctx, svc.DB, execID, outputs); logErr != nil {
				logger.Warn("Failed to log execution success", "error", logErr)
			}
		}

		return nil
	}
}
