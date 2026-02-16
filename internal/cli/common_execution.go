package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/router"
)

// auditContext holds common context for audit logging within a cost command.
type auditContext struct {
	logger  logging.AuditLogger
	traceID string
	params  map[string]string
	start   time.Time
	command string
}

// newAuditContext creates a new audit context.
func newAuditContext(ctx context.Context, command string, params map[string]string) *auditContext {
	return &auditContext{
		logger:  logging.AuditLoggerFromContext(ctx),
		traceID: logging.TraceIDFromContext(ctx),
		params:  params,
		start:   time.Now(),
		command: command,
	}
}

// logFailure logs an audit entry for a failed operation.
func (a *auditContext) logFailure(ctx context.Context, err error) {
	entry := logging.NewAuditEntry(a.command, a.traceID).
		WithParameters(a.params).
		WithError(err.Error()).
		WithDuration(a.start)
	a.logger.Log(ctx, *entry)
}

// logSuccess logs an audit entry for a successful operation.
func (a *auditContext) logSuccess(ctx context.Context, count int, cost float64) {
	entry := logging.NewAuditEntry(a.command, a.traceID).
		WithParameters(a.params).
		WithSuccess(count, cost).
		WithDuration(a.start)
	a.logger.Log(ctx, *entry)
}

// loadAndMapResources loads a Pulumi plan from planPath and returns its mapped resources.
// If loading or mapping fails the error is logged, audit.logFailure is invoked when audit is non-nil,
// and a wrapped error is returned.
// Parameters:
//   - ctx: context for cancellation and logging.
//   - planPath: filesystem path to the Pulumi plan to load.
//   - audit: optional auditContext used to record failures; may be nil.
//
// Returns the slice of mapped ResourceDescriptor on success, or an error describing the failure.
func loadAndMapResources(
	ctx context.Context,
	planPath string,
	audit *auditContext,
) ([]engine.ResourceDescriptor, error) {
	log := logging.FromContext(ctx)

	plan, err := ingest.LoadPulumiPlanWithContext(ctx, planPath)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Str("plan_path", planPath).Msg("failed to load Pulumi plan")
		if audit != nil {
			audit.logFailure(ctx, err)
		}
		return nil, fmt.Errorf("loading Pulumi plan: %w", err)
	}

	resources, err := ingest.MapResources(plan.GetResourcesWithContext(ctx))
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to map resources")
		if audit != nil {
			audit.logFailure(ctx, err)
		}
		return nil, fmt.Errorf("mapping resources: %w", err)
	}
	log.Debug().Ctx(ctx).Int("resource_count", len(resources)).Msg("resources loaded from plan")

	return resources, nil
}

// openPlugins opens the requested adapter plugins and returns the plugin clients,
// a cleanup function to release plugin resources, and an error if opening fails.
// The ctx is used for plugin initialization and cancellation. The adapter string
// selects which adapter plugins to load. The provided audit context is recorded
// when a failure occurs.
// Returns the loaded plugin clients, a cleanup function that should be called
// when the callers are finished with the plugins, and a non-nil error if opening
// openPlugins opens plugins for the specified adapter using the default registry.
// It returns the loaded plugin clients, a cleanup function that is guaranteed to be non-nil, and an error.
// If plugin opening fails the error is logged and, if audit is non-nil, recorded via audit.logFailure.
//
// Parameters:
//   - ctx: context for plugin operations and logging.
//   - adapter: name of the plugin adapter to open.
//   - audit: optional audit context used to record failures; may be nil.
//
// Returns:
//   - []*pluginhost.Client: slice of opened plugin clients (nil on error).
//   - func(): cleanup function to release plugin resources (never nil).
//   - error: non-nil if opening plugins failed.
func openPlugins(ctx context.Context, adapter string, audit *auditContext) ([]*pluginhost.Client, func(), error) {
	log := logging.FromContext(ctx)

	clients, cleanup, err := registry.NewDefault().Open(ctx, adapter)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Str("adapter", adapter).Msg("failed to open plugins")
		if audit != nil {
			audit.logFailure(ctx, err)
		}
		return nil, nil, fmt.Errorf("opening plugins: %w", err)
	}
	if cleanup == nil {
		cleanup = func() {}
	}
	log.Debug().Ctx(ctx).Int("plugin_count", len(clients)).Msg("plugins opened")

	return clients, cleanup, nil
}

// recommendationFetcher abstracts recommendation retrieval for testability.
type recommendationFetcher interface {
	GetRecommendationsForResources(
		ctx context.Context, resources []engine.ResourceDescriptor,
	) (*engine.RecommendationsResult, error)
}

// fetchAndMergeRecommendations fetches recommendations for the given resources
// and merges them into the corresponding cost results by ResourceID.
// It tries multiple ID formats (URN, cloud ID, ARN) to handle plugins that
// return recommendations keyed by cloud-native identifiers rather than Pulumi URNs.
// Errors are logged at WARN level but never propagated (FR-006).
func fetchAndMergeRecommendations(ctx context.Context, fetcher recommendationFetcher,
	resources []engine.ResourceDescriptor, results []engine.CostResult) {
	log := logging.FromContext(ctx)
	recsResult, err := fetcher.GetRecommendationsForResources(ctx, resources)
	if err != nil {
		log.Warn().Ctx(ctx).Err(err).
			Str("operation", "fetch_and_merge_recommendations").
			Msg("failed to fetch recommendations for detail view")
		return
	}
	if recsResult == nil || len(recsResult.Recommendations) == 0 {
		return
	}

	recMap := make(map[string][]engine.Recommendation)
	for _, rec := range recsResult.Recommendations {
		if rec.ResourceID == "" {
			log.Warn().Ctx(ctx).
				Str("operation", "fetch_and_merge_recommendations").
				Str("recommendation_type", rec.Type).
				Msg("skipping recommendation with empty ResourceID")
			continue
		}
		recMap[rec.ResourceID] = append(recMap[rec.ResourceID], rec)
	}

	// Build reverse-lookup maps from cloud-native IDs to result indices.
	// Resources and results share the same ID (URN), so we correlate via that.
	altIDToResultIdx := buildAltIDIndex(resources, results)

	merged := 0
	for i := range results {
		if recs, found := recMap[results[i].ResourceID]; found {
			results[i].Recommendations = recs
			merged += len(recs)
		}
	}

	// Second pass: match unmatched recommendations via cloud ID / ARN lookup.
	if merged < len(recsResult.Recommendations) {
		for recID, recs := range recMap {
			if idx, found := altIDToResultIdx[recID]; found {
				if len(results[idx].Recommendations) == 0 {
					results[idx].Recommendations = recs
					merged += len(recs)
				}
			}
		}
	}

	log.Debug().Ctx(ctx).
		Str("operation", "fetch_and_merge_recommendations").
		Int("recommendations_count", len(recsResult.Recommendations)).
		Int("merged_count", merged).
		Msg("merged recommendations into cost results")
}

// buildAltIDIndex builds a map from alternative resource identifiers (cloud ID,
// ARN) to cost result indices. This allows matching plugin recommendations that
// use cloud-native IDs instead of Pulumi URNs.
func buildAltIDIndex(
	resources []engine.ResourceDescriptor,
	results []engine.CostResult,
) map[string]int {
	// Map URN → result index for correlation.
	urnToIdx := make(map[string]int, len(results))
	for i, r := range results {
		urnToIdx[r.ResourceID] = i
	}

	altMap := make(map[string]int)
	for _, res := range resources {
		idx, ok := urnToIdx[res.ID]
		if !ok {
			continue
		}
		// Extract cloud-native identifiers from resource properties.
		for _, key := range []string{"pulumi:cloudId", "pulumi:arn", "id", "arn"} {
			if v, exists := res.Properties[key]; exists {
				if s, isStr := v.(string); isStr && s != "" {
					altMap[s] = idx
				}
			}
		}
	}
	return altMap
}

// detectPulumiProject locates the Pulumi binary, discovers the project directory,
// detectPulumiProject detects the Pulumi CLI, locates the Pulumi project directory starting
// from the current working directory, and resolves the Pulumi stack name.
// If the provided stack is empty, the current stack for the detected project is auto-detected.
//
// ctx provides cancellation and carries logger/trace information used by detection.
// stack is an optional stack name; when empty the function will query the Pulumi project for
// the current stack.
//
// Returns the project directory and the resolved stack name. An error is returned if the
// Pulumi binary cannot be found, the project directory cannot be located, or the current
// stack cannot be determined.
func detectPulumiProject(ctx context.Context, stack string) (string, string, error) {
	log := logging.FromContext(ctx)

	if _, binErr := pulumidetect.FindBinary(); binErr != nil {
		return "", "", fmt.Errorf("find pulumi binary: %w", binErr)
	}

	projectDir, err := pulumidetect.FindProject(".")
	if err != nil {
		return "", "", fmt.Errorf("find pulumi project: %w", err)
	}
	log.Debug().Ctx(ctx).
		Str("component", "pulumi").
		Str("operation", "detect_project").
		Str("project_dir", projectDir).
		Msg("detected Pulumi project")

	if stack == "" {
		detected, stackErr := pulumidetect.GetCurrentStack(ctx, projectDir)
		if stackErr != nil {
			return "", "", fmt.Errorf("detect current stack in %s: %w", projectDir, stackErr)
		}
		stack = detected
	}
	log.Debug().Ctx(ctx).
		Str("component", "pulumi").
		Str("operation", "detect_project").
		Str("stack", stack).
		Msg("using Pulumi stack")

	return projectDir, stack, nil
}

// pulumiMode represents the Pulumi CLI operation to execute.
type pulumiMode string

const (
	modePulumiPreview pulumiMode = "preview"
	modePulumiExport  pulumiMode = "export"
)

// resolveResourcesFromPulumi orchestrates auto-detection of a Pulumi project and
// execution of the appropriate Pulumi CLI command to produce resource descriptors.
//
// If `stack` is empty the current Pulumi stack for the detected project directory is used.
// `mode` must be either modePulumiPreview or modePulumiExport.
// The function returns an error if the Pulumi binary or project cannot be found, if the stack cannot be resolved,
// if the Pulumi command fails, if parsing the Pulumi output fails, or if an unsupported mode is provided.
//
// Returns:
//   - a slice of engine.ResourceDescriptor representing the mapped resources from the Pulumi output.
//   - an error if any step (binary/project discovery, stack resolution, command execution, parsing, or unsupported mode) fails.
func resolveResourcesFromPulumi(
	ctx context.Context,
	stack string,
	mode pulumiMode,
) ([]engine.ResourceDescriptor, error) {
	log := logging.FromContext(ctx)

	projectDir, resolvedStack, err := detectPulumiProject(ctx, stack)
	if err != nil {
		return nil, err
	}

	switch mode {
	case modePulumiPreview:
		log.Info().Ctx(ctx).Str("component", "pulumi").
			Msg("Running pulumi preview --json (this may take a moment)...")

		data, previewErr := pulumidetect.Preview(ctx, pulumidetect.PreviewOptions{
			ProjectDir: projectDir,
			Stack:      resolvedStack,
		})
		if previewErr != nil {
			return nil, fmt.Errorf("running pulumi preview: %w", previewErr)
		}

		plan, parseErr := ingest.ParsePulumiPlanWithContext(ctx, data)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing Pulumi preview output: %w", parseErr)
		}

		resources, mapErr := ingest.MapResources(plan.GetResourcesWithContext(ctx))
		if mapErr != nil {
			return nil, fmt.Errorf("mapping preview resources: %w", mapErr)
		}
		return resources, nil

	case modePulumiExport:
		log.Info().Ctx(ctx).Str("component", "pulumi").
			Msg("Running pulumi stack export...")

		data, exportErr := pulumidetect.StackExport(ctx, pulumidetect.ExportOptions{
			ProjectDir: projectDir,
			Stack:      resolvedStack,
		})
		if exportErr != nil {
			return nil, fmt.Errorf("running pulumi stack export: %w", exportErr)
		}

		state, parseErr := ingest.ParseStackExportWithContext(ctx, data)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing Pulumi stack export output: %w", parseErr)
		}

		customResources := state.GetCustomResourcesWithContext(ctx)
		resources, mapErr := ingest.MapStateResources(customResources)
		if mapErr != nil {
			return nil, fmt.Errorf("mapping state resources: %w", mapErr)
		}
		return resources, nil

	default:
		return nil, fmt.Errorf("unsupported Pulumi mode: %s", mode)
	}
}

// newEngineWithCache creates an Engine, wires the router and an optional cache.Cache.
// An optional cfg may be passed to reuse an already-loaded configuration; if nil,
// config.New() is called internally.
// The returned cleanup function must be called to release the cache database handle.
func newEngineWithCache(
	ctx context.Context,
	cmd *cobra.Command,
	clients []*pluginhost.Client,
	loader engine.SpecLoader,
	cfgs ...*config.Config,
) (*engine.Engine, func()) {
	var cfg *config.Config
	if len(cfgs) > 0 && cfgs[0] != nil {
		cfg = cfgs[0]
	} else {
		cfg = config.New()
	}
	eng := engine.New(clients, loader).
		WithRouter(createRouterForEngine(ctx, cfg, clients))

	cacheStore := initCacheFromConfig(ctx, cmd, cfg)
	cacheCleanup := func() {}
	if cacheStore != nil {
		eng = eng.WithCache(cacheStore)
		cacheCleanup = func() {
			if closeErr := cacheStore.Close(); closeErr != nil {
				log := logging.FromContext(ctx)
				log.Warn().Ctx(ctx).Err(closeErr).
					Str("component", "cache").
					Msg("failed to close cache store")
			}
		}
	}
	return eng, cacheCleanup
}

// InitCache creates a cache.Cache instance based on configuration precedence:
// CLI flag (--cache-ttl) > env var (FINFOCUS_CACHE_TTL) > config file > default.
// Returns nil when caching is disabled (TTL<=0) or initialization fails.
// When --cache-ttl is explicitly set to 0, caching is disabled regardless of config/env.
func InitCache(ctx context.Context, cmd *cobra.Command) cache.Cache {
	return initCacheFromConfig(ctx, cmd, config.New())
}

// initCacheFromConfig is the internal implementation of InitCache that accepts
// a pre-loaded config to avoid redundant config.New() calls.
func initCacheFromConfig(ctx context.Context, cmd *cobra.Command, cfg *config.Config) cache.Cache {
	log := logging.FromContext(ctx)

	// Determine cache TTL with precedence: CLI flag > env var > config > default (0)
	cacheTTL := 0

	// Start with config value
	if cfg.Cost.Cache.Enabled && cfg.Cost.Cache.TTLSeconds > 0 {
		cacheTTL = cfg.Cost.Cache.TTLSeconds
	}

	// Override with env var if set (any valid integer, including 0 to disable)
	envName := cache.EnvTTLSeconds
	envVal := os.Getenv(envName)
	if envVal == "" {
		envName = cache.EnvTTLSecondsLegacy
		envVal = os.Getenv(envName)
	}
	if envVal != "" {
		if ttl, err := strconv.Atoi(envVal); err == nil {
			cacheTTL = ttl
		} else {
			log.Warn().
				Ctx(ctx).
				Err(err).
				Str("component", "cache").
				Str("operation", "init").
				Str("env_var", envName).
				Str("value", envVal).
				Msg("invalid cache TTL env var, ignoring")
		}
	}

	// Override with CLI flag if explicitly set by user
	if cmd.Flags().Changed("cache-ttl") {
		if flagTTL, flagErr := cmd.Flags().GetInt("cache-ttl"); flagErr == nil {
			cacheTTL = flagTTL
			log.Debug().
				Ctx(ctx).
				Str("component", "cache").
				Str("operation", "init").
				Int("cache_ttl", cacheTTL).
				Msg("cache TTL overridden by --cache-ttl flag")
		}
	}

	// TTL<=0 means caching is disabled
	if cacheTTL <= 0 {
		return nil
	}

	// Determine cache directory using project-dir resolution:
	// 1. FINFOCUS_CACHE_DIR env var (explicit override)
	// 2. Resolved project dir ({projectDir}/.finfocus/)
	// 3. ~/.finfocus/ (global fallback)
	cacheDir := resolveCacheDir(ctx, cfg)

	// Use configured max size directly (0 means unlimited)
	cacheMaxSize := cfg.Cost.Cache.MaxSizeMB

	cacheStore, err := cache.NewBoltStore(ctx, cacheDir, true, cacheTTL, cacheMaxSize)
	if err != nil {
		if errors.Is(err, cache.ErrCacheLocked) {
			log.Warn().
				Ctx(ctx).
				Str("component", "cache").
				Str("operation", "init").
				Msg("cache database locked, proceeding without cache")
		} else {
			log.Warn().
				Ctx(ctx).
				Err(err).
				Str("component", "cache").
				Str("operation", "init").
				Msg("cache initialization failed, proceeding without cache")
		}
		return nil
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "cache").
		Str("operation", "init").
		Int("cache_ttl", cacheTTL).
		Str("cache_dir", cacheDir).
		Msg("cache initialized with BoltDB backend")

	return cacheStore
}

// resolveCacheDir determines the cache directory using the resolution chain:
// FINFOCUS_CACHE_DIR env > config > project dir > ~/.finfocus/.
func resolveCacheDir(ctx context.Context, cfg *config.Config) string {
	log := logging.FromContext(ctx)
	if dir := os.Getenv(cache.EnvCacheDir); dir != "" {
		return dir
	}
	if cfg.Cost.Cache.Directory != "" {
		return cfg.Cost.Cache.Directory
	}
	if projectDir := config.GetResolvedProjectDir(); projectDir != "" {
		return projectDir
	}
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		log.Warn().
			Ctx(ctx).
			Err(homeErr).
			Str("component", "cache").
			Str("operation", "init").
			Msg("failed to determine home directory, using relative cache path")
		return ".finfocus"
	}
	return filepath.Join(homeDir, ".finfocus")
}

// results and reports whether multiple distinct currencies were encountered.
// It returns the chosen currency and a boolean that is `true` if more than one
// distinct non-empty currency was present in the slice. If no result contains a
// extractCurrencyFromResults determines a canonical currency for a set of cost results.
// It scans results for the first non-empty currency and returns that currency along with
// a boolean indicating whether more than one distinct non-empty currency was observed.
// If no result contains a currency, it returns defaultCurrency and false.
//
// Parameters:
//   - results: slice of CostResult to inspect for currency information.
//
// Returns:
//   - string: the chosen currency (first non-empty found or defaultCurrency if none found).
//   - bool: true if multiple distinct non-empty currencies were detected, false otherwise.
func extractCurrencyFromResults(results []engine.CostResult) (string, bool) {
	currency := ""
	mixedCurrencies := false

	for _, r := range results {
		if r.Currency != "" {
			if currency == "" {
				currency = r.Currency
			} else if r.Currency != currency {
				mixedCurrencies = true
				break
			}
		}
	}

	if currency == "" {
		currency = defaultCurrency
	}

	return currency, mixedCurrencies
}

// printTimingOutput writes a brief timing summary to stderr via cmd when the output
// format is a table. It writes the number of resources analyzed, the elapsed time
// since start, and the resources-per-second rate. If the output format is not table
// (for example JSON or NDJSON), the function does nothing.
func printTimingOutput(cmd *cobra.Command, start time.Time, resourceCount int, output string) {
	if engine.OutputFormat(output) != engine.OutputTable {
		return
	}
	elapsed := time.Since(start)
	throughput := 0.0
	if elapsed.Seconds() > 0 {
		throughput = float64(resourceCount) / elapsed.Seconds()
	}
	cmd.PrintErrf("\nAnalyzed %d resources in %.1fs (%.1f resources/sec)\n",
		resourceCount, elapsed.Seconds(), throughput)
}

// evaluateBudgetStatus checks budget thresholds when all results share the same
// currency. It extracts the currency, obtains a scope filter from cmd via
// getBudgetScopeFilter, calls renderBudgetWithScope to produce a budgetResult,
// and returns any exit error from checkBudgetExitFromResult. Returns nil when
// currencies are mixed or no budget violation is detected.
func evaluateBudgetStatus(
	cmd *cobra.Command,
	results []engine.CostResult,
	totalCost float64,
) error {
	currency, mixedCurrencies := extractCurrencyFromResults(results)
	if mixedCurrencies {
		return nil
	}
	scopeFilter := getBudgetScopeFilter(cmd)
	budgetResult, budgetErr := renderBudgetWithScope(cmd, results, totalCost, currency, scopeFilter)
	return checkBudgetExitFromResult(cmd, budgetResult, budgetErr)
}

// createRouterForEngine creates an engine.Router from the user's routing
// configuration. It returns nil when cfg is nil, cfg.Routing is nil, or when
// router creation fails (logged at WARN level). A nil return is safe:
// engine.WithRouter(nil) preserves the default "query all plugins" behaviour.
//
// ctx is used for logging and cancellation.
// cfg is the application config; callers that already hold one should pass it to
// avoid a redundant config.New() call.
// clients are plugin host clients supplied to the router builder.
func createRouterForEngine(ctx context.Context, cfg *config.Config, clients []*pluginhost.Client) engine.Router {
	log := logging.FromContext(ctx)

	if cfg == nil || cfg.Routing == nil {
		return nil
	}

	r, err := router.NewRouter(
		router.WithClients(clients),
		router.WithConfig(cfg.Routing),
	)
	if err != nil {
		log.Warn().Ctx(ctx).Err(err).
			Str("component", "cli").
			Str("operation", "create_router").
			Msg("failed to create router, falling back to all-plugin mode")
		return nil
	}

	return router.NewEngineAdapter(r)
}
