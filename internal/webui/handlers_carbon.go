package webui

import (
	"net/http"

	"github.com/rshade/finfocus/internal/carbonreport"
)

func (s *Server) registerCarbon(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/carbon/actual/query", s.handleCarbonQuery)
}

func (s *Server) handleCarbonQuery(w http.ResponseWriter, r *http.Request) {
	var selectors carbonreport.Selectors
	if !s.decodeJSON(w, r, &selectors) {
		return
	}
	req, err := selectors.Request()
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	if s.session == nil || s.session.opts.HistoricalCarbon == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "historical carbon reporting unavailable")
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "historical carbon session unavailable")
		return
	}
	defer done()
	report, err := s.session.opts.HistoricalCarbon(ctx, req)
	if err != nil || report == nil {
		s.writeError(
			w,
			http.StatusBadGateway,
			"carbon_unavailable",
			"historical carbon unavailable; install aws-ce and enable FINFOCUS_AWS_CARBON_ENABLED=true or plugins.aws-ce.carbon_enabled=true",
		)
		return
	}
	s.WriteJSON(w, http.StatusOK, report)
}
