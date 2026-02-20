package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/config"
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
		Use:     "overview",
		Aliases: []string{"ov"},
		Short:   "Unified stack cost dashboard",
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
//
//nolint:funlen // Pipeline orchestrator with per-phase timing instrumentation.
func executeOverview(cmd *cobra.Command, params overviewParams) error {
	totalStart := time.Now()
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
	pt := logging.StartPhase(ctx, "cli", "overview", "date_validation")
	dateRange, err := resolveOverviewDateRange(params.fromStr, params.toStr, time.Now())
	pt.Done(ctx)
	if err != nil {
		return fmt.Errorf("invalid date range: %w", err)
	}

	// 2. Determine if we should use interactive TUI or plain text (early, before data load)
	isInteractive := shouldUseInteractiveTUI(cmd.OutOrStdout(), params.output, params.plain)

	if isInteractive {
		// Launch TUI immediately, load data in background
		return runInteractiveOverviewWithInit(ctx, cmd, params, dateRange, audit, totalStart)
	}

	// --- Plain text / non-interactive path ---

	// 3-5. Load data: state-first for auto-detect, full load for explicit files.
	pt = logging.StartPhase(ctx, "cli", "overview", "data_loading")
	stateResources, planSteps, stackName, isStateOnly, err := loadPlainOverviewData(ctx, cmd, params)
	pt.Done(ctx)
	if err != nil {
		wrappedErr := fmt.Errorf("resolve overview data: %w", err)
		audit.logFailure(ctx, wrappedErr)
		return wrappedErr
	}

	// 4. Detect pending changes (skipped in state-only mode).
	pt = logging.StartPhase(ctx, "cli", "overview", "change_detection")
	var hasChanges bool
	var changeCount int
	if !isStateOnly {
		hasChanges, changeCount = engine.DetectPendingChanges(ctx, planSteps)
	}
	pt.Done(ctx)

	// 5. Merge resources or build rows from state only.
	pt = logging.StartPhase(ctx, "cli", "overview", "resource_merge")
	var rows []engine.OverviewRow
	if isStateOnly {
		rows = engine.NewRowsFromState(ctx, stateResources)
	} else {
		rows, err = engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
		if err != nil {
			pt.Done(ctx)
			audit.logFailure(ctx, err)
			return fmt.Errorf("merging resources: %w", err)
		}
	}
	pt.Done(ctx)

	// 6. Pre-flight prompt (unless --yes or state-only)
	if !isStateOnly {
		printOverviewSummaryLine(cmd, params.yes, len(rows), hasChanges, changeCount)
	}

	// 7. Validate filter keys and apply resource filters
	pt = logging.StartPhase(ctx, "cli", "overview", "filter_apply")
	rows, err = validateAndApplyOverviewFilters(rows, params.filter)
	pt.Done(ctx)
	if err != nil {
		return err
	}

	// 8. Open plugins
	pt = logging.StartPhase(ctx, "cli", "overview", "plugin_open")
	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	pt.Done(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	// 9. Create engine
	pt = logging.StartPhase(ctx, "cli", "overview", "engine_create")
	cfg := config.New()
	eng := engine.New(clients, nil).
		WithRouter(createRouterForEngine(ctx, cfg, clients))
	pt.Done(ctx)

	// 10. Enrich rows (blocking, for plain text mode)
	pt = logging.StartPhase(ctx, "cli", "overview", "enrichment")
	rows = engine.EnrichOverviewRows(ctx, rows, eng, dateRange, nil)
	pt.Done(ctx)

	// 11. Build stack context
	stackCtx := engine.StackContext{
		StackName:      stackName,
		TimeWindow:     dateRange,
		HasChanges:     hasChanges,
		TotalResources: len(rows),
		PendingChanges: changeCount,
		GeneratedAt:    time.Now(),
		IsStateOnly:    isStateOnly,
	}

	// 12. Render output (plain text)
	renderErr := renderOverviewOutput(cmd, params.output, rows, stackCtx)
	if renderErr != nil {
		audit.logFailure(ctx, renderErr)
		return renderErr
	}

	log.Info().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "overview").
		Int64("total_elapsed_ms", time.Since(totalStart).Milliseconds()).
		Int("resource_count", len(rows)).
		Msg("overview total complete")

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

// loadPlainOverviewData loads state and plan data for the non-interactive (plain text) mode.
// For explicit file paths it delegates to resolveOverviewData directly (no change detection).
// For auto-detect mode it uses state-first loading: it runs change detection and calls
// promptForPreview to decide whether to run pulumi preview. isStateOnly is true when
// preview was skipped; the caller should then build rows via engine.NewRowsFromState.
func loadPlainOverviewData(
	ctx context.Context,
	cmd *cobra.Command,
	params overviewParams,
) ([]engine.StateResource, []engine.PlanStep, string, bool, error) {
	if params.pulumiState != "" || params.pulumiJSON != "" {
		// Explicit files provided: load both directly, bypass change detection.
		sr, ps, sn, err := resolveOverviewData(ctx, params)
		return sr, ps, sn, false, err
	}

	// Auto-detect mode: state-first loading with optional change detection and prompt.
	stateResources, manifestTime, projectDir, stackName, stateErr := loadStateForOverview(ctx, params)
	if stateErr != nil {
		return nil, nil, "", false, stateErr
	}

	signal, detectErr := pulumidetect.DetectChanges(ctx, manifestTime, projectDir)
	if detectErr != nil {
		log := logging.FromContext(ctx)
		log.Warn().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "overview_plain_init").
			Err(detectErr).
			Msg("change detection failed; assuming changes likely")
		signal = pulumidetect.ChangeSignal{HasLikelyChanges: true}
	}

	// Determine whether to run preview.
	// --yes: skip prompt, always run preview.
	// TTY without --yes: show prompt so user can choose.
	// Non-TTY without --yes: state-only (no prompt, no preview).
	skipPrompt := params.yes
	type fder interface{ Fd() uintptr }
	isTTY := false
	if f, ok := cmd.OutOrStdout().(fder); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	var shouldRunPreview bool
	if params.yes || isTTY {
		var promptErr error
		shouldRunPreview, promptErr = promptForPreview(cmd.OutOrStdout(), cmd.InOrStdin(), signal, skipPrompt)
		if promptErr != nil {
			return nil, nil, stackName, false, promptErr
		}
	}
	// Non-TTY without --yes: shouldRunPreview remains false → state-only.

	if shouldRunPreview {
		planSteps, planErr := loadPlanForOverview(ctx, params, projectDir, stackName)
		if planErr != nil {
			return nil, nil, stackName, false, planErr
		}
		return stateResources, planSteps, stackName, false, nil
	}
	return stateResources, nil, stackName, true, nil
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
	projectDir, resolvedStack, err := detectPulumiProject(ctx, params.stack)
	if err != nil {
		return nil, nil, "", fmt.Errorf("auto-detecting Pulumi project: %w", err)
	}

	// Run pulumi stack export
	pt := logging.StartPhase(ctx, "pulumi", "overview", "stack_export")
	defer pt.Done(ctx)

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
	ptPreview := logging.StartPhase(ctx, "pulumi", "overview", "preview")
	planSteps, planErr := resolveOverviewPlan(ctx, params.pulumiJSON, projectDir, resolvedStack)
	ptPreview.Done(ctx)
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

// promptForPreview displays a one-line prompt asking the user whether to run
// pulumi preview. It returns true if preview should run.
//
// signal is the ChangeSignal from pulumi.DetectChanges.
// skipPrompt is true when --yes was passed (always run preview) or when
// the output is not a TTY (non-interactive; skip preview for scripting).
// w is the writer for the prompt line; r is the reader for user input.
func promptForPreview(w io.Writer, r io.Reader, signal pulumidetect.ChangeSignal, skipPrompt bool) (bool, error) {
	// --yes always runs preview without prompting.
	if skipPrompt {
		return true, nil
	}

	// Stack has never been deployed — run preview automatically with a status line.
	if signal.IsFirstDeploy {
		msg := "Stack has not been deployed. Running pulumi preview to detect initial resources."
		if _, err := fmt.Fprintln(w, msg); err != nil {
			return false, fmt.Errorf("writing to output: %w", err)
		}
		return true, nil
	}

	// No likely changes — skip preview automatically.
	if !signal.HasLikelyChanges {
		return false, nil
	}

	// Likely changes detected — prompt the user (default Y).
	fileList := ""
	if len(signal.ModifiedFiles) > 0 {
		fileList = fmt.Sprintf(" (modified: %s)", strings.Join(signal.ModifiedFiles, ", "))
	}
	if _, err := fmt.Fprintf(w, "Changes detected since last deployment%s.\n", fileList); err != nil {
		return false, fmt.Errorf("writing to output: %w", err)
	}
	const previewPrompt = "Load pending changes? Runs pulumi preview, may take a few minutes. [Y/n]: "
	if _, err := fmt.Fprint(w, previewPrompt); err != nil {
		return false, fmt.Errorf("writing to output: %w", err)
	}

	var line string
	if _, err := fmt.Fscanln(r, &line); err != nil {
		// Empty input (just Enter) is treated as Y.
		line = ""
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == answerYes, nil
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

// runInteractiveOverviewWithInit launches the TUI immediately (before data
// loading) and loads data in a background goroutine, sending phase progress
// messages to keep the user informed. This eliminates the blank-terminal wait.
func runInteractiveOverviewWithInit(
	ctx context.Context,
	_ *cobra.Command,
	params overviewParams,
	dateRange engine.DateRange,
	audit *auditContext,
	totalStart time.Time,
) error {
	// Channel for passphrase: background goroutine blocks, TUI sends when user submits.
	passphraseChan := make(chan string, 1)

	// Create TUI model with nil rows → ViewStateInitializing.
	// previewCmd is injected after data loading completes; pass nil here and
	// let overviewInitAndEnrich update the model via OverviewDataReadyMsg extension.
	model, _ := tui.NewOverviewModel(ctx, nil, 0, passphraseChan, nil)

	// Create Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Derived context so the background goroutine stops when the TUI exits.
	enrichCtx, enrichCancel := context.WithCancel(ctx)
	defer enrichCancel()

	// Channel to pass the plugin cleanup function from background goroutine.
	cleanupChan := make(chan func(), 1)

	// Atomic counter so the background goroutine can report the row count
	// to the main goroutine for accurate audit logging.
	var rowCount atomic.Int64

	// Start data loading and enrichment in background
	go overviewInitAndEnrich(enrichCtx, p, params, dateRange, audit, cleanupChan, &rowCount, passphraseChan)

	// Run the TUI (blocks until user quits or error)
	finalModel, err := p.Run()
	enrichCancel()

	// Drain cleanup from the background goroutine with a short timeout.
	// The goroutine may still be mid-send after enrichCancel(), so we block
	// briefly to ensure the plugin cleanup callback is not lost.
	select {
	case cleanup := <-cleanupChan:
		if cleanup != nil {
			cleanup()
		}
	case <-time.After(2 * time.Second): //nolint:mnd // Grace period for goroutine to send cleanup.
		// Plugins were never opened (early exit or error before openPlugins)
	}

	if err != nil {
		audit.logFailure(ctx, err)
		return fmt.Errorf("running TUI: %w", err)
	}

	// Check if the TUI exited due to an initialization error. tea.Quit is a
	// normal termination so p.Run() returns nil; the actual error is stored
	// in the model's error state.
	if m, ok := finalModel.(tui.OverviewModel); ok && m.Err() != nil {
		initErr := m.Err()
		audit.logFailure(ctx, initErr)
		return fmt.Errorf("overview initialization: %w", initErr)
	}

	log := logging.FromContext(ctx)
	log.Info().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "overview").
		Int64("total_elapsed_ms", time.Since(totalStart).Milliseconds()).
		Msg("overview TUI complete")

	audit.logSuccess(ctx, int(rowCount.Load()), 0)
	return nil
}

// Phase index constants correspond to tui.PhaseNames (0-based).
// They are declared here to avoid mnd warnings for the magic numbers 2-5.
const (
	phaseLoadStackState  = 0
	phaseDetectChanges   = 1
	phaseMergeResources  = 2
	phaseStartPlugins    = 3
	phasePrepareEngine   = 4
	phaseEnrichResources = 5
)

// overviewInitAndEnrich performs data loading and enrichment in a background
// goroutine, sending phase progress and data messages to the Bubble Tea program.
//
//nolint:funlen // Two-phase loading path with change detection and conditional preview.
func overviewInitAndEnrich(
	enrichCtx context.Context,
	p *tea.Program,
	params overviewParams,
	dateRange engine.DateRange,
	audit *auditContext,
	cleanupChan chan<- func(),
	rowCount *atomic.Int64,
	passphraseChan chan string,
) {
	log := logging.FromContext(enrichCtx)

	// Pre-check: detect if stack uses passphrase encryption before loading.
	if err := checkAndPromptPassphrase(enrichCtx, p, params, passphraseChan); err != nil {
		p.Send(tui.OverviewInitErrorMsg{Err: err})
		return
	}

	// Phase 1: Load Pulumi state only (fast — no preview yet).
	p.Send(tui.OverviewPhaseMsg{Index: phaseLoadStackState, Phase: "Loading stack state..."})
	stateResources, manifestTime, projectDir, stackName, stateErr := loadStateForOverview(enrichCtx, params)
	if stateErr != nil {
		p.Send(tui.OverviewInitErrorMsg{Err: fmt.Errorf("resolve overview data: %w", stateErr)})
		return
	}

	// Phase 2: Lightweight change detection from the already-parsed manifest.
	p.Send(tui.OverviewPhaseMsg{Index: phaseDetectChanges, Phase: "Detecting changes..."})
	signal, detectErr := pulumidetect.DetectChanges(enrichCtx, manifestTime, projectDir)
	if detectErr != nil {
		log.Warn().
			Ctx(enrichCtx).
			Str("component", "cli").
			Str("operation", "overview_tui_init").
			Err(detectErr).
			Msg("change detection failed; assuming changes likely")
		signal = pulumidetect.ChangeSignal{HasLikelyChanges: true}
	}

	// Decide whether to run pulumi preview now.
	// In non-interactive (TUI) mode we skip the interactive prompt and rely on --yes
	// or the conservative heuristic (HasLikelyChanges=true → run preview; false → state-only).
	runPreviewNow := params.yes || signal.IsFirstDeploy || signal.HasLikelyChanges
	isStateOnly := !runPreviewNow

	// Phase 3: Merge resources (from state only when state-first, from state+plan when preview runs).
	p.Send(tui.OverviewPhaseMsg{Index: phaseMergeResources, Phase: "Merging resources..."})

	var rows []engine.OverviewRow
	var hasChanges bool
	var changeCount int

	if isStateOnly {
		// State-first path: build rows from state with no plan data.
		rows = engine.NewRowsFromState(enrichCtx, stateResources)
	} else {
		// Preview path: load plan steps and merge with state.
		planSteps, planErr := loadPlanForOverview(enrichCtx, params, projectDir, stackName)
		if planErr != nil {
			p.Send(tui.OverviewInitErrorMsg{Err: planErr})
			return
		}
		hasChanges, changeCount = engine.DetectPendingChanges(enrichCtx, planSteps)
		var mergeErr error
		rows, mergeErr = engine.MergeResourcesForOverview(enrichCtx, stateResources, planSteps)
		if mergeErr != nil {
			p.Send(tui.OverviewInitErrorMsg{Err: fmt.Errorf("merging resources: %w", mergeErr)})
			return
		}
	}

	log.Debug().
		Ctx(enrichCtx).
		Str("component", "cli").
		Str("operation", "overview_tui_init").
		Str("stack", stackName).
		Bool("has_changes", hasChanges).
		Int("change_count", changeCount).
		Bool("is_state_only", isStateOnly).
		Msg("overview data phase complete")

	// Apply filters.
	rows, filterErr := validateAndApplyOverviewFilters(rows, params.filter)
	if filterErr != nil {
		p.Send(tui.OverviewInitErrorMsg{Err: filterErr})
		return
	}
	rowCount.Store(int64(len(rows)))

	// Phase 4: Open plugins.
	p.Send(tui.OverviewPhaseMsg{Index: phaseStartPlugins, Phase: "Starting cost plugins..."})
	clients, cleanup, pluginErr := openPlugins(enrichCtx, params.adapter, audit)
	if pluginErr != nil {
		p.Send(tui.OverviewInitErrorMsg{Err: pluginErr})
		return
	}
	cleanupChan <- cleanup

	// Phase 5: Create engine.
	p.Send(tui.OverviewPhaseMsg{Index: phasePrepareEngine, Phase: "Preparing cost engine..."})
	cfg := config.New()
	eng := engine.New(clients, nil).
		WithRouter(createRouterForEngine(enrichCtx, cfg, clients))

	// Build the on-demand preview command (only used in state-only mode).
	var previewCmd tea.Cmd
	if isStateOnly {
		capturedParams := params
		capturedProjectDir := projectDir
		capturedStackName := stackName
		previewCmd = func() tea.Msg {
			return runBackgroundPreview(enrichCtx, capturedParams, capturedProjectDir, capturedStackName)
		}
	}

	// Signal data ready → transitions TUI from Initializing to Loading.
	copiedRows := make([]engine.OverviewRow, len(rows))
	copy(copiedRows, rows)
	p.Send(tui.OverviewDataReadyMsg{Rows: copiedRows, TotalCount: len(rows), StackName: stackName})

	// If state-only: update the model's isStateOnly and previewCmd via a SetStateOnlyMsg.
	// We reuse the existing message pattern to keep it simple.
	if isStateOnly {
		p.Send(tui.OverviewSetStateOnlyMsg{PreviewCmd: previewCmd})
	}

	// Phase 6: Enrichment.
	bridgeEnrichmentToTUI(enrichCtx, p, rows, eng, dateRange)
}

// loadStateForOverview loads Pulumi state only (without preview/plan).
// Returns stateResources, manifestTime, projectDir, stackName, and any error.
func loadStateForOverview(
	ctx context.Context, params overviewParams,
) ([]engine.StateResource, string, string, string, error) {
	if params.pulumiState != "" {
		// Use explicit file path — no manifest time or project dir available.
		log := logging.FromContext(ctx)
		log.Debug().Ctx(ctx).Str("state_path", params.pulumiState).Msg("loading Pulumi state from file")
		state, err := ingest.LoadStackExportWithContext(ctx, params.pulumiState)
		if err != nil {
			return nil, "", "", "", fmt.Errorf("loading Pulumi state: %w", err)
		}
		resources := convertStateResources(state.GetCustomResourcesWithContext(ctx))
		stackName := extractStackName(params.pulumiState)
		// No manifest time available from file path — treat as unknown.
		return resources, "", filepath.Dir(params.pulumiState), stackName, nil
	}

	// Auto-detect project and run pulumi stack export.
	projectDir, resolvedStack, err := detectPulumiProject(ctx, params.stack)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("auto-detecting Pulumi project: %w", err)
	}

	pt := logging.StartPhase(ctx, "pulumi", "overview", "stack_export")
	defer pt.Done(ctx)

	exportData, exportErr := pulumidetect.StackExport(ctx, pulumidetect.ExportOptions{
		ProjectDir: projectDir,
		Stack:      resolvedStack,
	})
	if exportErr != nil {
		return nil, "", "", "", fmt.Errorf("running pulumi stack export: %w", exportErr)
	}
	state, parseErr := ingest.ParseStackExportWithContext(ctx, exportData)
	if parseErr != nil {
		return nil, "", "", "", fmt.Errorf("parsing pulumi stack export: %w", parseErr)
	}
	resources := convertStateResources(state.GetCustomResourcesWithContext(ctx))
	return resources, state.Deployment.Manifest.Time, projectDir, resolvedStack, nil
}

// loadPlanForOverview loads plan steps from a file or runs pulumi preview.
// This is the preview-path helper used by overviewInitAndEnrich.
func loadPlanForOverview(
	ctx context.Context, params overviewParams, projectDir, stack string,
) ([]engine.PlanStep, error) {
	steps, err := resolveOverviewPlan(ctx, params.pulumiJSON, projectDir, stack)
	if err != nil {
		return nil, fmt.Errorf("loading plan: %w", err)
	}
	return steps, nil
}

// runBackgroundPreview runs pulumi preview in the background and returns an
// OverviewChangesReadyMsg with the resulting status map. It is used as a
// tea.Cmd payload for the on-demand 'p' key handler.
// If preview fails, it returns an empty OverviewChangesReadyMsg so the TUI
// remains usable in state-only mode.
func runBackgroundPreview(
	ctx context.Context,
	params overviewParams,
	projectDir, stack string,
) tui.OverviewChangesReadyMsg {
	l := logging.FromContext(ctx)
	previewStart := time.Now()

	planSteps, err := resolveOverviewPlan(ctx, params.pulumiJSON, projectDir, stack)
	if err != nil {
		l.Warn().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "phased_loading").
			Err(err).
			Msg("background preview failed; remaining in state-only mode")
		return tui.OverviewChangesReadyMsg{}
	}

	l.Info().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "phased_loading").
		Dur("dur", time.Since(previewStart)).
		Msg("background preview completed")

	hasChanges, changeCount := engine.DetectPendingChanges(ctx, planSteps)
	statusByURN := make(map[string]engine.ResourceStatus, len(planSteps))
	for _, step := range planSteps {
		statusByURN[step.URN] = engine.MapOperationToStatus(step.Op)
	}
	return tui.OverviewChangesReadyMsg{
		StatusByURN: statusByURN,
		HasChanges:  hasChanges,
		ChangeCount: changeCount,
	}
}

// checkAndPromptPassphrase detects if the Pulumi stack uses passphrase encryption.
// If PULUMI_CONFIG_PASSPHRASE is not set and the stack YAML contains an encryptionsalt,
// it sends OverviewPassphraseRequiredMsg to the TUI and blocks until the user provides
// the passphrase (or the context is cancelled).
//
// If passphrase auto-detection is not possible (e.g. using --pulumi-state files instead
// of auto-detect, or if the stack YAML cannot be read), the check is silently skipped
// and pulumi stack export will produce its own error if encryption is required.
func checkAndPromptPassphrase(
	ctx context.Context,
	p *tea.Program,
	params overviewParams,
	passphraseChan chan string,
) error {
	// Skip if passphrase already set
	if os.Getenv("PULUMI_CONFIG_PASSPHRASE") != "" {
		return nil
	}

	// Only auto-detect when not using explicit file flags
	if params.pulumiState != "" {
		return nil
	}

	// Locate the project directory and stack name
	projectDir, stackName, detectErr := detectPulumiProject(ctx, params.stack)
	if detectErr != nil {
		// If we can't detect the project, skip (let stack export fail naturally).
		return nil //nolint:nilerr // Intentional fail-open: passphrase check is best-effort.
	}

	// Read the stack YAML and check for encryptionsalt
	stackYAML := filepath.Join(projectDir, "Pulumi."+stackName+".yaml")
	data, readErr := os.ReadFile(stackYAML)
	if readErr != nil {
		// Stack YAML not found or unreadable — skip check, fail open.
		return nil //nolint:nilerr // Intentional fail-open: passphrase check is best-effort.
	}

	if !strings.Contains(string(data), "encryptionsalt:") {
		return nil
	}

	// Stack is encrypted and no passphrase is set — prompt the user via TUI
	p.Send(tui.OverviewPassphraseRequiredMsg{})

	// Block until the user provides the passphrase or the context is cancelled
	select {
	case pw := <-passphraseChan:
		if setenvErr := os.Setenv("PULUMI_CONFIG_PASSPHRASE", pw); setenvErr != nil {
			return fmt.Errorf("setting PULUMI_CONFIG_PASSPHRASE: %w", setenvErr)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// bridgeEnrichmentToTUI runs EnrichOverviewRows and bridges progress updates
// to the Bubble Tea program via Send().
func bridgeEnrichmentToTUI(
	enrichCtx context.Context,
	p *tea.Program,
	rows []engine.OverviewRow,
	eng *engine.Engine,
	dateRange engine.DateRange,
) {
	// Phase 6: Enriching resources
	p.Send(tui.OverviewPhaseMsg{Index: phaseEnrichResources, Phase: "Enriching resources..."})

	log := logging.FromContext(enrichCtx)

	progressChan := make(chan engine.OverviewRowUpdate, len(rows))
	go func() {
		engine.EnrichOverviewRows(enrichCtx, rows, eng, dateRange, progressChan)
	}()

	loadedCount := 0
	for update := range progressChan {
		select {
		case <-enrichCtx.Done():
			return
		default:
		}

		loadedCount++
		p.Send(tui.OverviewResourceLoadedMsg{
			Index: update.Index,
			Row:   update.Row,
		})

		if loadedCount%10 == 0 || loadedCount == len(rows) {
			p.Send(tui.OverviewLoadingProgressMsg{
				Loaded: loadedCount,
				Total:  len(rows),
			})

			percent := 0
			if len(rows) > 0 {
				percent = (loadedCount * 100) / len(rows) //nolint:mnd // Percentage calculation.
			}
			log.Debug().
				Ctx(enrichCtx).
				Str("component", "cli").
				Str("operation", "overview_tui_init").
				Int("loaded", loadedCount).
				Int("total", len(rows)).
				Int("percent", percent).
				Msg("enrichment progress")
		}
	}

	select {
	case <-enrichCtx.Done():
		return
	default:
		p.Send(tui.OverviewAllResourcesLoadedMsg{})
	}

	log.Info().
		Ctx(enrichCtx).
		Str("component", "cli").
		Str("operation", "overview_tui_init").
		Int("total_rows", len(rows)).
		Msg("enrichment complete")
}
