package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
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
