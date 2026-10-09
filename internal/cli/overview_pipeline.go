package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/tui"
)

// Pipeline phase numbers emitted via [OverviewPipeline.OnPhase]. They are
// 1-based so they line up with the phase checklist shown to users (the TUI
// adapter subtracts 1 for its 0-based phase indices, and the web SSE contract
// uses 1..6 directly).
const (
	// OverviewPhaseLoadStackState loads the Pulumi stack state (export or file).
	OverviewPhaseLoadStackState = 1
	// OverviewPhaseDetectChanges runs lightweight change detection.
	OverviewPhaseDetectChanges = 2
	// OverviewPhaseMergeResources merges state and plan into overview rows.
	OverviewPhaseMergeResources = 3
	// OverviewPhaseStartPlugins opens the cost plugin clients.
	OverviewPhaseStartPlugins = 4
	// OverviewPhasePrepareEngine constructs the cost engine.
	OverviewPhasePrepareEngine = 5
	// OverviewPhaseEnrichResources enriches each row with cost data.
	OverviewPhaseEnrichResources = 6
)

// Phase names emitted with the phase events. They are the exact strings the
// TUI loading checklist has always shown.
const (
	overviewPhaseLoadStackStateName  = "Loading stack state..."
	overviewPhaseDetectChangesName   = "Detecting changes..."
	overviewPhaseSkipDetectionName   = "Skipping change detection (--state-only)..."
	overviewPhaseMergeResourcesName  = "Merging resources..."
	overviewPhaseStartPluginsName    = "Starting cost plugins..."
	overviewPhasePrepareEngineName   = "Preparing cost engine..."
	overviewPhaseEnrichResourcesName = "Enriching resources..."
)

// PhaseError wraps an error with the pipeline phase at which it occurred, so
// [OverviewPipeline.OnError] can report the failing phase even when the
// failure surfaces several calls below the phase boundary.
type PhaseError struct {
	Phase int
	Err   error
}

// Error returns the wrapped error's message.
func (e *PhaseError) Error() string { return e.Err.Error() }

// Unwrap exposes the wrapped error for [errors.Is] and [errors.As].
func (e *PhaseError) Unwrap() error { return e.Err }

// OverviewPipelineData is the output of the loading phases (1-3): the merged,
// filtered rows plus the stack metadata later phases and renderers need.
type OverviewPipelineData struct {
	Rows         []engine.OverviewRow
	PlanSteps    []engine.PlanStep
	StackName    string
	ProjectDir   string
	IsStateOnly  bool
	HasChanges   bool
	ChangeCount  int
	DetectErrMsg string
}

// OverviewPipelineResult is the final pipeline state delivered via
// [OverviewPipeline.OnReady] and [OverviewPipeline.Result]. Rows are the
// enriched, cluster-expanded rows; the dismissal delta and per-row deltas are
// applied per the pipeline's mode (see overviewPipelineConfig).
type OverviewPipelineResult struct {
	Rows           []engine.OverviewRow
	Engine         *engine.Engine
	Clients        []*pluginhost.Client
	StackName      string
	ProjectDir     string
	HasChanges     bool
	ChangeCount    int
	IsStateOnly    bool
	ResourceCount  int
	ExpansionNotes []string
	Budget         *engine.BudgetResult
	BudgetErr      error
	RowsEnriched   int
}

// OverviewPipelineLoader performs the data-loading phases (1-3) and reports
// each phase through p.emitPhase. Three loaders cover the three frontends:
// [tuiOverviewLoader] (state-first with change detection and on-demand
// preview), [plainOverviewLoader] (the non-interactive CLI path with its
// preview prompt), and [autoDetectOverviewLoader] (the --web path).
type OverviewPipelineLoader func(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error)

// overviewPipelineConfig parameterizes a pipeline run. The zero-value
// behavior (nil seams) is the production wiring; the function seams exist so
// tests can drive the pipeline deterministically.
type overviewPipelineConfig struct {
	cmd       *cobra.Command
	params    overviewParams
	dateRange engine.DateRange
	audit     *auditContext
	loader    OverviewPipelineLoader
	// passphrases receives passphrases for encrypted stacks. Nil disables the
	// passphrase check entirely (the plain CLI path expects
	// PULUMI_CONFIG_PASSPHRASE in the environment).
	passphrases <-chan string
	// retryPassphrase and transientPassphrase restrict web unlock secrets to a loading call.
	retryPassphrase     bool
	transientPassphrase bool
	// wantBudget fetches budgets after enrichment.
	wantBudget bool
	// budgetLive launches the plugin budget fetch concurrently with
	// enrichment (TUI and web); false fetches synchronously after expansion
	// (plain path).
	budgetLive bool
	// budgetFallback evaluates config-based budgets when plugins return none
	// (TUI and web); the plain path reports plugin budgets only.
	budgetFallback bool
	// dismissalRows applies the dismissal delta to the pipeline's own row
	// slice after enrichment (plain and web final rows). The TUI receives the
	// delta on the per-row events instead, so it leaves this false to keep
	// the expansion replacement semantics unchanged.
	dismissalRows bool

	openPlugins    func(ctx context.Context, adapter string, audit *auditContext) ([]*pluginhost.Client, func(), error)
	newEngine      func(ctx context.Context, cmd *cobra.Command, clients []*pluginhost.Client) (*engine.Engine, func())
	enrichRows     func(ctx context.Context, rows []engine.OverviewRow, eng *engine.Engine, dateRange engine.DateRange, progressChan chan<- engine.OverviewRowUpdate) []engine.OverviewRow
	expandClusters func(ctx context.Context, rows []engine.OverviewRow, clients []*pluginhost.Client, pricer engine.ResourcePricer, cfg *config.Config) ([]engine.OverviewRow, []string)
}

// OverviewPipeline is the event-driven overview orchestration shared by the
// TUI, the plain CLI, and the web UI (research Decision 2). It runs the six
// overview phases — load stack state, detect changes, merge resources, start
// plugins, prepare the engine, enrich resources (with cluster expansion and
// budget fetch) — and reports every step through the callback fields below.
// All callbacks are optional; nil callbacks are skipped. Callbacks run on the
// goroutine that called [OverviewPipeline.Run] and must not block.
type OverviewPipeline struct {
	// OnPhase fires when a phase starts: phase is 1-based (1..6), name is the
	// user-facing phase label.
	OnPhase func(phase int, name string)
	// OnDataReady fires once the engine exists: rows are a copy of the
	// merged, pre-enrichment rows.
	OnDataReady func(rows []engine.OverviewRow, totalCount int, stackName string)
	// OnStateOnly fires after OnDataReady when the run is state-only; it
	// carries the short change-detection error message ("" when none).
	OnStateOnly func(detectErrMsg string)
	// OnRow fires per enriched row with the dismissal delta and the
	// pre-computed cost delta applied.
	OnRow func(index int, row engine.OverviewRow)
	// OnProgress fires every progressReportInterval rows and on the last row.
	OnProgress func(loaded, total int)
	// OnAllRowsLoaded fires once enrichment completes (not on cancellation).
	OnAllRowsLoaded func()
	// OnExpansion fires when cluster expansion changed the row set; rows
	// replaces the full row list.
	OnExpansion func(rows []engine.OverviewRow, notes []string)
	// OnBudget fires once the budget fetch resolves (plugin budgets, or the
	// config fallback when enabled).
	OnBudget func(result *engine.BudgetResult, err error)
	// OnError fires once for a fatal phase failure with the 1-based phase
	// number; Run returns the same error.
	OnError func(phase int, err error)
	// OnReady fires last on success with the final pipeline state.
	OnReady func(result OverviewPipelineResult)
	// OnPassphraseRequired fires when the stack is passphrase-encrypted and
	// no passphrase is available; Run blocks until one arrives on the
	// configured passphrases channel (or the context is cancelled).
	OnPassphraseRequired func()
	// OnPreviewReady fires from RunPreview with the on-demand preview result
	// (the TUI's OverviewChangesReadyMsg payload).
	OnPreviewReady func(msg tui.OverviewChangesReadyMsg)
	// ConfirmEnrichment, when set, runs after plugins open (phase 4) and
	// before the engine is created (phase 5): the plain CLI's pre-flight
	// confirmation. proceed=false stops the pipeline without error.
	ConfirmEnrichment func(ctx context.Context, data OverviewPipelineData, clientCount int) (proceed bool, err error)

	cfg overviewPipelineConfig

	mu             sync.Mutex
	passphrase     *string
	lastPhase      int
	cleanup        func()
	result         OverviewPipelineResult
	expansionNotes []string
	rowsEnriched   int
}

// newOverviewPipeline builds a pipeline. cfg.loader defaults to
// [tuiOverviewLoader] when nil.
func newOverviewPipeline(cfg overviewPipelineConfig) *OverviewPipeline {
	if cfg.loader == nil {
		cfg.loader = tuiOverviewLoader
	}
	return &OverviewPipeline{cfg: cfg}
}

// Passphrase returns the passphrase resolved before the loading phases, or
// nil when none was needed. Loaders thread it to Pulumi subprocess calls.
func (p *OverviewPipeline) Passphrase() *string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.passphrase
}

// Cleanup returns the combined plugin/cache cleanup, or nil when the run
// stopped before plugins opened. It is safe to call after Run returns.
func (p *OverviewPipeline) Cleanup() func() {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cleanup
}

// Result returns the final pipeline state captured when OnReady fired.
func (p *OverviewPipeline) Result() OverviewPipelineResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.result
}

// Engine returns the shared engine once phase 5 has constructed it, or nil
// before that point. It is available when OnDataReady fires and safe to read
// from HTTP handler goroutines while the pipeline is still enriching rows.
func (p *OverviewPipeline) Engine() *engine.Engine {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.result.Engine
}

// emitPhase reports a phase start and records it so failures can be
// attributed to the phase in flight.
func (p *OverviewPipeline) emitPhase(phase int, name string) {
	p.mu.Lock()
	p.lastPhase = phase
	p.mu.Unlock()
	if p.OnPhase != nil {
		p.OnPhase(phase, name)
	}
}

// emitError reports a fatal failure: the phase comes from a [PhaseError] when
// err carries one, otherwise from the last phase emitted.
func (p *OverviewPipeline) emitError(err error) {
	p.mu.Lock()
	phase := p.lastPhase
	p.mu.Unlock()
	if phaseErr, ok := errors.AsType[*PhaseError](err); ok {
		phase = phaseErr.Phase
	}
	if p.OnError != nil {
		p.OnError(phase, err)
	}
}

// Run executes the pipeline to completion (or the first fatal error),
// emitting events through the configured callbacks. Fatal errors are reported
// via OnError and returned; recoverable per-row failures surface on the rows
// themselves, as engine.EnrichOverviewRows records them.
func (p *OverviewPipeline) Run(ctx context.Context) error {
	// Pre-check: resolve a stack passphrase before loading when the pipeline
	// accepts passphrases.
	pw, passphraseErr := p.resolvePassphrase(ctx)
	if passphraseErr != nil {
		p.emitError(&PhaseError{Phase: OverviewPhaseLoadStackState, Err: passphraseErr})
		return passphraseErr
	}
	p.mu.Lock()
	p.passphrase = pw
	p.mu.Unlock()

	data, loadErr := p.loadWithPassphraseRetry(ctx)
	if p.cfg.transientPassphrase {
		p.clearPassphrase()
	}
	if loadErr != nil {
		p.emitError(loadErr)
		return loadErr
	}
	// Capture the stack identity early so RunPreview works while enrichment
	// is still in flight (the TUI 'p' key is live as soon as OnStateOnly fires).
	p.mu.Lock()
	p.result.StackName = data.StackName
	p.result.ProjectDir = data.ProjectDir
	p.mu.Unlock()

	clients, eng, proceed, startErr := p.startEngine(ctx, data)
	if startErr != nil {
		return startErr
	}
	if !proceed {
		return nil
	}

	var budgetChan <-chan budgetFetchResult
	if p.cfg.wantBudget && p.cfg.budgetLive {
		budgetChan = launchBudgetFetch(ctx, eng)
		// Join only the producer owned by this run before plugins close. The
		// producer closes the channel, so a previously consumed result drains safely.
		defer func() {
			for range budgetChan {
				// Intentionally discard the result while joining this owned producer.
			}
		}()
	}

	p.emitPhase(OverviewPhaseEnrichResources, overviewPhaseEnrichResourcesName)
	rows := p.enrich(ctx, data.Rows, eng)
	if err := ctx.Err(); err != nil {
		p.emitError(err)
		return err
	}
	rows = p.expand(ctx, rows, clients, eng)

	var budget *engine.BudgetResult
	var budgetErr error
	if p.cfg.wantBudget {
		budget, budgetErr = p.deliverBudget(ctx, eng, budgetChan, rows)
	}
	if err := ctx.Err(); err != nil {
		p.emitError(err)
		return err
	}

	p.mu.Lock()
	p.result = OverviewPipelineResult{
		Rows:           rows,
		Engine:         eng,
		Clients:        clients,
		StackName:      data.StackName,
		ProjectDir:     data.ProjectDir,
		HasChanges:     data.HasChanges,
		ChangeCount:    data.ChangeCount,
		IsStateOnly:    data.IsStateOnly,
		ResourceCount:  len(data.Rows),
		ExpansionNotes: p.expansionNotes,
		Budget:         budget,
		BudgetErr:      budgetErr,
		RowsEnriched:   p.rowsEnriched,
	}
	result := p.result
	p.mu.Unlock()
	if p.OnReady != nil {
		p.OnReady(result)
	}
	return nil
}

// startEngine runs phases 4-5: open plugins, the optional pre-flight
// confirmation, engine construction, and the data-ready/state-only events.
// proceed is false when the confirmation declined; the pipeline then stops
// quietly (no error, no ready event).
func (p *OverviewPipeline) startEngine(
	ctx context.Context, data *OverviewPipelineData,
) ([]*pluginhost.Client, *engine.Engine, bool, error) {
	p.emitPhase(OverviewPhaseStartPlugins, overviewPhaseStartPluginsName)
	clients, pluginCleanup, pluginErr := p.openPlugins(ctx)
	if pluginErr != nil {
		p.emitError(&PhaseError{Phase: OverviewPhaseStartPlugins, Err: pluginErr})
		return nil, nil, false, pluginErr
	}

	if p.ConfirmEnrichment != nil {
		proceed, confirmErr := p.ConfirmEnrichment(ctx, *data, len(clients))
		if confirmErr != nil {
			pluginCleanup()
			p.emitError(&PhaseError{Phase: OverviewPhaseStartPlugins, Err: confirmErr})
			return nil, nil, false, confirmErr
		}
		if !proceed {
			pluginCleanup()
			return nil, nil, false, nil
		}
	}

	p.emitPhase(OverviewPhasePrepareEngine, overviewPhasePrepareEngineName)
	eng, cacheCleanup := p.newEngine(ctx, clients)
	p.mu.Lock()
	p.result.Engine = eng
	p.cleanup = func() { cacheCleanup(); pluginCleanup() }
	p.mu.Unlock()

	if p.OnDataReady != nil {
		p.OnDataReady(slices.Clone(data.Rows), len(data.Rows), data.StackName)
	}
	if data.IsStateOnly && p.OnStateOnly != nil {
		p.OnStateOnly(data.DetectErrMsg)
	}
	return clients, eng, true, nil
}

// RunPreview runs the on-demand pulumi preview for a state-only session (the
// TUI 'p' key and the web preview endpoint) and reports the result via
// OnPreviewReady. It returns the same message the TUI consumes.
func (p *OverviewPipeline) RunPreview(ctx context.Context) tui.OverviewChangesReadyMsg {
	p.mu.Lock()
	result := p.result
	pw := p.passphrase
	p.mu.Unlock()
	msg := runBackgroundPreview(ctx, p.cfg.params, result.ProjectDir, result.StackName, pw)
	if p.OnPreviewReady != nil {
		p.OnPreviewReady(msg)
	}
	return msg
}

// resolvePassphrase runs the encrypted-stack check when the pipeline accepts
// passphrases; without a channel it is a no-op (plain CLI semantics).
func (p *OverviewPipeline) resolvePassphrase(ctx context.Context) (*string, error) {
	if p.cfg.passphrases == nil {
		return nil, nil //nolint:nilnil // nil passphrase = check disabled, nil error = no failure.
	}
	return resolveOverviewPassphrase(ctx, p.cfg.params, p.cfg.passphrases, func() {
		if p.OnPassphraseRequired != nil {
			p.OnPassphraseRequired()
		}
	})
}

// openPlugins runs the configured (or production) plugin opener.
func (p *OverviewPipeline) openPlugins(ctx context.Context) ([]*pluginhost.Client, func(), error) {
	if p.cfg.openPlugins != nil {
		return p.cfg.openPlugins(ctx, p.cfg.params.adapter, p.cfg.audit)
	}
	return openPlugins(ctx, p.cfg.params.adapter, p.cfg.audit)
}

// newEngine runs the configured (or production) engine constructor.
func (p *OverviewPipeline) newEngine(ctx context.Context, clients []*pluginhost.Client) (*engine.Engine, func()) {
	if p.cfg.newEngine != nil {
		return p.cfg.newEngine(ctx, p.cfg.cmd, clients)
	}
	eng, _, cacheCleanup := newEngineWithCache(ctx, p.cfg.cmd, clients, nil)
	return eng, cacheCleanup
}

// enrich runs phase 6: per-row enrichment with dismissal delta and
// pre-computed deltas on the emitted rows, progress events, and an optional
// dismissal pass over the pipeline's own row slice.
func (p *OverviewPipeline) enrich(
	ctx context.Context,
	rows []engine.OverviewRow,
	eng *engine.Engine,
) []engine.OverviewRow {
	log := logging.FromContext(ctx)

	dismissalRecords := loadDismissalRecordsForOverview(ctx)

	enrichFn := p.cfg.enrichRows
	if enrichFn == nil {
		enrichFn = engine.EnrichOverviewRows
	}

	progressChan := make(chan engine.OverviewRowUpdate, len(rows))
	enrichedRows := make(chan []engine.OverviewRow, 1)
	// Enrichment runs in its own goroutine so subscribers (the TUI event
	// loop, an SSE stream) stay responsive while rows are priced.
	go func() {
		enrichedRows <- enrichFn(ctx, rows, eng, p.cfg.dateRange, progressChan)
	}()

	loadedCount := 0
	for update := range progressChan {
		if ctx.Err() != nil {
			continue
		}
		loadedCount++
		p.handleRowUpdate(ctx, update, dismissalRecords, loadedCount, len(rows))
	}
	// Closing the progress channel may precede the worker's return. Wait for
	// the enrichment call itself before accessing rows or releasing plugins.
	rows = <-enrichedRows

	select {
	case <-ctx.Done():
		return rows
	default:
		if p.OnAllRowsLoaded != nil {
			p.OnAllRowsLoaded()
		}
	}

	if p.cfg.dismissalRows {
		rows = applyDismissalDeltaToRows(ctx, rows)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "overview_pipeline").
		Int("total_rows", len(rows)).
		Msg("enrichment complete")
	return rows
}

// handleRowUpdate processes one enriched row: the dismissal delta and the
// pre-computed cost delta go onto the emitted row so every frontend reads the
// same values as the CLI renderers, then the row and progress events fire.
func (p *OverviewPipeline) handleRowUpdate(
	ctx context.Context,
	update engine.OverviewRowUpdate,
	dismissalRecords map[string]*config.DismissalRecord,
	loadedCount, total int,
) {
	engine.ApplyDismissalDeltaToRow(&update.Row, dismissalRecords)
	if d, ok := engine.CalculateRowDelta(update.Row, time.Now().Day()); ok {
		val := d
		update.Row.ComputedDelta = &val
	}

	p.mu.Lock()
	p.rowsEnriched = loadedCount
	p.mu.Unlock()
	if p.OnRow != nil {
		p.OnRow(update.Index, update.Row)
	}

	if loadedCount%progressReportInterval != 0 && loadedCount != total {
		return
	}
	if p.OnProgress != nil {
		p.OnProgress(loadedCount, total)
	}
	percent := 0
	if total > 0 {
		percent = (loadedCount * 100) / total //nolint:mnd // Percentage calculation.
	}
	logging.FromContext(ctx).Debug().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "overview_pipeline").
		Int("loaded", loadedCount).
		Int("total", total).
		Int("percent", percent).
		Msg("enrichment progress")
}

// expand runs cluster expansion after enrichment and, when the row set
// changed, emits the expansion event with the expanded rows.
func (p *OverviewPipeline) expand(
	ctx context.Context,
	rows []engine.OverviewRow,
	clients []*pluginhost.Client,
	pricer engine.ResourcePricer,
) []engine.OverviewRow {
	expandFn := p.cfg.expandClusters
	if expandFn == nil {
		expandFn = expandOverviewClusters
	}
	expandedRows, notes := expandFn(ctx, rows, clients, pricer, p.cfg.params.cfg)
	if !overviewRowsExpanded(expandedRows) {
		return rows
	}
	p.mu.Lock()
	p.expansionNotes = notes
	p.mu.Unlock()
	if p.OnExpansion != nil {
		p.OnExpansion(expandedRows, notes)
	}
	return expandedRows
}

// deliverBudget resolves the budget fetch and emits the budget event. With a
// live channel the fetch was launched concurrently with enrichment; without
// one the fetch runs synchronously here (plain path). When the config
// fallback is enabled and plugins returned no budgets, config-based budgets
// are evaluated against the enriched total.
func (p *OverviewPipeline) deliverBudget(
	ctx context.Context,
	eng *engine.Engine,
	budgetChan <-chan budgetFetchResult,
	rows []engine.OverviewRow,
) (*engine.BudgetResult, error) {
	var br budgetFetchResult
	if budgetChan != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case br = <-budgetChan:
		}
		if err := ctx.Err(); err != nil {
			return nil, err // Cancelled: worker joined, no budget event.
		}
	} else {
		result, err := eng.GetBudgets(ctx, nil)
		if err != nil {
			logging.FromContext(ctx).Warn().
				Ctx(ctx).
				Str("component", "cli").
				Str("operation", "overview_budget_fetch").
				Err(err).
				Msg("budget fetch failed (non-fatal)")
		}
		br = budgetFetchResult{result: result, err: err}
	}

	if p.cfg.budgetFallback && (br.result == nil || len(br.result.Budgets) == 0) {
		cfg := config.GetGlobalConfig()
		var budgetsCfg *config.BudgetsConfig
		if cfg != nil {
			budgetsCfg = cfg.Cost.Budgets
		}
		totalCost := sumOverviewProjectedCost(rows)
		if configResult := engine.BuildConfigBudgetResult(ctx, budgetsCfg, totalCost); configResult != nil {
			if p.OnBudget != nil {
				p.OnBudget(configResult, nil)
			}
			return configResult, nil
		}
	}
	if p.OnBudget != nil {
		p.OnBudget(br.result, br.err)
	}
	return br.result, br.err
}

// tuiOverviewLoader is the state-first TUI loading flow: stack export (or
// explicit files), lightweight change detection, optional preview, merge, and
// filters.
func tuiOverviewLoader(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
	params := p.cfg.params
	log := logging.FromContext(ctx)
	pw := p.Passphrase()

	p.emitPhase(OverviewPhaseLoadStackState, overviewPhaseLoadStackStateName)
	stateResources, manifestTime, projectDir, stackName, stateErr := loadStateForOverview(ctx, params, pw)
	if stateErr != nil {
		return nil, &PhaseError{
			Phase: OverviewPhaseLoadStackState,
			Err:   fmt.Errorf("resolve overview data: %w", stateErr),
		}
	}

	// Record state resources to history store (fire-and-forget).
	historyStore, historyCleanup := initHistoryFromConfig(ctx, params.cfg)
	defer historyCleanup()
	recordHistorySnapshot(ctx, historyStore, stateResources)

	var isStateOnly bool
	var detectErr error
	if params.stateOnly {
		log.Info().Ctx(ctx).Msg("--state-only: skipping change detection")
		p.emitPhase(OverviewPhaseDetectChanges, overviewPhaseSkipDetectionName)
		isStateOnly = true
	} else {
		p.emitPhase(OverviewPhaseDetectChanges, overviewPhaseDetectChangesName)
		signal, dErr := pulumidetect.DetectChanges(ctx, manifestTime, projectDir)
		detectErr = dErr
		if detectErr != nil {
			log.Warn().Ctx(ctx).Err(detectErr).
				Msg("change detection failed; will fall back to state-only unless --yes was set")
		}
		isStateOnly = resolveIsStateOnly(params, signal, detectErr)
	}

	p.emitPhase(OverviewPhaseMergeResources, overviewPhaseMergeResourcesName)
	rows, planSteps, hasChanges, changeCount, mergePhaseErr := buildOverviewRows(
		ctx, isStateOnly, stateResources, params, projectDir, stackName, pw,
	)
	if mergePhaseErr != nil {
		if params.pulumiJSON != "" || ctx.Err() != nil {
			return nil, &PhaseError{Phase: OverviewPhaseMergeResources, Err: mergePhaseErr}
		}
		rows = engine.NewRowsFromState(ctx, stateResources)
		isStateOnly = true
		detectErr = mergePhaseErr
		if p.OnError != nil {
			p.OnError(OverviewPhaseDetectChanges, errors.New("preview unavailable; showing current state"))
		}
	}

	// Record plan lineage to history store (fire-and-forget).
	if !isStateOnly && len(planSteps) > 0 {
		recordHistoryPlanLineage(ctx, historyStore, planSteps)
	}

	log.Debug().Ctx(ctx).Str("stack", stackName).
		Bool("has_changes", hasChanges).Int("change_count", changeCount).
		Bool("is_state_only", isStateOnly).Msg("overview data phase complete")

	rows, filterErr := validateAndApplyOverviewFilters(rows, params.filter)
	if filterErr != nil {
		return nil, &PhaseError{Phase: OverviewPhaseMergeResources, Err: filterErr}
	}

	return &OverviewPipelineData{
		Rows:         rows,
		PlanSteps:    planSteps,
		StackName:    stackName,
		ProjectDir:   projectDir,
		IsStateOnly:  isStateOnly,
		HasChanges:   hasChanges,
		ChangeCount:  changeCount,
		DetectErrMsg: shortErrMsg(detectErr),
	}, nil
}

// plainOverviewLoader is the non-interactive CLI loading flow: explicit files
// or state-first auto-detect with the preview prompt, merge, and filters.
func plainOverviewLoader(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
	params := p.cfg.params

	p.emitPhase(OverviewPhaseLoadStackState, overviewPhaseLoadStackStateName)
	stateResources, planSteps, stackName, isStateOnly, err := loadPlainOverviewData(ctx, p.cfg.cmd, params)
	if err != nil {
		wrappedErr := fmt.Errorf("resolve overview data: %w", err)
		if p.cfg.audit != nil {
			p.cfg.audit.logFailure(ctx, wrappedErr)
		}
		return nil, &PhaseError{Phase: OverviewPhaseLoadStackState, Err: wrappedErr}
	}

	// Record state resources and plan lineage to history store (fire-and-forget).
	historyStore, historyCleanup := initHistoryFromConfig(ctx, params.cfg)
	defer historyCleanup()
	recordHistorySnapshot(ctx, historyStore, stateResources)
	if !isStateOnly {
		recordHistoryPlanLineage(ctx, historyStore, planSteps)
	}

	p.emitPhase(OverviewPhaseDetectChanges, overviewPhaseDetectChangesName)
	var hasChanges bool
	var changeCount int
	if !isStateOnly {
		hasChanges, changeCount = engine.DetectPendingChanges(ctx, planSteps)
	}

	p.emitPhase(OverviewPhaseMergeResources, overviewPhaseMergeResourcesName)
	var rows []engine.OverviewRow
	if isStateOnly {
		rows = engine.NewRowsFromState(ctx, stateResources)
	} else {
		var mergeErr error
		rows, mergeErr = engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
		if mergeErr != nil {
			wrappedErr := fmt.Errorf("merging resources: %w", mergeErr)
			if p.cfg.audit != nil {
				p.cfg.audit.logFailure(ctx, wrappedErr)
			}
			return nil, &PhaseError{Phase: OverviewPhaseMergeResources, Err: wrappedErr}
		}
	}

	rows, err = validateAndApplyOverviewFilters(rows, params.filter)
	if err != nil {
		return nil, &PhaseError{Phase: OverviewPhaseMergeResources, Err: err}
	}

	return &OverviewPipelineData{
		Rows:        rows,
		PlanSteps:   planSteps,
		StackName:   stackName,
		IsStateOnly: isStateOnly,
		HasChanges:  hasChanges,
		ChangeCount: changeCount,
	}, nil
}

// autoDetectOverviewLoader is the --web loading flow: explicit files, or the
// overview auto-detect path (detectPulumiProject, exportStateFromProject,
// loadOverviewFromAutoDetect) with the resolved passphrase threaded through.
// Explicit files use resolveOverviewData; project loads share
// loadOverviewProjectData with loadOverviewFromAutoDetect and retain the
// project directory for on-demand preview.
func autoDetectOverviewLoader(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
	params := p.cfg.params
	// Project launches use the same state-first loader as the TUI, including
	// recoverable change-detection failures and on-demand preview support.
	if params.pulumiState == "" && params.pulumiJSON == "" {
		return tuiOverviewLoader(ctx, p)
	}
	pw := p.Passphrase()

	p.emitPhase(OverviewPhaseLoadStackState, overviewPhaseLoadStackStateName)
	var (
		stateResources []engine.StateResource
		planSteps      []engine.PlanStep
		stackName      string
		projectDir     string
	)
	if params.pulumiState != "" {
		var err error
		stateResources, planSteps, stackName, err = resolveOverviewData(ctx, params)
		if err != nil {
			return nil, &PhaseError{Phase: OverviewPhaseLoadStackState, Err: err}
		}
	} else {
		var err error
		stateResources, planSteps, projectDir, stackName, err = loadOverviewProjectData(ctx, params, pw)
		if err != nil {
			return nil, &PhaseError{Phase: OverviewPhaseLoadStackState, Err: err}
		}
	}

	// Record state resources and plan lineage to history store (fire-and-forget).
	historyStore, historyCleanup := initHistoryFromConfig(ctx, params.cfg)
	defer historyCleanup()
	recordHistorySnapshot(ctx, historyStore, stateResources)
	recordHistoryPlanLineage(ctx, historyStore, planSteps)

	p.emitPhase(OverviewPhaseDetectChanges, overviewPhaseDetectChangesName)
	hasChanges, changeCount := engine.DetectPendingChanges(ctx, planSteps)

	p.emitPhase(OverviewPhaseMergeResources, overviewPhaseMergeResourcesName)
	rows, mergeErr := engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
	if mergeErr != nil {
		return nil, &PhaseError{
			Phase: OverviewPhaseMergeResources,
			Err:   fmt.Errorf("merging resources: %w", mergeErr),
		}
	}

	rows, filterErr := validateAndApplyOverviewFilters(rows, params.filter)
	if filterErr != nil {
		return nil, &PhaseError{Phase: OverviewPhaseMergeResources, Err: filterErr}
	}

	return &OverviewPipelineData{
		Rows:        rows,
		PlanSteps:   planSteps,
		StackName:   stackName,
		ProjectDir:  projectDir,
		HasChanges:  hasChanges,
		ChangeCount: changeCount,
	}, nil
}

// clearPassphrase releases the reference once the subprocess operation returns.
func (p *OverviewPipeline) clearPassphrase() { p.mu.Lock(); p.passphrase = nil; p.mu.Unlock() }

func (p *OverviewPipeline) loadWithPassphraseRetry(ctx context.Context) (*OverviewPipelineData, error) {
	for {
		data, err := p.cfg.loader(ctx, p)
		if err == nil || !p.cfg.retryPassphrase || p.cfg.passphrases == nil ||
			(!rejectedPassphrase(err) || p.Passphrase() == nil) {
			return data, err
		}
		p.clearPassphrase()
		p.emitError(
			&PhaseError{
				Phase: OverviewPhaseLoadStackState,
				Err:   errors.New("unable to decrypt stack; try the passphrase again"),
			},
		)
		if p.OnPassphraseRequired != nil {
			p.OnPassphraseRequired()
		}
		select {
		case pw, ok := <-p.cfg.passphrases:
			if !ok {
				return nil, errors.New("passphrase input closed")
			}
			p.mu.Lock()
			p.passphrase = &pw
			p.mu.Unlock()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
func rejectedPassphrase(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "incorrect passphrase") || strings.Contains(message, "invalid passphrase") ||
		strings.Contains(message, "failed to decrypt") ||
		strings.Contains(message, "decrypting secrets") ||
		strings.Contains(message, "unable to decrypt stack")
}

// RunPreviewWithError invokes the shared preview core and surfaces failures to
// the HTTP transport. Web passphrases are requested for this operation only.
func (p *OverviewPipeline) RunPreviewWithError(ctx context.Context) (tui.OverviewChangesReadyMsg, error) {
	p.mu.Lock()
	result := p.result
	p.mu.Unlock()
	retryAttempt := false
	for {
		msg, hadPassphrase, err := p.previewAttempt(ctx, result, retryAttempt)
		retry := err != nil && p.cfg.retryPassphrase && p.cfg.transientPassphrase && p.cfg.passphrases != nil &&
			hadPassphrase &&
			rejectedPassphrase(err)
		if !retry {
			if err == nil && p.OnPreviewReady != nil {
				p.OnPreviewReady(msg)
			}
			return msg, err
		}
		p.emitError(
			&PhaseError{
				Phase: OverviewPhaseDetectChanges,
				Err:   errors.New("unable to decrypt stack; try the passphrase again"),
			},
		)
		retryAttempt = true
	}
}

// previewAttempt scopes the passphrase to one subprocess call. Returning only
// whether one was provided releases the rejected secret before the next prompt.
func (p *OverviewPipeline) previewAttempt(
	ctx context.Context,
	result OverviewPipelineResult,
	retry bool,
) (tui.OverviewChangesReadyMsg, bool, error) {
	pw := p.Passphrase()
	if p.cfg.transientPassphrase {
		var err error
		if retry {
			pw, err = p.waitForPassphraseRetry(ctx)
		} else {
			pw, err = p.resolvePassphrase(ctx)
		}
		if err != nil {
			return tui.OverviewChangesReadyMsg{}, false, err
		}
	}
	msg, err := runBackgroundPreviewWithError(ctx, p.cfg.params, result.ProjectDir, result.StackName, pw)
	return msg, pw != nil, err
}

func (p *OverviewPipeline) waitForPassphraseRetry(ctx context.Context) (*string, error) {
	if p.OnPassphraseRequired != nil {
		p.OnPassphraseRequired()
	}
	select {
	case pw, ok := <-p.cfg.passphrases:
		if !ok {
			return nil, errors.New("passphrase input closed")
		}
		return &pw, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
