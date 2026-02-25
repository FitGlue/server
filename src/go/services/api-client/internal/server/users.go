// nolint:proto-json
package server

import (
	"encoding/json"
	"net/http"

	infraps "github.com/fitglue/server/src/go/pkg/infrastructure/pubsub"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
)

// getUserToken extracts the verified Firebase token from the request context
func getUserToken(r *http.Request) *auth.Token {
	token, ok := r.Context().Value(userContextKey).(*auth.Token)
	if !ok {
		return nil
	}
	return token
}

func (s *APIServer) registerUserRoutes(r chi.Router) {
	r.Get("/users/me", s.handleGetProfile)
	r.Put("/users/me", s.handleUpdateProfile)

	r.Get("/users/me/integrations", s.handleListIntegrations)
	r.Get("/users/me/integrations/{provider}", s.handleGetIntegration)
	r.Put("/users/me/integrations/{provider}", s.handleSetIntegration)
	r.Delete("/users/me/integrations/{provider}", s.handleDeleteIntegration)

	r.Get("/users/me/notification-prefs", s.handleGetNotificationPrefs)
	r.Put("/users/me/notification-prefs", s.handleUpdateNotificationPrefs)

	r.Get("/users/me/counters", s.handleListCounters)
	r.Put("/users/me/counters/{name}", s.handleUpdateCounter)

	// Connection actions (trigger sync, resync, etc)
	r.Post("/users/me/connections/{provider}/actions", s.handleConnectionActions)
}

func (s *APIServer) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	req := &userpb.GetProfileRequest{
		UserId: token.UID,
	}

	profile, err := s.userService.GetProfile(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, profile)
}

func (s *APIServer) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	var reqBody userpb.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody.Profile); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}

	reqBody.UserId = token.UID

	profile, err := s.userService.UpdateProfile(r.Context(), &reqBody)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, profile)
}

func (s *APIServer) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)

	req := &userpb.ListIntegrationsRequest{
		UserId: token.UID,
	}

	// Wait, ListIntegrations is in userpb, let me double check the proto definitions.
	// user.proto: "rpc ListIntegrations(ListIntegrationsRequest) returns (ListIntegrationsResponse);"
	res, err := s.userService.ListIntegrations(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleGetIntegration(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	provider := chi.URLParam(r, "provider")

	req := &userpb.GetIntegrationRequest{
		UserId:   token.UID,
		Provider: provider,
	}

	res, err := s.userService.GetIntegration(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleSetIntegration(w http.ResponseWriter, r *http.Request) {
	// token := getUserToken(r)
	// provider := chi.URLParam(r, "provider")

	// Request parsing for integration struct...
	WriteError(w, statusError(http.StatusNotImplemented, "not implemented natively yet"))
}

func (s *APIServer) handleDeleteIntegration(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	provider := chi.URLParam(r, "provider")

	req := &userpb.DeleteIntegrationRequest{
		UserId:   token.UID,
		Provider: provider,
	}

	res, err := s.userService.DeleteIntegration(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleGetNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	req := &userpb.GetNotificationPrefsRequest{
		UserId: token.UID,
	}
	res, err := s.userService.GetNotificationPrefs(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, res)
}

func (s *APIServer) handleUpdateNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	var req userpb.UpdateNotificationPrefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req.Prefs); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}
	req.UserId = token.UID
	res, err := s.userService.UpdateNotificationPrefs(r.Context(), &req)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, res)
}

func (s *APIServer) handleListCounters(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	req := &userpb.ListCountersRequest{
		UserId: token.UID,
	}
	res, err := s.userService.ListCounters(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, res)
}

func (s *APIServer) handleUpdateCounter(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	// name := chi.URLParam(r, "name")

	var req userpb.UpdateCounterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}
	req.UserId = token.UID
	// TODO: verify if name is part of the request body or path

	res, err := s.userService.UpdateCounter(r.Context(), &req)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, res)
}

func (s *APIServer) handleConnectionActions(w http.ResponseWriter, r *http.Request) {
	token := getUserToken(r)
	if token == nil {
		WriteError(w, statusError(http.StatusUnauthorized, "missing user context"))
		return
	}

	provider := chi.URLParam(r, "provider")
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, statusError(http.StatusBadRequest, "invalid request body"))
		return
	}

	if req.Action == "sync" {
		_, err := s.userService.GetIntegration(r.Context(), &userpb.GetIntegrationRequest{
			UserId:   token.UID,
			Provider: provider,
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				WriteError(w, statusError(http.StatusNotFound, "integration not found"))
			} else {
				WriteError(w, err)
			}
			return
		}

		evt, err := infraps.NewCloudEvent("api-client", "fitglue.connection.action", map[string]string{
			"action":   "sync_provider",
			"provider": provider,
			"user_id":  token.UID,
		})
		if err != nil {
			WriteError(w, statusError(http.StatusInternalServerError, "failed to create sync event"))
			return
		}

		// Use the correct topic or whatever fallback is required.
		_, err = s.publisher.PublishCloudEvent(r.Context(), "fitglue-activities-commands", evt)
		if err != nil {
			WriteError(w, statusError(http.StatusInternalServerError, "failed to trigger sync"))
			return
		}

		w.WriteHeader(http.StatusAccepted)
		return
	} else if req.Action == "clear" {
		_, err := s.userService.DeleteIntegration(r.Context(), &userpb.DeleteIntegrationRequest{
			UserId:   token.UID,
			Provider: provider,
		})
		if err != nil {
			WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	WriteError(w, statusError(http.StatusBadRequest, "unknown action"))
}

// statusError is a helper for manually generating an error satisfying gRPC status layout
func statusError(code int, msg string) error {
	// Simple wrapper for non-gRPC errors to use WriteError
	return &CustomError{HTTPCode: code, Msg: msg}
}

// CustomError helps map generic HTTP errors into our WriteError handler
type CustomError struct {
	HTTPCode int
	Msg      string
}

func (e *CustomError) Error() string {
	return e.Msg
}
