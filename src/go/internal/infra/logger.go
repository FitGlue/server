package infra

import (
	"context"
	"log/slog"
	"os"

	sentryPkg "github.com/fitglue/server/src/go/pkg/infrastructure/sentry"
)

// Logger provides a structured logging interface, wrapping slog.
type Logger interface {
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
	With(args ...any) Logger
}

// NewLogger creates a new default structured logger.
// The handler chain is: JSONHandler → SentryHandler
// Error-level logs are automatically captured by Sentry (if initialized).
func NewLogger() Logger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	sentryHandler := sentryPkg.NewSentryHandler(jsonHandler)
	return &slogger{
		logger: slog.New(sentryHandler),
	}
}

type slogger struct {
	logger *slog.Logger
}

func (l *slogger) Debug(ctx context.Context, msg string, args ...any) {
	l.logger.DebugContext(ctx, msg, args...)
}

func (l *slogger) Info(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, args...)
}

func (l *slogger) Warn(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, args...)
}

func (l *slogger) Error(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
}

func (l *slogger) With(args ...any) Logger {
	return &slogger{
		logger: l.logger.With(args...),
	}
}
