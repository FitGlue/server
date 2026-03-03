// nolint:proto-json
package server

import (
	"encoding/json"
	"net/http"

	billingpb "github.com/fitglue/server/src/go/pkg/types/pb/services/billing"
	"github.com/go-chi/chi/v5"
)

func (s *APIServer) registerBillingRoutes(r chi.Router) {
	r.Get("/billing/subscription", s.handleGetSubscription)
	r.Post("/billing/checkout", s.handleCreateCheckoutSession)
	r.Post("/billing/cancel", s.handleCancelSubscription)
	r.Get("/billing/tier", s.handleGetTierStatus)
	r.Post("/billing/trial", s.handleStartTrial)
	r.Post("/billing/portal", s.handleCreateBillingPortal)
}

func (s *APIServer) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &billingpb.GetSubscriptionRequest{
		UserId: token.UID,
	}

	res, err := s.billingService.GetSubscription(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleCreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	var req billingpb.CreateCheckoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}
	req.UserId = token.UID

	res, err := s.billingService.CreateCheckoutSession(r.Context(), &req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleCancelSubscription(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	var req billingpb.CancelSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}
	req.UserId = token.UID

	res, err := s.billingService.CancelSubscription(r.Context(), &req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleGetTierStatus(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &billingpb.GetTierStatusRequest{
		UserId: token.UID,
	}

	res, err := s.billingService.GetTierStatus(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleStartTrial(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &billingpb.StartTrialRequest{
		UserId: token.UID,
	}

	res, err := s.billingService.StartTrial(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	WriteJSON(w, res)
}

func (s *APIServer) handleCreateBillingPortal(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	var body struct {
		ReturnURL string `json:"return_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Default return URL if body is empty/invalid
		body.ReturnURL = ""
	}

	req := &billingpb.CreateBillingPortalSessionRequest{
		UserId:    token.UID,
		ReturnUrl: body.ReturnURL,
	}

	res, err := s.billingService.CreateBillingPortalSession(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}
