# API Contract: Enrichment Function Signatures

**Date**: 2026-02-18
**Feature**: 597-parallelize-enrichment

## Current Signatures (Before)

```go
// enrichActualCost writes to row.ActualCost and row.Error directly
func enrichActualCost(
    ctx context.Context,
    row *OverviewRow,
    eng *Engine,
    resource ResourceDescriptor,
    dateRange DateRange,
)

// enrichProjectedCost writes to row.ProjectedCost and row.Error directly
func enrichProjectedCost(
    ctx context.Context,
    row *OverviewRow,
    eng *Engine,
    resource ResourceDescriptor,
)

// enrichRecommendations writes to row.Recommendations only (no change needed)
func enrichRecommendations(
    ctx context.Context,
    row *OverviewRow,
    eng *Engine,
    resource ResourceDescriptor,
)
```

## New Signatures (After)

```go
// enrichActualCost writes to row.ActualCost, returns error separately
func enrichActualCost(
    ctx context.Context,
    row *OverviewRow,
    eng *Engine,
    resource ResourceDescriptor,
    dateRange DateRange,
) *OverviewRowError

// enrichProjectedCost writes to row.ProjectedCost, returns error separately
func enrichProjectedCost(
    ctx context.Context,
    row *OverviewRow,
    eng *Engine,
    resource ResourceDescriptor,
) *OverviewRowError

// enrichRecommendations unchanged - writes to row.Recommendations only
func enrichRecommendations(
    ctx context.Context,
    row *OverviewRow,
    eng *Engine,
    resource ResourceDescriptor,
)
```

## Public API Contract (Unchanged)

The public API `EnrichOverviewRow` and `EnrichOverviewRows` retain their exact
signatures. No callers need modification.

```go
// EnrichOverviewRow - signature unchanged
func EnrichOverviewRow(
    ctx context.Context,
    row *OverviewRow,
    eng *Engine,
    dateRange DateRange,
)

// EnrichOverviewRows - signature unchanged
func EnrichOverviewRows(
    ctx context.Context,
    rows []OverviewRow,
    eng *Engine,
    dateRange DateRange,
    progressChan chan<- OverviewRowUpdate,
) []OverviewRow
```

## Error Merge Contract

After `wg.Wait()`, errors are merged with the following precedence:

1. If `actualErr != nil` → `row.Error = actualErr`
2. Else if `projectedErr != nil` → `row.Error = projectedErr`
3. Else → `row.Error = nil`

This ensures deterministic behavior regardless of goroutine scheduling order.
