package errors

import (
	"errors"
	"testing"
)

// --- New ---

func TestNew_CreatesError(t *testing.T) {
	err := New(CodeUserNotFound, "user not found")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Code != CodeUserNotFound {
		t.Errorf("expected code %v, got %v", CodeUserNotFound, err.Code)
	}
	if err.Message != "user not found" {
		t.Errorf("expected message 'user not found', got %q", err.Message)
	}
	if err.Retryable {
		t.Error("expected non-retryable error from New()")
	}
}

func TestNewRetryable_CreatesRetryableError(t *testing.T) {
	err := NewRetryable(CodeStorageError, "storage failure")
	if !err.Retryable {
		t.Error("expected retryable error from NewRetryable()")
	}
}

// --- Error (string representation) ---

func TestFitGlueError_Error_WithoutCause(t *testing.T) {
	err := New(CodeUserNotFound, "not found")
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
	// Should contain code and message
	if len(msg) < 5 {
		t.Errorf("error string too short: %q", msg)
	}
}

func TestFitGlueError_Error_WithCause(t *testing.T) {
	cause := errors.New("underlying failure")
	err := New(CodeStorageError, "storage failed").WithCause(cause)
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message with cause")
	}
}

// --- Wrap ---

func TestWrap_PreservesCode(t *testing.T) {
	cause := errors.New("root cause")
	err := Wrap(cause, CodeEnricherFailed, "enricher failed")
	if err.Code != CodeEnricherFailed {
		t.Errorf("expected code %v, got %v", CodeEnricherFailed, err.Code)
	}
	if err.Cause != cause {
		t.Error("expected cause to be preserved")
	}
}

func TestWrapRetryable_IsRetryable(t *testing.T) {
	cause := errors.New("timeout")
	err := WrapRetryable(cause, CodeTimeoutError, "timed out")
	if !err.Retryable {
		t.Error("expected retryable error from WrapRetryable()")
	}
}

// --- Unwrap ---

func TestFitGlueError_Unwrap_NilCause(t *testing.T) {
	err := New(CodeUserNotFound, "not found")
	if err.Unwrap() != nil {
		t.Error("expected nil Unwrap() for error without cause")
	}
}

func TestFitGlueError_Unwrap_WithCause(t *testing.T) {
	cause := errors.New("root")
	err := Wrap(cause, CodeInternalError, "internal")
	if err.Unwrap() != cause {
		t.Error("expected Unwrap() to return the original cause")
	}
}

// --- WithCause ---

func TestWithCause_ChainsCause(t *testing.T) {
	cause := errors.New("db error")
	err := New(CodeStorageError, "storage failed").WithCause(cause)
	if err.Cause != cause {
		t.Error("expected cause to be set via WithCause")
	}
}

// --- WithMessage ---

func TestWithMessage_ReplacesMessage(t *testing.T) {
	err := New(CodeUserNotFound, "original").WithMessage("updated message")
	if err.Message != "updated message" {
		t.Errorf("expected 'updated message', got %q", err.Message)
	}
}

// --- WithMetadata ---

func TestWithMetadata_AddsKey(t *testing.T) {
	err := New(CodeValidationError, "invalid input").WithMetadata("field", "email")
	if err.Metadata["field"] != "email" {
		t.Errorf("expected metadata key 'field'='email', got %q", err.Metadata["field"])
	}
}

func TestWithMetadata_PreservesExisting(t *testing.T) {
	err := New(CodeValidationError, "invalid").
		WithMetadata("field", "email").
		WithMetadata("reason", "format")
	if err.Metadata["field"] != "email" || err.Metadata["reason"] != "format" {
		t.Errorf("expected both metadata keys preserved, got %v", err.Metadata)
	}
}

// --- IsRetryable ---

func TestIsRetryable_NilError(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsRetryable_NonRetryable(t *testing.T) {
	err := New(CodeUserNotFound, "not found")
	if IsRetryable(err) {
		t.Error("expected false for non-retryable error")
	}
}

func TestIsRetryable_RetryableError(t *testing.T) {
	err := NewRetryable(CodeStorageError, "storage error")
	if !IsRetryable(err) {
		t.Error("expected true for retryable error")
	}
}

func TestIsRetryable_NonFitGlueError(t *testing.T) {
	err := errors.New("plain error")
	if IsRetryable(err) {
		t.Error("expected false for non-FitGlue error")
	}
}

// --- GetCode ---

func TestGetCode_NilError(t *testing.T) {
	code := GetCode(nil)
	if code != "" {
		t.Errorf("expected empty code for nil error, got %q", code)
	}
}

func TestGetCode_FitGlueError(t *testing.T) {
	err := New(CodeEnricherFailed, "failed")
	code := GetCode(err)
	if code != CodeEnricherFailed {
		t.Errorf("expected %v, got %v", CodeEnricherFailed, code)
	}
}

func TestGetCode_NonFitGlueError(t *testing.T) {
	err := errors.New("plain error")
	code := GetCode(err)
	// GetCode returns CodeInternalError as fallback for non-FitGlue errors
	if code != CodeInternalError {
		t.Errorf("expected CodeInternalError fallback for non-FitGlue error, got %q", code)
	}
}

// --- Standard error interface compatibility ---

func TestFitGlueError_ImplementsStdError(t *testing.T) {
	var stdErr error = New(CodeInternalError, "internal")
	if stdErr.Error() == "" {
		t.Error("FitGlueError should implement standard error interface")
	}
}

func TestErrors_ErrorSignatureCompatibility(t *testing.T) {
	// Ensure wrapped errors are unwrappable via standard library
	cause := errors.New("root cause")
	wrapped := Wrap(cause, CodeInternalError, "wrapper")
	if !errors.Is(wrapped, cause) {
		t.Error("expected errors.Is to work through Unwrap chain")
	}
}
