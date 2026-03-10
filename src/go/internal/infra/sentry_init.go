package infra

import (
	"context"
	"log/slog"
	"os"

	sentryPkg "github.com/fitglue/server/src/go/pkg/infrastructure/sentry"
)

// InitSentry initializes the Sentry SDK using environment variables.
// Safe to call early in main() — if SENTRY_DSN is unset, Sentry is silently disabled.
func InitSentry() {
	dsn := os.Getenv("SENTRY_DSN")

	environment := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if environment == "" {
		environment = "fitglue-server-dev"
	}

	release := os.Getenv("SENTRY_RELEASE")
	if release == "" {
		release = os.Getenv("K_REVISION")
		if release == "" {
			release = "unknown"
		}
	}

	serverName := os.Getenv("K_SERVICE")

	tracesSampleRate := 0.1
	if environment == "fitglue-server-dev" {
		tracesSampleRate = 1.0
	}

	logger := NewLoggerWithComponent("sentry")

	if err := sentryPkg.Init(sentryPkg.Config{
		DSN:                dsn,
		Environment:        environment,
		Release:            release,
		ServerName:         serverName,
		TracesSampleRate:   tracesSampleRate,
		ProfilesSampleRate: tracesSampleRate,
	}, slog.Default()); err != nil {
		logger.Warn(context.Background(), "Sentry initialization failed", "error", err)
	}
}
