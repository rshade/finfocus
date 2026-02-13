# Engine Interface Contract

**Component**: Overview Engine Functions  
**Version**: v1.0.0  
**Date**: 2026-02-11

---

## Core Functions

### 1. MergeResourcesForOverview

Merges Pulumi state and preview into unified overview rows.

```go
// MergeResourcesForOverview creates a unified list of overview rows by merging
// Pulumi state (actual resources) with Pulumi preview (pending changes).
//
// The function:
// 1. Iterates through state resources (preserving order per FR-011)
// 2. Maps preview operations to ResourceStatus
// 3. Returns skeleton rows (costs not yet populated)
//
// Parameters:
//   - ctx: Context for logging and tracing
//   - state: Pulumi stack export (from `pulumi stack export`)
//   - plan: Pulumi preview JSON (from `pulumi preview --json`)
//
// Returns:
//   - []OverviewRow: Array of skeleton rows with URN, Type, Status populated
//   - error: Non-nil if state/plan cannot be merged
//
// Performance: O(n) where n is number of resources
// Memory: ~500 bytes per resource
func MergeResourcesForOverview(
    ctx context.Context,
    state *ingest.StackExport,
    plan *ingest.PulumiPlan,
) ([]OverviewRow, error)
```

**Test Coverage**: 95% (critical path)

---

### 2. EnrichOverviewRow

Enriches a single overview row with cost data from plugins.

```go
// EnrichOverviewRow fetches cost data for a single resource and populates
// ActualCost, ProjectedCost, and Recommendations fields.
//
// The function:
// 1. Calls plugin GetActualCost (if resource exists in state)
// 2. Calls plugin GetProjectedCost (if resource has pending changes)
// 3. Calls plugin GetRecommendations
// 4. Calculates CostDrift if both actual and projected exist
//
// Parameters:
//   - ctx: Context with timeout (30s default)
//   - row: Skeleton overview row to enrich
//   - plugins: Map of opened plugin clients
//   - dateRange: Time window for actual costs
//
// Returns:
//   - *OverviewRow: Enriched row (may have partial data if APIs fail)
//   - error: Non-nil only if catastrophic failure (not per-field)
//
// Performance: 1-3 seconds per resource (API latency)
// Concurrency: Safe to call from multiple goroutines
func EnrichOverviewRow(
    ctx context.Context,
    row *OverviewRow,
    plugins map[string]pluginhost.PluginClient,
    dateRange DateRange,
) (*OverviewRow, error)
```

**Error Handling**: Partial failures populate row.Error, don't fail entire operation.

---

### 3. CalculateCostDrift

Detects significant cost prediction discrepancies.

```go
// CalculateCostDrift compares actual MTD costs (extrapolated to full month)
// against projected monthly costs. Returns CostDriftData if drift exceeds 10%.
//
// Parameters:
//   - actualMTD: Month-to-date actual cost
//   - projected: Projected full-month cost
//   - dayOfMonth: Current day of month (for extrapolation)
//   - daysInMonth: Total days in current month
//
// Returns:
//   - *CostDriftData: Non-nil if drift > 10%
//   - error: Non-nil if inputs invalid or day < 3 (unreliable extrapolation)
//
// Formula: extrapolated = actualMTD * (daysInMonth / dayOfMonth)
//          drift% = ((extrapolated - projected) / projected) * 100
func CalculateCostDrift(
    actualMTD, projected float64,
    dayOfMonth, daysInMonth int,
) (*CostDriftData, error)
```

**Edge Cases**:
- Day 1-2: Return error "insufficient data"
- Deleted resources (projected=0): Return nil (no drift)
- New resources (actual=0): Return nil (no drift)

---

### 4. DetectPendingChanges

Determines if Pulumi preview has any changes.

```go
// DetectPendingChanges scans a Pulumi preview for create/update/delete/replace operations.
//
// Used for optimization: if no changes exist, skip projected cost queries (FR-008).
//
// Parameters:
//   - ctx: Context for logging
//   - plan: Pulumi preview JSON
//
// Returns:
//   - hasChanges: True if any change operations found
//   - changeCount: Number of resources with changes
//   - error: Non-nil if plan cannot be parsed
func DetectPendingChanges(
    ctx context.Context,
    plan *ingest.PulumiPlan,
) (hasChanges bool, changeCount int, err error)
```

---

### 5. CalculateProjectedDelta

Computes net change in monthly spend.

```go
// CalculateProjectedDelta calculates the total change in monthly cost
// if all pending infrastructure changes are applied.
//
// Logic:
//   - For updating resources: delta = projected - extrapolated_actual
//   - For creating resources: delta = +projected
//   - For deleting resources: delta = -extrapolated_actual
//
// Parameters:
//   - rows: Array of enriched overview rows
//   - currentDayOfMonth: For extrapolation of actual costs
//
// Returns:
//   - delta: Net monthly cost change (can be negative)
//   - currency: Currency code (assumes all same currency)
func CalculateProjectedDelta(
    rows []OverviewRow,
    currentDayOfMonth int,
) (delta float64, currency string)
```

---

## Supporting Types

### OverviewRow

See [../data-model.md](../data-model.md) for complete definition.

Key fields:
- `URN`: Resource identifier
- `Status`: ResourceStatus enum
- `ActualCost`: *ActualCostData
- `ProjectedCost`: *ProjectedCostData
- `Recommendations`: []Recommendation
- `CostDrift`: *CostDriftData
- `Error`: *OverviewRowError

---

## Concurrency Strategy

```go
// EnrichOverviewRows concurrently enriches multiple rows using goroutines.
//
// Pattern:
//   - Launch goroutine per resource (up to 10 concurrent per engine limit)
//   - Use sync.WaitGroup to track completion
//   - Send updates via channel for progressive loading
//
// Example:
func EnrichOverviewRows(
    ctx context.Context,
    rows []OverviewRow,
    plugins map[string]pluginhost.PluginClient,
    dateRange DateRange,
    progressChan chan<- OverviewRowUpdate,
) []OverviewRow {
    var wg sync.WaitGroup
    enriched := make([]OverviewRow, len(rows))
    sem := make(chan struct{}, 10) // Limit to 10 concurrent
    
    for i, row := range rows {
        wg.Add(1)
        go func(index int, r OverviewRow) {
            defer wg.Done()
            sem <- struct{}{} // Acquire
            defer func() { <-sem }() // Release
            
            enrichedRow, _ := EnrichOverviewRow(ctx, &r, plugins, dateRange)
            enriched[index] = *enrichedRow
            
            // Send progress update
            progressChan <- OverviewRowUpdate{Index: index, Row: *enrichedRow}
        }(i, row)
    }
    
    wg.Wait()
    close(progressChan)
    return enriched
}
```

---

## Error Propagation

**Philosophy**: Partial failures are acceptable. Don't fail entire command if one resource errors.

**Error Types**:
1. **Fatal**: State/preview cannot be loaded → fail command
2. **Per-Resource**: API call fails → populate row.Error, continue
3. **Transient**: Network timeout → retry once, then mark as error

**Error Logging**: All errors logged to audit trail with trace ID.

---

## Testing Requirements

### Unit Tests

- `MergeResourcesForOverview`: State + preview merging logic
- `CalculateCostDrift`: Edge cases (day 1, deleted resources, etc.)
- `DetectPendingChanges`: Empty plan, all operation types

### Integration Tests

- Full enrichment flow with mock plugins
- Concurrent enrichment with 100 resources
- Partial failure scenarios (50% API failures)

**Test Coverage Target**: 95% (critical path)

---

## Performance Benchmarks

```go
// benchmark_test.go
func BenchmarkMergeResourcesForOverview(b *testing.B) {
    // 100 resources: <5ms target
}

func BenchmarkCalculateCostDrift(b *testing.B) {
    // 1000 calculations: <10ms target
}

func BenchmarkEnrichOverviewRow(b *testing.B) {
    // 1 resource (with mock plugin): <10ms target
}
```

---

## References

- **Data Model**: `../data-model.md`
- **Existing Engine**: `internal/engine/projected.go`, `internal/engine/types.go`
- **Ingest Layer**: `internal/ingest/state.go`, `internal/ingest/pulumi_plan.go`

---

**Contract Version**: v1.0.0  
**Last Updated**: 2026-02-11
