package webui

import (
	"net/http"

	"github.com/rshade/finfocus/internal/forecast"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// ForecastQuery is the dashboard's `cost forecast` request. Omitted fields take
// the CLI defaults. GrowthRate is text, like the --growth-rate flag, so a bad
// value gets the CLI's message.
type ForecastQuery struct {
	Months     *int   `json:"months,omitempty"`
	GrowthType string `json:"growthType,omitempty"`
	GrowthRate string `json:"growthRate,omitempty"`
	NoHistory  bool   `json:"noHistory,omitempty"`
}

// ForecastResponse is the CLI JSON document plus display strings, so the page
// formats nothing itself.
type ForecastResponse struct {
	forecast.Document

	Display ForecastDisplay `json:"display"`
}

// ForecastDisplay holds the formatted budget and series, in Document.Series order.
type ForecastDisplay struct {
	Budget string                  `json:"budget,omitempty"`
	Series []ForecastSeriesDisplay `json:"series"`
}

// ForecastSeriesDisplay is one series' formatted points.
type ForecastSeriesDisplay struct {
	Name   string                 `json:"name"`
	Points []ForecastPointDisplay `json:"points"`
}

// ForecastPointDisplay is one point's month label and formatted value.
type ForecastPointDisplay struct {
	Month string `json:"month"`
	Value string `json:"value"`
}

const forecastMonthLayout = "Jan 2006"

func (s *Server) registerForecast(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/forecast/query", s.handleForecastQuery)
}

func (s *Server) handleForecastQuery(w http.ResponseWriter, r *http.Request) {
	var query ForecastQuery
	if !s.decodeJSON(w, r, &query) {
		return
	}
	if s.session == nil || s.session.opts.Forecast == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "forecast data is loading")
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "forecast data is loading")
		return
	}
	defer done()
	doc, err := s.session.opts.Forecast(ctx, query)
	if err != nil {
		if forecast.IsInputError(err) {
			s.writeError(w, http.StatusBadRequest, "invalid_forecast", err.Error())
			return
		}
		if forecast.IsUnprojectable(err) {
			s.writeError(w, http.StatusUnprocessableEntity, "no_forecast", err.Error())
			return
		}
		s.writeError(w, http.StatusBadGateway, "forecast_unavailable", "forecast data unavailable")
		return
	}
	s.WriteJSON(w, http.StatusOK, ForecastResponse{Document: doc, Display: forecastDisplay(doc)})
}

func forecastDisplay(doc forecast.Document) ForecastDisplay {
	display := ForecastDisplay{Series: make([]ForecastSeriesDisplay, len(doc.Series))}
	if doc.Budget != nil {
		display.Budget = viewmodel.FormatMoney(*doc.Budget, doc.Currency)
	}
	for i, series := range doc.Series {
		points := make([]ForecastPointDisplay, len(series.Points))
		for j, point := range series.Points {
			points[j] = ForecastPointDisplay{
				Month: point.Time.UTC().Format(forecastMonthLayout),
				Value: viewmodel.FormatMoney(point.Value, doc.Currency),
			}
		}
		display.Series[i] = ForecastSeriesDisplay{Name: series.Name, Points: points}
	}
	return display
}
