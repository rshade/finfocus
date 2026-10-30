package webui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// History statuses. NoHistory means there is nothing stored to read yet: the
// stack was never collected, the session has no stack, or a diff found no
// snapshots. It is not a failure.
const (
	HistoryStatusOK        = "ok"
	HistoryStatusNoHistory = "no_history"
)

const historyDateLayout = "2006-01-02"

// HistoryViewQuery is the dashboard's `cost history view` request. From and To
// use the --from and --to syntax, so a bad value gets the CLI's message.
type HistoryViewQuery struct {
	From     string
	To       string
	Currency string
	Strict   bool
}

// HistoryDiffQuery is the dashboard's `cost history diff` request. From and To
// are snapshot selectors (v35 or YYYY-MM-DD); an empty To is the newest.
type HistoryDiffQuery struct {
	From      string
	To        string
	Threshold float64
}

// HistoryViewResult is the `cost history view --output json` document plus the
// configured global budget the CLI chart draws. Budget is nil when unset.
type HistoryViewResult struct {
	Document       history.ViewDocument
	Budget         *float64
	BudgetCurrency string
}

// HistoryPoint is one resource's monthly cost at one snapshot, as the terminal
// detail view loads it.
type HistoryPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
}

// HistoryViewResponse is the view document plus status, budget, and display
// strings, so the page formats nothing itself.
type HistoryViewResponse struct {
	history.ViewDocument

	Status  string             `json:"status"`
	Budget  *float64           `json:"budget,omitempty"`
	Display HistoryViewDisplay `json:"display"`
}

// HistoryViewDisplay holds the formatted budget and one entry per snapshot, in
// snapshot order.
type HistoryViewDisplay struct {
	Budget    string                   `json:"budget,omitempty"`
	Snapshots []HistorySnapshotDisplay `json:"snapshots"`
}

// HistorySnapshotDisplay is one snapshot's version, UTC date, and total.
type HistorySnapshotDisplay struct {
	Version int    `json:"version"`
	Date    string `json:"date"`
	Total   string `json:"total"`
}

// HistoryDiffResponse is the diff document plus status and display strings.
type HistoryDiffResponse struct {
	history.DiffDocument

	Status  string             `json:"status"`
	Display HistoryDiffDisplay `json:"display"`
}

// HistoryDiffDisplay holds the formatted totals and one entry per item, in the
// document's order.
type HistoryDiffDisplay struct {
	Delta          string               `json:"delta"`
	From           string               `json:"from"`
	To             string               `json:"to"`
	UnchangedTotal string               `json:"unchangedTotal"`
	Added          []HistoryItemDisplay `json:"added"`
	Removed        []HistoryItemDisplay `json:"removed"`
	Changed        []HistoryItemDisplay `json:"changed"`
}

// HistoryItemDisplay is one diff item's formatted monthly cost and change.
type HistoryItemDisplay struct {
	MonthlyCost string `json:"monthlyCost"`
	Delta       string `json:"delta"`
}

// HistoryResourceResponse is one resource's series plus status and display strings.
type HistoryResourceResponse struct {
	Status  string                 `json:"status"`
	ID      string                 `json:"id"`
	Points  []HistoryPoint         `json:"points"`
	Display HistoryResourceDisplay `json:"display"`
}

// HistoryResourceDisplay holds one formatted entry per point, in point order.
type HistoryResourceDisplay struct {
	Points []HistoryPointDisplay `json:"points"`
}

// HistoryPointDisplay is one point's UTC date and amount.
type HistoryPointDisplay struct {
	Date   string `json:"date"`
	Amount string `json:"amount"`
}

type historyStatusResponse struct {
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
}

func (s *Server) registerHistory(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/history/view", s.handleHistoryView)
	mux.HandleFunc("GET /api/history/diff", s.handleHistoryDiff)
	mux.HandleFunc("GET /api/history/resource", s.handleHistoryResource)
}

func (s *Server) handleHistoryView(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	query := HistoryViewQuery{From: values.Get("from"), To: values.Get("to"), Currency: values.Get("currency")}
	if raw := values.Get("strict"); raw != "" {
		strict, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_history_query",
				fmt.Sprintf("invalid strict value %q (use true or false)", raw))
			return
		}
		query.Strict = strict
	}
	if s.session == nil || s.session.opts.HistoryView == nil {
		s.writeHistoryNotReady(w)
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeHistoryNotReady(w)
		return
	}
	result, err := s.session.opts.HistoryView(ctx, query)
	done()
	if err != nil {
		s.writeHistoryError(w, "history_view", "", err)
		return
	}
	s.WriteJSON(w, http.StatusOK, HistoryViewResponse{
		ViewDocument: result.Document,
		Status:       HistoryStatusOK,
		Budget:       result.Budget,
		Display:      historyViewDisplay(result),
	})
}

func (s *Server) handleHistoryDiff(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	query := HistoryDiffQuery{From: values.Get("from"), To: values.Get("to")}
	if raw := strings.TrimSpace(values.Get("threshold")); raw != "" {
		threshold, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_history_query",
				fmt.Sprintf("invalid threshold %q", raw))
			return
		}
		query.Threshold = threshold
	}
	if s.session == nil || s.session.opts.HistoryDiff == nil {
		s.writeHistoryNotReady(w)
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeHistoryNotReady(w)
		return
	}
	doc, err := s.session.opts.HistoryDiff(ctx, query)
	done()
	if err != nil {
		s.writeHistoryError(w, "history_diff", "", err)
		return
	}
	s.WriteJSON(w, http.StatusOK, HistoryDiffResponse{
		DiffDocument: doc,
		Status:       HistoryStatusOK,
		Display:      historyDiffDisplay(doc),
	})
}

func (s *Server) handleHistoryResource(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if strings.TrimSpace(id) == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_query", "resource id required")
		return
	}
	if s.session == nil || s.session.opts.HistoryResource == nil {
		s.writeHistoryNotReady(w)
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeHistoryNotReady(w)
		return
	}
	points, err := s.session.opts.HistoryResource(ctx, id)
	done()
	if err != nil {
		s.writeHistoryError(w, "history_resource", id, err)
		return
	}
	if points == nil {
		points = []HistoryPoint{}
	}
	display := HistoryResourceDisplay{Points: make([]HistoryPointDisplay, len(points))}
	for i, point := range points {
		display.Points[i] = HistoryPointDisplay{
			Date:   point.Timestamp.UTC().Format(historyDateLayout),
			Amount: viewmodel.FormatMoney(point.Amount, point.Currency),
		}
	}
	s.WriteJSON(w, http.StatusOK, HistoryResourceResponse{
		Status: HistoryStatusOK, ID: id, Points: points, Display: display,
	})
}

func (s *Server) writeHistoryNotReady(w http.ResponseWriter) {
	s.writeError(w, http.StatusServiceUnavailable, "not_ready", "cost history is loading")
}

// writeHistoryError answers "no history" as a 200 status, a caller mistake as a
// 400 with the CLI's message, and any other failure as a fixed 502 that never
// carries the Go error, which can name a file path.
func (s *Server) writeHistoryError(w http.ResponseWriter, operation, id string, err error) {
	switch {
	case history.IsNoHistory(err):
		s.WriteJSON(w, http.StatusOK, historyStatusResponse{Status: HistoryStatusNoHistory, ID: id})
	case history.IsQueryError(err):
		s.writeError(w, http.StatusBadRequest, "invalid_history_query", err.Error())
	default:
		level := zerolog.WarnLevel
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			level = zerolog.DebugLevel
		}
		s.logger.WithLevel(level).Err(err).Str("component", "webui").Str("operation", operation).
			Msg("cost history read failed")
		s.writeError(w, http.StatusBadGateway, "history_unavailable", "cost history unavailable")
	}
}

func historyViewDisplay(result HistoryViewResult) HistoryViewDisplay {
	display := HistoryViewDisplay{
		Snapshots: make([]HistorySnapshotDisplay, len(result.Document.Snapshots)),
	}
	if result.Budget != nil {
		display.Budget = viewmodel.FormatMoney(*result.Budget, result.BudgetCurrency)
	}
	for i, snapshot := range result.Document.Snapshots {
		display.Snapshots[i] = HistorySnapshotDisplay{
			Version: snapshot.Version,
			Date:    snapshot.Timestamp.UTC().Format(historyDateLayout),
			Total:   viewmodel.FormatMoney(snapshot.TotalMonthly, snapshot.Currency),
		}
	}
	return display
}

func historyDiffDisplay(doc history.DiffDocument) HistoryDiffDisplay {
	return HistoryDiffDisplay{
		Delta:          signedMoney(doc.Delta, doc.Currency),
		From:           viewmodel.FormatMoney(doc.From.TotalMonthly, doc.Currency),
		To:             viewmodel.FormatMoney(doc.To.TotalMonthly, doc.Currency),
		UnchangedTotal: viewmodel.FormatMoney(doc.UnchangedTotal, doc.Currency),
		Added:          historyItemsDisplay(doc.Added, doc.Currency),
		Removed:        historyItemsDisplay(doc.Removed, doc.Currency),
		Changed:        historyItemsDisplay(doc.Changed, doc.Currency),
	}
}

func historyItemsDisplay(items []history.DiffItemDocument, currency string) []HistoryItemDisplay {
	out := make([]HistoryItemDisplay, len(items))
	for i, item := range items {
		out[i] = HistoryItemDisplay{
			MonthlyCost: viewmodel.FormatMoney(item.MonthlyCost, currency),
			Delta:       signedMoney(item.Delta, currency),
		}
	}
	return out
}

// signedMoney marks an increase with "+"; FormatMoney already marks a decrease.
func signedMoney(amount float64, currency string) string {
	formatted := viewmodel.FormatMoney(amount, currency)
	if amount > 0 {
		return "+" + formatted
	}
	return formatted
}
