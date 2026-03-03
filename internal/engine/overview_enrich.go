package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/logging"
)

// overviewConcurrencyLimit is the maximum number of concurrent enrichment goroutines.
const overviewConcurrencyLimit = 10

// overviewEnricher is the subset of Engine methods used by the enrichment
// helpers. *Engine satisfies this interface automatically; it exists solely
// to enable lightweight test doubles without modifying the public API.
type overviewEnricher interface {
	GetActualCostWithOptionsAndErrors(ctx context.Context, request ActualCostRequest) (*CostResultWithErrors, error)
	GetProjectedCostWithErrors(ctx context.Context, resources []ResourceDescriptor) (*CostResultWithErrors, error)
	GetRecommendationsForResources(ctx context.Context, resources []ResourceDescriptor) (*RecommendationsResult, error)
}

// EnrichOverviewRow enriches a single OverviewRow by fetching actual costs,
// projected costs, and recommendations from the engine concurrently (up to 4
// goroutines per call for updating/replacing rows). When used with
// EnrichOverviewRows' worker pool, the maximum concurrent goroutines is
// approximately overviewConcurrencyLimit * 4. Partial
// failures from actual/projected cost are captured in row.Error with actual
// cost errors taking precedence; recommendation failures are logged but do
// not set row.Error.
func EnrichOverviewRow(ctx context.Context, row *OverviewRow, eng *Engine, dateRange DateRange) {
	enrichOverviewRow(ctx, row, eng, dateRange)
}

// enrichOverviewRow is the internal implementation of EnrichOverviewRow that
// accepts the overviewEnricher interface, enabling test doubles.
func enrichOverviewRow(ctx context.Context, row *OverviewRow, eng overviewEnricher, dateRange DateRange) {
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
	projectedResource := ResourceDescriptor{
		Type:       row.Type,
		ID:         row.URN,
		Provider:   resource.Provider,
		Properties: projectedPropertiesForRow(*row),
	}

	// Run enrichment calls concurrently. Each writes to a distinct field on the
	// row (ActualCost, ProjectedCost, BaselineProjectedCost, Recommendations).
	// Errors are captured in local variables and merged after all goroutines
	// complete, with actual cost errors taking precedence over projected.
	var wg sync.WaitGroup
	var actualErr, projectedErr *OverviewRowError

	// Fetch actual costs (skip for resources being created - they have no history)
	if row.Status != StatusCreating {
		wg.Add(1)
		go func() {
			defer wg.Done()
			actualErr = enrichActualCost(ctx, row, eng, resource, dateRange)
		}()
	}

	// Fetch projected costs
	wg.Add(1)
	go func() {
		defer wg.Done()
		projectedErr = enrichProjectedCost(ctx, row, eng, projectedResource)
	}()

	// For updates/replacements, also fetch projected cost at current-state
	// properties. Delta math can then use projected(new) - projected(current),
	// avoiding month-to-date extrapolation noise for unchanged pricing.
	if row.Status == StatusUpdating || row.Status == StatusReplacing {
		wg.Add(1)
		go func() {
			defer wg.Done()
			enrichBaselineProjectedCost(ctx, row, eng, resource)
		}()
	}

	// Fetch recommendations
	wg.Add(1)
	go func() {
		defer wg.Done()
		enrichRecommendations(ctx, row, eng, resource)
	}()

	// wg.Wait() establishes a happens-before edge: all goroutine writes to
	// actualErr, projectedErr, and the distinct row fields (ActualCost,
	// ProjectedCost, BaselineProjectedCost, Recommendations) are visible after
	// this point.
	wg.Wait()

	// Merge errors: actual cost error takes precedence (primary data source)
	if actualErr != nil {
		row.Error = actualErr
	} else if projectedErr != nil {
		row.Error = projectedErr
	}

	// Calculate cost drift when both actual and projected data exist
	if row.ActualCost != nil && row.ProjectedCost != nil {
		enrichCostDrift(row, dateRange)
	}
}

func projectedPropertiesForRow(row OverviewRow) map[string]interface{} {
	if len(row.ProjectedProperties) > 0 {
		return row.ProjectedProperties
	}
	return row.Properties
}

// enrichBaselineProjectedCost fetches projected cost using current-state
// properties and stores it in row.BaselineProjectedCost. Failures are logged
// at debug level and treated as non-fatal; delta math falls back when absent.
func enrichBaselineProjectedCost(
	ctx context.Context,
	row *OverviewRow,
	eng overviewEnricher,
	resource ResourceDescriptor,
) {
	log := logging.FromContext(ctx)

	result, err := eng.GetProjectedCostWithErrors(ctx, []ResourceDescriptor{resource})
	if err != nil {
		log.Debug().
			Ctx(ctx).
			Str("urn", row.URN).
			Err(err).
			Msg("failed to fetch baseline projected cost")
		return
	}
	if result == nil || len(result.Results) == 0 {
		return
	}
	costResult := result.Results[0]
	if costResult.Error != nil ||
		strings.HasPrefix(costResult.Notes, "ERROR:") ||
		strings.HasPrefix(costResult.Notes, "VALIDATION:") {
		return
	}
	row.BaselineProjectedCost = &ProjectedCostData{
		MonthlyCost: costResult.Monthly,
		Currency:    costResult.Currency,
	}
	if row.BaselineProjectedCost.Currency == "" {
		row.BaselineProjectedCost.Currency = defaultCurrency
	}
}

// enrichActualCost fetches actual cost data for a row. It writes to
// row.ActualCost directly and returns any classified error instead of
// writing to row.Error, enabling the caller to merge errors
// deterministically when running enrichment calls concurrently.
func enrichActualCost(
	ctx context.Context,
	row *OverviewRow,
	eng overviewEnricher,
	resource ResourceDescriptor,
	dateRange DateRange,
) *OverviewRowError {
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
		return classifyError(row.URN, err)
	}

	if result != nil && len(result.Results) > 0 {
		costResult := result.Results[0]
		// Skip results with errors (plugin responded but can't price this resource)
		if costResult.Error != nil ||
			strings.HasPrefix(costResult.Notes, "ERROR:") ||
			strings.HasPrefix(costResult.Notes, "VALIDATION:") {
			log.Debug().
				Ctx(ctx).
				Str("urn", row.URN).
				Str("notes", costResult.Notes).
				Msg("skipping actual cost result with error")
			return nil
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

	return nil
}

// enrichProjectedCost fetches projected cost data for a row. It writes to
// row.ProjectedCost directly and returns any classified error instead of
// writing to row.Error, enabling the caller to merge errors
// deterministically when running enrichment calls concurrently.
func enrichProjectedCost(
	ctx context.Context,
	row *OverviewRow,
	eng overviewEnricher,
	resource ResourceDescriptor,
) *OverviewRowError {
	log := logging.FromContext(ctx)

	result, err := eng.GetProjectedCostWithErrors(ctx, []ResourceDescriptor{resource})
	if err != nil {
		log.Warn().
			Ctx(ctx).
			Str("urn", row.URN).
			Err(err).
			Msg("failed to fetch projected cost")
		return classifyError(row.URN, err)
	}

	if result != nil && len(result.Results) > 0 {
		costResult := result.Results[0]
		if costResult.Error != nil ||
			strings.HasPrefix(costResult.Notes, "ERROR:") ||
			strings.HasPrefix(costResult.Notes, "VALIDATION:") {
			log.Debug().
				Ctx(ctx).
				Str("urn", row.URN).
				Str("notes", costResult.Notes).
				Msg("skipping projected cost result with error")
			return nil
		}
		row.ProjectedCost = &ProjectedCostData{
			MonthlyCost: costResult.Monthly,
			Currency:    costResult.Currency,
		}
		if row.ProjectedCost.Currency == "" {
			row.ProjectedCost.Currency = defaultCurrency
		}
		// Debug log when projected cost is $0 to help investigate intermittent $0.00 TUI costs (#723).
		if costResult.Monthly == 0 {
			log.Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("operation", "enrich_projected_cost").
				Str("urn", row.URN).
				Str("resource_type", row.Type).
				Str("adapter", costResult.Adapter).
				Str("notes", costResult.Notes).
				Msg("projected cost is $0.00: possible missing SKU/region or unpriced resource type")
		}
	}

	return nil
}

// enrichRecommendations fetches recommendations for a row.
func enrichRecommendations(ctx context.Context, row *OverviewRow, eng overviewEnricher, resource ResourceDescriptor) {
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
// It uses the elapsed runtime window (fractional days) between an effective start time and
// dateRange.End as the extrapolation denominator.
//
// Effective start time is dateRange.Start unless CreatedAt is within the window, in which
// case CreatedAt is used. This avoids treating pre-creation time as zero spend and fixes
// early-month bias caused by integer day-of-month rounding.
func enrichCostDrift(row *OverviewRow, dateRange DateRange) {
	refTime := dateRange.End
	daysInMonth := daysInCurrentMonth(refTime)
	elapsedDays := driftElapsedDays(dateRange.Start, refTime, row.CreatedAt)

	drift, err := CalculateCostDriftWithElapsedDays(
		row.ActualCost.MTDCost,
		row.ProjectedCost.MonthlyCost,
		elapsedDays,
		daysInMonth,
	)
	if err != nil {
		// Insufficient data (e.g., early in month) - skip drift
		return
	}
	row.CostDrift = drift
}

// driftElapsedDays returns the elapsed time in days used for drift math.
// It uses windowStart unless createdAt is within the window and before refTime.
// Returns 0 when the effective interval is empty or invalid.
func driftElapsedDays(windowStart, refTime time.Time, createdAt *time.Time) float64 {
	effectiveStart := windowStart
	if createdAt != nil && createdAt.After(effectiveStart) && createdAt.Before(refTime) {
		effectiveStart = *createdAt
	}
	if !effectiveStart.Before(refTime) {
		return 0
	}
	return refTime.Sub(effectiveStart).Hours() / hoursPerDay
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

	// Check sentinel errors first (typed checks are more reliable than substring matching).
	// context.Canceled and context.DeadlineExceeded use "canceled" and "deadline" which
	// don't match the existing substring patterns ("timeout", "connection", "network").
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		errType = ErrorTypeNetwork
	default:
		lower := strings.ToLower(msg)
		switch {
		case strings.Contains(lower, "auth") || strings.Contains(lower, "permission") || strings.Contains(lower, "forbidden"):
			errType = ErrorTypeAuth
		case strings.Contains(lower, "connection") || strings.Contains(lower, "network") || strings.Contains(lower, "timeout"):
			errType = ErrorTypeNetwork
		case strings.Contains(lower, "rate") || strings.Contains(lower, "throttle") || strings.Contains(lower, "too many"):
			errType = ErrorTypeRateLimit
		}
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
	enrichStart := time.Now()
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
		Int64("elapsed_ms", time.Since(enrichStart).Milliseconds()).
		Msg("row enrichment complete")

	return rows
}
