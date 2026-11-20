package webui

import (
	"net/http"
	"net/url"
)

func validPluginRegistry(name, action string) bool {
	return name == "" || (action != pluginActionRemove && pluginSlug(name))
}

func (s *Server) handlePluginRegistries(w http.ResponseWriter, r *http.Request) {
	if s.session == nil || s.session.opts.PluginRegistries == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin registries unavailable")
		return
	}
	rows, err := s.session.opts.PluginRegistries(r.Context())
	if err != nil {
		s.writePluginError(w, err)
		return
	}
	display := make([]PluginRegistryRow, 0, len(rows))
	for _, row := range rows {
		row.URL = displayPluginRegistryURL(row.URL)
		display = append(display, row)
	}
	s.writePlainJSON(w, http.StatusOK, struct {
		Registries []PluginRegistryRow `json:"registries"`
	}{Registries: display})
}

func displayPluginRegistryURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}

func (s *Server) handlePluginRegistryAdd(w http.ResponseWriter, r *http.Request) {
	if s.session == nil || s.session.opts.PluginRegistryAdd == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin registry settings unavailable")
		return
	}
	var request PluginRegistryRequest
	if !s.decodeJSON(w, r, &request) {
		return
	}
	if err := s.session.opts.PluginRegistryAdd(r.Context(), request); err != nil {
		s.writePluginError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePluginRegistryRemove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !pluginSlug(name) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "invalid registry name")
		return
	}
	if s.session == nil || s.session.opts.PluginRegistryRemove == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin registry settings unavailable")
		return
	}
	if err := s.session.opts.PluginRegistryRemove(r.Context(), name); err != nil {
		s.writePluginError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
