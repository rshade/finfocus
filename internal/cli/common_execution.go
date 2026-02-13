package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/registry"
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

// loadAndMapResources loads a Pulumi plan and maps its resources.
func loadAndMapResources(
	ctx context.Context,
	planPath string,
	audit *auditContext,
) ([]engine.ResourceDescriptor, error) {
	log := logging.FromContext(ctx)

	plan, err := ingest.LoadPulumiPlanWithContext(ctx, planPath)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Str("plan_path", planPath).Msg("failed to load Pulumi plan")
		audit.logFailure(ctx, err)
		return nil, fmt.Errorf("loading Pulumi plan: %w", err)
	}

	resources, err := ingest.MapResources(plan.GetResourcesWithContext(ctx))
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to map resources")
		audit.logFailure(ctx, err)
		return nil, fmt.Errorf("mapping resources: %w", err)
	}
	log.Debug().Ctx(ctx).Int("resource_count", len(resources)).Msg("resources loaded from plan")

	return resources, nil
}

// openPlugins opens the requested adapter plugins.
func openPlugins(ctx context.Context, adapter string, audit *auditContext) ([]*pluginhost.Client, func(), error) {
	log := logging.FromContext(ctx)

	clients, cleanup, err := registry.NewDefault().Open(ctx, adapter)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Str("adapter", adapter).Msg("failed to open plugins")
		audit.logFailure(ctx, err)
		return nil, nil, fmt.Errorf("opening plugins: %w", err)
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

	for i := range results {
		if recs, found := recMap[results[i].ResourceID]; found {
			results[i].Recommendations = recs
		}
	}

	log.Debug().Ctx(ctx).
		Str("operation", "fetch_and_merge_recommendations").
		Int("recommendations_count", len(recsResult.Recommendations)).
		Msg("merged recommendations into cost results")
}

// resolveResourcesFromPulumi orchestrates auto-detection of a Pulumi project and
// execution of the appropriate Pulumi CLI command to produce resource descriptors.
// mode must be "preview" or "export".
func resolveResourcesFromPulumi(
	ctx context.Context,
	stack string,
	mode string,
) ([]engine.ResourceDescriptor, error) {
	log := logging.FromContext(ctx)

	// Step 1: Find the Pulumi binary
	if _, err := pulumidetect.FindBinary(); err != nil {
		return nil, err
	}

	// Step 2: Find the Pulumi project
	projectDir, err := pulumidetect.FindProject(".")
	if err != nil {
		return nil, err
	}
	log.Debug().Ctx(ctx).Str("project_dir", projectDir).Msg("detected Pulumi project")

	// Step 3: Resolve stack
	if stack == "" {
		detected, stackErr := pulumidetect.GetCurrentStack(ctx, projectDir)
		if stackErr != nil {
			return nil, stackErr
		}
		stack = detected
	}
	log.Debug().Ctx(ctx).Str("stack", stack).Msg("using Pulumi stack")

	// Step 4: Execute the appropriate Pulumi CLI command
	switch mode {
	case "preview":
		log.Info().Ctx(ctx).Str("component", "pulumi").
			Msg("Running pulumi preview --json (this may take a moment)...")

		data, previewErr := pulumidetect.Preview(ctx, pulumidetect.PreviewOptions{
			ProjectDir: projectDir,
			Stack:      stack,
		})
		if previewErr != nil {
			return nil, previewErr
		}

		plan, parseErr := ingest.ParsePulumiPlanWithContext(ctx, data)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing Pulumi preview output: %w", parseErr)
		}

		return ingest.MapResources(plan.GetResourcesWithContext(ctx))

	case "export":
		log.Info().Ctx(ctx).Str("component", "pulumi").
			Msg("Running pulumi stack export...")

		data, exportErr := pulumidetect.StackExport(ctx, pulumidetect.ExportOptions{
			ProjectDir: projectDir,
			Stack:      stack,
		})
		if exportErr != nil {
			return nil, exportErr
		}

		state, parseErr := ingest.ParseStackExportWithContext(ctx, data)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing Pulumi stack export output: %w", parseErr)
		}

		customResources := state.GetCustomResourcesWithContext(ctx)
		return ingest.MapStateResources(customResources)

	default:
		return nil, fmt.Errorf("unsupported Pulumi mode: %s", mode)
	}
}

// extractCurrencyFromResults scans results to find a single canonical currency.
// It returns the currency code and a boolean indicating if mixed currencies were detected.
// If no currency is found, it defaults to "USD".
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
