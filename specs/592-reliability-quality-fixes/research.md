# Research: Reliability & Quality Fixes Batch

**Feature**: 592-reliability-quality-fixes
**Date**: 2026-02-14

## R-001: Worker Pool Concurrency Pattern (#602, #652)

**Decision**: Use Go's standard `runtime.NumCPU()` with optional CLI override via `--jobs` flag.
For overview enrichment, replace goroutine-per-row with fixed worker pool reading from a channel.

**Rationale**: The engine already uses `2 * runtime.NumCPU()` as default with
`FINFOCUS_CONCURRENCY_MULTIPLIER` env var override. Adding `--jobs` provides direct CLI
control without env var friction. Worker pool pattern is standard Go concurrency best practice
for bounded parallelism.

**Alternatives considered**:

- Functional options pattern for engine configuration: Rejected as over-engineered for a
  single parameter; direct field or method parameter is simpler
- `errgroup.Group` with `SetLimit()`: Considered but `overviewConcurrencyLimit` is already
  defined as a constant; a simple channel+WaitGroup is more explicit

## R-002: Cache Expired Entry Deletion Strategy (#653)

**Decision**: Synchronous deletion after releasing RLock, then acquiring write Lock inline.

**Rationale**: The current `go func() { s.mu.Lock() }()` pattern inside an RLock section
creates potential deadlock: if many readers hold RLock simultaneously, the goroutine's Lock
request blocks, and subsequent RLock requests also block (Go's RWMutex is writer-preferring).
Synchronous deletion is safe because `os.Remove()` is fast (microseconds on local filesystem)
and the caller already gets `ErrCacheExpired`.

**Alternatives considered**:

- Background janitor goroutine: Over-engineered for this use case; adds lifecycle management
  complexity for minimal benefit
- `sync.Once` per entry: Doesn't solve the RLock/Lock ordering issue
- Lazy deletion on next write: Leaves stale files on disk

## R-003: HTTP Context Propagation Pattern (#654)

**Decision**: Add `context.Context` as first parameter to all `GitHubClient` public methods.
Use `http.NewRequestWithContext(ctx, ...)` for all HTTP requests.

**Rationale**: Standard Go convention (`context.Context` as first parameter). Enables
cancellation via Cobra's `cmd.Context()` which is wired to OS signal handling. Removes
`//nolint:noctx` directives.

**Alternatives considered**:

- Store context in GitHubClient struct: Anti-pattern in Go; contexts should not be stored
- Use `http.Client.Timeout`: Only handles total timeout, not cancellation on Ctrl+C

## R-004: Stdio Proxy Shutdown Coordination (#656)

**Decision**: Use `sync.WaitGroup` for both io.Copy directions. On completion of either
direction, close the opposite stream to unblock the paired io.Copy.

**Rationale**: `io.Copy` blocks indefinitely until EOF or error. Without explicit stream
closure, the goroutine for the completed direction would leak. WaitGroup ensures proxy()
only returns when both directions finish.

**Alternatives considered**:

- Deadline-based shutdown: `net.Conn` supports `SetDeadline()` but stdin/stdout pipes don't
- Context-based cancellation: `io.Copy` doesn't accept context; would need custom wrapper
- Single-goroutine bidirectional copy: Not possible with blocking io.Copy

## R-005: Fuzz Test Failure Handling (#655)

**Decision**: Remove `|| true` from fuzz test steps. Rely on `if: always()` for artifact
upload steps to preserve corpus regardless of test outcome.

**Rationale**: `|| true` was added as a workaround but masks real failures. Go's fuzz
framework distinguishes between timeout (normal, exit 0) and crash (failure, non-zero exit).
With proper `--fuzztime` limits, timeout is the expected outcome; crashes should fail the
workflow.

**Alternatives considered**:

- Capture exit code and report separately: Adds complexity without benefit; Go fuzz already
  handles exit codes correctly
- Separate pass/fail steps: Unnecessary; GitHub Actions natively reports step failures

## R-006: Test Isolation Pattern (#605)

**Decision**: Use `os.Getwd()` + `t.TempDir()` + `os.Chdir()` + `t.Cleanup()` pattern.

**Rationale**: Standard Go test isolation pattern. `t.TempDir()` auto-cleans, `t.Cleanup()`
ensures working directory is restored even on test failure. The two affected tests
(`TestCostProjectedWithoutPulumiJson`, `TestStackFlagPassedThrough`) rely on auto-detection
not finding Pulumi files, which fails if run from a directory containing `Pulumi.yaml`.

**Alternatives considered**:

- Environment variable to disable auto-detection: Adds production code for test-only concern
- Mock filesystem: Over-engineered; simple chdir is sufficient

## R-007: DRY Helper Consolidation (#610)

**Decision**: Export `CountRecommendations` and `FormatRecommendationCount` from
`internal/engine/project.go`. Remove `formatRecsColumn` from `internal/tui/cost_view.go`.
Replace inline counting in TUI with `engine.CountRecommendations()`.

**Rationale**: Both functions are identical. Engine package is the natural home since
`CostResult` type lives there. Exporting (capitalizing) allows cross-package usage without
introducing a new shared package.

**Alternatives considered**:

- New `internal/shared/` package: Over-engineered for two small helpers
- Move to `internal/tui/`: Wrong direction; engine is the source-of-truth for cost data
