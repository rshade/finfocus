package webui

import "net/http"

// registerStacks adds the stack list and activation routes. They exist on the
// root server only: a stack's own tree has no knowledge of its siblings.
func (s *Server) registerStacks(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stacks", s.handleStackList)
	mux.HandleFunc("POST /api/stacks/{name}/activate", s.handleStackActivate)
}

// handleStackList answers with plain JSON, not RedactJSON: the body holds only
// stack names (restricted to [A-Za-z0-9_.-] at registration) and fixed state
// words.
func (s *Server) handleStackList(w http.ResponseWriter, _ *http.Request) {
	s.writePlainJSON(w, http.StatusOK, struct {
		Stacks []StackEntry `json:"stacks"`
	}{Stacks: s.stackEntries()})
}

func (s *Server) handleStackActivate(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.activateStack(r.PathValue("name"))
	if !ok {
		s.writeError(w, http.StatusNotFound, "not_found", "unknown stack")
		return
	}
	s.writePlainJSON(w, http.StatusAccepted, entry)
}
