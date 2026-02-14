package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/tui"
)

// overviewParams holds the parameters for the overview command.
type overviewParams struct {
	pulumiJSON   string
	pulumiState  string
	stack        string
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

When run inside a Pulumi project directory without explicit file flags, overview
auto-detects the project and current stack, then runs 'pulumi stack export' and
'pulumi preview --json' to gather state and plan data automatically.

Optionally provide --pulumi-state and/or --pulumi-json to use pre-exported files
instead of running Pulumi CLI commands.`,
		Example: `  # Auto-detect from current Pulumi project (recommended)
  finfocus overview

  # Auto-detect with a specific stack
  finfocus overview --stack production

  # Use pre-exported files
  finfocus overview --pulumi-state state.json --pulumi-json plan.json

  # Show overview with custom date range
  finfocus overview --from 2025-01-01 --to 2025-01-31

  # Non-interactive plain text output
  finfocus overview --plain --yes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeOverview(cmd, params)
		},
	}

	cmd.Flags().StringVar(&params.pulumiJSON, "pulumi-json", "", "path to Pulumi preview JSON")
	cmd.Flags().StringVar(&params.pulumiState, "pulumi-state", "", "path to Pulumi state JSON")
	cmd.Flags().StringVar(&params.stack, "stack", "",
		"Pulumi stack name for auto-detection (ignored with --pulumi-state/--pulumi-json)")
	cmd.Flags().StringVar(&params.fromStr, "from", "", "start date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&params.toStr, "to", "", "end date (YYYY-MM-DD or RFC3339, defaults to now)")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "restrict to a specific adapter plugin")
	cmd.Flags().StringVar(&params.output, "output", "table", "output format (table, json, ndjson)")
	cmd.Flags().StringSliceVar(&params.filter, "filter", nil, "resource filters")
	cmd.Flags().BoolVar(&params.plain, "plain", false, "force non-interactive plain text output")
	cmd.Flags().BoolVarP(&params.yes, "yes", "y", false, "skip confirmation prompts")
	cmd.Flags().BoolVar(&params.noPagination, "no-pagination", false, "disable pagination (plain mode only)")

	return cmd
}

// executeOverview runs the overview command pipeline. It validates the date range,
// loads Pulumi state and plan data (from files or via auto-detection), detects pending
// changes, merges and optionally filters resources, opens plugin clients, constructs an
// engine with a router, and either launches an interactive TUI or enriches and renders
// plain output. It records audit events for failures and successes.
//
// cmd is the Cobra command being executed; params contains the overview command flags
// and options. The function returns an error if any step of the pipeline fails.
func executeOverview(cmd *cobra.Command, params overviewParams) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	audit := newAuditContext(ctx, "overview", map[string]string{
		"pulumi_state": params.pulumiState,
		"pulumi_json":  params.pulumiJSON,
		"output":       params.output,
	})

	// 1. Validate flags
	dateRange, err := resolveOverviewDateRange(params.fromStr, params.toStr, time.Now())
	if err != nil {
		return fmt.Errorf("invalid date range: %w", err)
	}

	// 2. Load Pulumi state and plan (from files or auto-detect)
	stateResources, planSteps, stackName, err := resolveOverviewData(ctx, params)
	if err != nil {
		wrappedErr := fmt.Errorf("resolve overview data: %w", err)
		audit.logFailure(ctx, wrappedErr)
		return wrappedErr
	}

	// 3. Detect pending changes
	hasChanges, changeCount := engine.DetectPendingChanges(ctx, planSteps)

	// 4. Merge resources
	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
	if err != nil {
		audit.logFailure(ctx, err)
		return fmt.Errorf("merging resources: %w", err)
	}

	// 5. Pre-flight prompt (unless --yes)
	printOverviewSummaryLine(cmd, params.yes, len(rows), hasChanges, changeCount)

	// 6. Validate filter keys and apply resource filters
	rows, err = validateAndApplyOverviewFilters(rows, params.filter)
	if err != nil {
		return err
	}

	// 7. Open plugins
	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	if err != nil {
		return err
	}
	defer cleanup()

	// 8. Create engine
	eng := engine.New(clients, nil).
		WithRouter(createRouterForEngine(ctx, clients))

	// 9. Determine if we should use interactive TUI or plain text
	isInteractive := shouldUseInteractiveTUI(cmd.OutOrStdout(), params.output, params.plain)

	if isInteractive {
		// Launch interactive TUI with progressive loading
		return runInteractiveOverview(ctx, cmd, rows, eng, dateRange, audit)
	}

	// 10. Enrich rows (blocking, for plain text mode)
	rows = engine.EnrichOverviewRows(ctx, rows, eng, dateRange, nil)

	// 11. Build stack context
	stackCtx := engine.StackContext{
		StackName:      stackName,
		TimeWindow:     dateRange,
		HasChanges:     hasChanges,
		TotalResources: len(rows),
		PendingChanges: changeCount,
		GeneratedAt:    time.Now(),
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

// resolveOverviewData loads Pulumi state and plan data, either from explicit file
// paths or by auto-detecting the Pulumi project and running CLI commands.
func resolveOverviewData(
	ctx context.Context, params overviewParams,
) ([]engine.StateResource, []engine.PlanStep, string, error) {
	if params.pulumiState != "" {
		return loadOverviewFromFiles(ctx, params)
	}
	return loadOverviewFromAutoDetect(ctx, params)
}

// loadOverviewFromFiles loads state/plan from explicit file paths.
func loadOverviewFromFiles(
	ctx context.Context, params overviewParams,
) ([]engine.StateResource, []engine.PlanStep, string, error) {
	log := logging.FromContext(ctx)

	log.Debug().Ctx(ctx).Str("state_path", params.pulumiState).Msg("loading Pulumi state")
	state, err := ingest.LoadStackExportWithContext(ctx, params.pulumiState)
	if err != nil {
		return nil, nil, "", fmt.Errorf("loading Pulumi state: %w", err)
	}
	stateResources := convertStateResources(state.GetCustomResourcesWithContext(ctx))
	stackName := extractStackName(params.pulumiState)

	var planSteps []engine.PlanStep
	if params.pulumiJSON != "" {
		plan, planErr := ingest.LoadPulumiPlanWithContext(ctx, params.pulumiJSON)
		if planErr != nil {
			return nil, nil, "", fmt.Errorf("loading Pulumi plan: %w", planErr)
		}
		planSteps = convertPlanSteps(plan.Steps)
	}

	return stateResources, planSteps, stackName, nil
}

// loadOverviewFromAutoDetect discovers the Pulumi project/stack and runs
// both `pulumi stack export` and `pulumi preview --json` to gather data.
func loadOverviewFromAutoDetect(
	ctx context.Context, params overviewParams,
) ([]engine.StateResource, []engine.PlanStep, string, error) {
	log := logging.FromContext(ctx)

	projectDir, resolvedStack, err := detectPulumiProject(ctx, params.stack)
	if err != nil {
		return nil, nil, "", fmt.Errorf("auto-detecting Pulumi project: %w", err)
	}

	// Run pulumi stack export
	log.Info().Ctx(ctx).Str("component", "pulumi").Str("operation", "stack_export").
		Msg("Running pulumi stack export...")
	exportData, exportErr := pulumidetect.StackExport(ctx, pulumidetect.ExportOptions{
		ProjectDir: projectDir,
		Stack:      resolvedStack,
	})
	if exportErr != nil {
		return nil, nil, "", fmt.Errorf("running pulumi stack export: %w", exportErr)
	}
	state, parseErr := ingest.ParseStackExportWithContext(ctx, exportData)
	if parseErr != nil {
		return nil, nil, "", fmt.Errorf("parsing pulumi stack export: %w", parseErr)
	}
	stateResources := convertStateResources(state.GetCustomResourcesWithContext(ctx))

	// Resolve plan: from file if --pulumi-json provided, otherwise auto-detect
	planSteps, planErr := resolveOverviewPlan(ctx, params.pulumiJSON, projectDir, resolvedStack)
	if planErr != nil {
		return nil, nil, "", planErr
	}

	return stateResources, planSteps, resolvedStack, nil
}

// resolveOverviewPlan loads plan steps from a file or runs pulumi preview.
func resolveOverviewPlan(
	ctx context.Context, pulumiJSON, projectDir, stack string,
) ([]engine.PlanStep, error) {
	log := logging.FromContext(ctx)

	if pulumiJSON != "" {
		plan, err := ingest.LoadPulumiPlanWithContext(ctx, pulumiJSON)
		if err != nil {
			return nil, fmt.Errorf("loading Pulumi plan: %w", err)
		}
		return convertPlanSteps(plan.Steps), nil
	}

	log.Info().Ctx(ctx).Str("component", "pulumi").Str("operation", "preview").
		Msg("Running pulumi preview --json (this may take a moment)...")
	previewData, err := pulumidetect.Preview(ctx, pulumidetect.PreviewOptions{
		ProjectDir: projectDir,
		Stack:      stack,
	})
	if err != nil {
		return nil, fmt.Errorf("running pulumi preview: %w", err)
	}
	plan, err := ingest.ParsePulumiPlanWithContext(ctx, previewData)
	if err != nil {
		return nil, fmt.Errorf("parsing pulumi preview: %w", err)
	}
	return convertPlanSteps(plan.Steps), nil
}

// printOverviewSummaryLine prints a one-line pre-flight summary unless --yes.
func printOverviewSummaryLine(
	cmd *cobra.Command,
	skipPrompt bool,
	resourceCount int,
	hasChanges bool,
	changeCount int,
) {
	if skipPrompt {
		return
	}
	cmd.Printf("Overview: %d resources", resourceCount)
	if hasChanges {
		cmd.Printf(", %d pending changes", changeCount)
	}
	cmd.Println()
}

// resolveOverviewDateRange parses the from/to strings into a DateRange.
// If from is empty, defaults to the 1st of the current month.
// If to is empty, defaults to now. The now parameter controls the current
// time used for defaults, enabling deterministic testing.
func resolveOverviewDateRange(fromStr, toStr string, now time.Time) (engine.DateRange, error) {
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
			URN:        r.URN,
			Type:       r.Type,
			ID:         r.ID,
			Custom:     r.Custom,
			Properties: ingest.MergeProperties(r.Outputs, r.Inputs),
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
	base := filepath.Base(statePath)
	base = strings.TrimSuffix(base, ".json")
	if base == "" || base == "." {
		return "unknown"
	}
	return base
}

// validateAndApplyOverviewFilters validates filter keys and applies filters.
// Returns the filtered rows, or an error if an unknown key is found.
func validateAndApplyOverviewFilters(
	rows []engine.OverviewRow,
	filters []string,
) ([]engine.OverviewRow, error) {
	if len(filters) == 0 {
		return rows, nil
	}
	allowedKeys := map[string]bool{
		"type": true, "status": true, "provider": true,
	}
	for _, f := range filters {
		parts := splitFilter(f)
		if len(parts) != filterKeyValueParts {
			return nil, fmt.Errorf(
				"invalid filter %q: expected key=value format (allowed keys: type, status, provider)",
				f,
			)
		}
		if parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf(
				"invalid filter %q: key and value must be non-empty",
				f,
			)
		}
		if !allowedKeys[parts[0]] {
			return nil, fmt.Errorf(
				"unknown filter key %q (allowed: type, status, provider)",
				parts[0],
			)
		}
	}
	return applyOverviewFilters(rows, filters), nil
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
			panic("unexpected filter key in matchesOverviewFilters: " + key)
		}
	}
	return true
}

// splitFilter splits a "key=value" filter string.
func splitFilter(filter string) []string {
	left, right, found := strings.Cut(filter, "=")
	if found {
		return []string{left, right}
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
// It accepts an io.Writer (typically cmd.OutOrStdout()) and type-asserts to
// check for a file descriptor, ensuring cmd.SetOut() redirections are respected.
func shouldUseInteractiveTUI(w io.Writer, outputFormat string, plainFlag bool) bool {
	// Only use interactive TUI for table output
	if outputFormat != "table" {
		return false
	}

	// --plain flag forces plain text
	if plainFlag {
		return false
	}

	// Check if the writer has a file descriptor and is a TTY.
	type fder interface{ Fd() uintptr }
	if f, ok := w.(fder); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
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

	// Copy skeletonRows for the TUI model so the model has its own
	// independent slice, preventing a data race between OverviewModel.applyFilter
	// (reads) and EnrichOverviewRows (writes) on the backing array.
	copiedRows := make([]engine.OverviewRow, len(skeletonRows))
	copy(copiedRows, skeletonRows)

	// Create TUI model
	model, _ := tui.NewOverviewModel(ctx, copiedRows, len(copiedRows))

	// Create Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Derived context so the enrichment goroutine stops when the TUI exits.
	enrichCtx, enrichCancel := context.WithCancel(ctx)

	// Start enrichment in background
	go func() {
		progressChan := make(chan engine.OverviewRowUpdate, len(skeletonRows))

		// Launch enrichment goroutines
		go func() {
			engine.EnrichOverviewRows(enrichCtx, skeletonRows, eng, dateRange, progressChan)
		}()

		// Bridge progress channel to Bubble Tea messages
		loadedCount := 0
		for update := range progressChan {
			// Stop bridging when the TUI has exited.
			select {
			case <-enrichCtx.Done():
				return
			default:
			}

			loadedCount++

			// Send resource loaded message
			p.Send(tui.OverviewResourceLoadedMsg{
				Index: update.Index,
				Row:   update.Row,
			})

			// Send progress update every 10 resources or at completion
			if loadedCount%10 == 0 || loadedCount == len(skeletonRows) {
				p.Send(tui.OverviewLoadingProgressMsg{
					Loaded: loadedCount,
					Total:  len(skeletonRows),
				})

				// Pre-compute percentage to avoid IIFE inside zerolog chain.
				percent := 0
				if len(skeletonRows) > 0 {
					percent = (loadedCount * 100) / len(skeletonRows) //nolint:mnd // Percentage calculation.
				}

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

		// Send completion message only if context is still active.
		select {
		case <-enrichCtx.Done():
			return
		default:
			p.Send(tui.OverviewAllResourcesLoadedMsg{})
		}

		log.Info().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "overview_tui").
			Int("total_rows", len(skeletonRows)).
			Msg("enrichment complete")
	}()

	// Run the TUI
	_, err := p.Run()
	enrichCancel() // Stop enrichment goroutine when TUI exits.
	if err != nil {
		audit.logFailure(ctx, err)
		return fmt.Errorf("running TUI: %w", err)
	}

	// Log success
	audit.logSuccess(ctx, len(skeletonRows), 0)

	return nil
}