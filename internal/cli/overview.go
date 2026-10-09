package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/resourcetype"
	"github.com/rshade/finfocus/internal/tui"
)

// fdProvider is implemented by *[os.File] and other writers that expose a file descriptor.
// Used to detect whether an output stream is a TTY.
type fdProvider interface{ Fd() uintptr }

// overviewParams holds the parameters for the overview command.
type overviewParams struct {
	captureState    func([]ingest.StackExportResource)
	capturePlan     func(*ingest.PulumiPlan)
	pulumiJSON      string
	pulumiState     string
	stack           string
	fromStr         string
	toStr           string
	adapter         string
	output          string
	filter          []string
	plain           bool
	forceColor      bool
	noColor         bool
	yes             bool
	noPagination    bool
	exitOnThreshold bool
	exitCode        int
	budgetScope     string
	stateOnly       bool
	cfg             *config.Config
}

// NewOverviewCmd constructs the "overview" Cobra command that displays a unified stack cost
// dashboard combining Pulumi state and preview data with cost information, drift analysis,
// and recommendations. It supports auto-detection of the Pulumi project/stack or explicit
// --pulumi-state / --pulumi-json inputs, and is configured with flags for date range, adapter,
// output format, resource filtering, interactive/plain mode, pagination, confirmation behavior,
// and budget controls (exit-on-threshold, exit-code, notify, budget-scope).
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
  finfocus overview --plain --yes

  # Fast cost-only overview (skip pulumi preview)
  finfocus overview --state-only`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeOverview(cmd, params)
		},
	}

	cmd.Flags().StringVar(&params.pulumiJSON, "pulumi-json", "", "path to Pulumi preview JSON")
	cmd.Flags().StringVar(&params.pulumiState, "pulumi-state", "", "path to Pulumi state JSON")
	cmd.Flags().StringVarP(&params.stack, "stack", "s", "",
		"Pulumi stack name for auto-detection (ignored with --pulumi-state/--pulumi-json)")
	cmd.Flags().StringVar(&params.fromStr, "from", "", "start date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&params.toStr, "to", "", "end date (YYYY-MM-DD or RFC3339, defaults to now)")
	cmd.Flags().StringVarP(&params.adapter, "adapter", "a", "", "restrict to a specific adapter plugin")
	cmd.Flags().StringVar(&params.output, "output", "table", "output format (table, json, ndjson)")
	cmd.Flags().StringSliceVarP(&params.filter, "filter", "f", nil, "resource filters")
	cmd.Flags().BoolVar(&params.plain, "plain", false, "force non-interactive plain text output")
	cmd.Flags().BoolVar(&params.forceColor, "force-color", false, "force styled output in non-TTY environments")
	cmd.Flags().BoolVar(&params.noColor, "no-color", false, "disable all ANSI styling")
	cmd.Flags().BoolVarP(&params.yes, "yes", "y", false, "skip confirmation prompts")
	cmd.Flags().BoolVar(&params.noPagination, "no-pagination", false, "disable pagination (plain mode only)")
	cmd.Flags().BoolVar(&params.exitOnThreshold, "exit-on-threshold", false,
		"Exit with non-zero code when budget thresholds are exceeded (non-TTY only)")
	cmd.Flags().IntVar(&params.exitCode, "exit-code", 1,
		"Exit code to use when budget thresholds are exceeded (0-255)")
	cmd.Flags().Bool(notifyFlag, false, notifyFlagUsage)
	cmd.Flags().StringVar(&params.budgetScope, "budget-scope", "",
		"Filter budget scopes to display: global, provider, provider=aws, tag, type (comma-separated)")
	cmd.Flags().BoolVar(&params.stateOnly, "state-only", false,
		"skip pulumi preview (faster, but won't detect pending changes)")
	cmd.MarkFlagsMutuallyExclusive("state-only", "pulumi-json")
	addAccessibilityFlags(cmd)

	// StackContext.GeneratedAt (embedded via OverviewMetadata) varies between
	// otherwise-identical runs; tell __schema not to expect byte-identical output.
	ax.WithNonDeterministicFields[engine.OverviewJSONOutput](cmd)

	return cmd
}

// checkOverviewInputs rejects an unsupported output format and, when the run
// opted in to budget notifications, an invalid configuration.
func checkOverviewInputs(cmd *cobra.Command, output string) error {
	switch output {
	case outputFormatTable, outputFormatJSON, outputFormatNDJSON:
	default:
		return fmt.Errorf("unsupported output format: %s (supported: table, json, ndjson)", output)
	}
	return validateNotifyConfig(cmd)
}

// executeOverview orchestrates the overview command workflow: it validates the date range,
// loads Pulumi state and optionally a preview plan, detects pending changes, merges and
// filters resources, opens plugin clients, constructs an engine, and either launches an
// interactive TUI or enriches and renders plain output with optional budget evaluation.
func executeOverview(cmd *cobra.Command, params overviewParams) error {
	totalStart := time.Now()
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	log := logging.FromContext(ctx)
	params.output = resolveOutputFormat(cmd, "output", params.output)
	params = applyOverviewAccessibility(cmd, params)
	if err := checkOverviewInputs(cmd, params.output); err != nil {
		return err
	}
	audit := newAuditContext(ctx, "overview", map[string]string{
		"pulumi_state":     params.pulumiState,
		auditKeyPulumiJSON: params.pulumiJSON,
		auditKeyOutput:     params.output,
	})

	// Load config once and share across all history operations.
	if params.cfg == nil {
		params.cfg = config.New()
	}

	// 1. Validate flags
	if params.exitCode < config.MinExitCode || params.exitCode > config.MaxExitCode {
		return fmt.Errorf("--exit-code must be between %d and %d, got %d",
			config.MinExitCode, config.MaxExitCode, params.exitCode)
	}

	pt := logging.StartPhase(ctx, "cli", "overview", "date_validation")
	dateRange, err := resolveOverviewDateRange(params.fromStr, params.toStr, time.Now())
	pt.Done(ctx)
	if err != nil {
		return fmt.Errorf("invalid date range: %w", err)
	}

	// 2. Determine if we should use interactive TUI or plain text (early, before data load).
	// Styled mode shares the plain path until a styled renderer exists.
	if overviewInteractive(cmd.OutOrStdout(), params) {
		// Launch TUI immediately, load data in background
		return runInteractiveOverviewWithInit(ctx, cmd, params, dateRange, audit, totalStart)
	}

	// --- Plain text / non-interactive path ---

	// 3-10. Load, merge, filter, enrich, and expand through the shared
	// event-driven pipeline; only rendering and budget-exit evaluation remain
	// plain-path concerns.
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd:       cmd,
		params:    params,
		dateRange: dateRange,
		audit:     audit,
		loader:    plainOverviewLoader,
		// Budget data is only needed for JSON output; the plain path fetches
		// it synchronously after expansion and reports plugin budgets only.
		wantBudget:    params.output == outputFormatJSON,
		dismissalRows: true,
	})
	pipe.ConfirmEnrichment = func(
		_ context.Context, data OverviewPipelineData, clientCount int,
	) (bool, error) {
		// Pre-flight confirmation before pricing. Declining skips enrichment.
		stop, confirmErr := confirmPlainOverview(
			cmd, params, len(data.Rows), data.HasChanges, data.ChangeCount, clientCount, data.IsStateOnly,
		)
		if confirmErr != nil || stop {
			return false, confirmErr
		}
		return true, nil
	}
	var (
		finalErr    error
		finalResult OverviewPipelineResult
		ready       bool
	)
	pipe.OnReady = func(result OverviewPipelineResult) {
		ready = true
		finalResult = result
		// 11-14. Build context, compute result, render output, evaluate budgets.
		finalErr = finalizeOverviewOutput(ctx, cmd, params, result, dateRange, audit)
	}
	if runErr := runOverviewPipeline(ctx, pipe); runErr != nil {
		return runErr
	}
	if finalErr != nil {
		return finalErr
	}
	if !ready {
		// Pre-flight confirmation declined: not an error, nothing rendered.
		return nil
	}

	log.Info().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "overview").
		Int64("total_elapsed_ms", time.Since(totalStart).Milliseconds()).
		Int("resource_count", len(finalResult.Rows)).
		Msg("overview total complete")

	audit.logSuccess(ctx, len(finalResult.Rows), 0)
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
	if params.pulumiJSON != "" {
		if _, err := pulumidetect.FindProject("."); err != nil {
			return loadOverviewPlanOnly(ctx, params)
		}
	}
	return loadOverviewFromAutoDetect(ctx, params)
}

// finalizeOverviewOutput builds the stack context from the pipeline result,
// renders the overview in the requested output format, and evaluates budget
// thresholds. The budget result (fetched by the pipeline for JSON output) may
// be nil.
func finalizeOverviewOutput(
	ctx context.Context,
	cmd *cobra.Command,
	params overviewParams,
	result OverviewPipelineResult,
	dateRange engine.DateRange,
	audit *auditContext,
) error {
	stackCtx := engine.StackContext{
		StackName:      result.StackName,
		TimeWindow:     dateRange,
		HasChanges:     result.HasChanges,
		TotalResources: result.ResourceCount,
		PendingChanges: result.ChangeCount,
		GeneratedAt:    time.Now(),
		IsStateOnly:    result.IsStateOnly,
		ExpansionNotes: result.ExpansionNotes,
	}

	// Compute all display values once; renderers read the pre-computed result.
	overviewResult := engine.ComputeOverviewResult(result.Rows, time.Now().Day())

	// Render output.
	renderErr := renderOverviewOutput(cmd, params.output, overviewResult, stackCtx, result.Budget)
	if renderErr != nil {
		audit.logFailure(ctx, renderErr)
		return renderErr
	}

	// Budget evaluation (non-TTY path only).
	overrides := applyOverviewBudgetFlags(cmd, params)
	costResults, totalCost := overviewRowsToBudgetInputs(result.Rows)
	if budgetErr := evaluateBudgetStatusWithoutRender(cmd, costResults, totalCost, overrides); budgetErr != nil {
		audit.logFailure(ctx, budgetErr)
		return toAxExitError(ctx, budgetErr)
	}

	return nil
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
		// Propagate stateOnly so the caller uses NewRowsFromState when --state-only --pulumi-state is used.
		sr, ps, sn, err := resolveOverviewData(ctx, params)
		return sr, ps, sn, params.stateOnly, err
	}

	if params.stateOnly {
		stateResources, _, _, stackName, stateErr := loadStateForOverview(ctx, params, nil)
		if stateErr != nil {
			return nil, nil, "", false, stateErr
		}
		return stateResources, nil, stackName, true, nil
	}

	// Auto-detect mode: state-first loading with optional change detection and prompt.
	// In plain (non-TUI) mode the passphrase is not prompted; PULUMI_CONFIG_PASSPHRASE
	// is expected to already be set in the caller's environment.
	stateResources, manifestTime, projectDir, stackName, stateErr := loadStateForOverview(ctx, params, nil)
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
	isTTY := false
	if f, ok := cmd.OutOrStdout().(fdProvider); ok {
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
		planSteps, planErr := loadPlanForOverview(ctx, params, projectDir, stackName, nil)
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

// exportStateFromProject auto-detects the Pulumi project, runs `pulumi stack export`,
// parses the result, and returns the state resources plus metadata.
// passphrase is injected into the subprocess environment only (never [os.Setenv]).
// It is shared by loadOverviewFromAutoDetect and loadStateForOverview.
func exportStateFromProject(
	ctx context.Context, stack string, passphrase *string, capture ...func([]ingest.StackExportResource),
) ([]engine.StateResource, string, string, string, error) {
	projectDir, resolvedStack, err := detectPulumiProject(ctx, stack)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("auto-detecting Pulumi project: %w", err)
	}

	resources, manifestTime, err := exportStateForStack(ctx, projectDir, resolvedStack, passphrase, capture...)
	if err != nil {
		return nil, "", "", "", err
	}
	return resources, manifestTime, projectDir, resolvedStack, nil
}

// exportStateForStack runs `pulumi stack export` for an already-resolved project and stack
// and returns the custom state resources plus the manifest time.
func exportStateForStack(
	ctx context.Context, projectDir, stack string, passphrase *string,
	capture ...func([]ingest.StackExportResource),
) ([]engine.StateResource, string, error) {
	pt := logging.StartPhase(ctx, "pulumi", "overview", "stack_export")
	defer pt.Done(ctx)

	exportData, exportErr := pulumidetect.StackExport(ctx, pulumidetect.ExportOptions{
		ProjectDir: projectDir,
		Stack:      stack,
		Passphrase: passphrase,
	})
	if exportErr != nil {
		return nil, "", fmt.Errorf("running pulumi stack export: %w", exportErr)
	}
	state, parseErr := ingest.ParseStackExportWithContext(ctx, exportData)
	if parseErr != nil {
		return nil, "", fmt.Errorf("parsing pulumi stack export: %w", parseErr)
	}
	custom := state.GetCustomResourcesWithContext(ctx)
	for _, observer := range capture {
		if observer != nil {
			observer(custom)
		}
	}
	resources := convertStateResources(custom)
	return resources, state.Deployment.Manifest.Time, nil
}

// loadOverviewFromAutoDetect discovers the Pulumi project/stack and runs
// `pulumi stack export` and `pulumi preview --json` concurrently to gather data.
// The first failure cancels the other command.
// In the plain (non-TUI) mode no passphrase is prompted; PULUMI_CONFIG_PASSPHRASE
// is expected to be set in the caller's environment already.
func loadOverviewFromAutoDetect(
	ctx context.Context, params overviewParams,
) ([]engine.StateResource, []engine.PlanStep, string, error) {
	stateResources, planSteps, _, resolvedStack, err := loadOverviewProjectData(ctx, params, nil)
	return stateResources, planSteps, resolvedStack, err
}

// loadOverviewProjectData preserves the project identity for later previews
// while sharing auto-detection and passphrase handling with the plain CLI.
// Stack export and preview run concurrently; the first failure cancels the other.
func loadOverviewProjectData(
	ctx context.Context, params overviewParams, passphrase *string,
) ([]engine.StateResource, []engine.PlanStep, string, string, error) {
	if params.pulumiJSON != "" {
		if _, err := pulumidetect.FindProject("."); err != nil {
			state, steps, stack, loadErr := loadOverviewPlanOnly(ctx, params)
			return state, steps, "", stack, loadErr
		}
	}
	projectDir, resolvedStack, err := detectPulumiProject(ctx, params.stack)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("auto-detecting Pulumi project: %w", err)
	}

	var (
		stateResources []engine.StateResource
		planSteps      []engine.PlanStep
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var exportErr error
		stateResources, _, exportErr = exportStateForStack(gctx, projectDir, resolvedStack, passphrase)
		return exportErr
	})
	g.Go(func() error {
		pt := logging.StartPhase(gctx, "pulumi", "overview", "preview")
		defer pt.Done(gctx)
		var planErr error
		planSteps, planErr = resolveOverviewPlan(gctx, params.pulumiJSON, projectDir, resolvedStack, passphrase)
		return planErr
	})
	if waitErr := g.Wait(); waitErr != nil {
		return nil, nil, "", "", waitErr
	}

	return stateResources, planSteps, projectDir, resolvedStack, nil
}

// resolveOverviewPlan loads plan steps from a file or runs pulumi preview.
// passphrase is injected into the subprocess environment only (never [os.Setenv]).
func resolveOverviewPlan(
	ctx context.Context, pulumiJSON, projectDir, stack string, passphrase *string,
	capture ...func(*ingest.PulumiPlan),
) ([]engine.PlanStep, error) {
	log := logging.FromContext(ctx)

	if pulumiJSON != "" {
		plan, err := ingest.LoadPulumiPlanWithContext(ctx, pulumiJSON)
		if err != nil {
			return nil, fmt.Errorf("loading Pulumi plan: %w", err)
		}
		if len(capture) > 0 && capture[0] != nil {
			capture[0](plan)
		}
		return convertPlanSteps(plan.Steps), nil
	}

	log.Info().Ctx(ctx).Str("component", "pulumi").Str("operation", "preview").
		Msg("Running pulumi preview --json (this may take a moment)...")
	previewData, err := pulumidetect.Preview(ctx, pulumidetect.PreviewOptions{
		ProjectDir: projectDir,
		Stack:      stack,
		Passphrase: passphrase,
	})
	if err != nil {
		return nil, fmt.Errorf("running pulumi preview: %w", err)
	}
	plan, err := ingest.ParsePulumiPlanWithContext(ctx, previewData)
	if err != nil {
		return nil, fmt.Errorf("parsing pulumi preview: %w", err)
	}
	if len(capture) > 0 && capture[0] != nil {
		capture[0](plan)
	}
	return convertPlanSteps(plan.Steps), nil
}

// printOverviewSummaryLine prints the pre-flight summary and, on a terminal
// without --yes, asks whether to continue into pricing. proceed is false when
// the user declines. skipPrompt is true for --yes. isTerminal reports whether
// stdin is a terminal; non-terminals print the summary and continue.
// Empty input and EOF continue. "n" and "no" (any case) cancel.
func printOverviewSummaryLine(
	w io.Writer,
	r io.Reader,
	skipPrompt bool,
	isTerminal bool,
	resourceCount int,
	hasChanges bool,
	changeCount int,
	pluginCount int,
) (bool, error) {
	if err := writeOverviewSummary(w, resourceCount, hasChanges, changeCount, pluginCount); err != nil {
		return false, err
	}
	if skipPrompt || !isTerminal {
		return true, nil
	}
	if _, err := io.WriteString(w, "Continue? [Y/n] "); err != nil {
		return false, fmt.Errorf("writing overview prompt: %w", err)
	}
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("reading confirmation: %w", err)
		}
		return true, nil
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer == "n" || answer == "no" {
		if _, err := fmt.Fprintln(w, "Cancelled."); err != nil {
			return false, fmt.Errorf("writing cancellation: %w", err)
		}
		return false, nil
	}
	return true, nil
}

// writeOverviewSummary writes the one-line resource, change, and plugin counts.
func writeOverviewSummary(w io.Writer, resourceCount int, hasChanges bool, changeCount, pluginCount int) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Overview: %d resources", resourceCount)
	if hasChanges {
		fmt.Fprintf(&b, ", %d pending changes", changeCount)
	}
	fmt.Fprintf(&b, ", %d plugins\n", pluginCount)
	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("writing overview summary: %w", err)
	}
	return nil
}

// confirmPlainOverview asks whether to price the plain table view.
// stop is true when the user cancels; that is not an error.
// State-only and non-table output continue without a question.
func confirmPlainOverview(
	cmd *cobra.Command,
	params overviewParams,
	resourceCount int,
	hasChanges bool,
	changeCount int,
	pluginCount int,
	isStateOnly bool,
) (bool, error) {
	if isStateOnly || params.output != outputFormatTable {
		return false, nil
	}
	proceed, err := printOverviewSummaryLine(
		cmd.OutOrStdout(),
		cmd.InOrStdin(),
		params.yes,
		stdinIsTerminal(cmd.InOrStdin()),
		resourceCount,
		hasChanges,
		changeCount,
		pluginCount,
	)
	if err != nil {
		return false, fmt.Errorf("pre-flight prompt: %w", err)
	}
	if !proceed {
		return true, nil
	}
	return false, nil
}

// stdinIsTerminal reports whether r is an [os.File] attached to a terminal.
func stdinIsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
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
		var createdAt *time.Time
		if r.Created != nil {
			t := *r.Created // copy value to avoid aliasing the ingest layer's pointer
			createdAt = &t
		}
		result[i] = engine.StateResource{
			URN:        r.URN,
			Type:       r.Type,
			ID:         r.ID,
			Custom:     r.Custom,
			Properties: ingest.MergeProperties(r.Outputs, r.Inputs),
			CreatedAt:  createdAt,
		}
	}
	return result
}

// convertPlanSteps converts ingest.PulumiStep to engine.PlanStep.
// For update/replace operations, it also extracts property diffs
// by comparing OldState.Inputs to NewState.Inputs.
func convertPlanSteps(steps []ingest.PulumiStep) []engine.PlanStep {
	result := make([]engine.PlanStep, len(steps))
	for i, s := range steps {
		result[i] = engine.PlanStep{
			URN:  s.URN,
			Op:   s.Op,
			Type: s.Type,
		}
		switch s.Op {
		case "create", "update", "replace", "create-replacement":
			result[i].ProjectedProperties = projectedPropertiesFromStep(s)
		}
		switch s.Op {
		case "update", "replace", "create-replacement":
			result[i].PropertyDiffs = diffInputs(s.OldState, s.NewState)
		}
		if s.OldState != nil {
			result[i].OldCloudID = s.OldState.ID
		}
		if s.NewState != nil {
			result[i].NewCloudID = s.NewState.ID
		}
	}
	return result
}

// diffInputs compares OldState.Inputs and NewState.Inputs from a Pulumi step
// and returns a sorted slice of PropertyDiff for keys whose values differ.
func diffInputs(oldState, newState *ingest.PulumiState) []engine.PropertyDiff {
	if oldState == nil || newState == nil {
		return nil
	}
	oldInputs := oldState.Inputs
	newInputs := newState.Inputs
	if len(oldInputs) == 0 && len(newInputs) == 0 {
		return nil
	}

	// Collect all keys from both maps.
	keys := make(map[string]struct{}, len(oldInputs)+len(newInputs))
	for k := range oldInputs {
		keys[k] = struct{}{}
	}
	for k := range newInputs {
		keys[k] = struct{}{}
	}

	var diffs []engine.PropertyDiff
	for k := range keys {
		// Skip internal Pulumi metadata keys (e.g., __defaults).
		if strings.HasPrefix(k, "__") {
			continue
		}
		rawOld, oldOK := oldInputs[k]
		rawNew, newOK := newInputs[k]
		// Presence-aware comparison: a key existing in one map but not
		// the other is always a diff, even if the value is nil.
		if oldOK == newOK && reflect.DeepEqual(rawOld, rawNew) {
			continue
		}
		oldDisplay := formatDiffValue(rawOld)
		newDisplay := formatDiffValue(rawNew)
		diffs = append(diffs, engine.PropertyDiff{
			Key:      k,
			OldValue: oldDisplay,
			NewValue: newDisplay,
		})
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Key < diffs[j].Key
	})
	return diffs
}

// projectedPropertiesFromStep builds pricing properties for projected-cost calls
// by deep-merging old state with new inputs:
//
//	deepMerge(oldStateMerged, newState.inputs)
//
// This preserves unchanged state-derived fields while applying intended preview
// changes, avoiding mispricing from "new inputs only" payloads.
func projectedPropertiesFromStep(step ingest.PulumiStep) map[string]any {
	var oldMerged map[string]any
	if step.OldState != nil {
		oldMerged = ingest.MergeProperties(step.OldState.Outputs, step.OldState.Inputs)
	}

	var newInputs map[string]any
	switch {
	case step.NewState != nil && len(step.NewState.Inputs) > 0:
		newInputs = step.NewState.Inputs
	case len(step.Inputs) > 0:
		newInputs = step.Inputs
	}

	merged := deepMergeProperties(oldMerged, newInputs)
	if len(merged) == 0 {
		return nil
	}
	return merged
}

// deepMergeProperties recursively merges two maps and returns a fresh map.
// Nested map[string]interface{} values are merged; all other override values
// replace the base value.
func deepMergeProperties(base, override map[string]any) map[string]any {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := deepCopyProperties(base)
	if out == nil {
		out = make(map[string]any, len(override))
	}
	for k, v := range override {
		existing, exists := out[k]
		out[k] = deepMergeValue(exists, existing, v)
	}
	return out
}

func deepMergeValue(exists bool, baseValue, overrideValue any) any {
	if !exists {
		return deepCopyAny(overrideValue)
	}
	baseMap, baseIsMap := baseValue.(map[string]any)
	overrideMap, overrideIsMap := overrideValue.(map[string]any)
	if baseIsMap && overrideIsMap {
		return deepMergeProperties(baseMap, overrideMap)
	}
	return deepCopyAny(overrideValue)
}

func deepCopyProperties(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyAny(v)
	}
	return out
}

func deepCopyAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopyProperties(t)
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = deepCopyAny(t[i])
		}
		return out
	default:
		return t
	}
}

// formatDiffValue converts a property value to a human-readable string.
// Simple types use [fmt.Sprintf]; complex types (maps, slices) use compact JSON.
func formatDiffValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool, float64, int, int64:
		return fmt.Sprintf("%v", val)
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
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
	// Fscanln stops at first whitespace; accepts first word of input (e.g. "yes" from "yes please\n").
	if _, err := fmt.Fscanln(r, &line); err != nil {
		// EOF / unexpected-EOF means the user pressed Enter with no input — treat as Y.
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return false, fmt.Errorf("reading user input: %w", err)
		}
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

// Overview filter keys accepted in --filter expressions.
const (
	filterKeyType     = "type"
	filterKeyStatus   = "status"
	filterKeyProvider = "provider"
)

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
		filterKeyType: true, filterKeyStatus: true, filterKeyProvider: true,
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
		case filterKeyType:
			if row.Type != value {
				return false
			}
		case filterKeyStatus:
			if row.Status.String() != value {
				return false
			}
		case filterKeyProvider:
			if engine.ExtractProviderFromResourceType(row.Type) != resourcetype.NormalizeProvider(value) {
				return false
			}
		default:
			panic("unexpected filter key in matchesOverviewFilters: " + key)
		}
	}
	return true
}

// splitFilter splits a "key=value" filter string into its key and value.
// If '=' is present it returns a two-element slice [key, value]; otherwise
// it returns a single-element slice containing the original string.
func splitFilter(filter string) []string {
	left, right, found := strings.Cut(filter, "=")
	if found {
		return []string{left, right}
	}
	return []string{filter}
}

// renderOverviewOutput renders the pre-computed overview result using the specified
// output format ("table", "json", or "ndjson") to the command's stdout. When format
// is "json", the optional budgetResult is included in the top-level budgets array if non-nil.
func renderOverviewOutput(
	cmd *cobra.Command,
	outputFormat string,
	result engine.OverviewResult,
	stackCtx engine.StackContext,
	budgetResult *engine.BudgetResult,
) error {
	switch outputFormat {
	case outputFormatTable:
		if renderErr := engine.RenderOverviewAsTable(cmd.OutOrStdout(), result, stackCtx); renderErr != nil {
			return fmt.Errorf("rendering overview: %w", renderErr)
		}
	case outputFormatJSON:
		renderErr := engine.RenderOverviewAsJSON(
			cmd.Context(), cmd.OutOrStdout(), result, stackCtx, budgetResult,
		)
		if renderErr != nil {
			return fmt.Errorf("rendering overview: %w", renderErr)
		}
	case "ndjson":
		if renderErr := engine.RenderOverviewAsNDJSON(cmd.OutOrStdout(), result); renderErr != nil {
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

// overviewInteractive reports whether overview should launch the TUI.
// Table output is required. --plain, --no-color, NO_COLOR, a non-terminal
// writer, and TERM=dumb stay on the text path. --force-color selects styled
// text rather than the TUI.
func overviewInteractive(w io.Writer, params overviewParams) bool {
	if params.output != outputFormatTable {
		return false
	}
	access := tui.Accessibility{Plain: params.plain, NoColor: params.noColor, ForceColor: params.forceColor}
	return tui.DetectResolvedOutputMode(w, access) == tui.OutputModeInteractive
}

// runInteractiveOverviewWithInit launches the TUI immediately (before data
// loading) and loads data in a background goroutine, sending phase progress
// messages to keep the user informed. This eliminates the blank-terminal wait.
func runInteractiveOverviewWithInit(
	ctx context.Context,
	cmd *cobra.Command,
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
	p := tea.NewProgram(model)

	// Derived context so the background goroutine stops when the TUI exits.
	enrichCtx, enrichCancel := context.WithCancel(ctx)
	defer enrichCancel()

	// Channel to pass the plugin cleanup function from background goroutine.
	cleanupChan := make(chan func(), 1)

	// Atomic counter incremented per enriched row so the audit log reflects
	// actual progress even on early TUI exit.
	var rowCount atomic.Int64

	// enrichDone is closed when the background goroutine exits. The cleanup
	// path waits on this channel so plugin connections are not torn down while
	// enrichment gRPC calls are still in flight (see issue #716).
	enrichDone := make(chan struct{})

	// Start data loading and enrichment in background
	go func() {
		defer close(enrichDone)
		overviewInitAndEnrich(enrichCtx, cmd, p, params, dateRange, audit, cleanupChan, &rowCount, passphraseChan)
	}()

	// Run the TUI (blocks until user quits or error)
	finalModel, err := p.Run()
	enrichCancel()

	// Wait for the enrichment goroutine to finish before closing plugin
	// connections. This prevents a race where cleanup() tears down gRPC
	// connections while EnrichOverviewRows still has calls in flight.
	<-enrichDone

	// Drain cleanup from the background goroutine.
	select {
	case cleanup := <-cleanupChan:
		if cleanup != nil {
			cleanup()
		}
	default:
		// Plugins were never opened (early exit or error before openPlugins).
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

	// progressReportInterval controls how often enrichment progress is
	// reported to the TUI (every N rows).
	progressReportInterval = 10
)

// resolveIsStateOnly determines whether to operate in state-only mode (no pulumi preview).
//
// When params.yes is true the user has explicitly requested a preview; a detection error
// must NOT override that intent.  When params.yes is false a detection error is a safe
// signal to fall back to state-only so the user can trigger a preview manually with 'p'.
func resolveIsStateOnly(params overviewParams, signal pulumidetect.ChangeSignal, detectErr error) bool {
	if params.stateOnly {
		return true
	}
	explicitStateOnly := params.pulumiState != "" && params.pulumiJSON == "" && !params.yes
	runPreviewNow := !explicitStateOnly && (params.yes || signal.IsFirstDeploy || signal.HasLikelyChanges)
	isStateOnly := !runPreviewNow

	// If change detection failed and the user did NOT explicitly request a preview
	// (--yes), fall back to state-only so the user can decide whether to trigger
	// preview manually with 'p'.
	if detectErr != nil && !params.yes {
		isStateOnly = true
	}

	return isStateOnly
}

// overviewInitAndEnrich runs the shared OverviewPipeline with the TUI's
// loader and bridges every pipeline event to Bubble Tea messages. Errors are
// reported exclusively via Bubble Tea messages (OverviewInitErrorMsg); the
// combined cleanup function sent on cleanupChan must be invoked to release
// plugins and cache resources.
func overviewInitAndEnrich(
	enrichCtx context.Context,
	cmd *cobra.Command,
	p *tea.Program,
	params overviewParams,
	dateRange engine.DateRange,
	audit *auditContext,
	cleanupChan chan<- func(),
	rowCount *atomic.Int64,
	passphraseChan chan string,
) {
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd:            cmd,
		params:         params,
		dateRange:      dateRange,
		audit:          audit,
		loader:         tuiOverviewLoader,
		passphrases:    passphraseChan,
		wantBudget:     true,
		budgetLive:     true,
		budgetFallback: true,
	})
	pipe.OnPhase = func(phase int, name string) {
		p.Send(tui.OverviewPhaseMsg{Index: phase - 1, Phase: name})
	}
	pipe.OnDataReady = func(rows []engine.OverviewRow, totalCount int, stackName string) {
		p.Send(tui.OverviewDataReadyMsg{Rows: rows, TotalCount: totalCount, StackName: stackName})
	}
	pipe.OnStateOnly = func(detectErrMsg string) {
		// On-demand preview command for the 'p' key binding.
		previewCmd := func() tea.Msg { return pipe.RunPreview(enrichCtx) }
		p.Send(tui.OverviewSetStateOnlyMsg{PreviewCmd: previewCmd, DetectErrMsg: detectErrMsg})
	}
	pipe.OnRow = func(index int, row engine.OverviewRow) {
		rowCount.Add(1)
		p.Send(tui.OverviewResourceLoadedMsg{Index: index, Row: row})
	}
	pipe.OnProgress = func(loaded, total int) {
		p.Send(tui.OverviewLoadingProgressMsg{Loaded: loaded, Total: total})
	}
	pipe.OnAllRowsLoaded = func() { p.Send(tui.OverviewAllResourcesLoadedMsg{}) }
	pipe.OnExpansion = func(rows []engine.OverviewRow, notes []string) {
		p.Send(tui.OverviewExpansionReadyMsg{Rows: rows, Notes: notes})
	}
	pipe.OnBudget = func(result *engine.BudgetResult, err error) {
		p.Send(tui.BudgetDataReadyMsg{Result: result, Error: err})
	}
	pipe.OnError = func(_ int, err error) { p.Send(tui.OverviewInitErrorMsg{Err: err}) }
	pipe.OnPassphraseRequired = func() { p.Send(tui.OverviewPassphraseRequiredMsg{}) }

	_ = pipe.Run(enrichCtx)

	// Hand the combined cleanup to the caller once the pipeline is done;
	// nothing is sent when the run stopped before plugins opened.
	if cleanup := pipe.Cleanup(); cleanup != nil {
		cleanupChan <- cleanup
	}
}

// buildOverviewRows merges state resources with optional plan steps to produce
// the initial OverviewRow slice. Returns rows, planSteps, hasChanges, changeCount, and any error.
// planSteps is non-nil only when a preview was run (isStateOnly=false).
func buildOverviewRows(
	ctx context.Context,
	isStateOnly bool,
	stateResources []engine.StateResource,
	params overviewParams,
	projectDir, stackName string,
	pw *string,
) ([]engine.OverviewRow, []engine.PlanStep, bool, int, error) {
	if isStateOnly {
		return engine.NewRowsFromState(ctx, stateResources), nil, false, 0, nil
	}
	planSteps, planErr := loadPlanForOverview(ctx, params, projectDir, stackName, pw)
	if planErr != nil {
		return nil, nil, false, 0, planErr
	}
	hasChanges, changeCount := engine.DetectPendingChanges(ctx, planSteps)
	rows, mergeErr := engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
	if mergeErr != nil {
		return nil, nil, false, 0, fmt.Errorf("merging resources: %w", mergeErr)
	}
	return rows, planSteps, hasChanges, changeCount, nil
}

// buildPreviewCmd creates a Bubble Tea command that runs pulumi preview in the
// background, used in state-only mode for on-demand preview loading.
func buildPreviewCmd(
	ctx context.Context,
	params overviewParams,
	projectDir, stackName string,
	pw *string,
) tea.Cmd {
	return func() tea.Msg {
		return runBackgroundPreview(ctx, params, projectDir, stackName, pw)
	}
}

// launchBudgetFetch starts a goroutine to fetch budgets from plugins and
// returns a channel that will receive the result.
func launchBudgetFetch(ctx context.Context, eng *engine.Engine) <-chan budgetFetchResult {
	ch := make(chan budgetFetchResult, 1)
	go func() {
		defer close(ch)
		result, err := eng.GetBudgets(ctx, nil)
		if err != nil {
			log := logging.FromContext(ctx)
			log.Warn().
				Str("component", "overview").
				Str("operation", "launchBudgetFetch").
				Ctx(ctx).Err(err).
				Msg("budget fetch failed (non-fatal)")
		}
		ch <- budgetFetchResult{result: result, err: err}
	}()
	return ch
}

// budgetFetchResult holds the result of a concurrent budget plugin fetch.
type budgetFetchResult struct {
	result *engine.BudgetResult
	err    error
}

// sendBudgetResultToTUI drains the budget channel and sends the appropriate
// BudgetDataReadyMsg to the TUI via the pipeline's budget delivery stage. If
// plugins returned budgets, those are used; otherwise config-based budgets
// are evaluated as a fallback.
func sendBudgetResultToTUI(
	ctx context.Context,
	p *tea.Program,
	budgetChan <-chan budgetFetchResult,
	rows []engine.OverviewRow,
) {
	pipe := newOverviewPipeline(overviewPipelineConfig{
		wantBudget:     true,
		budgetLive:     true,
		budgetFallback: true,
	})
	pipe.OnBudget = func(result *engine.BudgetResult, err error) {
		p.Send(tui.BudgetDataReadyMsg{Result: result, Error: err})
	}
	_, _ = pipe.deliverBudget(ctx, nil, budgetChan, rows)
}

// loadStateForOverview loads Pulumi state only (without preview/plan).
// passphrase is injected into subprocess env only (never [os.Setenv]).
// Returns stateResources, manifestTime, projectDir, stackName, and any error.
func loadStateForOverview(
	ctx context.Context, params overviewParams, passphrase *string,
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
	return exportStateFromProject(ctx, params.stack, passphrase, params.captureState)
}

// loadPlanForOverview loads plan steps from a file or runs pulumi preview.
// passphrase is injected into subprocess env only (never [os.Setenv]).
// This is the preview-path helper used by overviewInitAndEnrich.
func loadPlanForOverview(
	ctx context.Context, params overviewParams, projectDir, stack string, passphrase *string,
) ([]engine.PlanStep, error) {
	return resolveOverviewPlan(ctx, params.pulumiJSON, projectDir, stack, passphrase, params.capturePlan)
}

// runBackgroundPreview runs pulumi preview in the background and returns an
// OverviewChangesReadyMsg with the resulting status map. It is used as a
// tea.Cmd payload for the on-demand 'p' key handler.
// passphrase is injected into subprocess env only (never [os.Setenv]).
// If preview fails, it returns an empty OverviewChangesReadyMsg so the TUI
// remains usable in state-only mode.
func runBackgroundPreview(
	ctx context.Context,
	params overviewParams,
	projectDir, stack string,
	passphrase *string,
) tui.OverviewChangesReadyMsg {
	msg, err := runBackgroundPreviewWithError(ctx, params, projectDir, stack, passphrase)
	if err != nil {
		logging.FromContext(ctx).
			Warn().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "phased_loading").
			Msg("background preview failed; remaining in state-only mode")
	}
	return msg
}

func runBackgroundPreviewWithError(
	ctx context.Context,
	params overviewParams,
	projectDir, stack string,
	passphrase *string,
) (tui.OverviewChangesReadyMsg, error) {
	l := logging.FromContext(ctx)
	previewStart := time.Now()

	planSteps, err := resolveOverviewPlan(ctx, params.pulumiJSON, projectDir, stack, passphrase)
	if err != nil {
		return tui.OverviewChangesReadyMsg{}, err
	}

	l.Info().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "phased_loading").
		Dur("dur", time.Since(previewStart)).
		Msg("background preview completed")

	// Record plan lineage to history store (fire-and-forget).
	historyStore, historyCleanup := initHistoryFromConfig(ctx, params.cfg)
	recordHistoryPlanLineage(ctx, historyStore, planSteps)
	historyCleanup()

	hasChanges, changeCount := engine.DetectPendingChanges(ctx, planSteps)
	statusByURN := engine.BuildStatusByURN(planSteps)
	propertyDiffsByURN := engine.BuildPropertyDiffsByURN(planSteps)
	projectedPropsByURN := engine.BuildProjectedPropertiesByURN(planSteps)
	return tui.OverviewChangesReadyMsg{
		StatusByURN:         statusByURN,
		PropertyDiffsByURN:  propertyDiffsByURN,
		ProjectedPropsByURN: projectedPropsByURN,
		HasChanges:          hasChanges,
		ChangeCount:         changeCount,
	}, nil
}

// shortErrMsg returns a short (≤60 char) representation of err suitable for
// embedding in a TUI status bar. Returns "" when err is nil.
func shortErrMsg(err error) string {
	const maxLen = 60
	if err == nil {
		return ""
	}
	msg := err.Error()
	if utf8.RuneCountInString(msg) > maxLen {
		runes := []rune(msg)
		msg = string(runes[:maxLen])
	}
	return msg
}

// checkAndPromptPassphrase detects if the Pulumi stack uses passphrase encryption
// and, when it does, asks the TUI for the passphrase. It is the TUI adapter
// over resolveOverviewPassphrase.
func checkAndPromptPassphrase(
	ctx context.Context,
	p *tea.Program,
	params overviewParams,
	passphraseChan chan string,
) (*string, error) {
	return resolveOverviewPassphrase(ctx, params, passphraseChan, func() {
		p.Send(tui.OverviewPassphraseRequiredMsg{})
	})
}

// resolveOverviewPassphrase detects if the Pulumi stack uses passphrase encryption.
// If PULUMI_CONFIG_PASSPHRASE is not set and the stack YAML contains an encryptionsalt,
// it invokes prompt (to surface the request to the user) and blocks until a
// passphrase arrives on passphraseChan (or the context is cancelled).
//
// Returns a *string passphrase and any error:
//   - nil    = not provided (subprocess inherits parent env via [os.Environ])
//   - &""    = explicitly empty passphrase (inject PULUMI_CONFIG_PASSPHRASE= into subprocess)
//   - &"sec" = non-empty passphrase (inject PULUMI_CONFIG_PASSPHRASE=sec)
//
// The returned passphrase is threaded to Pulumi subprocess calls via
// ExportOptions/PreviewOptions.Passphrase and never mutates the process-wide environment.
//
// If passphrase auto-detection is not possible (e.g. using --pulumi-state files instead
// of auto-detect, or if the stack YAML cannot be read), the check is silently skipped
// and pulumi stack export will produce its own error if encryption is required.
func resolveOverviewPassphrase(
	ctx context.Context,
	params overviewParams,
	passphraseChan <-chan string,
	prompt func(),
) (*string, error) {
	log := logging.FromContext(ctx)

	// Skip if passphrase already set in environment (including empty string).
	if val, ok := os.LookupEnv("PULUMI_CONFIG_PASSPHRASE"); ok {
		log.Debug().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "passphrase_check").
			Msg("PULUMI_CONFIG_PASSPHRASE is set in environment; using it")
		return &val, nil
	}

	// Skip if PULUMI_CONFIG_PASSPHRASE_FILE is set — Pulumi reads the file itself.
	if _, ok := os.LookupEnv("PULUMI_CONFIG_PASSPHRASE_FILE"); ok {
		log.Debug().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "passphrase_check").
			Msg("PULUMI_CONFIG_PASSPHRASE_FILE is set; letting Pulumi read it")
		return nil, nil //nolint:nilnil // nil passphrase = not needed, nil error = no failure.
	}

	// Only auto-detect when not using explicit file flags
	if params.pulumiState != "" {
		log.Debug().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "passphrase_check").
			Msg("using explicit --pulumi-state; skipping passphrase detection")
		return nil, nil //nolint:nilnil // nil passphrase = not needed, nil error = no failure.
	}

	// Locate the project directory and stack name
	projectDir, stackName, detectErr := detectPulumiProject(ctx, params.stack)
	if detectErr != nil {
		// If we can't detect the project, skip (let stack export fail naturally).
		log.Debug().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "passphrase_check").
			Err(detectErr).
			Msg("cannot detect project; skipping passphrase check")
		return nil, nil //nolint:nilnil // Intentional fail-open: passphrase check is best-effort.
	}

	// Read stack settings and check for encryptionsalt.
	// Uses short stack name fallback for fully-qualified stack refs (org/project/stack).
	data, readErr := readStackSettingsFile(projectDir, stackName)
	if readErr != nil {
		// Stack settings file not found or unreadable — skip check, fail open.
		log.Debug().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "passphrase_check").
			Err(readErr).
			Msg("cannot read stack YAML; skipping passphrase check")
		return nil, nil //nolint:nilnil // Intentional fail-open: passphrase check is best-effort.
	}

	if !strings.Contains(string(data), "encryptionsalt:") {
		return nil, nil //nolint:nilnil // nil passphrase = not needed, nil error = no failure.
	}

	// Stack is encrypted and no passphrase is set — surface the request and
	// block until the user provides the passphrase or the context is cancelled.
	prompt()
	select {
	case pw := <-passphraseChan:
		return &pw, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// stackSettingsNameCandidates returns candidate stack settings names in lookup order.
// For fully-qualified stack references (org/project/stack), it tries the short
// stack name first, then the original value.
func stackSettingsNameCandidates(stackName string) []string {
	trimmed := strings.TrimSpace(stackName)
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}
	if idx := strings.LastIndexAny(trimmed, `/\`); idx >= 0 && idx < len(trimmed)-1 {
		candidates = append([]string{trimmed[idx+1:]}, candidates...)
	}

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

// readStackSettingsFile reads Pulumi stack settings from projectDir using stackName.
// It checks Pulumi.<stack>.yaml then Pulumi.<stack>.yml for each candidate name.
func readStackSettingsFile(projectDir, stackName string) ([]byte, error) {
	candidates := stackSettingsNameCandidates(stackName)
	if len(candidates) == 0 {
		return nil, errors.New("stack name is required")
	}

	var lastErr error
	for _, candidate := range candidates {
		for _, ext := range []string{".yaml", ".yml"} {
			path := filepath.Join(projectDir, "Pulumi."+candidate+ext)
			data, err := os.ReadFile(path)
			if err == nil {
				return data, nil
			}
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return nil, lastErr
}

// bridgeEnrichmentToTUI runs the pipeline's enrichment stage and bridges
// progress updates to the Bubble Tea program via Send().
func bridgeEnrichmentToTUI(
	enrichCtx context.Context,
	p *tea.Program,
	rows []engine.OverviewRow,
	eng *engine.Engine,
	dateRange engine.DateRange,
	rowCount *atomic.Int64,
) {
	pipe := newOverviewPipeline(overviewPipelineConfig{dateRange: dateRange})
	pipe.OnPhase = func(phase int, name string) {
		p.Send(tui.OverviewPhaseMsg{Index: phase - 1, Phase: name})
	}
	pipe.OnRow = func(index int, row engine.OverviewRow) {
		rowCount.Add(1)
		p.Send(tui.OverviewResourceLoadedMsg{Index: index, Row: row})
	}
	pipe.OnProgress = func(loaded, total int) {
		p.Send(tui.OverviewLoadingProgressMsg{Loaded: loaded, Total: total})
	}
	pipe.OnAllRowsLoaded = func() { p.Send(tui.OverviewAllResourcesLoadedMsg{}) }
	pipe.emitPhase(OverviewPhaseEnrichResources, overviewPhaseEnrichResourcesName)
	pipe.enrich(enrichCtx, rows, eng)
}

// loadDismissalRecordsForOverview loads dismissal records for the overview dismissal
// delta pass. Returns nil (not an error) when the store is unavailable so that
// callers can safely skip the delta without aborting the overview.
func loadDismissalRecordsForOverview(ctx context.Context) map[string]*config.DismissalRecord {
	log := logging.FromContext(ctx)

	store, err := loadDismissalStore()
	if err != nil {
		log.Debug().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "overview_dismissal_delta").
			Err(err).
			Msg("dismissal store unavailable; skipping delta")
		return nil
	}
	return store.GetAllRecords()
}

// applyDismissalDeltaToRows loads the dismissal store and appends dismissed/snoozed
// recommendation stubs to each row's Recommendations slice. This enables the count
// badge to show "N(-M)" when M recs are dismissed without listing them in full.
// The function is non-fatal and will silently proceed without modifications when dismissal data is unavailable.
func applyDismissalDeltaToRows(ctx context.Context, rows []engine.OverviewRow) []engine.OverviewRow {
	records := loadDismissalRecordsForOverview(ctx)
	if len(records) == 0 {
		return rows
	}

	for i := range rows {
		engine.ApplyDismissalDeltaToRow(&rows[i], records)
	}
	return rows
}

// applyOverviewBudgetFlags reads budget CLI flags without writing them onto
// the global config. Unset flags stay nil so evaluation falls through to
// env, config, and the default.
func applyOverviewBudgetFlags(cmd *cobra.Command, params overviewParams) BudgetFlagOverrides {
	var overrides BudgetFlagOverrides
	if cmd != nil && cmd.Flags().Changed("exit-on-threshold") {
		value := params.exitOnThreshold
		overrides.ExitOnThreshold = &value
	}
	if cmd != nil && cmd.Flags().Changed("exit-code") {
		value := params.exitCode
		overrides.ExitCode = &value
	}
	return overrides
}

// overviewRowsToBudgetInputs converts enriched OverviewRows to the []engine.CostResult
// and totalCost that evaluateBudgetStatus expects. Rows with nil ProjectedCost are skipped,
// as are live cluster-allocation children, whose cost re-allocates node cost already
// represented by other rows.
func overviewRowsToBudgetInputs(rows []engine.OverviewRow) ([]engine.CostResult, float64) {
	var costResults []engine.CostResult
	var totalCost float64
	for _, row := range rows {
		if row.ProjectedCost == nil || row.ExpansionSource == engine.ExpansionSourceLive {
			continue
		}
		totalCost += row.ProjectedCost.MonthlyCost
		costResults = append(costResults, engine.CostResult{
			ResourceType: row.Type,
			ResourceID:   row.URN,
			Currency:     row.ProjectedCost.Currency,
			Monthly:      row.ProjectedCost.MonthlyCost,
		})
	}
	return costResults, totalCost
}

// sumOverviewProjectedCost sums the projected monthly cost across all enriched
// overview rows. Rows with nil ProjectedCost are skipped.
//
// This intentionally duplicates the summation loop in overviewRowsToBudgetInputs because
// callers in the TUI path (sendBudgetResultToTUI) need only the total cost without
// allocating a []CostResult slice, while the non-TTY budget evaluation path needs both.
// Live cluster-allocation children are skipped: their cost re-allocates node cost
// already represented by other rows.
func sumOverviewProjectedCost(rows []engine.OverviewRow) float64 {
	var total float64
	for _, row := range rows {
		if row.ProjectedCost != nil && row.ExpansionSource != engine.ExpansionSourceLive {
			total += row.ProjectedCost.MonthlyCost
		}
	}
	return total
}

// runOverviewPipeline owns plugin/cache cleanup for synchronous renderers.
func runOverviewPipeline(ctx context.Context, pipe *OverviewPipeline) error {
	defer func() {
		if cleanup := pipe.Cleanup(); cleanup != nil {
			cleanup()
		}
	}()
	return pipe.Run(ctx)
}

// loadOverviewPlanOnly accepts an explicit preview without requiring Pulumi
// installation or a project. Project launches still merge exported state.
func loadOverviewPlanOnly(
	ctx context.Context,
	params overviewParams,
) ([]engine.StateResource, []engine.PlanStep, string, error) {
	plan, err := ingest.LoadPulumiPlanWithContext(ctx, params.pulumiJSON)
	if err != nil {
		return nil, nil, "", fmt.Errorf("loading Pulumi plan: %w", err)
	}
	stack := params.stack
	if stack == "" {
		stack = extractStackName(params.pulumiJSON)
	}
	return nil, convertPlanSteps(plan.Steps), stack, nil
}
