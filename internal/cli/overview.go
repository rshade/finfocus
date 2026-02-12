package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/tui"
)

// overviewParams holds the parameters for the overview command.
type overviewParams struct {
	pulumiJSON   string
	pulumiState  string
	fromStr      string
	toStr        string
	adapter      string
	output       string
	filter       []string
	plain        bool
	yes          bool
	noPagination bool
}

// NewOverviewCmd creates the "overview" command that provides a unified
// cost dashboard combining state, plan, actual costs, projected costs,
// drift, and recommendations.
func NewOverviewCmd() *cobra.Command {
	var params overviewParams

	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Unified stack cost dashboard",
		Long: `Display a unified cost dashboard combining Pulumi state and plan data
with actual costs, projected costs, drift analysis, and recommendations.

Requires at least --pulumi-state to show current resources. Optionally provide
--pulumi-json to detect pending infrastructure changes and their cost impact.`,
		Example: `  # Show overview for current stack state
  finfocus overview --pulumi-state state.json

  # Show overview with pending changes from plan
  finfocus overview --pulumi-state state.json --pulumi-json plan.json

  # Show overview with custom date range
  finfocus overview --pulumi-state state.json --from 2025-01-01 --to 2025-01-31

  # Non-interactive plain text output
  finfocus overview --pulumi-state state.json --plain --yes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeOverview(cmd, params)
		},
	}

	cmd.Flags().StringVar(&params.pulumiJSON, "pulumi-json", "", "path to Pulumi preview JSON")
	cmd.Flags().StringVar(&params.pulumiState, "pulumi-state", "", "path to Pulumi state JSON")
	cmd.Flags().StringVar(&params.fromStr, "from", "", "start date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&params.toStr, "to", "", "end date (YYYY-MM-DD or RFC3339, defaults to now)")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "restrict to a specific adapter plugin")
	cmd.Flags().StringVar(&params.output, "output", "table", "output format (table, json, ndjson)")
	cmd.Flags().StringSliceVar(&params.filter, "filter", nil, "resource filters")
	cmd.Flags().BoolVar(&params.plain, "plain", false, "force non-interactive plain text output")
	cmd.Flags().BoolVarP(&params.yes, "yes", "y", false, "skip confirmation prompts")
	cmd.Flags().BoolVar(&params.noPagination, "no-pagination", false, "disable pagination (plain mode only)")

	// pulumi-state is required
	_ = cmd.MarkFlagRequired("pulumi-state")

	return cmd
}

// executeOverview is the main execution pipeline for the overview command.
//
//nolint:funlen // Pipeline orchestration is inherently sequential with many steps.
func executeOverview(cmd *cobra.Command, params overviewParams) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	log := logging.FromContext(ctx)
	audit := newAuditContext(ctx, "overview", map[string]string{
		"pulumi_state": params.pulumiState,
		"pulumi_json":  params.pulumiJSON,
		"output":       params.output,
	})

	// 1. Validate flags
	dateRange, err := resolveOverviewDateRange(params.fromStr, params.toStr)
	if err != nil {
		return fmt.Errorf("invalid date range: %w", err)
	}

	// 2. Load Pulumi state
	log.Debug().Ctx(ctx).Str("state_path", params.pulumiState).Msg("loading Pulumi state")
	state, err := ingest.LoadStackExportWithContext(ctx, params.pulumiState)
	if err != nil {
		audit.logFailure(ctx, err)
		return fmt.Errorf("loading Pulumi state: %w", err)
	}

	stateResources := convertStateResources(state.GetCustomResourcesWithContext(ctx))

	// 3. Load Pulumi plan (optional)
	var planSteps []engine.PlanStep
	if params.pulumiJSON != "" {
		log.Debug().Ctx(ctx).Str("plan_path", params.pulumiJSON).Msg("loading Pulumi plan")
		plan, planErr := ingest.LoadPulumiPlanWithContext(ctx, params.pulumiJSON)
		if planErr != nil {
			audit.logFailure(ctx, planErr)
			return fmt.Errorf("loading Pulumi plan: %w", planErr)
		}
		planSteps = convertPlanSteps(plan.Steps)
	}

	// 4. Detect pending changes
	hasChanges, changeCount, _ := engine.DetectPendingChanges(ctx, planSteps)

	// 5. Merge resources
	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
	if err != nil {
		audit.logFailure(ctx, err)
		return fmt.Errorf("merging resources: %w", err)
	}

	// 6. Pre-flight prompt (unless --yes)
	if !params.yes {
		cmd.Printf("Overview: %d resources", len(rows))
		if hasChanges {
			cmd.Printf(", %d pending changes", changeCount)
		}
		cmd.Println()
	}

	// 7. Apply resource filters
	if len(params.filter) > 0 {
		rows = applyOverviewFilters(rows, params.filter)
	}

	// 8. Open plugins
	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	if err != nil {
		return err
	}
	defer cleanup()

	// 9. Create engine
	eng := engine.New(clients, nil)

	// 10. Determine if we should use interactive TUI or plain text
	isInteractive := shouldUseInteractiveTUI(params.output, params.plain)

	if isInteractive {
		// Launch interactive TUI with progressive loading
		return runInteractiveOverview(ctx, cmd, rows, eng, dateRange, audit)
	}

	// 10. Enrich rows (blocking, for plain text mode)
	progressChan := make(chan engine.OverviewRowUpdate, len(rows))
	rows = engine.EnrichOverviewRows(ctx, rows, eng, dateRange, progressChan)

	// 11. Build stack context
	stackCtx := engine.StackContext{
		StackName:      extractStackName(params.pulumiState),
		TimeWindow:     dateRange,
		HasChanges:     hasChanges,
		TotalResources: len(rows),
		PendingChanges: changeCount,
	}

	// 12. Render output (plain text)
	renderErr := renderOverviewOutput(cmd, params.output, rows, stackCtx)
	if renderErr != nil {
		audit.logFailure(ctx, renderErr)
		return renderErr
	}

	audit.logSuccess(ctx, len(rows), 0)
	return nil
}

// resolveOverviewDateRange parses the from/to strings into a DateRange.
// If from is empty, defaults to the 1st of the current month.
// If to is empty, defaults to now.
func resolveOverviewDateRange(fromStr, toStr string) (engine.DateRange, error) {
	now := time.Now()

	var from time.Time
	if fromStr == "" {
		// Default to 1st of current month
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	} else {
		parsed, err := ParseTime(fromStr)
		if err != nil {
			return engine.DateRange{}, fmt.Errorf("parsing 'from' date: %w", err)
		}
		from = parsed
	}

	var to time.Time
	if toStr == "" {
		to = now
	} else {
		parsed, err := ParseTime(toStr)
		if err != nil {
			return engine.DateRange{}, fmt.Errorf("parsing 'to' date: %w", err)
		}
		to = parsed
	}

	if !to.After(from) {
		return engine.DateRange{}, errors.New("'to' date must be after 'from' date")
	}

	return engine.DateRange{Start: from, End: to}, nil
}

// convertStateResources converts ingest.StackExportResource to engine.StateResource.
func convertStateResources(resources []ingest.StackExportResource) []engine.StateResource {
	result := make([]engine.StateResource, len(resources))
	for i, r := range resources {
		result[i] = engine.StateResource{
			URN:    r.URN,
			Type:   r.Type,
			ID:     r.ID,
			Custom: r.Custom,
		}
	}
	return result
}

// convertPlanSteps converts ingest.PulumiStep to engine.PlanStep.
func convertPlanSteps(steps []ingest.PulumiStep) []engine.PlanStep {
	result := make([]engine.PlanStep, len(steps))
	for i, s := range steps {
		result[i] = engine.PlanStep{
			URN:  s.URN,
			Op:   s.Op,
			Type: s.Type,
		}
	}
	return result
}

// extractStackName extracts a stack name from the state file path.
func extractStackName(statePath string) string {
	// Use the filename without extension as a stack name fallback
	base := statePath
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '/' || base[i] == '\\' || base[i] == os.PathSeparator {
			base = base[i+1:]
			break
		}
	}
	// Remove .json extension
	if len(base) > 5 && base[len(base)-5:] == ".json" {
		base = base[:len(base)-5]
	}
	if base == "" {
		return "unknown"
	}
	return base
}

// applyOverviewFilters filters overview rows based on filter expressions.
func applyOverviewFilters(rows []engine.OverviewRow, filters []string) []engine.OverviewRow {
	if len(filters) == 0 {
		return rows
	}

	filtered := make([]engine.OverviewRow, 0, len(rows))
	for _, row := range rows {
		if matchesOverviewFilters(row, filters) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

// matchesOverviewFilters checks if a row matches all filter expressions.
func matchesOverviewFilters(row engine.OverviewRow, filters []string) bool {
	for _, filter := range filters {
		parts := splitFilter(filter)
		if len(parts) != filterKeyValueParts {
			continue
		}
		key, value := parts[0], parts[1]
		switch key {
		case "type":
			if row.Type != value {
				return false
			}
		case "status":
			if row.Status.String() != value {
				return false
			}
		case "provider":
			if engine.ExtractProviderFromResourceType(row.Type) != value {
				return false
			}
		default:
			// Unknown filter key - skip
		}
	}
	return true
}

// splitFilter splits a "key=value" filter string.
func splitFilter(filter string) []string {
	for i, ch := range filter {
		if ch == '=' {
			return []string{filter[:i], filter[i+1:]}
		}
	}
	return []string{filter}
}

// renderOverviewOutput dispatches to the correct renderer based on the output format.
func renderOverviewOutput(
	cmd *cobra.Command,
	outputFormat string,
	rows []engine.OverviewRow,
	stackCtx engine.StackContext,
) error {
	switch outputFormat {
	case "table":
		if renderErr := engine.RenderOverviewAsTable(cmd.OutOrStdout(), rows, stackCtx); renderErr != nil {
			return fmt.Errorf("rendering overview: %w", renderErr)
		}
	case "json":
		if renderErr := engine.RenderOverviewAsJSON(cmd.OutOrStdout(), rows, stackCtx); renderErr != nil {
			return fmt.Errorf("rendering overview: %w", renderErr)
		}
	case "ndjson":
		if renderErr := engine.RenderOverviewAsNDJSON(cmd.OutOrStdout(), rows); renderErr != nil {
			return fmt.Errorf("rendering overview: %w", renderErr)
		}
	default:
		return fmt.Errorf(
			"unsupported output format: %s (supported: table, json, ndjson)",
			outputFormat,
		)
	}
	return nil
}

// shouldUseInteractiveTUI determines if the interactive TUI should be used.
func shouldUseInteractiveTUI(outputFormat string, plainFlag bool) bool {
	// Only use interactive TUI for table output
	if outputFormat != "table" {
		return false
	}

	// --plain flag forces plain text
	if plainFlag {
		return false
	}

	// Check if stdout is a TTY
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// runInteractiveOverview launches the interactive TUI with progressive loading.
func runInteractiveOverview(
	ctx context.Context,
	_ *cobra.Command,
	skeletonRows []engine.OverviewRow,
	eng *engine.Engine,
	dateRange engine.DateRange,
	audit *auditContext,
) error {
	log := logging.FromContext(ctx)

	// Create TUI model
	model, _ := tui.NewOverviewModel(ctx, skeletonRows, len(skeletonRows))

	// Create Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Start enrichment in background
	go func() {
		progressChan := make(chan engine.OverviewRowUpdate, len(skeletonRows))

		// Launch enrichment goroutines
		go func() {
			engine.EnrichOverviewRows(ctx, skeletonRows, eng, dateRange, progressChan)
		}()

		// Bridge progress channel to Bubble Tea messages
		loadedCount := 0
		for update := range progressChan {
			loadedCount++

			// Send resource loaded message
			p.Send(tui.OverviewResourceLoadedMsg{
				Index: update.Index,
				Row:   update.Row,
			})

			// Send progress update every 10 resources or at completion
			if loadedCount%10 == 0 || loadedCount == len(skeletonRows) {
				percent := 0
				if len(skeletonRows) > 0 {
					percent = (loadedCount * 100) / len(skeletonRows) //nolint:mnd // Percentage calculation.
				}
				p.Send(tui.OverviewLoadingProgressMsg{
					Loaded: loadedCount,
					Total:  len(skeletonRows),
				})

				log.Debug().
					Ctx(ctx).
					Str("component", "cli").
					Str("operation", "overview_tui").
					Int("loaded", loadedCount).
					Int("total", len(skeletonRows)).
					Int("percent", percent).
					Msg("enrichment progress")
			}
		}

		// Send completion message
		p.Send(tui.OverviewAllResourcesLoadedMsg{})

		log.Info().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "overview_tui").
			Int("total_rows", len(skeletonRows)).
			Msg("enrichment complete")
	}()

	// Run the TUI
	_, err := p.Run()
	if err != nil {
		audit.logFailure(ctx, err)
		return fmt.Errorf("running TUI: %w", err)
	}

	// Log success
	audit.logSuccess(ctx, len(skeletonRows), 0)

	return nil
}
