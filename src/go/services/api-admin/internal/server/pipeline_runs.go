package server

import (
	"net/http"
	"strconv"

	pipelinepb "github.com/fitglue/server/src/go/pkg/types/pb/services/pipeline"
)

func (s *APIServer) handleAdminPipelineRuns(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	source := r.URL.Query().Get("source")
	userID := r.URL.Query().Get("user_id")
	pageToken := r.URL.Query().Get("page_token")

	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = int32(parsed)
		}
	}

	res, err := s.pipelineSvc.AdminListPipelineRuns(r.Context(), &pipelinepb.AdminListPipelineRunsRequest{
		Status:    status,
		Source:    source,
		UserId:    userID,
		Limit:     limit,
		PageToken: pageToken,
	})
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}
