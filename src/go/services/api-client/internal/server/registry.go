package server

import (
	"net/http"

	registrypb "github.com/fitglue/server/src/go/pkg/types/pb/services/registry"
	"github.com/go-chi/chi/v5"
)

func (s *APIServer) registerRegistryRoutes(r chi.Router) {
	// Note: Registry routes are typically public or require different auth
	// But according to the task list, they are mounted under /api/registry and proxy to RegistryService.
	r.Get("/registry/plugins", s.handleListPlugins)
	r.Get("/registry/plugins/{id}", s.handleGetPlugin)
	r.Get("/registry/plugins/{id}/icon", s.handleGetPluginIcon)
	r.Get("/registry/categories", s.handleListCategories)
	r.Get("/registry/sources", s.handleListSources)
}

func (s *APIServer) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	req := &registrypb.ListPluginsRequest{
		Category: r.URL.Query().Get("category"),
	}

	res, err := s.registrySvc.ListPlugins(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	req := &registrypb.GetPluginRequest{
		PluginId: chi.URLParam(r, "id"),
	}

	res, err := s.registrySvc.GetPlugin(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleGetPluginIcon(w http.ResponseWriter, r *http.Request) {
	req := &registrypb.GetPluginIconRequest{
		PluginId: chi.URLParam(r, "id"),
	}

	res, err := s.registrySvc.GetPluginIcon(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	// For icons, we need to write the raw bytes and set the content type
	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=86400") // 1 day cache
	_, _ = w.Write(res.IconData)
}

func (s *APIServer) handleListCategories(w http.ResponseWriter, r *http.Request) {
	req := &registrypb.ListCategoriesRequest{}

	res, err := s.registrySvc.ListCategories(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleListSources(w http.ResponseWriter, r *http.Request) {
	req := &registrypb.ListSourcesRequest{}

	res, err := s.registrySvc.ListSources(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}

func (s *APIServer) handleGetPluginRegistry(w http.ResponseWriter, r *http.Request) {
	req := &registrypb.GetPluginRegistryRequest{}

	res, err := s.registrySvc.GetPluginRegistry(r.Context(), req)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, res)
}
