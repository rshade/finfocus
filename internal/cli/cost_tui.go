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
	opts := actualTableOptions(cmd, groupBy, estimateConfidence, showBreakdown, showConfidence)
	if mode == tui.OutputModeInteractive && !showBreakdown && !showConfidence {
		return runInteractiveActualCostTUI(ctx, resultWithErrors, engine.GroupBy(groupBy), opts.Trends)
	}

	if err := renderActualResourceTable(cmd.OutOrStdout(), resultWithErrors.Results, groupBy, opts); err != nil {
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
	opts engine.CostTableOptions,
) error {
	if engine.GroupBy(groupBy).IsTimeBasedGrouping() {
		return renderActualCostOutput(w, engine.OutputTable, results, groupBy, opts.ShowConfidence)
	}
	return engine.RenderActualCostTable(w, results, opts)
}

func actualTableOptions(
	cmd *cobra.Command,
	groupBy string,
	estimateConfidence, showBreakdown, showConfidence bool,
) engine.CostTableOptions {
	opts := engine.CostTableOptions{
		ShowBreakdown:  showBreakdown,
		ShowConfidence: estimateConfidence || showConfidence,
	}
	if engine.GroupBy(groupBy).IsTimeBasedGrouping() {
		opts.ShowConfidence = estimateConfidence
		return opts
	}
	opts.Trends, opts.TotalTrend = costTableTrends(cmd)
	return opts
}

func runInteractiveActualCostTUI(
	ctx context.Context,
	resultWithErrors *engine.CostResultWithErrors,
	groupBy engine.GroupBy,
	trends map[string]string,
) error {
	model := tui.NewCostViewModelFromActual(ctx, resultWithErrors.Results, groupBy).WithTrends(trends)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run interactive TUI: %w", err)
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
