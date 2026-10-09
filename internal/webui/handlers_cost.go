package webui

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

const queryTimeout = 2 * time.Minute

// CostQuery carries actual-cost selectors without duplicating domain computation.
type CostQuery struct {
	Filter  string `json:"filter"`
	Sort    string `json:"sort"`
	GroupBy string `json:"groupBy"`
	Tag     string `json:"tag"`
	Page    int    `json:"page"`
}

// actualCostQueryResponse identifies the trusted engine summary's domain maps.
// Credential-like service names are labels, not resource property keys.
type actualCostQueryResponse struct {
	viewmodel.ActualCostPage

	Errors []actualQueryError `json:"errors"`
}

func (s *Server) registerCost(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/cost/actual/query", s.handleActualQuery)
	mux.HandleFunc("GET /api/cost/actual/resource", s.handleActualResource)
}

// beginQuery joins active operations on shutdown and links request/session cancellation.
func (s *Session) beginQuery(request context.Context) (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.eng == nil {
		return nil, nil, errors.New("session unavailable")
	}
	s.workers.Add(1)
	ctx, cancel := context.WithTimeout(request, queryTimeout)
	stop := context.AfterFunc(s.ctx, cancel)
	var once sync.Once
	return ctx, func() { once.Do(func() { stop(); cancel(); s.workers.Done() }) }, nil
}

func (s *Server) handleActualQuery(w http.ResponseWriter, r *http.Request) {
	var query CostQuery
	if !s.decodeJSON(w, r, &query) {
		return
	}
	sort, valid := sortField(query.Sort)
	tags, err := engine.ParseActualTagFilter(query.Tag)
	if !valid || !engine.GroupBy(query.GroupBy).IsValid() || err != nil || query.Page < 0 ||
		engine.GroupBy(query.GroupBy).IsTimeBasedGrouping() && !viewmodel.TimeAggregationSortSupported(sort) {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "invalid cost filter, sort, grouping or page")
		return
	}
	if s.session == nil || s.session.opts.ActualCosts == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "actual cost data is loading")
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "actual cost data is loading")
		return
	}
	defer done()
	result, err := s.session.opts.ActualCosts(ctx, query.GroupBy, tags)
	if err != nil || result == nil {
		done()
		s.writeError(w, http.StatusBadGateway, "cost_unavailable", "actual cost data unavailable")
		return
	}
	result = cloneData(result)
	done()
	trends := map[string]string{}
	totalTrend := ""
	if s.session.opts.Trends != nil {
		trends, totalTrend = s.session.opts.Trends()
	}
	page, err := viewmodel.BuildActualCostPage(
		r.Context(),
		result.Results,
		engine.GroupBy(query.GroupBy),
		query.Filter,
		sort,
		query.Page,
		trends,
		totalTrend,
	)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, "cost_unavailable", "actual cost aggregation unavailable")
		return
	}
	errors := make([]actualQueryError, 0, len(result.Errors))
	for _, failure := range result.Errors {
		errors = append(
			errors,
			actualQueryError{
				ResourceID:   failure.ResourceID,
				ResourceType: failure.ResourceType,
				PluginName:   failure.PluginName,
				Message:      "Actual cost unavailable",
			},
		)
	}
	s.WriteJSON(w, http.StatusOK, actualCostQueryResponse{ActualCostPage: page, Errors: errors})
}

func (s *Server) handleActualResource(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	group := r.URL.Query().Get("groupBy")
	tags, tagErr := engine.ParseActualTagFilter(r.URL.Query().Get("tag"))
	if !engine.GroupBy(group).IsValid() || tagErr != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "invalid cost grouping or tag")
		return
	}
	if id == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "resource id required")
		return
	}
	if s.session == nil || s.session.opts.ActualCosts == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "actual cost data is loading")
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "actual cost data is loading")
		return
	}
	defer done()
	result, err := s.session.opts.ActualCosts(ctx, group, tags)
	if err != nil || result == nil {
		done()
		s.writeError(w, http.StatusBadGateway, "cost_unavailable", "actual cost data unavailable")
		return
	}
	result = cloneData(result)
	done()
	for _, cost := range result.Results {
		if cost.ResourceID == id &&
			(r.URL.Query().Get("type") == "" || cost.ResourceType == r.URL.Query().Get("type")) {
			s.WriteJSON(w, http.StatusOK, viewmodel.BuildActualCostDetail(cost))
			return
		}
	}
	s.writeError(w, http.StatusNotFound, "not_found", "resource not found")
}

type actualQueryError struct {
	ResourceID   string `json:"resourceId"`
	ResourceType string `json:"resourceType"`
	PluginName   string `json:"pluginName"`
	Message      string `json:"message"`
}
