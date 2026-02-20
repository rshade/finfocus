# Data Model: Parallelize Per-Row Enrichment Sub-Calls

**Date**: 2026-02-18
**Feature**: 597-parallelize-enrichment

## Entities

This feature does not introduce new data entities. It modifies the internal control flow
of `EnrichOverviewRow` while preserving the existing data model.

### Modified Entity: OverviewRow (unchanged schema)

The `OverviewRow` struct is unchanged. The parallelization affects **how** fields are
populated, not **what** fields exist.

| Field           | Type               | Writer (current)       | Writer (parallel)     | Conflict Risk |
|-----------------|--------------------|------------------------|-----------------------|---------------|
| ActualCost      | *ActualCostData    | enrichActualCost       | goroutine 1           | None          |
| ProjectedCost   | *ProjectedCostData | enrichProjectedCost    | goroutine 2           | None          |
| Recommendations | []Recommendation   | enrichRecommendations  | goroutine 3           | None          |
| CostDrift       | *CostDriftData     | enrichCostDrift        | main (after Wait)     | None          |
| Error           | *OverviewRowError  | enrichActualCost, enrichProjectedCost | main (after Wait) | **Resolved** |

### Field Write Isolation

- **goroutine 1** (enrichActualCost): writes `row.ActualCost`, returns `*OverviewRowError`
- **goroutine 2** (enrichProjectedCost): writes `row.ProjectedCost`, returns `*OverviewRowError`
- **goroutine 3** (enrichRecommendations): writes `row.Recommendations` only
- **main thread** (post-Wait): writes `row.Error` (merged), `row.CostDrift`

## State Transitions

No new state transitions. The `ResourceStatus` enum and its effect on enrichment
behavior (skip actual cost for `StatusCreating`) is unchanged.

## Validation Rules

No new validation rules. All existing `OverviewRow.Validate()` rules remain as-is.
