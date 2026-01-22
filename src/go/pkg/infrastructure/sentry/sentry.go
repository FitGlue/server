package sentry

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
)

type Config struct {
	DSN                string
	Environment        string
	Release            string
	ServerName         string
	TracesSampleRate   float64
	ProfilesSampleRate float64
}

// Init initializes Sentry for Go Cloud Functions.
// Safe to call multiple times - will only initialize once.
func Init(cfg Config, logger *slog.Logger) error {
	if cfg.DSN == "" {
		if logger != nil {
			logger.Warn("Sentry DSN not configured - error tracking disabled")
		}
		return nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:                cfg.DSN,
		Environment:        cfg.Environment,
		Release:            cfg.Release,
		ServerName:         cfg.ServerName,
		TracesSampleRate:   cfg.TracesSampleRate,
		ProfilesSampleRate: cfg.ProfilesSampleRate,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Filter out sensitive data
			if event.Request != nil {
				if event.Request.Headers != nil {
					delete(event.Request.Headers, "Authorization")
					delete(event.Request.Headers, "Cookie")
				}
			}
			return event
		},
	})

	if err != nil {
		if logger != nil {
			logger.Error("Failed to initialize Sentry", "error", err)
		}
		return fmt.Errorf("sentry init: %w", err)
	}

	if logger != nil {
		logger.Info("Sentry initialized", "environment", cfg.Environment, "release", cfg.Release)
	}

	return nil
}

// CaptureException captures an exception in Sentry with additional context.
func CaptureException(err error, context map[string]interface{}, logger *slog.Logger) {
	if err == nil {
		return
	}

	if context != nil {
		sentry.ConfigureScope(func(scope *sentry.Scope) {
			for key, value := range context {
				scope.SetContext(key, sentry.Context(map[string]interface{}{
					"value": value,
				}))
			}
		})
	}

	sentry.CaptureException(err)

	if logger != nil {
		logger.Debug("Exception captured in Sentry", "error", err.Error())
	}
}

// CaptureMessage captures a message in Sentry.
func CaptureMessage(message string, level sentry.Level, context map[string]interface{}, logger *slog.Logger) {
	if context != nil {
		sentry.ConfigureScope(func(scope *sentry.Scope) {
			for key, value := range context {
				scope.SetContext(key, sentry.Context(map[string]interface{}{
					"value": value,
				}))
			}
		})
	}

	sentry.CaptureMessage(message)

	if logger != nil {
		logger.Debug("Message captured in Sentry", "message", message, "level", level)
	}
}

// Flush waits for all events to be sent to Sentry.
// Call this before function termination to ensure events are sent.
func Flush(timeout time.Duration) bool {
	return sentry.Flush(timeout)
}

// RecoverAndCapture recovers from a panic and captures it in Sentry.
func RecoverAndCapture(logger *slog.Logger) {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("panic: %v", r)
		}
		CaptureException(err, nil, logger)
		Flush(2 * time.Second)
		panic(r) // Re-panic after capturing
	}
}
