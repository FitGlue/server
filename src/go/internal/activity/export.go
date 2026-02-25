package activity

import (
	"context"

	pbsvc "github.com/fitglue/server/src/go/pkg/types/pb/services/activity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExportData generates a privacy compliance data export for the user.
func (s *Service) ExportData(ctx context.Context, req *pbsvc.ExportDataRequest) (*pbsvc.ExportDataResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// ARCHITECTURAL NOTE:
	// A full GDPR export requires aggregating data across User, Pipeline, Billing, and Activity stores.
	// Implementing this exclusively within ActivityService breaks domain boundaries because
	// ActivityService should not access the User database directly.
	//
	// This functionality should be orchestrated at the API Gateway layer (which can call all internal services)
	// or handled by a dedicated asynchronous Job Service.

	s.logger.Warn(ctx, "ExportData triggered but deferred due to cross-domain boundaries", "userId", req.UserId)
	return nil, status.Error(codes.Unimplemented, "ExportData requires cross-service orchestration and is deferred")
}
