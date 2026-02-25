// nolint:proto-json
package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	pipelinepb "github.com/fitglue/server/src/go/pkg/types/pb/services/pipeline"
	"github.com/go-chi/chi/v5"
)

func (s *APIServer) registerPipelineRoutes(r chi.Router) {
	r.Get("/users/me/pipelines", s.handleListPipelines)
	r.Get("/users/me/pipelines/{id}", s.handleGetPipeline)
	r.Post("/users/me/pipelines", s.handleCreatePipeline)
	r.Put("/users/me/pipelines/{id}", s.handleUpdatePipeline)
	r.Delete("/users/me/pipelines/{id}", s.handleDeletePipeline)

	r.Get("/users/me/pipelines/{id}/runs", s.handleListPipelineRuns)
	r.Get("/users/me/pipelines/{id}/runs/{runId}", s.handleGetPipelineRun)

	r.Post("/users/me/pending-inputs/{inputId}/submit", s.handleSubmitInput)
	r.Post("/users/me/activities/{id}/repost", s.handleRepostActivity)
}

func (s *APIServer) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &pipelinepb.ListPipelinesRequest{
		UserId: token.UID,
	}

	res, err := s.pipelineSvc.ListPipelines(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &pipelinepb.GetPipelineRequest{
		UserId:     token.UID,
		PipelineId: chi.URLParam(r, "id"),
	}

	res, err := s.pipelineSvc.GetPipeline(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	var reqBody pipelinepb.CreatePipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody.Pipeline); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}
	reqBody.UserId = token.UID

	res, err := s.pipelineSvc.CreatePipeline(r.Context(), &reqBody)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	WriteJSON(w, res)
}

func (s *APIServer) handleUpdatePipeline(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	var reqBody pipelinepb.UpdatePipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody.Pipeline); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}
	reqBody.UserId = token.UID
	reqBody.PipelineId = chi.URLParam(r, "id")

	res, err := s.pipelineSvc.UpdatePipeline(r.Context(), &reqBody)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleDeletePipeline(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &pipelinepb.DeletePipelineRequest{
		UserId:     token.UID,
		PipelineId: chi.URLParam(r, "id"),
	}

	_, err := s.pipelineSvc.DeletePipeline(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) handleListPipelineRuns(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	pipelineID := chi.URLParam(r, "id")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	pageToken := r.URL.Query().Get("page_token")

	req := &pipelinepb.ListPipelineRunsRequest{
		UserId:     token.UID,
		PipelineId: pipelineID,
		Limit:      int32(limit),
		PageToken:  pageToken,
	}

	res, err := s.pipelineSvc.ListPipelineRuns(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleGetPipelineRun(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &pipelinepb.GetPipelineRunRequest{
		UserId: token.UID,
		RunId:  chi.URLParam(r, "runId"),
	}

	res, err := s.pipelineSvc.GetPipelineRun(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleSubmitInput(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	var reqBody pipelinepb.SubmitInputRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}

	reqBody.UserId = token.UID
	reqBody.PendingInputId = chi.URLParam(r, "inputId")

	_, err := s.pipelineSvc.SubmitInput(r.Context(), &reqBody)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) handleRepostActivity(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &pipelinepb.RepostActivityRequest{
		UserId:     token.UID,
		ActivityId: chi.URLParam(r, "id"),
	}

	_, err := s.pipelineSvc.RepostActivity(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
