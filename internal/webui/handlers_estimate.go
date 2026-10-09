package webui

import (
	"context"
	"errors"
	"net/http"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

var errEstimateResourceNotFound = errors.New("estimate resource not found")

type estimateQuery struct {
	URN         string            `json:"urn"`
	Overrides   map[string]string `json:"overrides"`
	PricingMode string            `json:"pricingMode"`
}

type estimateMode struct {
	ID             string             `json:"id"`
	Label          string             `json:"label"`
	Rate           string             `json:"rate"`
	Details        engine.PricingMode `json:"details"`
	DetailsDisplay string             `json:"detailsDisplay"`
}

func (s *Server) registerEstimate(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/estimate/resources", s.handleEstimateResources)
	mux.HandleFunc("GET /api/estimate/baseline", s.handleEstimateBaseline)
	mux.HandleFunc("POST /api/estimate/recalculate", s.handleEstimateRecalculate)
}

func (s *Session) estimateResources(ctx context.Context) ([]engine.ResourceDescriptor, error) {
	resources, err := s.opts.EstimateResources(ctx)
	if err != nil {
		return nil, err
	}
	resources = cloneData(resources)
	rows := s.Rows()
	for i := range resources {
		for _, row := range rows {
			if row.URN == resources[i].ID && row.ProjectedProperties != nil {
				resources[i].Properties = cloneData(row.ProjectedProperties)
				break
			}
		}
	}
	return resources, nil
}

func (s *Server) beginEstimate(w http.ResponseWriter, r *http.Request) (context.Context, func(), bool) {
	if s.session == nil || s.session.opts.EstimateResources == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "estimate resources are loading")
		return nil, nil, false
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "estimate resources are loading")
		return nil, nil, false
	}
	return ctx, done, true
}

func (s *Server) handleEstimateResources(w http.ResponseWriter, r *http.Request) {
	ctx, done, ok := s.beginEstimate(w, r)
	if !ok {
		return
	}
	defer done()
	resources, err := s.session.estimateResources(ctx)
	done()
	if err != nil {
		s.writeEstimateError(w, err)
		return
	}
	if resources == nil {
		resources = []engine.ResourceDescriptor{}
	}
	s.WriteJSON(w, http.StatusOK, struct {
		Resources []engine.ResourceDescriptor `json:"resources"`
	}{resources})
}

func (s *Server) estimateResource(ctx context.Context, urn string) (*engine.ResourceDescriptor, error) {
	resources, err := s.session.estimateResources(ctx)
	if err != nil {
		return nil, err
	}
	for _, resource := range resources {
		if resource.ID == urn {
			return &resource, nil
		}
	}
	return nil, errEstimateResourceNotFound
}

func (s *Server) handleEstimateBaseline(w http.ResponseWriter, r *http.Request) {
	query := estimateQuery{URN: r.URL.Query().Get("urn"), PricingMode: r.URL.Query().Get("pricingMode")}
	if query.URN == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "resource urn required")
		return
	}
	s.serveEstimate(w, r, query, true)
}

func (s *Server) handleEstimateRecalculate(w http.ResponseWriter, r *http.Request) {
	var query estimateQuery
	if !s.decodeJSON(w, r, &query) {
		return
	}
	if query.URN == "" || len(query.Overrides) == 0 {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "resource urn and property overrides required")
		return
	}
	s.serveEstimate(w, r, query, false)
}

func (s *Server) serveEstimate(w http.ResponseWriter, r *http.Request, query estimateQuery, baseline bool) {
	ctx, done, ok := s.beginEstimate(w, r)
	if !ok {
		return
	}
	defer done()
	resource, err := s.estimateResource(ctx, query.URN)
	if errors.Is(err, errEstimateResourceNotFound) {
		done()
		s.writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	if err != nil {
		done()
		s.writeEstimateError(w, err)
		return
	}
	eng := s.session.Engine()
	var result *engine.EstimateResult

	if baseline {
		result, err = eng.EstimateBaseline(ctx, resource, query.PricingMode)
	} else {
		result, err = eng.EstimateCost(
			ctx,
			&engine.EstimateRequest{
				Resource:          resource,
				PropertyOverrides: query.Overrides,
				PricingMode:       query.PricingMode,
			},
		)
	}
	if err != nil {
		done()
		s.writeEstimateError(w, err)
		return
	}
	result = cloneData(result)
	if result == nil || result.Baseline == nil || result.Modified == nil || result.Baseline.Error != nil ||
		result.Modified.Error != nil {
		done()
		s.writeEstimateError(w, errors.New("estimate result unavailable"))
		return
	}
	display := viewmodel.BuildEstimateDisplay(ctx, resource, result, query.Overrides)
	if !baseline {
		done()
		s.WriteJSON(w, http.StatusOK, struct {
			*engine.EstimateResult

			Display     viewmodel.EstimateDisplay `json:"display"`
			PricingMode string                    `json:"pricingMode"`
		}{result, display, query.PricingMode})
		return
	}
	modes := []estimateMode{}
	for _, mode := range eng.DiscoverPricingSpec(ctx, resource).Modes {
		modes = append(
			modes,
			estimateMode{
				ID:             mode.ID(),
				Label:          mode.BillingMode + " — " + mode.Plugin,
				Rate:           viewmodel.FormatPricingRate(mode),
				Details:        mode,
				DetailsDisplay: viewmodel.FormatPricingDetails(mode),
			},
		)
	}
	done()
	s.WriteJSON(w, http.StatusOK, struct {
		Resource     *engine.ResourceDescriptor `json:"resource"`
		Result       *engine.EstimateResult     `json:"result"`
		PricingModes []estimateMode             `json:"pricingModes"`
		PricingMode  string                     `json:"pricingMode"`
		Display      viewmodel.EstimateDisplay  `json:"display"`
	}{resource, result, modes, query.PricingMode, display})
}

func (s *Server) writeEstimateError(w http.ResponseWriter, err error) {
	if errors.Is(err, engine.ErrInvalidPricingMode) {
		s.writeError(w, http.StatusBadRequest, "invalid_pricing_mode", "invalid pricing mode")
		return
	}
	s.writeError(w, http.StatusBadGateway, "estimate_unavailable", "estimate data unavailable")
}
