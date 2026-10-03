package cli

import (
	"context"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

// renderProjectedDiff writes the cost projected diff. Table output is the diff
// table, including when stdout is a TTY, so before/after is what the command
// shows. JSON keeps summary.totalMonthly as the after total.
func renderProjectedDiff(
	cmd *cobra.Command, outputFormat string, diff *engine.DiffResult, showBreakdown bool,
) error {
	fmtType := engine.OutputFormat(config.GetOutputFormat(outputFormat))
	if !isValidOutputFormat(fmtType) {
		return fmt.Errorf("unsupported output format: %s", fmtType)
	}
	breakdown := showBreakdown && fmtType == engine.OutputTable
	return engine.RenderProjectedDiff(cmd.OutOrStdout(), fmtType, diff, breakdown)
}

// mergeProjectedDiffRecommendations attaches recommendations to the after cost
// of every entry except deletes. A fetch error is logged and does not fail
// the command.
func mergeProjectedDiffRecommendations(
	ctx context.Context,
	fetcher recommendationFetcher,
	resources []engine.ResourceDescriptor,
	diff *engine.DiffResult,
) {
	if diff == nil {
		return
	}
	recResources := make([]engine.ResourceDescriptor, 0, len(resources))
	recResults := make([]engine.CostResult, 0, len(diff.Entries))
	indexes := make([]int, 0, len(diff.Entries))
	for i := range diff.Entries {
		if diff.Entries[i].Operation == engine.DiffOperationDelete || diff.Entries[i].After == nil {
			continue
		}
		if i >= len(resources) {
			continue
		}
		recResources = append(recResources, resources[i])
		recResults = append(recResults, *diff.Entries[i].After)
		indexes = append(indexes, i)
	}
	fetchAndMergeRecommendations(ctx, fetcher, recResources, recResults)
	for j, i := range indexes {
		diff.Entries[i].After.Recommendations = recResults[j].Recommendations
	}
}

// RenderCostOutput routes the cost results to the appropriate rendering function
// based on the detected output mode (Plain, Styled, or Interactive).
// The context parameter enables trace ID propagation for contextual logging.
func RenderCostOutput(
	ctx context.Context,
	cmd *cobra.Command,
	outputFormat string,
	resultWithErrors *engine.CostResultWithErrors,
	showBreakdown bool,
) error {
	// 1. Determine and validate output format.
	fmtType := engine.OutputFormat(config.GetOutputFormat(outputFormat))

	// Validate format is supported before proceeding
	if !isValidOutputFormat(fmtType) {
		return fmt.Errorf("unsupported output format: %s", fmtType)
	}

	// 2. If output format is explicitly structured (JSON/NDJSON), bypass TUI completely.
	// This satisfies FR-004: Maintain output for --output json/ndjson.
	// --show-breakdown is a table-only flag and does not change these payloads.
	if fmtType == engine.OutputJSON || fmtType == engine.OutputNDJSON {
		return engine.RenderResults(cmd.OutOrStdout(), fmtType, resultWithErrors.Results)
	}

	if showBreakdown {
		return renderPlainProjected(cmd.OutOrStdout(), resultWithErrors, true)
	}

	// 2. Detect the appropriate output mode for this command's writer.
	mode := outputModeFromCmd(cmd)

	// 3. Route to specific renderer
	switch mode {
	case tui.OutputModeInteractive:
		return runInteractiveTUI(ctx, resultWithErrors)

	case tui.OutputModeStyled:
		return renderStyledOutput(ctx, cmd.OutOrStdout(), resultWithErrors)

	case tui.OutputModePlain:
		return renderPlainOutput(cmd.OutOrStdout(), resultWithErrors)

	default:
		return renderPlainOutput(cmd.OutOrStdout(), resultWithErrors)
	}
}

// RenderActualCostOutput routes actual cost results to the appropriate rendering function.
// The context parameter enables trace ID propagation for contextual logging.
func RenderActualCostOutput(
	ctx context.Context,
	cmd *cobra.Command,
	outputFormat string,
	resultWithErrors *engine.CostResultWithErrors,
	groupBy string,
	estimateConfidence bool,
	showBreakdown bool,
	showConfidence bool,
) error {
	fmtType := engine.OutputFormat(config.GetOutputFormat(outputFormat))

	// Validate format is supported before proceeding
	if !isValidOutputFormat(fmtType) {
		return fmt.Errorf("unsupported output format: %s", fmtType)
	}

	if fmtType == engine.OutputJSON || fmtType == engine.OutputNDJSON {
		// Table flags do not change JSON or NDJSON. estimate-confidence still
		// decides whether the confidence field is kept in those payloads.
		return renderActualCostOutput(cmd.OutOrStdout(), fmtType, resultWithErrors.Results, groupBy, estimateConfidence)
	}

	mode := outputModeFromCmd(cmd)
	if mode == tui.OutputModeInteractive && !showBreakdown && !showConfidence {
		return runInteractiveActualCostTUI(ctx, resultWithErrors, engine.GroupBy(groupBy))
	}

	if err := renderActualResourceTable(
		cmd.OutOrStdout(), resultWithErrors.Results, groupBy, estimateConfidence, showBreakdown, showConfidence,
	); err != nil {
		return err
	}
	displayErrorSummary(cmd, resultWithErrors, engine.OutputTable)
	return nil
}

// renderActualResourceTable writes the actual-cost table. Time-based grouping
// keeps the cross-provider table, which has no per-resource component rows.
func renderActualResourceTable(
	w io.Writer,
	results []engine.CostResult,
	groupBy string,
	estimateConfidence, showBreakdown, showConfidence bool,
) error {
	if engine.GroupBy(groupBy).IsTimeBasedGrouping() {
		return renderActualCostOutput(w, engine.OutputTable, results, groupBy, estimateConfidence)
	}
	return engine.RenderActualCostTable(w, results, engine.CostTableOptions{
		ShowBreakdown:  showBreakdown,
		ShowConfidence: estimateConfidence || showConfidence,
	})
}

func runInteractiveTUI(ctx context.Context, resultWithErrors *engine.CostResultWithErrors) error {
	p := tea.NewProgram(tui.NewCostViewModel(ctx, resultWithErrors.Results))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run interactive TUI: %w", err)
	}
	return nil
}

func runInteractiveActualCostTUI(
	ctx context.Context,
	resultWithErrors *engine.CostResultWithErrors,
	groupBy engine.GroupBy,
) error {
	p := tea.NewProgram(tui.NewCostViewModelFromActual(ctx, resultWithErrors.Results, groupBy))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run interactive TUI: %w", err)
	}
	return nil
}

// renderPlainOutput renders the standard table output (legacy behavior).
func renderPlainOutput(w io.Writer, resultWithErrors *engine.CostResultWithErrors) error {
	return renderPlainProjected(w, resultWithErrors, false)
}

func renderPlainProjected(w io.Writer, resultWithErrors *engine.CostResultWithErrors, showBreakdown bool) error {
	var err error
	if showBreakdown {
		err = engine.RenderCostTable(w, resultWithErrors.Results, engine.CostTableOptions{ShowBreakdown: true})
	} else {
		err = engine.RenderResults(w, engine.OutputTable, resultWithErrors.Results)
	}
	if err != nil {
		return err
	}

	if resultWithErrors.HasErrors() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ERRORS")
		fmt.Fprintln(w, "======")
		fmt.Fprint(w, resultWithErrors.ErrorSummary())
	}
	return nil
}

// renderStyledOutput renders the styled summary using Lip Gloss (T011).
// The ctx parameter enables trace ID propagation for contextual logging.
func renderStyledOutput(ctx context.Context, w io.Writer, resultWithErrors *engine.CostResultWithErrors) error {
	summary := tui.RenderCostSummary(ctx, resultWithErrors.Results, tui.TerminalWidth())
	fmt.Fprint(w, summary)

	// Display error summary using plain text format.
	// Error styling is intentionally kept simple for readability across terminals.
	if resultWithErrors.HasErrors() {
		fmt.Fprintln(w)
		fmt.Fprint(w, resultWithErrors.ErrorSummary())
	}

	return nil
}

// isValidOutputFormat checks if the provided format is one of the supported output formats.
func isValidOutputFormat(format engine.OutputFormat) bool {
	switch format {
	case engine.OutputTable, engine.OutputJSON, engine.OutputNDJSON:
		return true
	default:
		return false
	}
}
