package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

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
	if err := engine.RenderProjectedDiff(cmd.OutOrStdout(), fmtType, diff, breakdown); err != nil {
		return err
	}
	// NDJSON carries no stack line, so an unchanged plan is explained on stderr.
	if summary, ok := engine.NoChangesStackSummary(diff); ok && fmtType == engine.OutputNDJSON {
		fmt.Fprint(cmd.ErrOrStderr(), summary)
	}
	return nil
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
	// Pricing omits internal types and separates state-origin entries, so
	// diff indexes do not necessarily match the original inventory indexes.
	type resourceKey struct{ id, resourceType string }
	byResource := make(map[resourceKey]engine.ResourceDescriptor, len(resources))
	for _, resource := range resources {
		if resource.Origin == engine.OriginState || resource.Operation == "delete-replaced" {
			continue
		}
		byResource[resourceKey{resource.ID, resource.Type}] = resource
	}
	recResources := make([]engine.ResourceDescriptor, 0, len(resources))
	recResults := make([]engine.CostResult, 0, len(diff.Entries))
	indexes := make([]int, 0, len(diff.Entries))
	for i := range diff.Entries {
		if diff.Entries[i].Operation == engine.DiffOperationDelete || diff.Entries[i].After == nil {
			continue
		}
		resource, ok := byResource[resourceKey{diff.Entries[i].ResourceID, diff.Entries[i].ResourceType}]
		if !ok {
			continue
		}
		recResources = append(recResources, resource)
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
		return renderActualMachineOutput(cmd.OutOrStdout(), fmtType, resultWithErrors, groupBy, estimateConfidence)
	}

	mode := outputModeFromCmd(cmd)
	opts := actualTableOptions(cmd, groupBy, estimateConfidence, showBreakdown, showConfidence)
	if mode == tui.OutputModeInteractive && !showBreakdown && !showConfidence && resultWithErrors.HistoryImport == nil {
		stack, _ := cmd.Flags().GetString("stack")
		return runInteractiveActualCostTUI(ctx, resultWithErrors, engine.GroupBy(groupBy), opts.Trends, stack)
	}

	if err := renderActualResourceTable(cmd.OutOrStdout(), resultWithErrors.Results, groupBy, opts); err != nil {
		return err
	}
	displayErrorSummary(cmd, resultWithErrors, engine.OutputTable)
	if resultWithErrors.HistoryImport != nil {
		_, err := fmt.Fprintf(cmd.OutOrStdout(),
			"Cloud history import incomplete: %d unreadable updates; totals may be incomplete.\n",
			resultWithErrors.HistoryImport.UnreadableUpdates)
		return err
	}
	return nil
}

// renderActualMachineOutput adds import status around the normal payload, so
// time-based grouping and confidence handling retain their existing behavior.
func renderActualMachineOutput(
	writer io.Writer,
	format engine.OutputFormat,
	view *engine.CostResultWithErrors,
	groupBy string,
	estimateConfidence bool,
) error {
	if view.HistoryImport == nil {
		return renderActualCostOutput(writer, format, view.Results, groupBy, estimateConfidence)
	}
	if format == engine.OutputNDJSON {
		if err := renderActualCostOutput(writer, format, view.Results, groupBy, estimateConfidence); err != nil {
			return err
		}
		return json.NewEncoder(writer).Encode(struct {
			Type          string                      `json:"type"`
			HistoryImport *engine.HistoryImportStatus `json:"historyImport"`
		}{"historyImport", view.HistoryImport})
	}
	results := view.Results
	if results == nil {
		results = []engine.CostResult{}
	}
	var payload bytes.Buffer
	if err := renderActualCostOutput(&payload, format, results, groupBy, estimateConfidence); err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(struct {
		Results       json.RawMessage             `json:"results"`
		HistoryImport *engine.HistoryImportStatus `json:"historyImport"`
	}{json.RawMessage(payload.Bytes()), view.HistoryImport})
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
	stack string,
) error {
	model := tui.NewCostViewModelFromActual(ctx, resultWithErrors.Results, groupBy).WithTrends(trends)
	if strings.TrimSpace(stack) != "" {
		model.WithHistoryLoader(costActualHistoryLoader(stack))
	}
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
