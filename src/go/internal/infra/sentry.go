package infra

import "context"

// ErrorReporter defines an interface for capturing panics and reporting errors (e.g. Sentry).
type ErrorReporter interface {
	CaptureException(ctx context.Context, err error)
	CaptureMessage(ctx context.Context, msg string)
	Flush(timeout int) bool
}
