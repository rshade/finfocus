# Implementation Plan: Reliability & Quality Fixes Batch

**Branch**: `592-reliability-quality-fixes` | **Date**: 2026-02-14 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/592-reliability-quality-fixes/spec.md`

## Summary

Batch of 8 targeted fixes addressing concurrency control, resource lifecycle management,
code deduplication, test isolation, and CI reliability across the finfocus codebase. The
changes span CLI flags (#602), engine internals (#652, #653), plugin host (#656), registry
HTTP handling (#654), test infrastructure (#605), code quality (#610), and CI workflows (#655).

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: Cobra v1.10.2 (CLI), gRPC v1.79.1 (plugins), zerolog v1.34.0 (logging), testify v1.11.1 (testing)
**Storage**: Local filesystem (cache files at `~/.finfocus/cache/`), no database
**Testing**: `go test` with testify assertions, `make test` / `make test-race`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application with plugin architecture
**Performance Goals**: Bounded goroutine count, clean shutdown within 5 seconds
**Constraints**: No breaking changes to public API, no new dependencies
**Scale/Scope**: 8 tickets, ~12 files modified, ~400 lines changed

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: No plugin changes; all fixes are in core orchestration/infrastructure
- [x] **Test-Driven Development**: Tests planned for each change; 80%+ coverage maintained
- [x] **Cross-Platform Compatibility**: No platform-specific code introduced; existing cross-platform patterns preserved
- [x] **Documentation Integrity**: CLAUDE.md will be updated to reflect new `--jobs` flag and timing output
- [x] **Protocol Stability**: No protocol buffer changes; no cross-repo coordination needed
- [x] **Implementation Completeness**: All 8 fixes are complete implementations; no stubs or TODOs
- [x] **Quality Gates**: `make test` and `make lint` must pass before completion
- [x] **Multi-Repo Coordination**: All changes are within finfocus-core; no cross-repo dependencies

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/592-reliability-quality-fixes/
├── plan.md              # This file
├── research.md          # Phase 0: Implementation research
├── quickstart.md        # Phase 1: Verification guide
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (files to modify)

```text
internal/
├── cli/
│   ├── cost_projected.go          # #602: Add --jobs flag, timing output
│   ├── cost_actual.go             # #602: Add --jobs flag, timing output
│   └── cost_projected_test.go     # #605: Isolate auto-detection tests
├── engine/
│   ├── engine.go                  # #602: Accept jobs option in worker pool
│   ├── overview_enrich.go         # #652: Refactor to worker-pool model
│   ├── project.go                 # #610: Export CountRecommendations, FormatRecommendationCount
│   ├── project_test.go            # #610: Update test for exported names
│   └── cache/
│       └── store.go               # #653: Remove goroutine from expired entry cleanup
├── pluginhost/
│   └── stdio.go                   # #656: Add WaitGroup to proxy, deterministic shutdown
├── registry/
│   └── github.go                  # #654: Thread context through HTTP requests
└── tui/
    └── cost_view.go               # #610: Replace duplicates with engine.CountRecommendations

.github/workflows/
└── nightly.yml                    # #655: Remove || true from fuzz test steps
```

**Structure Decision**: All changes modify existing files within the established Go package
structure. No new packages or files are needed. The project already uses the standard Go
layout with `internal/` packages.

## Implementation Details by Ticket

### #602: --jobs Flag and Timing Output

**Approach**: Add `--jobs`/`-j` int flag to `costProjectedParams` and `costActualParams`
structs. Pass the value to a new `WithJobs(n int)` engine option. Engine's `getWorkerCount()`
already exists and uses `2 * runtime.NumCPU()` with env var override; add a direct override
when jobs > 0. Timing wraps the engine call with `time.Now()` / `time.Since()` and prints
to stderr for table format only.

**Files**: `internal/cli/cost_projected.go`, `internal/cli/cost_actual.go`, `internal/engine/engine.go`

**Key decisions**:

- Flag type: `int`, default `0` (auto mode)
- Validation: reject negative values, cap at resource count
- Timing output to stderr via `cmd.ErrOrStderr().Write()`
- Only show timing for `table` output format (check `outputFormat` before printing)

### #605: Test Isolation with Temp Directories

**Approach**: Add `os.Getwd()` + `t.TempDir()` + `os.Chdir()` pattern with `t.Cleanup()`
to restore original directory. Applied to `TestCostProjectedWithoutPulumiJson` and
`TestStackFlagPassedThrough`.

**Files**: `internal/cli/cost_projected_test.go`

### #610: DRY Recommendation Helpers

**Approach**: Export `countRecommendations` as `CountRecommendations` and
`formatRecommendationCount` as `FormatRecommendationCount` in `internal/engine/project.go`.
Update TUI package to import and use `engine.CountRecommendations()` and
`engine.FormatRecommendationCount()`. Remove `formatRecsColumn` from TUI.

**Files**: `internal/engine/project.go`, `internal/engine/project_test.go`, `internal/tui/cost_view.go`

### #652: Overview Enrichment Worker Pool

**Approach**: Replace goroutine-per-row + semaphore pattern with a fixed worker pool.
Start `overviewConcurrencyLimit` workers that read row indices from a channel. Workers
call `EnrichOverviewRow()` and send progress updates. This limits goroutine creation to
exactly `overviewConcurrencyLimit` + 1 (main goroutine), regardless of row count.

**Files**: `internal/engine/overview_enrich.go`

**Pattern**:

```text
1. Create jobs channel (buffered to row count)
2. Start overviewConcurrencyLimit worker goroutines
3. Send all row indices to jobs channel, then close it
4. Workers range over jobs, calling EnrichOverviewRow() for each
5. WaitGroup.Wait() then close progress channel
```

### #653: Cache Expired Entry Cleanup

**Approach**: Replace `go func() { s.mu.Lock(); os.Remove(...) }()` with inline deletion.
The current code holds RLock when spawning the goroutine that needs a write Lock, which is
a potential deadlock. Fix: release RLock, acquire write Lock, delete file, release Lock,
then return the error. Since Get() returns `ErrCacheExpired` for expired entries, the caller
doesn't use the result, so the synchronous deletion overhead is negligible.

**Files**: `internal/engine/cache/store.go`

### #654: Context-Cancelable HTTP Requests

**Approach**: Thread `context.Context` through all `GitHubClient` public methods. Replace
all `http.NewRequest()` with `http.NewRequestWithContext(ctx, ...)`. Remove `//nolint:noctx`
directives. Update callers in CLI plugin commands to pass `cmd.Context()`.

**Files**: `internal/registry/github.go`, plus callers in `internal/cli/plugin_install.go`,
`internal/cli/plugin_update.go`, and any other registry consumers.

**Methods requiring context parameter addition**:

- `GetLatestRelease(ctx, owner, repo)`
- `GetReleaseByTag(ctx, owner, repo, tag)`
- `ListStableReleases(ctx, owner, repo, limit)`
- `FindReleaseWithAsset(ctx, owner, repo, assetPattern)`
- `FindReleaseWithFallbackInfo(ctx, owner, repo, binaryName)`
- `DownloadAsset(ctx, url, destPath, progress)`
- `fetchRelease(ctx, url)` (internal)

### #655: Remove Fuzz Test Masking

**Approach**: Remove `|| true` from all 4 fuzz test steps. Keep `if: always()` on the
corpus upload/cache steps so artifacts are preserved regardless of test outcome.

**Files**: `.github/workflows/nightly.yml`

### #656: Stdio Proxy Goroutine Lifecycle

**Approach**: Add `sync.WaitGroup` to coordinate both `io.Copy` directions in `proxy()`.
When either direction completes, close the opposite stream to unblock the other `io.Copy`.
Wait for both to complete before returning.

**Files**: `internal/pluginhost/stdio.go`

**Pattern**:

```text
1. Accept connection
2. var wg sync.WaitGroup; wg.Add(2)
3. Goroutine 1: io.Copy(stdin, conn) → on return, close conn read side
4. Goroutine 2: io.Copy(conn, stdout) → on return, close conn write side
5. wg.Wait()
6. Clean up
```

## Implementation Order

Dependencies between tickets are minimal. Recommended order for clean commits:

1. **#655** (CI fix) - Zero code risk, immediate value
2. **#605** (test isolation) - Low risk, improves test reliability for subsequent changes
3. **#610** (DRY helpers) - Low risk refactor, no behavior change
4. **#653** (cache fix) - Isolated fix, no dependents
5. **#656** (stdio proxy) - Isolated fix in pluginhost
6. **#654** (context HTTP) - Medium scope, cascading signature changes
7. **#652** (overview worker pool) - Medium scope, concurrency refactor
8. **#602** (--jobs flag) - Largest change, builds on engine understanding

## Complexity Tracking

No constitution violations. No complexity justifications needed.
