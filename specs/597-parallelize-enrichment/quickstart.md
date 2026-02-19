# Quickstart: Parallelize Per-Row Enrichment Sub-Calls

**Date**: 2026-02-18
**Feature**: 597-parallelize-enrichment

## Overview

This feature parallelizes the three independent enrichment sub-calls within
`EnrichOverviewRow` (actual cost, projected cost, recommendations) to reduce
per-row wall-clock time when plugin latency is non-trivial.

## What Changes

### Before (sequential)

```text
EnrichOverviewRow
  ├── enrichActualCost      ─── 50ms ──┐
  ├── enrichProjectedCost   ─── 50ms ──┤  Total: ~150ms
  ├── enrichRecommendations ─── 50ms ──┘
  └── enrichCostDrift (sync)
```

### After (parallel)

```text
EnrichOverviewRow
  ├── goroutine: enrichActualCost      ─── 50ms ──┐
  ├── goroutine: enrichProjectedCost   ─── 50ms ──┤  Total: ~50ms
  ├── goroutine: enrichRecommendations ─── 50ms ──┘
  ├── wg.Wait()
  ├── merge errors (actual > projected)
  └── enrichCostDrift (sync)
```

## Files Modified

| File | Change |
|------|--------|
| `internal/engine/overview_enrich.go` | Refactor `EnrichOverviewRow` to use goroutines; change `enrichActualCost` and `enrichProjectedCost` return types |
| `internal/engine/overview_enrich_test.go` | Add race-condition tests, error precedence tests, parallel correctness tests |

## How to Verify

```bash
# Run all tests with race detector
go test -race ./internal/engine/...

# Run full test suite
make test

# Run linting
make lint
```

## Key Design Decisions

1. **All three in parallel**: Not just actual + projected. Recommendations also run
   concurrently since it writes to a distinct field.
2. **Local error variables**: `enrichActualCost` and `enrichProjectedCost` return
   `*OverviewRowError` instead of writing to `row.Error`. Errors are merged after
   `wg.Wait()` with actual cost error taking precedence.
3. **No new sync primitives on Engine**: The Engine is already safe for concurrent
   method calls (write-once fields, thread-safe cache).
