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
		// Skip results with errors
		if costResult.Error != nil ||
			strings.HasPrefix(costResult.Notes, "ERROR:") ||
			strings.HasPrefix(costResult.Notes, "VALIDATION:") {
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
		if costResult.Error != nil ||
			strings.HasPrefix(costResult.Notes, "ERROR:") ||
			strings.HasPrefix(costResult.Notes, "VALIDATION:") {
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

// ExtractProviderFromResourceType returns the provider name extracted from resourceType,
// or an empty string if no provider can be determined.
func ExtractProviderFromResourceType(resourceType string) string {
	return extractProviderFromType(resourceType)
}

// enrichWorker consumes row indices from the jobs channel, enriches each corresponding
// OverviewRow in place, and optionally sends progress updates; it exits when the jobs
// channel is closed or the context is cancelled.
//
// Parameters:
//   - ctx: context to observe cancellation and deadlines.
//   - jobs: channel providing indices of rows to process.
//   - rows: slice of OverviewRow to be updated in place by index.
//   - eng: engine used to perform enrichment operations.
//   - dateRange: date range passed to enrichment functions.
//   - progressChan: optional channel to receive OverviewRowUpdate values for each
//     completed row; may be nil to disable progress reporting.
func enrichWorker(
	ctx context.Context,
	jobs <-chan int,
	rows []OverviewRow,
	eng *Engine,
	dateRange DateRange,
	progressChan chan<- OverviewRowUpdate,
) {
	for idx := range jobs {
		if ctx.Err() != nil {
			return
		}

		EnrichOverviewRow(ctx, &rows[idx], eng, dateRange)

		if progressChan != nil {
			select {
			case progressChan <- OverviewRowUpdate{Index: idx, Row: rows[idx]}:
			case <-ctx.Done():
			}
		}
	}
}

// EnrichOverviewRows concurrently enriches each OverviewRow in rows with cost and recommendation data.
// It runs a fixed-size worker pool (capped by overviewConcurrencyLimit or the number of rows), respects
// context cancellation, and records any per-row failures in each row's Error field without returning an error.
// If progressChan is non-nil, the function sends OverviewRowUpdate messages as rows are processed and closes
// progressChan before returning.
//
// Parameters:
//   - ctx: the context to observe for cancellation and logging.
//   - rows: the slice of OverviewRow to enrich; enrichment is performed in-place and the same slice is returned.
//   - eng: the Engine used to fetch costs and recommendations.
//   - dateRange: the date range used for cost calculations.
//   - progressChan: optional channel that receives progress updates for each enriched row; may be nil.
//
// Returns:
//
//	The input slice of OverviewRow populated with enrichment results (ActualCost, ProjectedCost, Recommendations,
//	CostDrift) and any per-row Error values set for partial failures.
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

	numWorkers := overviewConcurrencyLimit
	if len(rows) < numWorkers {
		numWorkers = len(rows)
	}

	jobs := make(chan int, len(rows))
	var wg sync.WaitGroup

	// Start fixed number of workers.
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			enrichWorker(ctx, jobs, rows, eng, dateRange, progressChan)
		}()
	}

	// Send all jobs to the worker pool.
sendLoop:
	for i := range rows {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(jobs)

	// Wait for all workers to finish before returning, then close the
	// progress channel synchronously.
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
