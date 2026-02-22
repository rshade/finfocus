# Research: Overview Cost Caching

**Feature**: 600-overview-cache
**Date**: 2026-02-22

## R1: Cache Wiring Pattern in Existing Commands

### R1 Question

How do `cost projected`, `cost actual`, and `cost recommendations` wire caching?

### R1 Findings

All three commands use the identical `newEngineWithCache()` helper from
`internal/cli/common_execution.go:389-419`:

| Command | File:Line | Loader | Config |
|---|---|---|---|
| `cost projected` | `cost_projected.go:212` | `spec.NewLoader(specDir)` | `cfg` |
| `cost actual` | `cost_actual.go:215` | `nil` | (default) |
| `cost recommendations` | `cost_recommendations.go:192` | `nil` | (default) |

Pattern:

```go
eng, cacheCleanup := newEngineWithCache(ctx, cmd, clients, loader, cfg)
defer cacheCleanup()
```

The helper internally:

1. Creates `config.New()` if no config supplied
2. Constructs engine with router via `createRouterForEngine()`
3. Resolves cache TTL: flag > env > config > default (0)
4. Creates BoltDB store if TTL > 0
5. Attaches cache via `eng.WithCache()`
6. Returns no-op cleanup if cache was nil

### R1 Decision

Reuse `newEngineWithCache()` for both overview paths. Pass `nil` as loader
(overview doesn't use local YAML specs).

## R2: TUI Goroutine Cleanup Strategy

### R2 Question

How should cache cleanup work for the TUI background goroutine?

### R2 Findings

Current TUI cleanup flow (`runInteractiveOverviewWithInit`):

1. Background goroutine (`overviewInitAndEnrich`) opens plugins and sends
   the cleanup function via `cleanupChan` (line 913)
2. Main goroutine runs TUI (blocks at `p.Run()`)
3. After TUI exits, main goroutine drains `cleanupChan` with 2s timeout
4. Calls the plugin cleanup function

The goroutine always returns after enrichment completes or on context
cancellation. A `defer` inside the goroutine fires at the right time.

### R2 Decision

Use `defer cacheCleanup()` inside the goroutine. Keep plugin cleanup via
`cleanupChan` unchanged. This is the minimal-diff approach.

### R2 Alternatives Rejected

- **Composed cleanup via channel**: Requires moving `cleanupChan <- cleanup`
  after engine creation. Risk: if error occurs between openPlugins and
  newEngineWithCache, plugin cleanup is lost.
- **Separate cache cleanup channel**: Over-engineered for a single `defer`.

## R3: cmd Parameter Propagation

### R3 Question

How to give the TUI goroutine access to `*cobra.Command`?

### R3 Findings

- `runInteractiveOverviewWithInit` already receives `cmd *cobra.Command` but
  discards it as `_ *cobra.Command` (line 718)
- `overviewInitAndEnrich` does not currently receive `cmd`
- `newEngineWithCache` needs `cmd` to check `cmd.Flags().Lookup("cache-ttl").Changed`

### R3 Decision

1. Rename `_` to `cmd` in `runInteractiveOverviewWithInit` signature
2. Add `cmd *cobra.Command` parameter to `overviewInitAndEnrich`
3. Pass `cmd` through at the call site
