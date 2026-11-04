package webui

import (
	"net/http"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// AnomalyRow is one anomaly as the page receives it: the same fields the CLI
// prints, plus the display text the browser shows.
type AnomalyRow struct {
	engine.Anomaly

	Display viewmodel.AnomalyDisplay `json:"display"`
}

// AnomalyPage is the Anomalies view payload. Removing summary and every
// display object leaves exactly the CLI JSON.
type AnomalyPage struct {
	Considered int          `json:"considered"`
	Summary    string       `json:"summary"`
	Anomalies  []AnomalyRow `json:"anomalies"`
}

// handleAnomalies serves the same anomaly list as `finfocus cost anomalies`.
// It selects from the session's current overview rows and the session stack's
// daily actuals and starts no pipeline. Drift is set during enrichment, so
// before the session is ready an empty list would be wrong, and the handler
// answers not_ready instead.
func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	if !s.session.IsReady() {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "overview data is loading")
		return
	}
	var series []history.DailyPoint
	if s.session.opts.DailyPoints != nil {
		points, err := s.session.opts.DailyPoints(r.Context())
		if err != nil {
			s.logger.Warn().
				Str("component", "webui").
				Str("operation", "anomalies").
				Err(err).
				Msg("reading daily actuals")
			s.writeError(w, http.StatusInternalServerError, "daily_actuals", "daily actuals unavailable")
			return
		}
		series = points
	}
	report := engine.SelectAnomalies(s.session.Rows(), series)
	page := AnomalyPage{
		Considered: report.Considered,
		Summary:    viewmodel.AnomalySummary(len(report.Anomalies), report.Considered),
		Anomalies:  make([]AnomalyRow, 0, len(report.Anomalies)),
	}
	for _, anomaly := range report.Anomalies {
		page.Anomalies = append(page.Anomalies, AnomalyRow{
			Anomaly: anomaly,
			Display: viewmodel.BuildAnomalyDisplay(anomaly),
		})
	}
	s.WriteJSON(w, http.StatusOK, page)
}
