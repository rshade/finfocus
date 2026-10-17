package webui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/config"
)

const pluginOpIDBytes = 16

const (
	pluginActionInstall = "install"
	pluginActionUpdate  = "update"
	pluginActionRemove  = "remove"
	pluginMessageLimit  = 400
)

var pluginURLPattern = regexp.MustCompile(`https?://\S+`)

type pluginOps struct {
	mu      sync.Mutex
	byID    map[string]*pluginOperation
	running map[string]struct{}
}

type pluginOperation struct {
	mu       sync.Mutex
	id       string
	name     string
	status   string
	errText  string
	events   []PluginProgress
	subs     []chan struct{}
	finished bool
}

func (s *Server) registerPlugins(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/plugins/catalog", s.handlePluginCatalog)
	mux.HandleFunc("POST /api/plugins/{name}/plan", s.handlePluginPlan)
	mux.HandleFunc("POST /api/plugins/{name}/install", s.handlePluginInstall)
	mux.HandleFunc("POST /api/plugins/{name}/update", s.handlePluginUpdate)
	mux.HandleFunc("DELETE /api/plugins/{name}/{version}", s.handlePluginRemove)
	mux.HandleFunc("GET /api/plugins/operations/{id}/stream", s.handlePluginStream)
	mux.HandleFunc("POST /api/plugins/{name}/validate", s.handlePluginValidate)
	mux.HandleFunc("GET /api/plugins/{name}/settings", s.handlePluginSettingsGet)
	mux.HandleFunc("POST /api/plugins/{name}/settings/preview", s.handlePluginSettingsPreview)
	mux.HandleFunc("POST /api/plugins/{name}/settings", s.handlePluginSettingsSave)
}

func (s *Server) operations() *pluginOps {
	s.opsOnce.Do(func() {
		s.ops = &pluginOps{
			byID:    map[string]*pluginOperation{},
			running: map[string]struct{}{},
		}
	})
	return s.ops
}

func (s *Server) dropPluginOps() {
	if s.ops == nil {
		return
	}
	ops := s.operations()
	ops.mu.Lock()
	defer ops.mu.Unlock()
	ops.byID = map[string]*pluginOperation{}
	ops.running = map[string]struct{}{}
}

func (s *Server) handlePluginCatalog(w http.ResponseWriter, r *http.Request) {
	if s.session == nil || s.session.opts.PluginCatalog == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin catalog unavailable")
		return
	}
	rows, err := s.session.opts.PluginCatalog(r.Context())
	if err != nil {
		s.writePluginError(w, err)
		return
	}
	if rows == nil {
		rows = []PluginCatalogRow{}
	}
	s.writePlainJSON(w, http.StatusOK, struct {
		Plugins []PluginCatalogRow `json:"plugins"`
	}{Plugins: rows})
}

func (s *Server) handlePluginPlan(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !pluginSlug(name) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "catalog installs accept a registry name only")
		return
	}
	if s.session == nil || s.session.opts.PluginPlan == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin planner unavailable")
		return
	}
	var request PluginPlanRequest
	if !s.decodeJSON(w, r, &request) {
		return
	}
	if request.Action != pluginActionInstall && request.Action != pluginActionUpdate &&
		request.Action != pluginActionRemove {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "action must be install, update, or remove")
		return
	}
	if request.Action == pluginActionRemove && request.Version == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "remove requires a version")
		return
	}
	plan, err := s.session.opts.PluginPlan(r.Context(), name, request)
	if err != nil {
		s.writePluginError(w, err)
		return
	}
	s.writePlainJSON(w, http.StatusOK, plan)
}

func (s *Server) handlePluginInstall(w http.ResponseWriter, r *http.Request) {
	s.startPluginChange(w, r, pluginActionInstall)
}

func (s *Server) handlePluginUpdate(w http.ResponseWriter, r *http.Request) {
	s.startPluginChange(w, r, pluginActionUpdate)
}

func (s *Server) handlePluginRemove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	if !pluginSlug(name) || version == "" || strings.Contains(version, "/") {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "remove requires an installed version")
		return
	}
	var request PluginOperationRequest
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Version = version
	s.startConfirmed(w, r, name, pluginActionRemove, request)
}

func (s *Server) startPluginChange(w http.ResponseWriter, r *http.Request, action string) {
	name := r.PathValue("name")
	if !pluginSlug(name) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "catalog installs accept a registry name only")
		return
	}
	var request PluginOperationRequest
	if !s.decodeJSON(w, r, &request) {
		return
	}
	s.startConfirmed(w, r, name, action, request)
}

func (s *Server) startConfirmed(
	w http.ResponseWriter,
	r *http.Request,
	name, action string,
	request PluginOperationRequest,
) {
	if s.session == nil || s.session.opts.PluginPlan == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin planner unavailable")
		return
	}
	run := s.session.opts.PluginInstall
	if action == pluginActionRemove {
		run = s.session.opts.PluginRemove
	}
	if run == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin installer unavailable")
		return
	}
	request.CatalogOnly = true
	request.RequireChecksumFile = true
	asked := request.Version
	if action != pluginActionRemove && request.AcceptFallbackFrom != "" {
		asked = request.AcceptFallbackFrom
	}
	plan, err := s.session.opts.PluginPlan(r.Context(), name, PluginPlanRequest{Action: action, Version: asked})
	if err != nil {
		s.writePluginError(w, err)
		return
	}
	if !confirmationMatches(plan, request) {
		s.writeError(w, http.StatusConflict, "conflict", "confirmation does not match the current plan")
		return
	}
	if !s.reservePlugin(name) {
		s.writeError(w, http.StatusConflict, "busy", "plugin is currently being modified")
		return
	}
	if s.session.opts.PluginPrepare != nil {
		if prepErr := s.session.opts.PluginPrepare(r.Context(), name, request); prepErr != nil {
			s.releasePlugin(name)
			s.writePluginError(w, prepErr)
			return
		}
	}
	op, err := s.newPluginOperation(name)
	if err != nil {
		s.releasePlugin(name)
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin operation unavailable")
		return
	}
	ctx := s.session.Context()
	go func() {
		defer s.releasePlugin(name)
		runErr := run(ctx, name, request, op.add)
		op.finish(runErr)
	}()
	s.writePlainJSON(w, http.StatusAccepted, PluginAccepted{OperationID: op.id})
}

func (s *Server) reservePlugin(name string) bool {
	ops := s.operations()
	ops.mu.Lock()
	defer ops.mu.Unlock()
	if _, busy := ops.running[name]; busy {
		return false
	}
	ops.running[name] = struct{}{}
	return true
}

func (s *Server) releasePlugin(name string) {
	ops := s.operations()
	ops.mu.Lock()
	defer ops.mu.Unlock()
	delete(ops.running, name)
}

func (s *Server) newPluginOperation(name string) (*pluginOperation, error) {
	raw := make([]byte, pluginOpIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	op := &pluginOperation{
		id:     hex.EncodeToString(raw),
		name:   name,
		status: "running",
		events: []PluginProgress{},
	}
	ops := s.operations()
	ops.mu.Lock()
	ops.byID[op.id] = op
	ops.mu.Unlock()
	return op, nil
}

func (s *Server) pluginOperation(id string) *pluginOperation {
	ops := s.operations()
	ops.mu.Lock()
	defer ops.mu.Unlock()
	return ops.byID[id]
}

func (op *pluginOperation) add(event PluginProgress) {
	if op == nil {
		return
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	op.events = append(op.events, event)
	op.notifyLocked()
}

func (op *pluginOperation) finish(err error) {
	op.mu.Lock()
	defer op.mu.Unlock()
	op.finished = true
	if err != nil {
		op.status = "failed"
		op.errText = safePluginMessage(err)
	} else {
		op.status = "succeeded"
	}
	op.notifyLocked()
}

func (op *pluginOperation) notifyLocked() {
	for _, ch := range op.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (op *pluginOperation) watch() (PluginOperationSnapshot, <-chan struct{}) {
	op.mu.Lock()
	defer op.mu.Unlock()
	snap := op.snapshotLocked()
	if op.finished {
		return snap, nil
	}
	ch := make(chan struct{}, 1)
	op.subs = append(op.subs, ch)
	return snap, ch
}

func (op *pluginOperation) snapshotLocked() PluginOperationSnapshot {
	events := append([]PluginProgress{}, op.events...)
	if events == nil {
		events = []PluginProgress{}
	}
	return PluginOperationSnapshot{Status: op.status, Events: events, Error: op.errText}
}

func (s *Server) handlePluginStream(w http.ResponseWriter, r *http.Request) {
	op := s.pluginOperation(r.PathValue("id"))
	if op == nil {
		s.writeError(w, http.StatusNotFound, "not_found", "operation not found")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	snap, updates := op.watch()
	if err := writePluginSSE(w, controller, "snapshot", snap); err != nil {
		return
	}
	if updates == nil {
		_ = writePluginSSE(w, controller, "done", PluginOperationDone{Status: snap.Status, Error: snap.Error})
		return
	}
	s.followPluginOperation(r, op, w, controller, updates, len(snap.Events))
}

func writePluginSSE(w http.ResponseWriter, controller *http.ResponseController, name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_ = controller.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
	if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data); err != nil {
		return err
	}
	return controller.Flush()
}

func (s *Server) followPluginOperation(
	r *http.Request,
	op *pluginOperation,
	w http.ResponseWriter,
	controller *http.ResponseController,
	updates <-chan struct{},
	seen int,
) {
	heartbeat := time.NewTicker(streamHeartbeatInterval)
	defer heartbeat.Stop()
	sessionDone := s.session.Context().Done()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-sessionDone:
			return
		case <-updates:
			if !writePluginProgress(op, w, controller, &seen) {
				return
			}
		case <-heartbeat.C:
			_ = controller.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}

func writePluginProgress(
	op *pluginOperation, w http.ResponseWriter, controller *http.ResponseController, seen *int,
) bool {
	op.mu.Lock()
	current := op.snapshotLocked()
	finished := op.finished
	op.mu.Unlock()
	for _, event := range current.Events[*seen:] {
		if err := writePluginSSE(w, controller, "progress", event); err != nil {
			return false
		}
	}
	*seen = len(current.Events)
	if !finished {
		return true
	}
	_ = writePluginSSE(w, controller, "done", PluginOperationDone{Status: current.Status, Error: current.Error})
	return false
}

func (s *Server) handlePluginValidate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !pluginSlug(name) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "catalog installs accept a registry name only")
		return
	}
	if s.session == nil || s.session.opts.PluginValidate == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin validation unavailable")
		return
	}
	var discard struct{}
	if !s.decodeJSON(w, r, &discard) {
		return
	}
	result, err := s.session.opts.PluginValidate(s.launchCtx(r), name)
	if err != nil {
		s.writePluginError(w, err)
		return
	}
	s.writePlainJSON(w, http.StatusOK, result)
}

func (s *Server) handlePluginSettingsGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !pluginSlug(name) {
		s.writeError(w, http.StatusNotFound, "not_found", "plugin not found")
		return
	}
	if s.session == nil || s.session.opts.PluginSettings == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin settings unavailable")
		return
	}
	view, err := s.session.opts.PluginSettings(s.launchCtx(r), name)
	if err != nil {
		s.writePluginError(w, err)
		return
	}
	s.writePlainJSON(w, http.StatusOK, stripSecretSettings(view))
}

// launchCtx keeps the session's plugin log file and trace on a short probe.
// A bare request context does not carry them, so a plugin's stderr is discarded
// whenever the dashboard's stderr is not a terminal. The returned context is
// canceled when the request ends. Do not use it for a process that must stay
// up: CommandContext kills the plugin when this context is canceled.
func (s *Server) launchCtx(r *http.Request) context.Context {
	ctx, cancel := context.WithCancel(s.session.Context())
	context.AfterFunc(r.Context(), cancel)
	return ctx
}

func (s *Server) handlePluginSettingsPreview(w http.ResponseWriter, r *http.Request) {
	s.handlePluginSettingsWrite(w, r, true)
}

func (s *Server) handlePluginSettingsSave(w http.ResponseWriter, r *http.Request) {
	s.handlePluginSettingsWrite(w, r, false)
}

func (s *Server) handlePluginSettingsWrite(w http.ResponseWriter, r *http.Request, dryRun bool) {
	name := r.PathValue("name")
	if !pluginSlug(name) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "plugin name is not a catalog slug")
		return
	}
	callback := s.configCallback(dryRun)
	if callback == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "configuration editor unavailable")
		return
	}
	var request config.WriteRequest
	if !s.decodeJSON(w, r, &request) {
		return
	}
	if request.BaseVersion == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "baseVersion is required")
		return
	}
	for _, change := range request.Changes {
		if !pluginChangeKey(name, change.Key) {
			s.writeError(w, http.StatusBadRequest, "invalid_request", "settings only change this plugin")
			return
		}
	}
	if s.session.opts.PluginSettingsPrepare != nil {
		next, err := s.session.opts.PluginSettingsPrepare(r.Context(), name, request)
		if err != nil {
			s.writePluginError(w, err)
			return
		}
		request = next
	}
	result, err := callback(r.Context(), request)
	if err != nil {
		s.writeConfigError(w, err)
		return
	}
	if dryRun {
		s.writePlainJSON(w, http.StatusOK, configPreviewResponse{
			Diff:       result.Diff,
			Validation: publicValidation(result.Validation),
		})
		return
	}
	// Keep the reopened plugins on the session context. launchCtx ends with this
	// request, and CommandContext would then kill the processes later queries use.
	if s.session.opts.ReloadPlugins != nil {
		if reloadErr := s.session.opts.ReloadPlugins(s.session.Context()); reloadErr != nil {
			s.logger.Warn().Str("component", "webui").Str("operation", "reload_plugins").Str("plugin", name).
				Msg("saved plugin settings could not reopen plugins")
		}
	}
	if s.session.opts.PluginSettings == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin settings unavailable")
		return
	}
	view, err := s.session.opts.PluginSettings(s.session.Context(), name)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "plugin settings unavailable")
		return
	}
	body := struct {
		PluginSettingsView

		Message string `json:"message"`
	}{PluginSettingsView: stripSecretSettings(view), Message: pluginSettingsSaved}
	s.writePlainJSON(w, http.StatusOK, body)
}

func (s *Server) writePluginError(w http.ResponseWriter, err error) {
	var status *StatusError
	if errors.As(err, &status) && status != nil {
		s.writeError(w, status.Status, status.Code, status.Text)
		return
	}
	s.logger.Warn().Str("component", "webui").Str("operation", "plugins").Msg("plugin request failed")
	s.writeError(w, http.StatusInternalServerError, "internal", "plugin request failed")
}

func safePluginMessage(err error) string {
	if err == nil {
		return ""
	}
	var status *StatusError
	if errors.As(err, &status) && status != nil && status.Text != "" {
		return status.Text
	}
	message := pluginURLPattern.ReplaceAllString(err.Error(), "[url]")
	if len(message) > pluginMessageLimit {
		message = message[:pluginMessageLimit]
	}
	return message
}
