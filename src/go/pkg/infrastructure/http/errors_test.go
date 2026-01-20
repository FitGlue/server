package httputil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseErrorResponse_Success(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Body:       http.NoBody,
	}

	err := ParseErrorResponse(resp)
	if err != nil {
		t.Errorf("Expected nil error for 200 response, got: %v", err)
	}
}

func TestParseErrorResponse_Error(t *testing.T) {
	body := `{"error": "Found invalid exercise template id"}`
	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    httptest.NewRequest("POST", "https://api.example.com/test", nil),
	}

	err := ParseErrorResponse(resp)
	if err == nil {
		t.Fatal("Expected error for 400 response")
	}

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("Expected *HTTPError, got %T", err)
	}

	if httpErr.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", httpErr.StatusCode)
	}

	if !strings.Contains(httpErr.Body, "invalid exercise template") {
		t.Errorf("Expected body to contain error message, got: %s", httpErr.Body)
	}

	if !strings.Contains(httpErr.Error(), "invalid exercise template") {
		t.Errorf("Expected Error() to contain body, got: %s", httpErr.Error())
	}
}

func TestParseErrorResponse_BodyRewrap(t *testing.T) {
	body := `{"error": "test"}`
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    httptest.NewRequest("GET", "https://api.example.com/test", nil),
	}

	_ = ParseErrorResponse(resp)

	// Body should be re-wrapped and readable
	rewrappedBody := make([]byte, 100)
	n, _ := resp.Body.Read(rewrappedBody)
	if string(rewrappedBody[:n]) != body {
		t.Errorf("Body not properly re-wrapped, got: %s", string(rewrappedBody[:n]))
	}
}

func TestWrapResponseError(t *testing.T) {
	body := `Access denied`
	resp := &http.Response{
		StatusCode: 403,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	err := WrapResponseError(resp, "API call failed")
	if err == nil {
		t.Fatal("Expected error")
	}

	expected := "API call failed (status 403): Access denied"
	if err.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, err.Error())
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 10) != "hello" {
		t.Error("Short string should not be truncated")
	}

	long := strings.Repeat("a", 600)
	truncated := truncate(long, 500)
	if len(truncated) != 503 { // 500 + "..."
		t.Errorf("Expected length 503, got %d", len(truncated))
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Error("Truncated string should end with ...")
	}
}
