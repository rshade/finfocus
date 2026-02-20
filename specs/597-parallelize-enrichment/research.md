# Research: Parallelize Per-Row Enrichment Sub-Calls

**Date**: 2026-02-18
**Feature**: 597-parallelize-enrichment

## R1: Engine Thread-Safety for Concurrent Sub-Calls

**Decision**: All three engine methods (`GetProjectedCostWithErrors`, `GetActualCostWithOptionsAndErrors`,
`GetRecommendationsForResources`) are safe to call concurrently on the same `*Engine` instance.

**Rationale**:

- The `Engine` struct has write-once fields set during construction (`clients`, `loader`,
  `cache`, `router`, `dismissalStore`, `jobs`). No fields are mutated after initialization.
- Each engine method creates local channels, wait groups, and result accumulators with no
  shared mutable state across calls.
- The cache interface (`cache.Cache`) provides internal synchronization (BoltDB `View()`
  for reads, `Batch()` for writes).
- The plugin client slice `e.clients` is read-only during operation.

**Alternatives considered**:

- **Wrap Engine in a mutex**: Unnecessary overhead since the Engine is already safe for
  concurrent reads. Would serialize calls and negate the parallelization benefit.
- **Clone Engine per goroutine**: Wasteful; the shared state is all read-only.

## R2: `row.Error` Race Condition Resolution

**Decision**: Refactor `enrichActualCost` and `enrichProjectedCost` to return `*OverviewRowError` instead
of writing directly to `row.Error`. The caller (`EnrichOverviewRow`) merges errors after
`wg.Wait()` with actual cost error taking precedence.

**Rationale**:

- The current code has `enrichProjectedCost` checking `if row.Error == nil` before writing
  (line 110). This is a classic read-then-write race when running concurrently.
- Using local error variables eliminates the race entirely without needing synchronization
  primitives (no mutex, no `sync.Once`).
- Error precedence (actual > projected) is cleanly expressed in the merge logic.

**Alternatives considered**:

- **`sync.Once` for first error**: Only captures the first error, losing the second one
  entirely. Less informative than choosing by precedence.
- **`sync.Mutex` on row.Error**: Adds unnecessary locking overhead for a simple merge
  that can be done after `wg.Wait()`.
- **Accept race as benign (last-writer-wins)**: Violates Go race detector and makes
  behavior non-deterministic. Rejected.

## R3: Parallelization Strategy (All Three vs Two Plus Sequential)

**Decision**: Run all three enrichment calls in parallel (actual + projected + recommendations).

**Rationale**:

- `enrichRecommendations` writes to `row.Recommendations` exclusively and never touches
  `row.Error`, so there is no field conflict with the other two goroutines.
- Adding one more goroutine (3 vs 2) per row is negligible overhead: max 30 goroutines
  total (10 workers x 3 sub-calls) is well within Go's goroutine capacity.
- Maximum parallelism yields the best wall-clock improvement at scale.

**Alternatives considered**:

- **Two in parallel (actual + projected), recommendations sequential**: Simpler but
  leaves 33% of potential savings on the table. The issue itself described this as a
  conservative option.
- **Sequential (status quo)**: Rejected; the entire purpose of the feature is to
  parallelize.

## R4: Enrichment Function Signature Refactoring

**Decision**: Change `enrichActualCost` and `enrichProjectedCost` to return `*OverviewRowError`
instead of writing to `row.Error` directly. They continue to write to their respective
cost data fields (`row.ActualCost`, `row.ProjectedCost`) since those are distinct fields
with no concurrent access.

**Rationale**:

- Only `row.Error` is the contested field. Actual and projected cost data fields are
  written by different goroutines with no overlap.
- Returning the error allows the caller to apply deterministic merge logic.
- `enrichRecommendations` does not need a signature change since it never writes to
  `row.Error`.

**Alternatives considered**:

- **Return full result structs**: Over-engineered for this change. The functions already
  write directly to distinct row fields; only the error needs extraction.
- **Use channels for error passing**: Unnecessary complexity when local variables and
  `wg.Wait()` suffice.

## R5: Goroutine Lifecycle and Leak Prevention

**Decision**: Use `sync.WaitGroup` to ensure all goroutines complete before `EnrichOverviewRow` returns.
Cost drift calculation runs after `wg.Wait()`.

**Rationale**:

- `sync.WaitGroup` is the standard Go pattern for waiting on a bounded number of goroutines.
- The function must not return before all goroutines finish writing to the row, otherwise
  the caller (enrichWorker) would copy an incomplete row to progressChan.
- Context cancellation is already handled by the engine methods themselves; the goroutines
  will return quickly when the context is done.

**Alternatives considered**:

- **`errgroup.Group`**: Provides automatic error propagation but the enrichment functions
  never fail (they capture errors in the row). The `errgroup` semantics (cancel on first
  error) would be counterproductive since we want all calls to complete.
