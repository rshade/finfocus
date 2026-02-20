# Feature Specification: Parallelize Per-Row Enrichment Sub-Calls

**Feature Branch**: `597-parallelize-enrichment`
**Created**: 2026-02-18
**Status**: Draft
**Input**: GitHub Issue #694 - perf(engine): parallelize per-row enrichment sub-calls

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Faster Overview Enrichment for Large Stacks (Priority: P1)

As a FinFocus user running `cost overview` against a large Pulumi stack (100+ resources), I want each resource's actual cost, projected cost, and recommendation enrichment calls to execute in parallel so that the overview command completes faster.

**Why this priority**: This is the core value proposition. Sequential per-row enrichment becomes a measurable bottleneck as stack size grows and plugin latency increases. Running independent gRPC calls concurrently directly reduces wall-clock time for the most common use case.

**Independent Test**: Can be fully tested by running `EnrichOverviewRow` against a resource with all three data sources available and measuring that wall-clock time decreases when sub-calls overlap.

**Acceptance Scenarios**:

1. **Given** a stack with resources that have actual costs, projected costs, and recommendations available, **When** `EnrichOverviewRow` is called, **Then** actual cost, projected cost, and recommendation data are all populated in the resulting `OverviewRow` exactly as they were when fetched sequentially.
2. **Given** a stack with 100 resources where each plugin call takes ~50ms, **When** overview enrichment runs, **Then** per-row enrichment wall-clock time is measurably reduced compared to the sum of individual call durations.
3. **Given** a resource with `StatusCreating`, **When** enrichment runs, **Then** actual cost fetch is skipped (not dispatched as a goroutine) while projected cost and recommendations still run concurrently.

---

### User Story 2 - Thread-Safe Error Handling Under Concurrency (Priority: P1)

As a FinFocus user, I want the parallel enrichment to correctly capture errors from any of the concurrent calls without data races, so that partial failures are reported accurately.

**Why this priority**: Correctness is non-negotiable. The current sequential code has a shared `row.Error` field written by both `enrichActualCost` and `enrichProjectedCost`. Running these concurrently introduces a race condition that must be resolved to avoid undefined behavior.

**Independent Test**: Can be fully tested by triggering error conditions in actual cost and projected cost fetches concurrently and verifying the resulting `OverviewRow.Error` is deterministic and race-free.

**Acceptance Scenarios**:

1. **Given** a resource where the actual cost fetch fails with a network error, **When** enrichment runs concurrently, **Then** `row.Error` captures the actual cost error and projected cost data (if successful) is still populated.
2. **Given** a resource where the projected cost fetch fails but actual cost succeeds, **When** enrichment runs concurrently, **Then** `row.Error` captures the projected cost error and actual cost data is still populated.
3. **Given** a resource where both actual and projected cost fetches fail, **When** enrichment runs concurrently, **Then** `row.Error` captures the actual cost error (preferred over projected) and the result is deterministic across multiple runs.
4. **Given** concurrent enrichment execution, **When** tests run with the `-race` flag, **Then** no data race warnings are reported on any `OverviewRow` field.

---

### User Story 3 - Cost Drift Calculation After Parallel Completion (Priority: P2)

As a FinFocus user, I want cost drift to be correctly calculated after both actual and projected costs have been fetched in parallel, so that drift data remains accurate.

**Why this priority**: Cost drift depends on results from both parallel paths. This is a correctness requirement that follows naturally from the parallelization but must be explicitly verified.

**Independent Test**: Can be fully tested by providing a resource with both actual and projected costs available, running parallel enrichment, and verifying that cost drift is calculated with the correct values.

**Acceptance Scenarios**:

1. **Given** a resource where both actual cost ($150 MTD) and projected cost ($200/month) are available, **When** enrichment completes in parallel, **Then** cost drift is calculated using both values exactly as it was in the sequential implementation.
2. **Given** a resource where actual cost succeeds but projected cost fails, **When** enrichment completes, **Then** cost drift is not calculated (requires both values).

---

### Edge Cases

- What happens when the context is cancelled mid-enrichment? All goroutines must respect context cancellation and not leak.
- What happens when one enrichment call hangs while others complete quickly? The wait mechanism must still block until all goroutines finish before proceeding to cost drift.
- What happens when `EnrichOverviewRows` dispatches multiple rows to the worker pool, each of which now spawns internal goroutines? The existing concurrency limit (10 workers) bounds row-level parallelism; sub-call parallelism within each row adds at most 3 goroutines per row.
- What happens when all three enrichment calls fail? The error from the actual cost call takes precedence, and the row contains no cost data and no drift.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST execute actual cost and projected cost enrichment calls concurrently within `EnrichOverviewRow`.
- **FR-002**: System MUST execute recommendation enrichment concurrently with actual and projected cost enrichment (all three in parallel).
- **FR-003**: System MUST eliminate the shared `row.Error` race condition by using local error variables in each goroutine and merging errors after all goroutines complete.
- **FR-004**: System MUST prefer the actual cost error over the projected cost error when both fail, since actual cost is the primary data source for the overview.
- **FR-005**: System MUST skip the actual cost goroutine for resources with `StatusCreating` (preserving existing behavior).
- **FR-006**: System MUST calculate cost drift only after all concurrent enrichment calls have completed.
- **FR-007**: System MUST pass the Go race detector (`-race` flag) with zero warnings when running enrichment tests.
- **FR-008**: System MUST produce identical `OverviewRow` field values compared to the current sequential implementation for the same inputs (behavioral equivalence).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Per-row enrichment wall-clock time is reduced by at least 40% when individual sub-call latency exceeds 10ms, verified via benchmark or instrumented test.
- **SC-002**: All existing overview enrichment tests pass without modification to expected outputs.
- **SC-003**: Race detector tests pass with zero data race warnings on the enrichment code path.
- **SC-004**: No goroutine leaks detected: all spawned goroutines complete before `EnrichOverviewRow` returns.
- **SC-005**: Error precedence is deterministic: when both actual and projected cost fail, the actual cost error is always captured in `row.Error`.

## Assumptions

- The three enrichment functions (`enrichActualCost`, `enrichProjectedCost`, `enrichRecommendations`) are safe to call concurrently on the same Engine instance. The engine's plugin clients support concurrent gRPC calls (standard for gRPC).
- The `OverviewRow` fields written by each enrichment function (`ActualCost`, `ProjectedCost`, `Recommendations`) are distinct pointer/slice fields. Writing to different fields of the same struct from different goroutines is safe in Go when no two goroutines write to the same field.
- The existing worker pool model (`EnrichOverviewRows` with `overviewConcurrencyLimit = 10`) is unchanged. This feature adds sub-call parallelism within each worker's `EnrichOverviewRow` call.
- The `enrichActualCost` and `enrichProjectedCost` functions will be refactored to return error information to the caller rather than writing directly to `row.Error`, enabling deterministic error merging.
