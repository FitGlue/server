package infra

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingUnaryInterceptor returns a new unary server interceptor that logs the start and end of gRPC calls.
func LoggingUnaryInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		reqID := uuid.New().String()

		logger.Info(ctx, "gRPC Request Received", "method", info.FullMethod, "req_id", reqID)

		resp, err := handler(ctx, req)
		duration := time.Since(start)

		if err != nil {
			st, _ := status.FromError(err)
			// Using Error-level might be noisy for standard errors (e.g. NotFound, Unauthenticated).
			// Depending on conventions, Info/Warn is safer to avoid sentry spam, but we'll use Info and log the error context.
			logger.Info(ctx, "gRPC Request Finished (Failed)",
				"method", info.FullMethod,
				"req_id", reqID,
				"duration_ms", duration.Milliseconds(),
				"grpc_code", st.Code().String(),
				"error_msg", err.Error(),
			)
		} else {
			logger.Info(ctx, "gRPC Request Finished",
				"method", info.FullMethod,
				"req_id", reqID,
				"duration_ms", duration.Milliseconds(),
				"grpc_code", "OK",
			)
		}

		return resp, err
	}
}
