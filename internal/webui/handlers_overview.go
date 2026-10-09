package webui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

const maxJSONBodyBytes = 1 << 20

// OverviewQuery carries per-tab view state. All computation delegates to viewmodel.
type OverviewQuery struct {
	Filter   string   `json:"filter"`
	Sort     string   `json:"sort"`
	Page     int      `json:"page"`
	Expanded []string `json:"expanded"`
}

// OverviewPage is a sorted, filtered and cluster-flattened overview page.
type OverviewPage struct {
	Rows       []engine.OverviewRowResult `json:"rows"`
	Totals     OverviewTotalsPayload      `json:"totals"`
	Page       int                        `json:"page"`
	TotalPages int                        `json:"totalPages"`
	Expanded   []string                   `json:"expanded"`
}

// ResourceDetail exposes the same row details and budget health as the CLI.
type ResourceDetail struct {
	Display               viewmodel.OverviewDetailDisplay          `json:"display"`
	BudgetDisplay         viewmodel.BudgetDisplay                  `json:"budgetDisplay"`
	ActiveRecommendations []viewmodel.OverviewDetailRecommendation `json:"activeRecommendations"`
	Row                   engine.OverviewRowResult                 `json:"row"`
	Budgets               []engine.BudgetHealthResult              `json:"budgets"`
}

func (s *Server) registerOverview(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/overview/stream", s.handleOverviewStream)
	mux.HandleFunc("POST /api/overview/query", s.handleOverviewQuery)
	mux.HandleFunc("POST /api/overview/cluster/toggle", s.handleClusterToggle)
	mux.HandleFunc("POST /api/overview/preview", s.handlePreview)
	mux.HandleFunc("GET /api/overview/budget", s.handleBudget)
	mux.HandleFunc("GET /api/overview/resource", s.handleOverviewResource)
	mux.HandleFunc("POST /api/passphrase", s.handlePassphrase)
}

// decodeJSON rejects unknown fields, oversized input, and trailing values.
func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "request must contain one JSON value")
		return false
	}
	return true
}

// WriteJSON redacts every transport payload before serialization.
func (s *Server) WriteJSON(w http.ResponseWriter, status int, value any) {
	data, err := RedactJSON(value)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "response unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}
func sortField(value string) (viewmodel.SortField, bool) {
	switch value {
	case "", "cost":
		return viewmodel.SortByCost, true
	case "name":
		return viewmodel.SortByName, true
	case "type":
		return viewmodel.SortByType, true
	case "delta":
		return viewmodel.SortByDelta, true
	default:
		return 0, false
	}
}
func (s *Server) overviewPage(w http.ResponseWriter, q OverviewQuery) (OverviewPage, bool) {
	field, ok := sortField(q.Sort)
	if !ok || q.Page < 0 {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "invalid sort field or page")
		return OverviewPage{}, false
	}
	rows := s.session.Rows()
	result := engine.ComputeOverviewResult(rows, s.session.opts.DayOfMonth)
	filtered := viewmodel.FilterOverviewRows(result.Rows, q.Filter)
	viewmodel.SortOverviewRows(filtered, field)
	expanded := make(map[string]bool, len(q.Expanded))
	for _, urn := range q.Expanded {
		expanded[urn] = true
	}
	entries, pagination := viewmodel.FlattenClusterRows(filtered, expanded, q.Filter != "", q.Page)
	display := make([]engine.OverviewRowResult, 0, len(entries))
	for _, entry := range entries {
		display = append(display, entry.Row)
	}
	if q.Expanded == nil {
		q.Expanded = []string{}
	}
	return OverviewPage{
		Rows:       display,
		Totals:     totalsPayload(result.Summary),
		Page:       pagination.Page,
		TotalPages: pagination.TotalPages,
		Expanded:   q.Expanded,
	}, true
}
func (s *Server) handleOverviewQuery(w http.ResponseWriter, r *http.Request) {
	var q OverviewQuery
	if !s.decodeJSON(w, r, &q) {
		return
	}
	page, ok := s.overviewPage(w, q)
	if ok {
		s.WriteJSON(w, http.StatusOK, page)
	}
}
func (s *Server) handleClusterToggle(w http.ResponseWriter, r *http.Request) {
	var q struct {
		OverviewQuery

		URN string `json:"urn"`
	}
	if !s.decodeJSON(w, r, &q) {
		return
	}
	found := false
	for _, row := range s.session.Rows() {
		if row.URN == q.URN && len(row.ChildURNs) > 0 {
			found = true
			break
		}
	}
	if !found {
		s.writeError(w, http.StatusNotFound, "not_found", "cluster not found")
		return
	}
	if index := slices.Index(q.Expanded, q.URN); index >= 0 {
		q.Expanded = slices.Delete(q.Expanded, index, index+1)
	} else {
		q.Expanded = append(q.Expanded, q.URN)
	}
	page, ok := s.overviewPage(w, q.OverviewQuery)
	if ok {
		s.WriteJSON(w, http.StatusOK, page)
	}
}
func (s *Server) handleBudget(w http.ResponseWriter, _ *http.Request) {
	s.session.mu.Lock()
	budget := cloneData(s.session.budget)
	s.session.mu.Unlock()
	s.WriteJSON(w, http.StatusOK, budget)
}
func (s *Server) handleOverviewResource(w http.ResponseWriter, r *http.Request) {
	urn := r.URL.Query().Get("urn")
	for _, row := range engine.ComputeOverviewResult(s.session.Rows(), s.session.opts.DayOfMonth).Rows {
		if row.URN == urn {
			s.session.mu.Lock()
			budget := cloneData(s.session.budget)
			s.session.mu.Unlock()
			active := viewmodel.OverviewDetailRecommendations(row.Recommendations)
			row.Recommendations = make([]engine.Recommendation, 0, len(active))
			for _, rec := range active {
				row.Recommendations = append(row.Recommendations, rec.Recommendation)
			}
			s.WriteJSON(
				w,
				http.StatusOK,
				ResourceDetail{
					Row:                   row,
					Budgets:               budget.Budgets,
					BudgetDisplay:         budget.Display,
					Display:               viewmodel.BuildOverviewDetailDisplay(row, s.session.opts.DayOfMonth),
					ActiveRecommendations: active,
				},
			)
			return
		}
	}
	s.writeError(w, http.StatusNotFound, "not_found", "resource not found")
}
func (s *Server) handlePreview(w http.ResponseWriter, _ *http.Request) {
	if err := s.session.startPreview(); err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "unavailable", "preview unavailable")
		return
	}
	s.session.mu.Lock()
	preview := cloneData(s.session.preview)
	s.session.mu.Unlock()
	s.WriteJSON(w, http.StatusAccepted, preview)
}
func (s *Server) handlePassphrase(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Passphrase string `json:"passphrase"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	s.session.mu.Lock()
	required := s.session.passphraseRequired
	submit := s.session.opts.SubmitPassphrase
	if required && submit != nil {
		s.session.passphraseRequired = false
		// Broadcast before handing the secret to Pulumi, so an aborted POST or a
		// closed modal cannot leave another tab's prompt state stale.
		s.session.publishLocked("snapshot", s.session.snapshotLocked())
	}
	s.session.mu.Unlock()
	if !required || submit == nil {
		s.writeError(w, http.StatusConflict, "not_waiting", "stack is not waiting for a passphrase")
		return
	}
	err := submit(r.Context(), request.Passphrase)
	request.Passphrase = ""
	if err != nil {
		s.session.RequirePassphrase("")
		s.writeError(w, http.StatusServiceUnavailable, "unavailable", "passphrase submission unavailable")
		return
	}
	s.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
