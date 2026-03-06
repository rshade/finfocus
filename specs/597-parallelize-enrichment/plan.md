# Implementation Plan: Parallelize Per-Row Enrichment Sub-Calls

**Branch**: `597-parallelize-enrichment` | **Date**: 2026-02-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/597-parallelize-enrichment/spec.md`

## Summary

Refactor `EnrichOverviewRow` in `internal/engine/overview_enrich.go` to run the three
independent enrichment sub-calls (actual cost, projected cost, recommendations) concurrently
using goroutines and `sync.WaitGroup`. Resolve the `row.Error` race condition by changing
`enrichActualCost` and `enrichProjectedCost` to return `*OverviewRowError` instead of writing
to the shared field directly, with deterministic error merging (actual cost error takes
precedence) after all goroutines complete.

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: `sync` (stdlib), zerolog (logging)
**Storage**: N/A (no storage changes)
**Testing**: `go test -race`, testify (assert/require)
**Target Platform**: Linux, macOS, Windows (amd64, arm64)
**Project Type**: Single Go project (CLI tool)
**Performance Goals**: 40%+ wall-clock reduction per row when sub-call latency exceeds 10ms
**Constraints**: Max 30 concurrent goroutines (10 workers x 3 sub-calls); zero race detector warnings
**Scale/Scope**: 2 files modified (`overview_enrich.go`, `overview_enrich_test.go`)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in core engine. No plugin changes.
- [x] **Test-Driven Development**: Tests planned with race detector coverage. 80%+ minimum.
- [x] **Cross-Platform Compatibility**: Uses only Go stdlib concurrency (`sync.WaitGroup`, goroutines). Works on all platforms.
- [x] **Documentation Integrity**: Internal refactor with no public API changes. CLAUDE.md updated. No docs/ changes needed.
- [x] **Protocol Stability**: No protocol buffer changes. No gRPC interface changes.
- [x] **Implementation Completeness**: Full implementation planned. No stubs or TODOs.
- [x] **Quality Gates**: `make test` and `make lint` required before completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes. Only `finfocus-core` affected.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/597-parallelize-enrichment/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0: concurrency safety analysis
├── data-model.md        # Phase 1: field write isolation analysis
├── quickstart.md        # Phase 1: before/after overview
├── contracts/
│   └── enrichment-api.md  # Phase 1: function signature changes
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/engine/
├── overview_enrich.go       # Modified: parallelize EnrichOverviewRow,
│                            #   refactor enrichActualCost/enrichProjectedCost signatures
└── overview_enrich_test.go  # Modified: add race-condition tests,
                             #   error precedence tests, parallel correctness tests
```

**Structure Decision**: This is a focused refactor within the existing `internal/engine`
package. No new files or directories are created. Only `overview_enrich.go` and
`overview_enrich_test.go` are modified.

## Implementation Details

### Step 1: Refactor Enrichment Function Signatures

Change `enrichActualCost` and `enrichProjectedCost` to return `*OverviewRowError` instead
of writing to `row.Error` directly.

**enrichActualCost** (current → new):

- Current: writes `row.Error = classifyError(row.URN, err)` on line 76
- New: returns `classifyError(row.URN, err)` instead
- Still writes `row.ActualCost` directly (no conflict)

**enrichProjectedCost** (current → new):

- Current: checks `if row.Error == nil` then writes `row.Error` on lines 110-112
- New: returns `classifyError(row.URN, err)` unconditionally
- Still writes `row.ProjectedCost` directly (no conflict)

**enrichRecommendations**: No signature change. Writes only to `row.Recommendations`.

### Step 2: Parallelize EnrichOverviewRow

Replace the sequential calls with goroutines and `sync.WaitGroup`:

1. Create `sync.WaitGroup` and local error variables
2. Launch goroutines for each enrichment call (skip actual cost for `StatusCreating`)
3. Wait for all goroutines to complete
4. Merge errors: actual cost error takes precedence over projected cost error
5. Calculate cost drift (unchanged, runs after Wait)

### Step 3: Add Tests

New test cases for `overview_enrich_test.go`:

1. **Parallel correctness**: Verify all three fields are populated after parallel execution
2. **Error from actual cost only**: Verify `row.Error` captures actual cost error
3. **Error from projected cost only**: Verify `row.Error` captures projected cost error
4. **Both errors**: Verify actual cost error takes precedence
5. **Race detector**: All tests run with `-race` flag in CI (`make test-race`)
6. **StatusCreating skip**: Verify only 2 goroutines launched (no actual cost)

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Data race on OverviewRow fields | Low | High | Local error variables + distinct field writes + race detector tests |
| Goroutine leak | Low | Medium | `sync.WaitGroup` ensures completion before return |
| Increased goroutine count (10 workers x 3) | Low | Low | 30 goroutines is trivial for Go runtime |
| Behavioral regression | Low | High | FR-008 requires identical outputs; existing tests must pass |
