package engine

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/logging"
)

// overviewConcurrencyLimit is the maximum number of concurrent enrichment goroutines.
const overviewConcurrencyLimit = 10

// EnrichOverviewRow enriches a single OverviewRow by fetching actual costs,
// projected costs, and recommendations from the engine. Partial failures are
// captured in row.Error; the function never fails to allow batch processing
// to continue.
func EnrichOverviewRow(ctx context.Context, row *OverviewRow, eng *Engine, dateRange DateRange) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "enrich_overview_row").
		Str("urn", row.URN).
		Str("status", row.Status.String()).
		Msg("enriching overview row")

	resource := ResourceDescriptor{
		Type:       row.Type,
		ID:         row.URN,
		Provider:   extractProviderFromType(row.Type),
		Properties: row.Properties,
	}

	// Fetch actual costs (skip for resources being created - they have no history)
	if row.Status != StatusCreating {
		enrichActualCost(ctx, row, eng, resource, dateRange)
	}

	// Fetch projected costs (useful for resources with pending changes or active resources)
	enrichProjectedCost(ctx, row, eng, resource)

	// Fetch recommendations
	enrichRecommendations(ctx, row, eng, resource)

	// Calculate cost drift when both actual and projected data exist
	if row.ActualCost != nil && row.ProjectedCost != nil {
		enrichCostDrift(row, dateRange)
	}
}

// enrichActualCost fetches actual cost data for a row.
func enrichActualCost(
	ctx context.Context,
	row *OverviewRow,
	eng *Engine,
	resource ResourceDescriptor,
	dateRange DateRange,
) {
	log := logging.FromContext(ctx)

	request := ActualCostRequest{
		Resources: []ResourceDescriptor{resource},
		From:      dateRange.Start,
		To:        dateRange.End,
	}

	result, err := eng.GetActualCostWithOptionsAndErrors(ctx, request)
	if err != nil {
		log.Warn().
			Ctx(ctx).
			Str("urn", row.URN).
			Err(err).
			Msg("failed to fetch actual cost")
		row.Error = classifyError(row.URN, err)
		return
	}

	if result != nil && len(result.Results) > 0 {
		costResult := result.Results[0]
		// Skip results with error notes
		if strings.HasPrefix(costResult.Notes, "ERROR:") || strings.HasPrefix(costResult.Notes, "VALIDATION:") {
			return
		}
		row.ActualCost = &ActualCostData{
			MTDCost:  costResult.TotalCost,
			Currency: costResult.Currency,
			Period:   dateRange,
		}
		if row.ActualCost.Currency == "" {
			row.ActualCost.Currency = defaultCurrency
		}
	}
}

// enrichProjectedCost fetches projected cost data for a row.
func enrichProjectedCost(ctx context.Context, row *OverviewRow, eng *Engine, resource ResourceDescriptor) {
	log := logging.FromContext(ctx)

	result, err := eng.GetProjectedCostWithErrors(ctx, []ResourceDescriptor{resource})
	if err != nil {
		log.Warn().
			Ctx(ctx).
			Str("urn", row.URN).
			Err(err).
			Msg("failed to fetch projected cost")
		if row.Error == nil {
			row.Error = classifyError(row.URN, err)
		}
		return
	}

	if result != nil && len(result.Results) > 0 {
		costResult := result.Results[0]
		if strings.HasPrefix(costResult.Notes, "ERROR:") || strings.HasPrefix(costResult.Notes, "VALIDATION:") {
			return
		}
		row.ProjectedCost = &ProjectedCostData{
			MonthlyCost: costResult.Monthly,
			Currency:    costResult.Currency,
		}
		if row.ProjectedCost.Currency == "" {
			row.ProjectedCost.Currency = defaultCurrency
		}
	}
}

// enrichRecommendations fetches recommendations for a row.
func enrichRecommendations(ctx context.Context, row *OverviewRow, eng *Engine, resource ResourceDescriptor) {
	log := logging.FromContext(ctx)

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{resource})
	if err != nil {
		log.Warn().
			Ctx(ctx).
			Str("urn", row.URN).
			Err(err).
			Msg("failed to fetch recommendations")
		return // recommendations are optional, don't set error
	}

	if result != nil && len(result.Recommendations) > 0 {
		row.Recommendations = result.Recommendations
	}
}

// enrichCostDrift calculates cost drift for a row that has both actual and projected costs.
// It uses the dateRange end time to determine the day-of-month and days-in-month, which
// ensures correct drift for historical queries rather than always using the current date.
func enrichCostDrift(row *OverviewRow, dateRange DateRange) {
	refTime := dateRange.End
	dayOfMonth := refTime.Day()
	daysInMonth := daysInCurrentMonth(refTime)

	drift, err := CalculateCostDrift(
		row.ActualCost.MTDCost,
		row.ProjectedCost.MonthlyCost,
		dayOfMonth,
		daysInMonth,
	)
	if err != nil {
		// Insufficient data (e.g., early in month) - skip drift
		return
	}
	row.CostDrift = drift
}

// daysInCurrentMonth returns the number of days in the month of the given time.
func daysInCurrentMonth(t time.Time) int {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// classifyError converts a Go error into an OverviewRowError with an appropriate ErrorType.
// It uses substring matching intentionally: upstream plugins and gRPC do not expose typed
// or sentinel errors for auth/network/rate-limit conditions, so errors.Is/errors.As checks
// would be dead code.
func classifyError(urn string, err error) *OverviewRowError {
	msg := err.Error()
	errType := ErrorTypeUnknown

	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "auth") || strings.Contains(lower, "permission") || strings.Contains(lower, "forbidden"):
		errType = ErrorTypeAuth
	case strings.Contains(lower, "connection") || strings.Contains(lower, "network") || strings.Contains(lower, "timeout"):
		errType = ErrorTypeNetwork
	case strings.Contains(lower, "rate") || strings.Contains(lower, "throttle") || strings.Contains(lower, "too many"):
		errType = ErrorTypeRateLimit
	}

	// Truncate message if too long (rune-safe to avoid splitting multi-byte characters)
	if len(msg) > maxMessageLen {
		runes := []rune(msg)
		if len(runes) > maxMessageLen {
			msg = string(runes[:maxMessageLen])
		}
	}

	return &OverviewRowError{
		URN:       urn,
		ErrorType: errType,
		Message:   msg,
		Retryable: errType == ErrorTypeNetwork || errType == ErrorTypeRateLimit,
	}
}

// ExtractProviderFromResourceType extracts the provider name from a resource
// type string (e.g., "aws:ec2:Instance" -> "aws"). This is an exported wrapper
// around the internal extractProviderFromType function for use by CLI code.
func ExtractProviderFromResourceType(resourceType string) string {
	return extractProviderFromType(resourceType)
}

// EnrichOverviewRows enriches all rows concurrently with a semaphore limit.
// Updates are sent on progressChan as each row completes. The channel is closed
// when all rows are done. Returns the enriched rows slice.
func EnrichOverviewRows(
	ctx context.Context,
	rows []OverviewRow,
	eng *Engine,
	dateRange DateRange,
	progressChan chan<- OverviewRowUpdate,
) []OverviewRow {
	log := logging.FromContext(ctx)
	log.Info().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "enrich_overview_rows").
		Int("row_count", len(rows)).
		Msg("starting concurrent row enrichment")

	var wg sync.WaitGroup
	sem := make(chan struct{}, overviewConcurrencyLimit)

	for i := range rows {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			EnrichOverviewRow(ctx, &rows[idx], eng, dateRange)

			if progressChan != nil {
				select {
				case progressChan <- OverviewRowUpdate{
					Index: idx,
					Row:   rows[idx],
				}:
				case <-ctx.Done():
				}
			}
		}(i)
	}

	// Wait for all goroutines to finish before returning, then close the
	// progress channel synchronously. A previous implementation used a
	// separate goroutine for closing, which raced with this wg.Wait().
	wg.Wait()
	if progressChan != nil {
		close(progressChan)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "enrich_overview_rows").
		Int("row_count", len(rows)).
		Msg("row enrichment complete")

	return rows
}
