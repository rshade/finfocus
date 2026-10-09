package cli

import (
	"context"
	"errors"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// webData owns the shared CLI domain pipelines. Its immutable source snapshot is
// captured before engine publication; the gate bounds concurrent plugin queries.
type webData struct {
	cmd            *cobra.Command
	params         overviewParams
	dates          engine.DateRange
	audit          *auditContext
	eng            *engine.Engine
	history        history.Store
	cache          cache.Cache
	clients        []*pluginhost.Client
	raw            []engine.ResourceDescriptor
	planned        []engine.ResourceDescriptor
	planErr        error
	sourceErr      error
	sourceCaptured bool
	gate           chan struct{}
	trends         map[string]string
	totalTrend     string
	resolveScorer  func(context.Context, string, config.ResolvedScoring,
		[]*pluginhost.Client, *auditContext) (*pluginhost.Client, string, func())
	fetchRecommendations func(context.Context, *cobra.Command, *engine.Engine,
		[]engine.ResourceDescriptor, bool) (*engine.RecommendationsResult, error)
	scoreRecommendations func(context.Context, *cobra.Command, costRecommendationsParams,
		config.ResolvedScoring, []*pluginhost.Client, cache.Cache, *auditContext,
		*engine.RecommendationsResult) (bool, func(), error)
	mergeDismissed func(context.Context, *engine.RecommendationsResult) error
}

func newWebData(cmd *cobra.Command, params overviewParams, dates engine.DateRange, audit *auditContext) *webData {
	if params.cfg == nil {
		params.cfg = config.New()
	}
	return &webData{
		cmd: cmd, params: params, dates: dates, audit: audit, gate: make(chan struct{}, 1),
		resolveScorer: resolveScorerClient, fetchRecommendations: fetchRecommendationsWithProgress,
		scoreRecommendations: runScoringStep, mergeDismissed: mergeDismissedRecommendations,
	}
}

func (d *webData) overviewParams() overviewParams {
	params := d.params
	params.captureState = d.captureState
	params.capturePlan = d.capturePlan
	return params
}

func (d *webData) captureState(resources []ingest.StackExportResource) {
	d.raw, d.sourceErr = ingest.MapStateResources(resources)
	d.sourceCaptured = true
}

// capturePlan keeps only real initial preview sources, before synthetic expansion.
func (d *webData) capturePlan(plan *ingest.PulumiPlan) {
	d.planned, d.planErr = ingest.MapResources(plan.GetResources())
	d.planned = cloneWebResources(d.planned)
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

func (d *webData) loadResources(ctx context.Context) ([]engine.ResourceDescriptor, error) {
	if d.params.pulumiJSON != "" || d.params.pulumiState != "" {
		return loadActualResources(
			ctx,
			d.cmd,
			costActualParams{planPath: d.params.pulumiJSON, statePath: d.params.pulumiState},
			d.audit,
		)
	}
	if d.sourceErr != nil {
		return nil, d.sourceErr
	}
	if !d.sourceCaptured {
		return nil, errors.New("stack source unavailable")
	}
	return cloneWebResources(d.raw), nil
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
	resources, err := d.loadResources(ctx)
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

// estimateResources loads real source descriptors independently of synthetic
// overview cluster rows. Preview properties are overlaid by the session transport.
func (d *webData) estimateResources(ctx context.Context) ([]engine.ResourceDescriptor, error) {
	unlock, err := d.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	resources, err := d.loadResources(ctx)
	if err != nil {
		return nil, err
	}
	if d.planErr != nil {
		return nil, d.planErr
	}
	indices := make(map[string]int, len(resources))
	for i, resource := range resources {
		indices[resource.ID] = i
	}
	for _, resource := range cloneWebResources(d.planned) {
		if index, exists := indices[resource.ID]; exists {
			resources[index] = resource
		} else {
			indices[resource.ID] = len(resources)
			resources = append(resources, resource)
		}
	}

	return ApplyFilters(ctx, resources, d.params.filter)
}
