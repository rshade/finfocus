// Package esctest serves the Pulumi ESC HTTP paths the esc-sdk calls.
package esctest

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"time"
)

// Variable is one environmentVariables entry.
type Variable struct {
	Value  any
	Secret bool
	// Rotate appends the open count to the value on every open.
	Rotate bool
}

// Environment is one org/project/name the server can open.
type Environment struct {
	Variables map[string]Variable
	// ETag is the value of the ETag header on HEAD.
	ETag string
	// Tags maps a revision tag to a revision number.
	Tags map[string]int
	// Status, when 400 or higher, is returned for every call to this environment.
	Status int
	// Message is the error model's message. An empty message still sends one
	// unless OmitMessage is set.
	Message string
	// OmitMessage sends a body with code and no message, which the SDK cannot decode.
	OmitMessage bool
	// Delay waits before the response and stops early when the request is canceled.
	Delay time.Duration
	// MalformedTrace returns a trace object that panics the esc-sdk.
	MalformedTrace bool
}

// Server is an httptest server for the ESC API under /api/esc.
type Server struct {
	URL   string
	token string

	http *httptest.Server

	mu       sync.Mutex
	envs     map[string]*Environment
	opens    map[string]int
	heads    map[string]int
	tagReads map[string]int
	sessions map[string][]byte
	nextID   int
	requests int
}

// New starts a server that requires Authorization: token <token>.
// envs is keyed by org/project/name. The map is copied.
func New(token string, envs map[string]*Environment) *Server {
	copied := make(map[string]*Environment, len(envs))
	maps.Copy(copied, envs)
	srv := &Server{
		token:    token,
		envs:     copied,
		opens:    map[string]int{},
		heads:    map[string]int{},
		tagReads: map[string]int{},
		sessions: map[string][]byte{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/esc/environments/{org}/{project}/{env}/open", srv.handleOpen)
	mux.HandleFunc("POST /api/esc/environments/{org}/{project}/{env}/versions/{version}/open", srv.handleOpenVersion)
	mux.HandleFunc("GET /api/esc/environments/{org}/{project}/{env}/open/{id}", srv.handleRead)
	mux.HandleFunc("HEAD /api/esc/environments/{org}/{project}/{env}", srv.handleHead)
	mux.HandleFunc("GET /api/esc/environments/{org}/{project}/{env}/versions/tags/{tag}", srv.handleTag)
	srv.http = httptest.NewServer(mux)
	srv.URL = srv.http.URL
	return srv
}

// Close shuts the server down.
func (s *Server) Close() { s.http.Close() }

// Opens returns how many open calls the environment has received.
func (s *Server) Opens(org, project, name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opens[refKey(org, project, name)]
}

// Heads returns how many HEAD calls the environment has received.
func (s *Server) Heads(org, project, name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.heads[refKey(org, project, name)]
}

// TagReads returns how many revision-tag reads the environment has received.
func (s *Server) TagReads(org, project, name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tagReads[refKey(org, project, name)]
}

// Requests returns how many ESC API calls this server has received.
func (s *Server) Requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

func (s *Server) note() {
	s.mu.Lock()
	s.requests++
	s.mu.Unlock()
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	s.note()
	s.open(w, r, r.PathValue("org"), r.PathValue("project"), r.PathValue("env"))
}

func (s *Server) handleOpenVersion(w http.ResponseWriter, r *http.Request) {
	s.note()
	s.open(w, r, r.PathValue("org"), r.PathValue("project"), r.PathValue("env"))
}

func (s *Server) open(w http.ResponseWriter, r *http.Request, org, project, name string) {
	if !s.authorize(w, r) {
		return
	}
	env := s.environment(org, project, name)
	if env == nil {
		writeMissing(w)
		return
	}
	if !s.wait(w, r, env) {
		return
	}
	if writeStatus(w, env) {
		s.addOpen(org, project, name)
		return
	}
	id, _ := s.snapshot(org, project, name, env)
	s.addOpen(org, project, name)
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	s.note()
	if !s.authorize(w, r) {
		return
	}
	org, project, name := r.PathValue("org"), r.PathValue("project"), r.PathValue("env")
	env := s.environment(org, project, name)
	if env == nil {
		writeMissing(w)
		return
	}
	if !s.wait(w, r, env) {
		return
	}
	if writeStatus(w, env) {
		return
	}
	s.mu.Lock()
	body := append([]byte(nil), s.sessions[r.PathValue("id")]...)
	s.mu.Unlock()
	if body == nil {
		writeMissing(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) handleHead(w http.ResponseWriter, r *http.Request) {
	s.note()
	if !s.authorize(w, r) {
		return
	}
	org, project, name := r.PathValue("org"), r.PathValue("project"), r.PathValue("env")
	env := s.environment(org, project, name)
	if env == nil {
		writeMissing(w)
		return
	}
	if !s.wait(w, r, env) {
		return
	}
	s.mu.Lock()
	s.heads[refKey(org, project, name)]++
	etag := env.ETag
	s.mu.Unlock()
	if writeStatus(w, env) {
		return
	}
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTag(w http.ResponseWriter, r *http.Request) {
	s.note()
	if !s.authorize(w, r) {
		return
	}
	org, project, name := r.PathValue("org"), r.PathValue("project"), r.PathValue("env")
	env := s.environment(org, project, name)
	if env == nil {
		writeMissing(w)
		return
	}
	if !s.wait(w, r, env) {
		return
	}
	s.mu.Lock()
	s.tagReads[refKey(org, project, name)]++
	revision, ok := env.Tags[r.PathValue("tag")]
	s.mu.Unlock()
	if writeStatus(w, env) {
		return
	}
	if !ok {
		writeMissing(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"revision": revision,
		"name":     r.PathValue("tag"),
	})
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") == "token "+s.token {
		return true
	}
	writeJSON(w, http.StatusUnauthorized, map[string]any{
		fieldMessage: "unauthorized",
		fieldCode:    http.StatusUnauthorized,
	})
	return false
}

func (s *Server) environment(org, project, name string) *Environment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.envs[refKey(org, project, name)]
}

func (s *Server) addOpen(org, project, name string) {
	s.mu.Lock()
	s.opens[refKey(org, project, name)]++
	s.mu.Unlock()
}

func (s *Server) snapshot(org, project, name string, env *Environment) (string, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	id := "open-" + strconv.Itoa(s.nextID)
	openN := s.opens[refKey(org, project, name)] + 1
	body := environmentBody(env, openN)
	s.sessions[id] = body
	return id, body
}

func (s *Server) wait(w http.ResponseWriter, r *http.Request, env *Environment) bool {
	if env.Delay <= 0 {
		return true
	}
	timer := time.NewTimer(env.Delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-r.Context().Done():
		w.WriteHeader(http.StatusGatewayTimeout)
		return false
	}
}

func environmentBody(env *Environment, openN int) []byte {
	vars := map[string]any{}
	for name, variable := range env.Variables {
		value := variable.Value
		if variable.Rotate {
			value = fmt.Sprintf("%v-%d", variable.Value, openN)
		}
		vars[name] = map[string]any{
			"value":  value,
			"secret": variable.Secret,
			"trace":  trace(env.MalformedTrace),
		}
	}
	body := map[string]any{
		"properties": map[string]any{
			"environmentVariables": map[string]any{
				"value": vars,
				"trace": map[string]any{},
			},
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return []byte(`{"properties":{}}`)
	}
	return encoded
}

func trace(malformed bool) map[string]any {
	if malformed {
		return map[string]any{}
	}
	return map[string]any{"def": map[string]any{"environment": "proj/env"}}
}

func writeStatus(w http.ResponseWriter, env *Environment) bool {
	if env.Status < http.StatusBadRequest {
		return false
	}
	if env.OmitMessage {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(env.Status)
		_, _ = fmt.Fprintf(w, `{"code":%d}`, env.Status)
		return true
	}
	message := env.Message
	if message == "" {
		message = "request failed"
	}
	writeJSON(w, env.Status, map[string]any{fieldMessage: message, fieldCode: env.Status})
	return true
}

func writeMissing(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]any{
		fieldMessage: "not found",
		fieldCode:    http.StatusNotFound,
	})
}

const (
	fieldMessage = "message"
	fieldCode    = "code"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		http.Error(w, `{"message":"encode","code":500}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

func refKey(org, project, name string) string {
	return org + "/" + project + "/" + name
}
