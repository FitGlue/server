package framework

// TerminalError represents an error that won't succeed on retry (e.g. malformed data).
// By returning this from a Cloud Function handler, the framework will still log
// the failure and report it to Sentry, but it will ACK the message to Pub/Sub
// (by returning a nil error to the CloudEvents SDK) to prevent infinite retries.
type TerminalError struct {
	Message string
}

func (e *TerminalError) Error() string {
	return e.Message
}

func NewTerminalError(msg string) *TerminalError {
	return &TerminalError{Message: msg}
}
