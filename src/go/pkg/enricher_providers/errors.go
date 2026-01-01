package enricher_providers

import (
	"fmt"
	"time"
)

// RetryableError indicates that the enrichment failed due to a transient issue (e.g. data lag)
// and should be retried after a delay.
type RetryableError struct {
	Err        error
	RetryAfter time.Duration
	Reason     string
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error (after %v): %s: %v", e.RetryAfter, e.Reason, e.Err)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError creates a new RetryableError
func NewRetryableError(err error, after time.Duration, reason string) *RetryableError {
	return &RetryableError{
		Err:        err,
		RetryAfter: after,
		Reason:     reason,
	}
}
