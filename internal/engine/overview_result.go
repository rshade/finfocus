package engine

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// OverviewRowResult holds display-ready values for a single resource row.
// All values are pre-computed by ComputeOverviewResult (or
// ComputeOverviewRowResult) so that every renderer (table, JSON, NDJSON, TUI)
// reads identical data instead of recomputing it independently.
type OverviewRowResult struct {
	// Identity.
	URN         string
	Type        string
	DisplayName string // rightmost URN component
	Status      ResourceStatus

	// Cluster expansion.
	ParentURN       string
	ChildURNs       []string
	ExpansionSource string
	LiveChildName   string // "ns/<namespace>" or "(idle)" for a live child, else ""

	// Cost values (nil = not applicable).
	ActualMTD    *float64 // month-to-date actual cost
	Projected    *float64 // projected monthly cost
	Delta        *float64 // status-aware delta (from ComputedDelta)
	DriftPct     *float64 // drift percentage
	DriftWarning bool     // whether drift exceeds the warning threshold

	// Pre-formatted display strings in the canonical table format.
	ResourceDisplay  string // URN, or "↳ " + child name for an expansion child
	StatusDisplay    string // status icon + label
	ActualDisplay    string // FormatOverviewCurrency, "-", or "ERR"
	ProjectedDisplay string // FormatOverviewCurrency, "-", or "ERR"
	DeltaDisplay     string // FormatOverviewDelta or "-"
	DriftDisplay     string // signed percent with optional warning icon, or "-"
	RecsDisplay      string // "N", "N(-M)", or "-"

	// Metadata.
	ActiveRecs    int
	DismissedRecs int
	HasError      bool

	// Detail data (for the TUI detail view). Pointers and slices are shared
	// with the source row, not copied.
	PropertyDiffs         []PropertyDiff
	Recommendations       []Recommendation
	Warnings              []OverviewWarning
	ActualCost            *ActualCostData
	ProjectedCost         *ProjectedCostData
	BaselineProjectedCost *ProjectedCostData
	CostDrift             *CostDriftData
	Error                 *OverviewRowError

	// Source is the original enriched row, retained for JSON/NDJSON
	// serialization (which marshal the full OverviewRow schema) and for
	// day-dependent extrapolation helpers in the TUI detail view.
	Source OverviewRow
}

// OverviewTotals holds pre-computed aggregate totals across all rows. It is
// serialized as OverviewSummary in the JSON output.
// Computed once by ComputeOverviewResult and shared by every renderer.
type OverviewTotals struct {
	TotalActual    float64
	TotalProjected float64
	TotalDelta     float64
	TotalSavings   float64
	Currency       string
	// Errors collects per-row errors for the JSON errors array.
	Errors []OverviewRowError
	// MixedCurrencies is set when rows carry different non-empty currencies.
	// The totals are then meaningless, so the table and JSON renderers return
	// ErrMixedCurrencies; NDJSON has no totals and still renders.
	MixedCurrencies bool
}

// OverviewResult holds all pre-computed display values for the overview.
type OverviewResult struct {
	Rows    []OverviewRowResult
	Summary OverviewTotals
}

// ComputeOverviewResult computes every display value for the overview once:
// per-row deltas (via PopulateComputedDeltas), per-row display values, and the
// aggregate summary. Renderers consume the result without recomputing.
// The caller's rows are not modified.
func ComputeOverviewResult(rows []OverviewRow, dayOfMonth int) OverviewResult {
	rows = slices.Clone(rows)
	PopulateComputedDeltas(rows, dayOfMonth)

	results := make([]OverviewRowResult, len(rows))
	for i := range rows {
		results[i] = ComputeOverviewRowResult(rows[i])
	}

	return OverviewResult{Rows: results, Summary: summarizeOverviewRows(rows)}
}

// ComputeOverviewRowResult computes display-ready values for a single row.
// It reads the pre-computed ComputedDelta: PopulateComputedDeltas (or
// ComputeOverviewResult) must run first so deltas match across renderers.
func ComputeOverviewRowResult(row OverviewRow) OverviewRowResult {
	res := OverviewRowResult{
		URN:                   row.URN,
		Type:                  row.Type,
		DisplayName:           ExtractResourceDisplayName(row.URN),
		Status:                row.Status,
		ParentURN:             row.ParentURN,
		ChildURNs:             row.ChildURNs,
		ExpansionSource:       row.ExpansionSource,
		LiveChildName:         LiveChildName(row),
		Delta:                 row.ComputedDelta,
		HasError:              row.Error != nil,
		PropertyDiffs:         row.PropertyDiffs,
		Recommendations:       row.Recommendations,
		Warnings:              row.Warnings,
		ActualCost:            row.ActualCost,
		ProjectedCost:         row.ProjectedCost,
		BaselineProjectedCost: row.BaselineProjectedCost,
		CostDrift:             row.CostDrift,
		Error:                 row.Error,
		Source:                row,
	}

	res.ResourceDisplay = overviewResourceDisplay(row, res.LiveChildName)
	res.StatusDisplay = StatusIcon(row.Status) + " " + row.Status.String()

	if row.ActualCost != nil {
		mtd := row.ActualCost.MTDCost
		res.ActualMTD = &mtd
	}
	if row.ProjectedCost != nil {
		monthly := row.ProjectedCost.MonthlyCost
		res.Projected = &monthly
	}
	if row.CostDrift != nil {
		pct := row.CostDrift.PercentDrift
		res.DriftPct = &pct
		res.DriftWarning = row.CostDrift.IsWarning
	}
	if len(row.Recommendations) > 0 {
		res.ActiveRecs, res.DismissedRecs = CountRecsActiveAndDismissed(row.Recommendations)
	}

	res.computeDisplayStrings()
	return res
}

// computeDisplayStrings derives the canonical table-format display strings.
func (r *OverviewRowResult) computeDisplayStrings() {
	if r.HasError {
		r.ActualDisplay = "ERR"
		r.ProjectedDisplay = "ERR"
		r.DeltaDisplay = "-"
		r.DriftDisplay = "-"
		r.RecsDisplay = "-"
		return
	}

	r.ActualDisplay = "-"
	if r.ActualMTD != nil {
		r.ActualDisplay = FormatOverviewCurrency(*r.ActualMTD)
	}

	r.ProjectedDisplay = "-"
	if r.Projected != nil {
		r.ProjectedDisplay = FormatOverviewCurrency(*r.Projected)
	}

	r.DeltaDisplay = "-"
	if r.Delta != nil {
		r.DeltaDisplay = FormatOverviewDelta(*r.Delta)
	}

	r.DriftDisplay = "-"
	if r.DriftPct != nil {
		r.DriftDisplay = FormatOverviewDrift(*r.DriftPct, r.DriftWarning)
	}

	r.RecsDisplay = "-"
	if total := r.ActiveRecs + r.DismissedRecs; total > 0 {
		r.RecsDisplay = strconv.Itoa(total)
		if r.DismissedRecs > 0 {
			r.RecsDisplay = fmt.Sprintf("%d(-%d)", total, r.DismissedRecs)
		}
	}
}

// FormatOverviewDrift formats a drift percentage with an explicit +/- sign
// and appends a warning icon when the drift exceeds the warning threshold.
func FormatOverviewDrift(percentDrift float64, isWarning bool) string {
	sign := "+"
	if percentDrift < 0 {
		sign = ""
	}
	result := fmt.Sprintf("%s%.0f%%", sign, percentDrift)
	if isWarning {
		result += " \u26a0"
	}
	return result
}

// checkCurrency validates that currency is consistent. On first non-empty
// value it sets *current; on subsequent non-empty values it returns
// ErrMixedCurrencies if they differ.
func checkCurrency(current *string, next string) error {
	if next == "" {
		return nil
	}
	if *current == "" {
		*current = next
	} else if next != *current {
		return ErrMixedCurrencies
	}
	return nil
}

// summarizeOverviewRows computes the aggregate summary across overview rows
// with currency consistency checking. Per-row deltas are summed from the
// pre-computed ComputedDelta values so the summary matches the rendered rows.
func summarizeOverviewRows(rows []OverviewRow) OverviewTotals {
	var s OverviewTotals
	for i := range rows {
		if rows[i].ComputedDelta != nil {
			s.TotalDelta += *rows[i].ComputedDelta
		}
		if err := s.accumulateRow(&rows[i]); err != nil {
			s.MixedCurrencies = true
			break
		}
	}
	if s.Currency == "" {
		s.Currency = defaultCurrency
	}
	return s
}

// accumulateRow adds a single row's costs, savings, and errors to the summary.
// Live allocation children re-allocate node cost already represented by the
// cluster's node rows, so their costs are skipped to avoid double counting.
func (s *OverviewTotals) accumulateRow(row *OverviewRow) error {
	if row.Error != nil {
		s.Errors = append(s.Errors, *row.Error)
		return nil
	}
	if row.ExpansionSource == ExpansionSourceLive {
		return nil
	}
	if row.ActualCost != nil {
		s.TotalActual += row.ActualCost.MTDCost
		if err := checkCurrency(&s.Currency, row.ActualCost.Currency); err != nil {
			return err
		}
	}
	if row.ProjectedCost != nil {
		s.TotalProjected += row.ProjectedCost.MonthlyCost
		if err := checkCurrency(&s.Currency, row.ProjectedCost.Currency); err != nil {
			return err
		}
	}
	for _, rec := range row.Recommendations {
		if rec.Status != RecommendationStatusDismissed &&
			rec.Status != RecommendationStatusSnoozed {
			s.TotalSavings += rec.EstimatedSavings
		}
	}
	return nil
}

// overviewResourceDisplay returns the RESOURCE column text: the URN, or for
// an expansion child a "↳ " indent before the namespace (live rows) or URN
// (projected rows).
func overviewResourceDisplay(row OverviewRow, liveChildName string) string {
	if row.ParentURN == "" {
		return row.URN
	}
	if liveChildName != "" {
		return "↳ " + liveChildName
	}
	return "↳ " + row.URN
}

// ExtractResourceDisplayName returns the rightmost "::"-separated URN
// component for display.
func ExtractResourceDisplayName(urn string) string {
	if urn == "" {
		return urn
	}
	parts := strings.Split(urn, "::")
	return parts[len(parts)-1]
}

// TruncateOverviewResource shortens s to maxLen runes, appending an ellipsis
// when truncation occurs. It is rune-aware so multibyte UTF-8 characters are
// never split. Returns "" when maxLen <= 0.
func TruncateOverviewResource(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= truncateMinLen {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-truncateMinLen]) + "..."
}
