package webui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rshade/finfocus/internal/config"
)

const configSaveMessage = "Saved. The running session keeps its startup configuration and does not reload this file."

type configSaveResponse struct {
	config.Page

	Message string `json:"message"`
}

type configPreviewResponse struct {
	Diff       []config.DiffEntry      `json:"diff"`
	Validation config.ValidationResult `json:"validation"`
}

type configValidationBody struct {
	Error  string                   `json:"error"`
	Code   string                   `json:"code"`
	Fields []config.ValidationError `json:"fields"`
}

func (s *Server) registerConfig(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/config", s.handleConfigGet)
	mux.HandleFunc("POST /api/config/preview", s.handleConfigPreview)
	mux.HandleFunc("POST /api/config", s.handleConfigSave)
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	if s.session == nil || s.session.opts.ConfigView == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "configuration editor unavailable")
		return
	}
	page, err := s.session.opts.ConfigView(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "configuration unavailable")
		return
	}
	s.writePlainJSON(w, http.StatusOK, page)
}

func (s *Server) handleConfigPreview(w http.ResponseWriter, r *http.Request) {
	s.handleConfigWrite(w, r, true)
}

func (s *Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	s.handleConfigWrite(w, r, false)
}

func (s *Server) handleConfigWrite(w http.ResponseWriter, r *http.Request, dryRun bool) {
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
	if s.session.opts.ConfigView == nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "configuration editor unavailable")
		return
	}
	page, err := s.session.opts.ConfigView(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "configuration unavailable")
		return
	}
	s.writePlainJSON(w, http.StatusOK, configSaveResponse{Page: page, Message: configSaveMessage})
}

func (s *Server) configCallback(dryRun bool) func(context.Context, config.WriteRequest) (config.PatchResult, error) {
	if s.session == nil {
		return nil
	}
	if dryRun {
		return s.session.opts.ConfigPreview
	}
	return s.session.opts.ConfigSave
}

func (s *Server) writeConfigError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, config.ErrStaleConfig):
		s.writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, config.ErrProjectUnavailable):
		s.writeError(w, http.StatusBadRequest, "project_unavailable", err.Error())
	default:
		if rejected, isRejected := errors.AsType[*config.InvalidChangeError](err); isRejected {
			s.writeError(w, http.StatusBadRequest, "invalid_request", rejected.Error())
			return
		}
		invalid, ok := errors.AsType[*config.TierValidationError](err)
		if !ok {
			s.logger.Warn().Err(err).Msg("configuration write failed")
			s.writeError(w, http.StatusInternalServerError, "internal", "configuration write failed")
			return
		}
		s.writePlainJSON(w, http.StatusBadRequest, configValidationBody{
			Error:  invalid.Error(),
			Code:   "validation",
			Fields: publicValidation(invalid.Result).Errors,
		})
	}
}

func (s *Server) writePlainJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "response unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

func publicValidation(result config.ValidationResult) config.ValidationResult {
	result.File = ""
	for i := range result.Errors {
		result.Errors[i].File = ""
	}
	for i := range result.Warnings {
		result.Warnings[i].File = ""
	}
	if result.Errors == nil {
		result.Errors = []config.ValidationError{}
	}
	if result.Warnings == nil {
		result.Warnings = []config.ValidationWarning{}
	}
	return result
}
