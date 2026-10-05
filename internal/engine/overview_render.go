package engine

import (
	"context"
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

// RenderOverviewAsTable writes a formatted ASCII table of the overview result.
// All row values and totals are read from the pre-computed OverviewResult;
// this function performs formatting only.
func RenderOverviewAsTable(w io.Writer, result OverviewResult, stackCtx StackContext) error {
	tw := tabwriter.NewWriter(w, 0, 0, tabwriterPadding, ' ', 0)

	projectedHeader := "PROJECTED"
	projectedSep := "---------"
	if stackCtx.IsStateOnly {
		projectedHeader = "PROJECTED*"
		projectedSep = "----------"
	}

	// Header
	header := "RESOURCE\tTYPE\tSTATUS\tACTUAL(MTD)\t" + projectedHeader + "\tDELTA\tDRIFT%\tRECS\tWARN\n"
	if _, err := fmt.Fprint(tw, header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	sep := "--------\t----\t------\t-----------\t" + projectedSep + "\t-----\t------\t----\t----\n"
	if _, err := fmt.Fprint(tw, sep); err != nil {
		return fmt.Errorf("writing separator: %w", err)
	}

	// Rows
	for _, row := range result.Rows {
		resource := TruncateOverviewResource(row.ResourceDisplay, colWidthResource)
		resType := TruncateOverviewResource(row.Type, colWidthType)

		warn := FormatOverviewWarnings(row.Warnings)
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			resource, resType, row.StatusDisplay,
			row.ActualDisplay, row.ProjectedDisplay, row.DeltaDisplay,
			row.DriftDisplay, row.RecsDisplay, warn,
		); err != nil {
			return fmt.Errorf("writing row: %w", err)
		}
	}

	// Summary footer
	if err := renderSummaryFooter(tw, result.Summary, stackCtx); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing table: %w", err)
	}

	// State-only footnote written after flush so tabwriter alignment does not affect it.
	if stackCtx.IsStateOnly {
		footnote := fmt.Sprintf(
			"* projected at current state (%dh/mo) — rerun with --yes to include pending changes",
			HoursPerMonth,
		)
		if _, err := fmt.Fprintln(w, footnote); err != nil {
			return fmt.Errorf("writing state-only footnote: %w", err)
		}
	}
	// Cluster-expansion footnotes (live data preferred, assumed context).
	for _, note := range stackCtx.ExpansionNotes {
		if _, err := fmt.Fprintln(w, "† "+note); err != nil {
			return fmt.Errorf("writing expansion footnote: %w", err)
		}
	}
	return nil
}

// FormatOverviewWarnings renders the Warn column. An empty list is "-".
// Names stay in derivation order.
func FormatOverviewWarnings(warnings []OverviewWarning) string {
	if len(warnings) == 0 {
		return "-"
	}
	parts := make([]string, len(warnings))
	for i, warning := range warnings {
		parts[i] = string(warning)
	}
	return strings.Join(parts, ",")
}

// renderSummaryFooter writes the summary line at the bottom of the table,
// reading the pre-computed totals from the OverviewSummary.
func renderSummaryFooter(tw *tabwriter.Writer, summary OverviewSummary, stackCtx StackContext) error {
	if summary.MixedCurrencies {
		return ErrMixedCurrencies
	}
	if _, err := fmt.Fprintf(tw, "\t\t\t\t\t\t\t\t\n"); err != nil {
		return err
	}

	if _, writeErr := fmt.Fprintf(tw, "SUMMARY\t%s\t%d resources\t%s\t%s\t%s\t\t\t\n",
		stackCtx.StackName,
		stackCtx.TotalResources,
		FormatOverviewCurrency(summary.TotalActual)+" "+summary.Currency,
		FormatOverviewCurrency(summary.TotalProjected)+" "+summary.Currency,
		FormatOverviewDelta(summary.TotalDelta)+" "+summary.Currency,
	); writeErr != nil {
		return writeErr
	}

	if summary.TotalSavings > 0 {
		if _, writeErr := fmt.Fprintf(tw, "\t\t\t\tPotential Savings:\t%s %s\t\t\t\n",
			FormatOverviewCurrency(summary.TotalSavings), summary.Currency); writeErr != nil {
			return writeErr
		}
	}

	if stackCtx.HasChanges {
		if _, writeErr := fmt.Fprintf(tw,
			"\t\t\t\t%d pending changes\t\t\t\t\n",
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

// OverviewJSONSummary holds aggregated summary statistics for the JSON output.
type OverviewJSONSummary struct {
	TotalActualMTD   float64 `json:"totalActualMTD"`
	ProjectedMonthly float64 `json:"projectedMonthly"`
	ProjectedDelta   float64 `json:"projectedDelta"`
	PotentialSavings float64 `json:"potentialSavings"`
	Currency         string  `json:"currency"`
}

// OverviewJSONOutput is the top-level JSON output structure.
type OverviewJSONOutput struct {
	Metadata  OverviewMetadata     `json:"metadata"`
	Resources []OverviewRow        `json:"resources"`
	Summary   OverviewJSONSummary  `json:"summary"`
	Budgets   []BudgetHealthResult `json:"budgets,omitempty"`
	Errors    []OverviewRowError   `json:"errors"`
}

// RenderOverviewAsJSON renders the overview result as a structured JSON object
// with metadata, resource array, summary, optional budgets, and errors.
// Resources are marshaled from each row's Source OverviewRow so the serialized
// schema is unchanged; totals come from the pre-computed summary.
//
// Parameters:
//   - ctx: context for logging and tracing; threaded to CalculateBudgetHealthResults.
//   - w: destination writer for the JSON output.
//   - result: pre-computed overview result from ComputeOverviewResult.
//   - stackCtx: stack context and metadata used in the output; GeneratedAt will be populated with the current time if zero.
//   - budgetResult: optional budget data; when non-nil and non-empty, converted budgets are included
//     in the `budgets` field. May be nil, in which case no budget entries are emitted.
//
// Returns ErrMixedCurrencies when the rows carry different currencies, or an
// error if encoding/writing the JSON output fails.
func RenderOverviewAsJSON(
	ctx context.Context, w io.Writer, result OverviewResult,
	stackCtx StackContext, budgetResult *BudgetResult,
) error {
	if result.Summary.MixedCurrencies {
		return ErrMixedCurrencies
	}

	// Initialize resources to empty slice so JSON produces [] instead of null.
	resources := make([]OverviewRow, len(result.Rows))
	for i := range result.Rows {
		resources[i] = result.Rows[i].Source
	}

	// Ensure errors is non-nil for consistent JSON output.
	errs := result.Summary.Errors
	if errs == nil {
		errs = []OverviewRowError{}
	}

	// Use caller-provided GeneratedAt if set, otherwise fall back to time.Now().
	if stackCtx.GeneratedAt.IsZero() {
		stackCtx.GeneratedAt = time.Now()
	}

	// Convert budget data to BudgetHealthResult entries when available.
	var budgets []BudgetHealthResult
	if budgetResult != nil && len(budgetResult.Budgets) > 0 {
		budgets = CalculateBudgetHealthResults(ctx, budgetResult.Budgets)
	}

	// Build output structure
	output := OverviewJSONOutput{
		Metadata: OverviewMetadata{
			StackContext: stackCtx,
		},
		Resources: resources,
		Summary: OverviewJSONSummary{
			TotalActualMTD:   result.Summary.TotalActual,
			ProjectedMonthly: result.Summary.TotalProjected,
			ProjectedDelta:   result.Summary.TotalDelta,
			PotentialSavings: result.Summary.TotalSavings,
			Currency:         result.Summary.Currency,
		},
		Budgets: budgets,
		Errors:  errs,
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
// with no metadata wrapper or summary. Rows are marshaled from each result
// row's Source OverviewRow, which carries the pre-computed ComputedDelta.
func RenderOverviewAsNDJSON(w io.Writer, result OverviewResult) error {
	for _, row := range result.Rows {
		data, marshalErr := json.Marshal(row.Source)
		if marshalErr != nil {
			return fmt.Errorf("marshaling row: %w", marshalErr)
		}
		if _, writeErr := fmt.Fprintf(w, "%s\n", data); writeErr != nil {
			return fmt.Errorf("writing NDJSON line: %w", writeErr)
		}
	}
	return nil
}
