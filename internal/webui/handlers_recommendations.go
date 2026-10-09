package webui

import (
	"net/http"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/viewmodel"
)

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
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "recommendation data is loading")
		return
	}
	defer done()
	result, err := s.session.opts.Recommendations(ctx, query.IncludeDismissed)
	if err != nil || result == nil {
		done()
		s.writeError(w, http.StatusBadGateway, "recommendations_unavailable", "recommendation data unavailable")
		return
	}
	safe := cloneData(result)
	done()
	for i := range safe.Errors {
		safe.Errors[i].Error = "Recommendation data unavailable"
	}
	s.WriteJSON(w, http.StatusOK, viewmodel.BuildRecommendationPage(safe, query.Filter, sort, query.Page))
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
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "recommendation data is loading")
		return
	}
	defer done()
	result, err := s.session.opts.Recommendations(ctx, true)
	if err != nil || result == nil {
		done()
		s.writeError(w, http.StatusBadGateway, "recommendations_unavailable", "recommendation data unavailable")
		return
	}
	result = cloneData(result)
	done()
	for _, rec := range result.Recommendations {
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
