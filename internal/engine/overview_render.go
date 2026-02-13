package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// Column widths for the overview table.
const (
	colWidthResource = 34
	colWidthType     = 24
	colWidthStatus   = 12
	colWidthActual   = 14
	colWidthProj     = 14
	colWidthDelta    = 14
	colWidthDrift    = 10
	colWidthRecs     = 6
)

// StatusIcon returns a single-character icon for a ResourceStatus.
func StatusIcon(status ResourceStatus) string {
	switch status {
	case StatusActive:
		return "\u2713" // check mark
	case StatusCreating:
		return "+"
	case StatusUpdating:
		return "~"
	case StatusDeleting:
		return "-"
	case StatusReplacing:
		return "\u21bb" // clockwise arrow
	default:
		return "?"
	}
}

// FormatOverviewCurrency formats an amount as "$X,XXX.XX".
// Negative values are formatted as "-$X,XXX.XX".
func FormatOverviewCurrency(amount float64) string {
	if amount == 0 {
		return "$0.00"
	}
	negative := amount < 0
	abs := math.Abs(amount)
	formatted := formatWithCommas(abs)
	if negative {
		return "-$" + formatted
	}
	return "$" + formatted
}

// FormatOverviewDelta formats a delta amount with a +/- prefix.
// Positive values get "+$", negative get "-$", zero gets "$0.00".
func FormatOverviewDelta(amount float64) string {
	if amount == 0 {
		return "$0.00"
	}
	abs := math.Abs(amount)
	formatted := formatWithCommas(abs)
	if amount > 0 {
		return "+$" + formatted
	}
	return "-$" + formatted
}

// centsMultiplier converts fractional dollars to cents.
const centsMultiplier = 100

// commaGroupSize is the number of digits between commas in formatted numbers.
const commaGroupSize = 3

// tabwriterPadding is the minimum padding between columns in the overview table.
const tabwriterPadding = 2

// truncateMinLen is the minimum truncation length below which no ellipsis is added.
const truncateMinLen = 3

// formatWithCommas formats a positive float64 as "X,XXX.XX".
func formatWithCommas(amount float64) string {
	whole := int64(amount)
	frac := amount - float64(whole)
	cents := int64(math.Round(frac * centsMultiplier))

	// Handle rounding up to next dollar
	if cents >= centsMultiplier {
		whole++
		cents -= centsMultiplier
	}

	// Format whole part with commas
	wholeStr := strconv.FormatInt(whole, 10)
	if len(wholeStr) > commaGroupSize {
		var parts []string
		for len(wholeStr) > commaGroupSize {
			parts = append([]string{wholeStr[len(wholeStr)-commaGroupSize:]}, parts...)
			wholeStr = wholeStr[:len(wholeStr)-commaGroupSize]
		}
		parts = append([]string{wholeStr}, parts...)
		wholeStr = strings.Join(parts, ",")
	}

	return fmt.Sprintf("%s.%02d", wholeStr, cents)
}

// truncateResource shortens a URN to fit the resource column.
func truncateResource(urn string, maxLen int) string {
	if len(urn) <= maxLen {
		return urn
	}
	if maxLen <= truncateMinLen {
		return urn[:maxLen]
	}
	return urn[:maxLen-3] + "..."
}

// RenderOverviewAsTable writes a formatted ASCII table of the overview rows.
func RenderOverviewAsTable(w io.Writer, rows []OverviewRow, stackCtx StackContext) error {
	tw := tabwriter.NewWriter(w, 0, 0, tabwriterPadding, ' ', 0)

	// Header
	if _, err := fmt.Fprintf(tw, "RESOURCE\tTYPE\tSTATUS\tACTUAL(MTD)\tPROJECTED\tDELTA\tDRIFT%%\tRECS\n"); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := fmt.Fprintf(tw, "--------\t----\t------\t-----------\t---------\t-----\t------\t----\n"); err != nil {
		return fmt.Errorf("writing separator: %w", err)
	}

	// Rows
	for _, row := range rows {
		resource := truncateResource(row.URN, colWidthResource)
		resType := truncateResource(row.Type, colWidthType)
		statusStr := StatusIcon(row.Status) + " " + row.Status.String()

		var actual, projected, delta, drift, recs string

		if row.Error != nil {
			actual = "ERR"
			projected = "ERR"
			delta = "-"
			drift = "-"
			recs = "-"
		} else {
			actual = formatActualColumn(row)
			projected = formatProjectedColumn(row)
			delta = formatDeltaColumn(row)
			drift = formatDriftColumn(row)
			recs = formatRecsColumn(row)
		}

		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			resource, resType, statusStr,
			actual, projected, delta, drift, recs,
		); err != nil {
			return fmt.Errorf("writing row: %w", err)
		}
	}

	// Summary footer
	if err := renderSummaryFooter(tw, rows, stackCtx); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	return tw.Flush()
}

func formatActualColumn(row OverviewRow) string {
	if row.ActualCost == nil {
		return "-"
	}
	return FormatOverviewCurrency(row.ActualCost.MTDCost)
}

func formatProjectedColumn(row OverviewRow) string {
	if row.ProjectedCost == nil {
		return "-"
	}
	return FormatOverviewCurrency(row.ProjectedCost.MonthlyCost)
}

// formatDeltaColumn formats the delta column. When CostDrift is available its
// pre-computed Delta is used for consistency with the TUI's buildOverviewTable.
// Otherwise falls back to Projected - MTD Actual.
func formatDeltaColumn(row OverviewRow) string {
	if row.CostDrift != nil {
		return FormatOverviewDelta(row.CostDrift.Delta)
	}
	if row.ProjectedCost == nil && row.ActualCost == nil {
		return "-"
	}
	projected := 0.0
	if row.ProjectedCost != nil {
		projected = row.ProjectedCost.MonthlyCost
	}
	actual := 0.0
	if row.ActualCost != nil {
		actual = row.ActualCost.MTDCost
	}
	return FormatOverviewDelta(projected - actual)
}

func formatDriftColumn(row OverviewRow) string {
	if row.CostDrift == nil {
		return "-"
	}
	sign := "+"
	if row.CostDrift.PercentDrift < 0 {
		sign = ""
	}
	result := fmt.Sprintf("%s%.0f%%", sign, row.CostDrift.PercentDrift)
	if row.CostDrift.IsWarning {
		result += " \u26a0"
	}
	return result
}

func formatRecsColumn(row OverviewRow) string {
	if len(row.Recommendations) == 0 {
		return "-"
	}
	return strconv.Itoa(len(row.Recommendations))
}

// overviewRowTotals holds aggregated totals from overview rows.
type overviewRowTotals struct {
	actual    float64
	projected float64
	savings   float64
	currency  string
	errors    []OverviewRowError
}

// aggregateOverviewRows computes totals across overview rows with currency
// consistency checking. Returns ErrMixedCurrencies if different non-empty
// currencies are encountered.
func aggregateOverviewRows(rows []OverviewRow) (overviewRowTotals, error) {
	var t overviewRowTotals
	for _, row := range rows {
		if row.Error != nil {
			t.errors = append(t.errors, *row.Error)
			continue
		}
		if row.ActualCost != nil {
			t.actual += row.ActualCost.MTDCost
			if err := checkCurrency(&t.currency, row.ActualCost.Currency); err != nil {
				return t, err
			}
		}
		if row.ProjectedCost != nil {
			t.projected += row.ProjectedCost.MonthlyCost
			if err := checkCurrency(&t.currency, row.ProjectedCost.Currency); err != nil {
				return t, err
			}
		}
		for _, rec := range row.Recommendations {
			t.savings += rec.EstimatedSavings
		}
	}
	if t.currency == "" {
		t.currency = "USD"
	}
	return t, nil
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

// renderSummaryFooter writes the summary line at the bottom of the table.
func renderSummaryFooter(tw *tabwriter.Writer, rows []OverviewRow, stackCtx StackContext) error {
	if _, err := fmt.Fprintf(tw, "\t\t\t\t\t\t\t\n"); err != nil {
		return err
	}

	t, aggErr := aggregateOverviewRows(rows)
	if aggErr != nil {
		return aggErr
	}

	totalDelta := t.projected - t.actual

	if _, writeErr := fmt.Fprintf(tw, "SUMMARY\t%s\t%d resources\t%s\t%s\t%s\t\t\n",
		stackCtx.StackName,
		stackCtx.TotalResources,
		FormatOverviewCurrency(t.actual)+" "+t.currency,
		FormatOverviewCurrency(t.projected)+" "+t.currency,
		FormatOverviewDelta(totalDelta)+" "+t.currency,
	); writeErr != nil {
		return writeErr
	}

	if t.savings > 0 {
		if _, writeErr := fmt.Fprintf(tw, "\t\t\t\tPotential Savings:\t%s %s\t\t\n",
			FormatOverviewCurrency(t.savings), t.currency); writeErr != nil {
			return writeErr
		}
	}

	if stackCtx.HasChanges {
		if _, writeErr := fmt.Fprintf(tw,
			"\t\t\t\t%d pending changes\t\t\t\n",
			stackCtx.PendingChanges,
		); writeErr != nil {
			return writeErr
		}
	}

	return nil
}

// OverviewMetadata holds metadata information for the JSON output.
// It embeds StackContext so field promotion avoids duplication.
// GeneratedAt is promoted from StackContext.
type OverviewMetadata struct {
	StackContext
}

// OverviewSummary holds aggregated summary statistics for the JSON output.
type OverviewSummary struct {
	TotalActualMTD   float64 `json:"totalActualMTD"`
	ProjectedMonthly float64 `json:"projectedMonthly"`
	ProjectedDelta   float64 `json:"projectedDelta"`
	PotentialSavings float64 `json:"potentialSavings"`
	Currency         string  `json:"currency"`
}

// OverviewJSONOutput is the top-level JSON output structure.
type OverviewJSONOutput struct {
	Metadata  OverviewMetadata   `json:"metadata"`
	Resources []OverviewRow      `json:"resources"`
	Summary   OverviewSummary    `json:"summary"`
	Errors    []OverviewRowError `json:"errors"`
}

// RenderOverviewAsJSON renders the overview rows as a structured JSON object
// with metadata, resource array, summary, and errors.
func RenderOverviewAsJSON(w io.Writer, rows []OverviewRow, stackCtx StackContext) error {
	t, err := aggregateOverviewRows(rows)
	if err != nil {
		return err
	}

	// Initialize resources to empty slice so JSON produces [] instead of null.
	resources := rows
	if resources == nil {
		resources = []OverviewRow{}
	}

	// Ensure errors is non-nil for consistent JSON output.
	errs := t.errors
	if errs == nil {
		errs = []OverviewRowError{}
	}

	// Use caller-provided GeneratedAt if set, otherwise fall back to time.Now().
	if stackCtx.GeneratedAt.IsZero() {
		stackCtx.GeneratedAt = time.Now()
	}

	// Build output structure
	output := OverviewJSONOutput{
		Metadata: OverviewMetadata{
			StackContext: stackCtx,
		},
		Resources: resources,
		Summary: OverviewSummary{
			TotalActualMTD:   t.actual,
			ProjectedMonthly: t.projected,
			ProjectedDelta:   t.projected - t.actual,
			PotentialSavings: t.savings,
			Currency:         t.currency,
		},
		Errors: errs,
	}

	// Marshal with indentation
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if encErr := encoder.Encode(output); encErr != nil {
		return fmt.Errorf("encoding JSON: %w", encErr)
	}

	return nil
}

// RenderOverviewAsNDJSON renders each overview row as a separate JSON line
// with no metadata wrapper or summary.
func RenderOverviewAsNDJSON(w io.Writer, rows []OverviewRow) error {
	for _, row := range rows {
		data, marshalErr := json.Marshal(row)
		if marshalErr != nil {
			return fmt.Errorf("marshaling row: %w", marshalErr)
		}
		if _, writeErr := fmt.Fprintf(w, "%s\n", data); writeErr != nil {
			return fmt.Errorf("writing NDJSON line: %w", writeErr)
		}
	}
	return nil
}
