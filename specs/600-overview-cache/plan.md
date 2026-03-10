# Implementation Plan: Overview Cost Caching

**Branch**: `600-overview-cache` | **Date**: 2026-02-22 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/600-overview-cache/spec.md`

## Summary

Wire the existing cache infrastructure (`newEngineWithCache`) into both engine
construction sites of the overview command (plain-text and TUI paths), enabling
opt-in BoltDB caching with consistent `--cache-ttl` behavior. No new cache code
is needed; this is a wiring-only change.

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: `github.com/charmbracelet/bubbletea`, `go.etcd.io/bbolt` (via existing cache infrastructure)
**Storage**: BoltDB cache (existing `internal/engine/cache/store.go`)
**Testing**: `go test` with `testify/assert` and `testify/require`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module
**Performance Goals**: Second overview run >=50% faster when cache TTL is active
**Constraints**: Zero behavioral change when caching is disabled (TTL=0, the default)
**Scale/Scope**: 2 engine construction sites, ~20 lines of production code changed

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration-layer wiring, not a new provider integration. Caching sits between CLI and engine, no plugin changes needed.
- [x] **Test-Driven Development**: Tests planned for both paths (plain-text and TUI). Coverage target: 80%+ for modified code.
- [x] **Cross-Platform Compatibility**: No platform-specific code introduced. BoltDB is already cross-platform.
- [x] **Documentation Integrity**: CLAUDE.md engine construction table will be updated to show overview uses cache.
- [x] **Protocol Stability**: No protocol buffer changes. Cache key format unchanged.
- [x] **Implementation Completeness**: Full implementation with no stubs or TODOs. Both execution paths covered.
- [x] **Quality Gates**: `make lint` and `make test` required before completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. Uses existing `finfocus-spec` SDK.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/600-overview-cache/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (minimal - no new data model)
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (files modified)

```text
internal/cli/
├── overview.go              # Two engine construction sites updated
└── common_execution.go      # No changes needed (reuse as-is)

internal/engine/cache/
└── (no changes)             # Existing cache infrastructure reused
```

**Structure Decision**: No new files or directories. This is a wiring change to
two existing engine construction sites in `overview.go`, using the existing
`newEngineWithCache()` helper from `common_execution.go`.

## Research (Phase 0)

### R1: How do other cost commands wire caching?

**Decision**: Use the `newEngineWithCache()` helper from `common_execution.go`.

**Rationale**: All three cost commands (`cost projected`, `cost actual`,
`cost recommendations`) use the identical pattern:

```go
eng, cacheCleanup := newEngineWithCache(ctx, cmd, clients, loader, cfg)
defer cacheCleanup()
```

This helper:

1. Creates `config.New()` internally if no config passed
2. Constructs `engine.New(clients, loader).WithRouter(createRouterForEngine(...))`
3. Calls `initCacheFromConfig(ctx, cmd, cfg)` to resolve TTL and create BoltDB store
4. Attaches cache via `eng.WithCache(cacheStore)` if non-nil
5. Returns a cleanup function that closes the BoltDB file

**Alternatives Considered**:

- Calling `InitCache()` separately and attaching manually: Rejected because
  `newEngineWithCache()` already encapsulates the full pattern including router wiring.
- Creating a new helper specific to overview: Rejected because the existing helper
  is general-purpose and already used by 3 commands.

### R2: How should TUI cache cleanup work?

**Decision**: Use `defer cacheCleanup()` inside the `overviewInitAndEnrich`
goroutine, keeping plugin cleanup via `cleanupChan` unchanged.

**Rationale**: The goroutine always returns after enrichment completes (or on
context cancellation/error). The `defer` fires at the right time - after all
engine operations are done. The existing plugin cleanup via `cleanupChan` remains
unchanged, keeping the delta minimal.

**Alternatives Considered**:

- Composing cache+plugin cleanup into a single function sent via `cleanupChan`:
  Would require moving the `cleanupChan <- cleanup` send to after engine creation,
  risking plugin cleanup loss if an error occurs between openPlugins and
  newEngineWithCache. More complex for no benefit.
- Adding a second cleanup channel for cache: Over-engineered for a simple `defer`.

### R3: How to pass `cmd` to the TUI goroutine?

**Decision**: Add `cmd *cobra.Command` as a parameter to `overviewInitAndEnrich`
and rename `_ *cobra.Command` in `runInteractiveOverviewWithInit`.

**Rationale**: `newEngineWithCache` needs `cmd` to check whether `--cache-ttl`
was explicitly set (via `cmd.Flags().Lookup("cache-ttl").Changed`). The `cmd`
parameter is already available in `runInteractiveOverviewWithInit` but currently
discarded via `_`.

**Alternatives Considered**:

- Reading `--cache-ttl` value before the goroutine and passing it in `params`:
  Would require modifying `overviewParams` struct and duplicating TTL resolution
  logic. The helper already handles this.

## Design (Phase 1)

### Data Model

No new data model. The existing `cache.BoltStore` and `cache.Cache` interface are
reused without modification. Cache key format for projected costs is unchanged:
`projected/{provider}/{type}/{region}/{sku}`.

### Contracts

No new APIs or contracts. The feature uses the existing `--cache-ttl` flag
(persistent on root command, line 123 of `root.go`) and the existing
`newEngineWithCache()` helper.

### Implementation Details

#### Change 1: Plain-text path (`executeOverview`, line ~197-202)

**Before**:

```go
// 9. Create engine
pt = logging.StartPhase(ctx, "cli", "overview", "engine_create")
cfg := config.New()
eng := engine.New(clients, nil).
    WithRouter(createRouterForEngine(ctx, cfg, clients))
pt.Done(ctx)
```

**After**:

```go
// 9. Create engine (with optional cache)
pt = logging.StartPhase(ctx, "cli", "overview", "engine_create")
eng, cacheCleanup := newEngineWithCache(ctx, cmd, clients, nil)
defer cacheCleanup()
pt.Done(ctx)
```

Key observations:

- `newEngineWithCache` creates `config.New()` internally, so the local `cfg` is
  removed (it was only used for router creation, which the helper handles).
- `loader` is `nil` (overview never uses local YAML specs).
- `cacheCleanup` is deferred immediately after engine creation, matching the
  pattern in other cost commands.

#### Change 2: TUI path - function signature (`overviewInitAndEnrich`)

**Before**:

```go
func overviewInitAndEnrich(
    enrichCtx context.Context,
    p *tea.Program,
    params overviewParams,
    dateRange engine.DateRange,
    audit *auditContext,
    cleanupChan chan<- func(),
    rowCount *atomic.Int64,
    passphraseChan chan string,
) {
```

**After**:

```go
func overviewInitAndEnrich(
    enrichCtx context.Context,
    cmd *cobra.Command,
    p *tea.Program,
    params overviewParams,
    dateRange engine.DateRange,
    audit *auditContext,
    cleanupChan chan<- func(),
    rowCount *atomic.Int64,
    passphraseChan chan string,
) {
```

#### Change 3: TUI path - engine construction (line ~915-919)

**Before**:

```go
// Phase 5: Create engine.
p.Send(tui.OverviewPhaseMsg{Index: phasePrepareEngine, Phase: "Preparing cost engine..."})
cfg := config.New()
eng := engine.New(clients, nil).
    WithRouter(createRouterForEngine(enrichCtx, cfg, clients))
```

**After**:

```go
// Phase 5: Create engine (with optional cache).
p.Send(tui.OverviewPhaseMsg{Index: phasePrepareEngine, Phase: "Preparing cost engine..."})
eng, cacheCleanup := newEngineWithCache(enrichCtx, cmd, clients, nil)
defer cacheCleanup()
```

#### Change 4: Call site update (`runInteractiveOverviewWithInit`)

**Before**:

```go
func runInteractiveOverviewWithInit(
    ctx context.Context,
    _ *cobra.Command,
    ...
) error {
    ...
    go overviewInitAndEnrich(enrichCtx, p, params, dateRange, audit, cleanupChan, &rowCount, passphraseChan)
```

**After**:

```go
func runInteractiveOverviewWithInit(
    ctx context.Context,
    cmd *cobra.Command,
    ...
) error {
    ...
    go overviewInitAndEnrich(enrichCtx, cmd, p, params, dateRange, audit, cleanupChan, &rowCount, passphraseChan)
```

### Test Strategy

1. **Unit tests**: Verify that `executeOverview` and the TUI path construct cached
   engines when `--cache-ttl` is set. Since these are integration-heavy functions,
   focus on verifiable behavior: second run with same stack uses cache.

2. **Existing tests**: All existing overview tests must pass unchanged (caching is
   disabled by default with TTL=0).

3. **Manual verification**: Run `finfocus overview --cache-ttl 300` twice against
   a real Pulumi stack and verify `(cached)` appears in the adapter field on the
   second run.

## Complexity Tracking

No violations - no tracking needed.
