// Package httputil provides HTTP error handling utilities.
package httputil

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// MaxErrorBodySize is the maximum size of error body to include in error messages
const MaxErrorBodySize = 500

// HTTPError represents an HTTP error with status code and response body
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	URL        string
}

func (e *HTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("%s (status %d): %s", e.Status, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("%s (status %d)", e.Status, e.StatusCode)
}

// truncate truncates a string to maxLen, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ParseErrorResponse checks if the response is an error (4xx/5xx) and returns
// a rich HTTPError containing the response body. Returns nil for success responses.
// The response body is re-wrapped so the caller can still read it.
func ParseErrorResponse(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Re-wrap body so caller can still read it if needed
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	bodyStr := ""
	if err == nil && len(bodyBytes) > 0 {
		bodyStr = truncate(string(bodyBytes), MaxErrorBodySize)
	}

	return &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     http.StatusText(resp.StatusCode),
		Body:       bodyStr,
		URL:        resp.Request.URL.String(),
	}
}

// WrapResponseError reads the response body and returns a formatted error.
// Unlike ParseErrorResponse, this does not re-wrap the body (for simple error cases).
func WrapResponseError(resp *http.Response, message string) error {
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	bodyStr := truncate(string(bodyBytes), MaxErrorBodySize)
	if bodyStr != "" {
		return fmt.Errorf("%s (status %d): %s", message, resp.StatusCode, bodyStr)
	}
	return fmt.Errorf("%s (status %d)", message, resp.StatusCode)
}
