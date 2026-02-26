package server

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	pipelinepb "github.com/fitglue/server/src/go/pkg/types/pb/services/pipeline"
)

// registerRepostRoutes adds the 3 repost variant routes that the web frontend calls.
// These all delegate to the same PipelineService.RepostActivity RPC with a mode discriminator.
func (s *APIServer) registerRepostRoutes(r chi.Router) {
	r.Post("/repost/missed-destination", s.handleRepostMissedDestination)
	r.Post("/repost/retry-destination", s.handleRepostRetryDestination)
	r.Post("/repost/full-pipeline", s.handleRepostFullPipeline)
}

// handleGetRecaptchaConfig returns the reCAPTCHA v3 site key from environment config.
// This is an unauthenticated endpoint used by the registration page.
func (s *APIServer) handleGetRecaptchaConfig(w http.ResponseWriter, r *http.Request) {
	siteKey := os.Getenv("RECAPTCHA_SITE_KEY")
	if siteKey == "" {
		WriteError(w, statusError(http.StatusNotFound, "reCAPTCHA not configured"))
		return
	}
	WriteJSON(w, map[string]string{"siteKey": siteKey})
}

// handleIntegrationRequest handles unauthenticated integration request form submissions
// from the contact page. Stores the request for review.
func (s *APIServer) handleIntegrationRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Integration string `json:"integration"`
		Email       string `json:"email,omitempty"`
		WebsiteURL  string `json:"website_url,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}

	// Honeypot check: if website_url is set, it's a bot
	if body.WebsiteURL != "" {
		w.WriteHeader(http.StatusOK)
		WriteJSON(w, map[string]string{"status": "ok"})
		return
	}

	if body.Integration == "" {
		WriteError(w, statusError(http.StatusBadRequest, "integration name is required"))
		return
	}

	// Log the request (storage can be added later)
	s.logger.Info(r.Context(), "integration request received", "integration", body.Integration, "email", body.Email)

	w.WriteHeader(http.StatusOK)
	WriteJSON(w, map[string]string{"status": "ok"})
}

// repostRequestBody is the JSON shape sent by the frontend repost functions
type repostRequestBody struct {
	ActivityID  string `json:"activityId"`
	Destination string `json:"destination,omitempty"`
}

func (s *APIServer) handleRepostMissedDestination(w http.ResponseWriter, r *http.Request) {
	s.handleRepostWithMode(w, r, "missed-destination")
}

func (s *APIServer) handleRepostRetryDestination(w http.ResponseWriter, r *http.Request) {
	s.handleRepostWithMode(w, r, "retry-destination")
}

func (s *APIServer) handleRepostFullPipeline(w http.ResponseWriter, r *http.Request) {
	s.handleRepostWithMode(w, r, "full-pipeline")
}

func (s *APIServer) handleRepostWithMode(w http.ResponseWriter, r *http.Request, mode string) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	var body repostRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}

	if body.ActivityID == "" {
		WriteError(w, statusError(http.StatusBadRequest, "activityId is required"))
		return
	}

	req := &pipelinepb.RepostActivityRequest{
		UserId:      token.UID,
		ActivityId:  body.ActivityID,
		Mode:        mode,
		Destination: body.Destination,
	}

	_, err := s.pipelineSvc.RepostActivity(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, map[string]interface{}{
		"success": true,
		"message": "Repost initiated",
	})
}
