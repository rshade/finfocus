package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/forecast"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/stackload"
	"github.com/rshade/finfocus/internal/webui"
)

// webData owns the shared CLI domain pipelines. Its immutable source snapshot is
// captured before engine publication; the gate bounds concurrent plugin queries.
type webData struct {
	cmd               *cobra.Command
	params            overviewParams
	dates             engine.DateRange
	audit             *auditContext
	eng               *engine.Engine
	history           history.Store
	cache             cache.Cache
	clients           []*pluginhost.Client
	snapshot          *stackload.Snapshot
	publishGraph      func(engine.DependencyGraph)
	terraformDeployed []engine.ResourceDescriptor
	terraformPlanned  []engine.ResourceDescriptor
	terraformSource   bool
	gate              chan struct{}
	trends            map[string]string
	totalTrend        string
	resolveScorer     func(context.Context, string, config.ResolvedScoring,
		[]*pluginhost.Client, *auditContext) (*pluginhost.Client, string, func())
	fetchRecommendations func(context.Context, *cobra.Command, *engine.Engine,
		[]engine.ResourceDescriptor, bool) (*engine.RecommendationsResult, error)
	scoreRecommendations func(context.Context, *cobra.Command, costRecommendationsParams,
		config.ResolvedScoring, []*pluginhost.Client, cache.Cache, *auditContext,
		*engine.RecommendationsResult) (bool, func(), error)
	mergeDismissed func(context.Context, *engine.RecommendationsResult) error
	loadDismissals func(context.Context) map[string]*config.DismissalRecord
	priceForecast  func(context.Context) ([]engine.CostResult, error)
	readHistory    func(string) ([]history.CostSnapshot, error)
	// historyDirOverride replaces the cost history directory; empty uses the
	// CLI's resolution (environment, config with the project-tier gate, default).
	historyDirOverride string
	// mergedCfg is the project-merged config the CLI history commands read.
	// params.cfg is config.New(), which has no project overlay.
	mergedCfg     *config.Config
	forecastNow   time.Time
	reloadCleanup func()
	replaced      bool
	escMu         sync.Mutex
	escSource     *esc.Source
}

func newWebData(cmd *cobra.Command, params overviewParams, dates engine.DateRange, audit *auditContext) *webData {
	if params.cfg == nil {
		params.cfg = config.New()
	}
	data := &webData{
		cmd: cmd, params: params, dates: dates, audit: audit, gate: make(chan struct{}, 1),
		resolveScorer: resolveScorerClient, fetchRecommendations: fetchRecommendationsWithProgress,
		scoreRecommendations: runScoringStep, mergeDismissed: mergeDismissedRecommendations,
		loadDismissals: loadDismissalRecordsForOverview, readHistory: readForecastHistory,
	}
	data.priceForecast = data.sessionForecastCosts
	if cmd != nil {
		data.noteESC(cmd.Context())
	}
	return data
}

// ESCSource returns the session's current ESC resolver.
func (d *webData) ESCSource() *esc.Source {
	if d == nil {
		return nil
	}
	d.escMu.Lock()
	defer d.escMu.Unlock()
	return d.escSource
}

func (d *webData) noteESC(ctx context.Context) {
	src, ok := esc.SourceFromContext(ctx)
	if !ok {
		return
	}
	d.escMu.Lock()
	d.escSource = src
	d.escMu.Unlock()
}

func (d *webData) freshESC(ctx context.Context) context.Context {
	d.escMu.Lock()
	current := d.escSource
	d.escMu.Unlock()
	if current == nil {
		if src, ok := esc.SourceFromContext(ctx); ok {
			current = src
		}
	}
	if current == nil {
		return ctx
	}
	next := current.Fresh()
	d.escMu.Lock()
	d.escSource = next
	d.escMu.Unlock()
	return esc.WithSource(ctx, next)
}

// bindPipelineSource publishes source data before query callbacks become available.
func (d *webData) bindPipelineSource(ctx context.Context, pipe *OverviewPipeline) {
	onDataReady := pipe.OnDataReady
	pipe.OnDataReady = func(rows []engine.OverviewRow, total int, stack string) {
		result := pipe.Result()
		if err := d.setPipelineSource(ctx, result); err != nil {
			return
		}
		d.publishSnapshotGraph(result.Snapshot)
		onDataReady(rows, total, stack)
	}
}

// commitPreview publishes a preview's source, then appends and prices its
// plan-only rows. The dependency graph is published only when those rows
// priced, so a failed preview never pairs the new graph with the rows the
// session keeps.
func (d *webData) commitPreview(
	ctx context.Context,
	result OverviewPipelineResult,
	rows []engine.OverviewRow,
) ([]engine.OverviewRow, error) {
	if err := d.setPipelineSource(ctx, result); err != nil {
		return nil, err
	}
	merged, err := d.appendPreviewRows(ctx, rows)
	if err != nil {
		return nil, err
	}
	d.publishSnapshotGraph(result.Snapshot)
	return merged, nil
}

// publishSnapshotGraph hands the snapshot's dependency graph to the session.
func (d *webData) publishSnapshotGraph(snapshot *stackload.Snapshot) {
	if d.publishGraph != nil && snapshot != nil {
		d.publishGraph(snapshot.Graph())
	}
}

// setPipelineSource publishes the pipeline's source under the query gate.
func (d *webData) setPipelineSource(ctx context.Context, result OverviewPipelineResult) error {
	unlock, err := d.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	d.snapshot = result.Snapshot
	d.terraformSource = result.TerraformResources != nil
	d.terraformPlanned = cloneWebResources(result.TerraformResources)
	d.terraformDeployed = nil
	for _, resource := range cloneWebResources(result.TerraformResources) {
		if resource.Operation == engine.DiffOperationCreate {
			continue
		}
		if resource.OldProperties != nil {
			resource.Properties = deepCopyProperties(resource.OldProperties)
		}
		d.terraformDeployed = append(d.terraformDeployed, resource)
	}
	return nil
}

var errPreviewPricing = errors.New("preview rows could not be priced")

// appendPreviewRows adds plan-only creates through the merge the CLI Overview
// uses, applies the launch filters to the added rows, then prices only those
// rows. The query gate stays held until pricing finishes, like a query lease,
// so a settings save, install, or remove cannot reload and close the clients
// mid-pricing. A row left unpriced by a failure returns the rows with
// errPreviewPricing so the preview is not marked loaded and can run again.
func (d *webData) appendPreviewRows(ctx context.Context, rows []engine.OverviewRow) ([]engine.OverviewRow, error) {
	unlock, err := d.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()

	var steps []engine.PlanStep
	if d.snapshot != nil {
		steps = d.snapshot.Steps
	}
	merged := engine.AppendPlanOnlyRows(rows, steps)
	added, err := validateAndApplyOverviewFilters(merged[len(rows):], d.params.filter)
	if err != nil {
		return nil, err
	}
	if len(added) > 0 && d.eng != nil {
		engine.EnrichOverviewRows(ctx, added, d.eng, d.dates, nil)
	}
	failed := countPricingFailures(added)
	if d.loadDismissals != nil {
		if records := d.loadDismissals(ctx); len(records) > 0 {
			for i := range added {
				engine.ApplyDismissalDeltaToRow(&added[i], records)
			}
		}
	}
	out := slices.Concat(merged[:len(rows)], added)
	if failed > 0 {
		logging.FromContext(ctx).Warn().
			Ctx(ctx).
			Str("component", "webui").
			Str("operation", "append_preview_rows").
			Int("failed_rows", failed).
			Int("added_rows", len(added)).
			Msg("preview rows could not be priced; preview not marked loaded")
		return out, errPreviewPricing
	}
	return out, nil
}

func countPricingFailures(rows []engine.OverviewRow) int {
	failed := 0
	for i := range rows {
		if rows[i].Error != nil || rows[i].ProjectedPricingFailed {
			failed++
		}
	}
	return failed
}

func (d *webData) newEngine(
	ctx context.Context,
	cmd *cobra.Command,
	clients []*pluginhost.Client,
) (*engine.Engine, func()) {
	var cleanup func()
	d.eng, d.history, d.cache, cleanup = newEngineWithCacheAndHistory(ctx, cmd, clients, nil, d.params.cfg)
	d.clients = append([]*pluginhost.Client(nil), clients...)
	scorerCleanup := func() {}
	if cfg := d.params.cfg.Scoring.Resolve(); cfg.Enabled {
		scorer, _, release := d.resolveScorer(ctx, d.params.adapter, cfg, clients, d.audit)
		scorerCleanup = release
		if scorer != nil && !slices.Contains(d.clients, scorer) {
			d.clients = append(d.clients, scorer)
		}
	}
	d.trends, d.totalTrend = costTableTrends(cmd)
	return d.eng, func() { scorerCleanup(); cleanup() }
}

// reloadPlugins opens a new plugin set for cost queries. The first call leaves
// the overview pipeline's original clients open; later calls close only the
// set this method installed.
func (d *webData) reloadPlugins(ctx context.Context) error {
	unlock, err := d.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	ctx = d.freshESC(ctx)
	clients, pluginCleanup, err := openPlugins(ctx, d.params.adapter, d.audit)
	if err != nil {
		return err
	}
	// Reuse the cache and history the overview pipeline already holds. Opening
	// a second Bolt file fails while that process keeps the lock, and the new
	// engine would then answer cost queries with neither store.
	cfg := d.params.cfg
	eng := engine.New(clients, nil).
		WithRouter(createRouterForEngine(ctx, cfg, clients)).
		WithPricingSpecFallback(cfg.Cost.PricingSpecFallback)
	if profileErr := applyUsageProfile(d.cmd, eng, usageProfileSource(cfg)); profileErr != nil {
		pluginCleanup()
		return profileErr
	}
	if d.cache != nil {
		eng = eng.WithCache(d.cache)
	}
	scorerCleanup := func() {}
	nextClients := append([]*pluginhost.Client(nil), clients...)
	if resolved := cfg.Scoring.Resolve(); resolved.Enabled {
		scorer, _, release := d.resolveScorer(ctx, d.params.adapter, resolved, clients, d.audit)
		scorerCleanup = release
		if scorer != nil && !slices.Contains(nextClients, scorer) {
			nextClients = append(nextClients, scorer)
		}
	}
	d.eng = eng
	d.clients = nextClients
	next := func() {
		scorerCleanup()
		pluginCleanup()
	}
	if d.replaced && d.reloadCleanup != nil {
		d.reloadCleanup()
	}
	d.reloadCleanup = next
	d.replaced = true
	return nil
}

// closeReloaded releases plugins opened by reloadPlugins. It does not take the
// query gate; the server has already stopped.
func (d *webData) closeReloaded() {
	if !d.replaced || d.reloadCleanup == nil {
		return
	}
	d.reloadCleanup()
	d.reloadCleanup = nil
	d.replaced = false
}

func (d *webData) lock(ctx context.Context) (func(), error) {
	select {
	case d.gate <- struct{}{}:
		return func() { <-d.gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func cloneWebResources(resources []engine.ResourceDescriptor) []engine.ResourceDescriptor {
	out := append([]engine.ResourceDescriptor(nil), resources...)
	for i := range out {
		out[i].Properties = deepCopyProperties(out[i].Properties)
		out[i].OldProperties = deepCopyProperties(out[i].OldProperties)
		if out[i].Refs != nil {
			refs := make(map[string][]string, len(out[i].Refs))
			for key, urns := range out[i].Refs {
				refs[key] = append([]string(nil), urns...)
			}
			out[i].Refs = refs
		}
	}
	return out
}

func (d *webData) loadResources(_ context.Context) ([]engine.ResourceDescriptor, error) {
	if d.terraformSource {
		return cloneWebResources(d.terraformDeployed), nil
	}
	if d.snapshot == nil {
		return nil, errors.New("stack source unavailable")
	}
	return d.snapshot.ActualDescriptors(), nil
}

func (d *webData) liveResources() ([]engine.ResourceDescriptor, error) {
	if d.terraformSource {
		return cloneWebResources(d.terraformPlanned), nil
	}
	if d.snapshot == nil {
		return nil, errors.New("stack source unavailable")
	}
	// Pricing descriptors preserve individual preview operations. The live web
	// inventory instead represents each resource after the preview is applied:
	// one descriptor per replacement, with deletions left out.
	var resources []engine.ResourceDescriptor
	for _, resource := range collapseLiveReplacements(d.snapshot.Descriptors()) {
		if resource.Operation == engine.DiffOperationDelete {
			continue
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (d *webData) actualCosts(
	ctx context.Context,
	group string,
	tags map[string]string,
) (*engine.CostResultWithErrors, error) {
	unlock, err := d.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	resources, err := d.loadResources(ctx)
	if err != nil {
		return nil, err
	}
	recordDescriptorHistory(ctx, d.history, resources)
	resources = enrichWithHistoricalResources(ctx, d.history, resources, d.dates.Start, d.dates.End)
	resources, err = ApplyFilters(ctx, resources, d.params.filter)
	if err != nil {
		return nil, err
	}
	resources = resolveResourceTypes(ctx, d.clients, d.cache, resources)
	request := buildActualCostRequest(
		costActualParams{adapter: d.params.adapter, groupBy: group},
		resources,
		d.dates.Start,
		d.dates.End,
	)
	request.Tags = tags
	result, err := d.eng.GetActualCostWithOptionsAndErrors(ctx, request)
	if err != nil {
		return nil, err
	}
	fetchAndMergeRecommendations(ctx, d.eng, resources, result.Results)
	return result, nil
}

func (d *webData) recommendations(ctx context.Context, include bool) (*engine.RecommendationsResult, error) {
	unlock, err := d.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	resources, err := d.liveResources()
	if err != nil {
		return nil, err
	}
	resources, err = ApplyFilters(ctx, resources, d.params.filter)
	if err != nil {
		return nil, err
	}
	// Progress/scorer output belongs to the terminal CLI. Use an isolated writer
	// while retaining the exact fetch, annotate, score and dismissal-merge pipeline.
	silent := &cobra.Command{}
	silent.SetOut(io.Discard)
	silent.SetErr(io.Discard)
	resources = prepareRecommendationResources(ctx, d.clients, d.cache, resources)
	result, err := d.fetchRecommendations(ctx, silent, d.eng, resources, include)
	if err != nil || result == nil {
		return result, err
	}
	annotateActiveStatus(result)
	params := costRecommendationsParams{includeDismissed: include}
	_, cleanup, err := d.scoreRecommendations(
		ctx,
		silent,
		params,
		d.params.cfg.Scoring.Resolve(),
		d.clients,
		d.cache,
		d.audit,
		result,
	)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	if include {
		if mergeErr := d.mergeDismissed(ctx, result); mergeErr != nil {
			logging.FromContext(ctx).
				Warn().
				Ctx(ctx).
				Str("component", "cli").
				Str("operation", "web_recommendations").
				Msg("dismissed recommendations unavailable")
		}
	}
	return result, nil
}

func (d *webData) tableTrends() (map[string]string, string) { return d.trends, d.totalTrend }

// recommendationHistory reads one recommendation's lifecycle through the
// dashboard engine and the same dismissal store the CLI history command uses.
func (d *webData) recommendationHistory(
	ctx context.Context,
	id string,
) ([]config.LifecycleEvent, []string, error) {
	eng, release, err := d.queryEngine(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer release()
	store, err := loadDismissalStore()
	if err != nil {
		return nil, nil, err
	}
	return eng.GetRecommendationHistory(ctx, store, id)
}

// queryEngine returns the engine cost and estimate queries should use. After a
// plugin reload this is the replacement, not the overview pipeline's original.
func (d *webData) queryEngine(ctx context.Context) (*engine.Engine, func(), error) {
	unlock, err := d.lock(ctx)
	if err != nil {
		return nil, nil, err
	}
	if d.eng == nil {
		unlock()
		return nil, nil, errors.New("overview data unavailable")
	}
	return d.eng, unlock, nil
}

// estimateResources loads real source descriptors independently of synthetic
// overview cluster rows. Preview properties are overlaid by the session transport.
func (d *webData) estimateResources(ctx context.Context) ([]engine.ResourceDescriptor, error) {
	unlock, err := d.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	resources, err := d.liveResources()
	if err != nil {
		return nil, err
	}

	resources, err = ApplyFilters(ctx, resources, d.params.filter)
	if err != nil {
		return nil, err
	}
	return prepareEstimateResources(ctx, d.clients, resources), nil
}

// forecast answers the dashboard's `cost forecast` query for the session stack.
// It prices the resources the session already holds with its query engine and
// builds the document through the same buildForecast the CLI calls.
func (d *webData) forecast(ctx context.Context, query webui.ForecastQuery) (forecast.Document, error) {
	settings, err := webForecastSettings(query, d.params.cfg)
	if err != nil {
		return forecast.Document{}, err
	}
	settings.now = d.forecastNow
	costs, stack, unchanged, err := d.forecastInputs(ctx)
	if err != nil {
		return forecast.Document{}, err
	}
	built, err := buildForecast(costs, settings, func() ([]history.CostSnapshot, error) {
		return d.readHistory(stack)
	})
	if err != nil {
		return forecast.Document{}, explainNoCostData(err, unchanged)
	}
	return built.document(), nil
}

// explainNoCostData says why a forecast with nothing to project came up empty
// when unchanged resources were left out of it. The forecast covers the
// resources a preview changes, so a preview whose changes have no price leaves
// nothing, while the Overview still totals the whole stack. The CLI message is
// not changed, and the error still matches forecast.ErrNoCostData.
func explainNoCostData(err error, unchanged int) error {
	if !errors.Is(err, forecast.ErrNoCostData) || unchanged == 0 {
		return err
	}
	noun := "resources are"
	if unchanged == 1 {
		noun = "resource is"
	}
	return fmt.Errorf("%w (the preview's changes have no price, and %d unchanged %s not part of the forecast)",
		err, unchanged, noun)
}

// forecastInputs prices the forecast and captures the stack under the query
// gate, then releases it: the model fit and the history read need neither
// the session engine nor its clients.
//
// The count of unchanged resources the forecast leaves out is read here, with
// the priced inputs, so an error explanation describes the same snapshot even
// if a preview replaces it afterwards.
func (d *webData) forecastInputs(ctx context.Context) ([]engine.CostResult, string, int, error) {
	unlock, err := d.lock(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	defer unlock()
	costs, err := d.priceForecast(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	return costs, d.forecastStack(), d.unchangedExcludedFromForecast(), nil
}

func (d *webData) unchangedExcludedFromForecast() int {
	if d.snapshot == nil {
		return 0
	}
	unchanged := 0
	for _, resource := range d.snapshot.Descriptors() {
		if resource.Origin == engine.OriginState {
			unchanged++
		}
	}
	return unchanged
}

// webForecastSettings validates a dashboard query with the CLI's rules and
// order: growth, then months.
func webForecastSettings(query webui.ForecastQuery, cfg *config.Config) (forecastSettings, error) {
	growthType, rate, err := forecast.ParseGrowth(query.GrowthType, query.GrowthRate)
	if err != nil {
		return forecastSettings{}, err
	}
	months := forecast.DefaultMonths
	if query.Months != nil {
		months = *query.Months
	}
	if err = forecast.ValidateMonths(months); err != nil {
		return forecastSettings{}, err
	}
	settings := forecastSettings{
		months:     months,
		growthType: growthType,
		rate:       rate,
		noHistory:  query.NoHistory,
		cfg:        cfg,
	}
	inputs := forecastModelInputs{
		model:      query.Model,
		confidence: query.Confidence,
		growthCap:  query.GrowthCap,
		steps:      query.Steps,
		excludes:   query.Exclude,
	}
	if query.SpendHistory != nil {
		inputs.spendCSV = strings.NewReader(*query.SpendHistory)
	}
	if err = parseForecastModel(inputs, &settings); err != nil {
		return forecastSettings{}, err
	}
	return settings, nil
}

// sessionForecastCosts prices sessionForecastResources with the session query
// engine. The caller holds the query gate.
func (d *webData) sessionForecastCosts(ctx context.Context) ([]engine.CostResult, error) {
	if d.eng == nil {
		return nil, errors.New("overview data unavailable")
	}
	resources, err := d.sessionForecastResources(ctx)
	if err != nil {
		return nil, err
	}
	diff, err := d.eng.GetProjectedCostDiff(ctx, resources)
	if err != nil {
		return nil, fmt.Errorf("calculating projected costs: %w", err)
	}
	return diff.AfterCosts(), nil
}

// sessionForecastResources is the membership `cost forecast` prices:
// plan-origin resources (#1903), or a Terraform session's planned resources,
// after the launch filters.
func (d *webData) sessionForecastResources(ctx context.Context) ([]engine.ResourceDescriptor, error) {
	var resources []engine.ResourceDescriptor
	switch {
	case d.terraformSource:
		resources = cloneWebResources(d.terraformPlanned)
	case d.snapshot != nil:
		resources = planOriginResources(d.snapshot.Descriptors())
	default:
		return nil, errors.New("stack source unavailable")
	}
	resources, err := ApplyFilters(ctx, resources, d.params.filter)
	if err != nil {
		return nil, err
	}
	return resolveResourceTypes(ctx, d.clients, d.cache, resources), nil
}

func (d *webData) forecastStack() string {
	if d.snapshot != nil && d.snapshot.StackName != "" {
		return d.snapshot.StackName
	}
	return d.params.stack
}
