package infra

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

// responseWriter is a minimal wrapper to capture the HTTP status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs HTTP requests and CloudEvents.
func LoggingMiddleware(logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()
		reqID := uuid.New().String()

		ceID := r.Header.Get("Ce-Id")
		ceType := r.Header.Get("Ce-Type")
		ceSource := r.Header.Get("Ce-Source")

		if ceID != "" {
			logger.Info(ctx, "Event Received",
				"req_id", reqID,
				"ce_id", ceID,
				"ce_type", ceType,
				"ce_source", ceSource,
			)
		} else {
			logger.Info(ctx, "HTTP Request Received",
				"req_id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
			)
		}

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		logger.Info(ctx, "Request Finished",
			"req_id", reqID,
			"status_code", rw.status,
			"duration_ms", duration.Milliseconds(),
		)
	})
}
