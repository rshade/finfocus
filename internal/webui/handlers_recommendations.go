package webui

import (
	"context"
	"net/http"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// recommendationHistoryResponse is the browser form of command-line history
// JSON, plus warnings for plugins that failed to answer.
type recommendationHistoryResponse struct {
	RecommendationID string                  `json:"recommendation_id"`
	Events           []config.LifecycleEvent `json:"events"`
	EventCount       int                     `json:"event_count"`
	Warnings         []string                `json:"warnings"`
}

// RecommendationQuery is independent tab state for read-only recommendation views.
type RecommendationQuery struct {
	Filter           string `json:"filter"`
	Sort             string `json:"sort"`
	IncludeDismissed bool   `json:"includeDismissed"`
	Page             int    `json:"page"`
}

func recommendationSort(value string) (viewmodel.RecommendationSortField, bool) {
	switch value {
	case "", "savings":
		return viewmodel.SortBySavings, true
	case "resource", "name":
		return viewmodel.SortByResourceID, true
	case "action", "type":
		return viewmodel.SortByActionType, true
	default:
		return viewmodel.SortBySavings, false
	}
}

func (s *Server) registerRecommendations(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/recommendations/query", s.handleRecommendationsQuery)
	mux.HandleFunc("GET /api/recommendations/item", s.handleRecommendationItem)
	mux.HandleFunc("GET /api/recommendations/history", s.handleRecommendationHistory)
}

func (s *Server) handleRecommendationsQuery(w http.ResponseWriter, r *http.Request) {
	var query RecommendationQuery
	if !s.decodeJSON(w, r, &query) {
		return
	}
	sort, valid := recommendationSort(query.Sort)
	if !valid || query.Page < 0 {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "invalid recommendation sort or page")
		return
	}
	if s.session == nil || s.session.opts.Recommendations == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "recommendation data is loading")
		return
	}
	safe, ok := s.fetchRecommendations(r.Context(), w, query.IncludeDismissed)
	if !ok {
		return
	}
	s.session.setPresentedRecommendations(viewmodel.FilterRecommendations(
		safe.Recommendations,
		query.Filter,
	))
	s.WriteJSON(w, http.StatusOK, viewmodel.BuildRecommendationPage(safe, query.Filter, sort, query.Page))
}

// fetchRecommendations runs the recommendations callback once and writes the
// error response itself when it cannot return a sanitized result.
func (s *Server) fetchRecommendations(
	parent context.Context,
	w http.ResponseWriter,
	includeDismissed bool,
) (*engine.RecommendationsResult, bool) {
	ctx, done, err := s.session.beginQuery(parent)
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "recommendation data is loading")
		return nil, false
	}
	defer done()
	result, err := s.session.opts.Recommendations(ctx, includeDismissed)
	if err != nil || result == nil {
		done()
		s.writeError(w, http.StatusBadGateway, "recommendations_unavailable", "recommendation data unavailable")
		return nil, false
	}
	safe := cloneData(result)
	for i := range safe.Errors {
		safe.Errors[i].Error = "Recommendation data unavailable"
	}
	return safe, true
}

func (s *Server) handleRecommendationItem(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "recommendation id required")
		return
	}
	if s.session == nil || s.session.opts.Recommendations == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "recommendation data is loading")
		return
	}
	items, queried := s.session.presentedRecommendations()
	if !queried {
		safe, ok := s.fetchRecommendations(r.Context(), w, false)
		if !ok {
			return
		}
		items = s.session.initPresentedRecommendations(safe.Recommendations)
	}
	for _, rec := range items {
		if recommendationMatches(r, rec, id) {
			s.WriteJSON(
				w,
				http.StatusOK,
				map[string]any{
					"item":           rec,
					"savingsDisplay": viewmodel.RecommendationSavingsDisplay(rec),
					"actionDisplay":  proto.ActionTypeLabelFromString(rec.Type),
					"scoreDisplay":   viewmodel.ScoreDisplay(rec.Scores),
				},
			)
			return
		}
	}
	s.writeError(w, http.StatusNotFound, "not_found", "recommendation not found")
}

func (s *Server) handleRecommendationHistory(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "recommendation id required")
		return
	}
	resolved, ok := s.recommendationIDForHistory(w, r, id)
	if !ok {
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "recommendation data is loading")
		return
	}
	defer done()
	events, warnings, err := s.session.opts.RecommendationHistory(ctx, resolved)
	if err != nil {
		done()
		s.writeError(w, http.StatusBadGateway, "history_unavailable", "recommendation history unavailable")
		return
	}
	if events == nil {
		events = []config.LifecycleEvent{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	done()
	s.WriteJSON(w, http.StatusOK, recommendationHistoryResponse{
		RecommendationID: resolved,
		Events:           events,
		EventCount:       len(events),
		Warnings:         warnings,
	})
}

// recommendationIDForHistory applies the detail endpoint's id rule and writes
// the error response when the id cannot be used. The returned id is the
// recommendation's own id, which is the dismissal id.
func (s *Server) recommendationIDForHistory(w http.ResponseWriter, r *http.Request, id string) (string, bool) {
	if s.session == nil || s.session.opts.Recommendations == nil || s.session.opts.RecommendationHistory == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "recommendation data is loading")
		return "", false
	}
	items, queried := s.session.presentedRecommendations()
	if !queried {
		safe, ok := s.fetchRecommendations(r.Context(), w, false)
		if !ok {
			return "", false
		}
		items = s.session.initPresentedRecommendations(safe.Recommendations)
	}
	for _, rec := range items {
		if recommendationMatches(r, rec, id) && rec.ID != "" {
			return rec.ID, true
		}
	}
	s.writeError(w, http.StatusNotFound, "not_found", "recommendation not found")
	return "", false
}

func recommendationMatches(r *http.Request, rec engine.Recommendation, id string) bool {
	if key := r.URL.Query().Get("key"); key != "" {
		return viewmodel.RecommendationDetailKey(rec) == key
	}
	if kind := r.URL.Query().Get("type"); kind != "" && kind != rec.Type {
		return false
	}
	if description := r.URL.Query().Get("description"); description != "" && description != rec.Description {
		return false
	}
	return rec.ID == id || rec.ID == "" && rec.ResourceID == id
}
