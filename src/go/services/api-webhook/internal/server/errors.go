package server

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// APIError represents the standard JSON error shape returned to clients
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError converts a generic error or gRPC status error into the appropriate HTTP response
// mapped according to gRPC codes.
func WriteError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	var httpCode int
	var errCode string

	if customErr, isCustom := err.(*CustomError); isCustom {
		httpCode = customErr.HTTPCode
		errCode = "CLIENT_ERROR"
	} else if !ok {
		// Not a gRPC error
		httpCode = http.StatusInternalServerError
		errCode = "INTERNAL_ERROR"
	} else {
		// Map gRPC code to HTTP status code
		switch st.Code() {
		case codes.NotFound:
			httpCode = http.StatusNotFound
			errCode = "NOT_FOUND"
		case codes.InvalidArgument:
			httpCode = http.StatusBadRequest
			errCode = "INVALID_ARGUMENT"
		case codes.PermissionDenied:
			httpCode = http.StatusForbidden
			errCode = "PERMISSION_DENIED"
		case codes.Unauthenticated:
			httpCode = http.StatusUnauthorized
			errCode = "UNAUTHENTICATED"
		case codes.AlreadyExists:
			httpCode = http.StatusConflict
			errCode = "ALREADY_EXISTS"
		case codes.Unimplemented:
			httpCode = http.StatusNotImplemented
			errCode = "NOT_IMPLEMENTED"
		case codes.Unavailable:
			httpCode = http.StatusServiceUnavailable
			errCode = "UNAVAILABLE"
		case codes.FailedPrecondition:
			httpCode = http.StatusPreconditionFailed
			errCode = "FAILED_PRECONDITION"
		case codes.ResourceExhausted:
			httpCode = http.StatusTooManyRequests
			errCode = "RESOURCE_EXHAUSTED"
		case codes.DeadlineExceeded:
			httpCode = http.StatusGatewayTimeout
			errCode = "DEADLINE_EXCEEDED"
		default:
			httpCode = http.StatusInternalServerError
			errCode = "INTERNAL_ERROR"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)

	msg := "An unknown error occurred"
	if err != nil {
		if customErr, isCustom := err.(*CustomError); isCustom {
			msg = customErr.Msg
		} else if st != nil {
			msg = st.Message()
		} else {
			msg = err.Error()
		}
	}

	apiErr := APIError{
		Code:    errCode,
		Message: msg,
	}

	_ = json.NewEncoder(w).Encode(apiErr)
}
