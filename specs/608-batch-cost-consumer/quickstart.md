# Quickstart: BatchCost RPC Consumer Implementation

**Feature**: 608-batch-cost-consumer
**Date**: 2026-03-31

## Prerequisites

- Go 1.25.8 (see `go.mod`)
- finfocus-spec v0.6.0 already in `go.mod`
- Familiarity with `internal/engine/engine.go` worker pool pattern
- Familiarity with `internal/proto/adapter.go` mapping patterns

## Implementation Order

### Step 1: Adapter Interface + Mapping (internal/proto/adapter.go)

1. Add `BatchCost` to `CostSourceClient` interface (line ~708)
2. Implement `clientAdapter.BatchCost` as pass-through
3. Add `batchMappedResult` type and `mapBatchProjectedResults` / `mapBatchActualResults` functions
4. Update mock in `adapter_test.go` to implement new interface method
5. Write unit tests for response mapping (success, error, unsupported, nil data)

**Test**: `go test -v -run TestBatchCost ./internal/proto/...`

### Step 2: Engine Batch Helpers (internal/engine/engine_batch.go — new file)

1. Define internal types: `pluginBatch`, `indexedResource`, `batchResult`, `batchOptions`
2. Implement `groupResourcesByPlugin` — calls `selectPluginMatchesForResource` per resource
3. Implement `chunkResources` — splits resources by chunk size
4. Implement `executeBatchForPlugin` — chunks, calls `BatchCost` RPC, maps results, handles errors
5. Write unit tests for each helper

**Test**: `go test -v -run TestBatch ./internal/engine/...`

### Step 3: Engine Integration (internal/engine/engine.go)

1. In `GetProjectedCost`: before worker pool setup, call batch path
   - `groupResourcesByPlugin(ctx, resources, "BatchCost")`
   - For each batch-capable plugin: pre-check cache, `executeBatchForPlugin`, cache results
   - Collect handled indices
   - Pass remaining resources (non-batch plugins, fallback) to existing worker pool
2. Same pattern in `GetActualCostWithOptions` with `COST_QUERY_TYPE_ACTUAL` and date range
3. Write integration-level unit tests: full `GetProjectedCost` call with mock batch plugin

**Test**: `go test -v -run TestGetProjectedCost ./internal/engine/...`

### Step 4: Fallback Path (internal/engine/engine.go)

1. In the batch integration: catch batch-level errors, log warning, redirect to per-resource pool
2. For nil/empty results from batch: queue for fallback plugin if `ShouldFallback` is true
3. Write tests for: batch error → fallback, partial failure, all-fail batch

**Test**: `go test -v -run TestBatchFallback ./internal/engine/...`

## Verification

```bash
# Unit tests
go test -v ./internal/proto/...
go test -v ./internal/engine/...

# Full test suite
make test

# Lint
make lint

# Coverage check
go test -coverprofile=coverage.out ./internal/engine/... ./internal/proto/...
go tool cover -func=coverage.out | grep -E "engine_batch|adapter"
```

## Key Files to Read First

| File | Why |
|------|-----|
| `internal/engine/engine.go:362-530` | `GetProjectedCost` — the method you're modifying |
| `internal/engine/engine.go:258-343` | `selectPluginMatchesForResource` — routing you'll reuse |
| `internal/proto/adapter.go:666-708` | `CostSourceClient` interface — you're adding `BatchCost` |
| `internal/proto/adapter.go:1048-1128` | `clientAdapter.GetProjectedCost` — mapping pattern to follow |
| `internal/engine/cache/key.go` | Cache key builders you'll reuse |
| `internal/engine/engine.go:45-50` | `batchProcessingThreshold` constant |

## Design Constraints

- Dry-run is out of scope (always `false` in `BatchCostRequest`)
- Cache pre-check: exclude cached resources from batch
- `max_batch_size` of 0 means "use default" (100)
- Response result count must match request resource count (mismatch → fall back)
- `$0.00` is a valid cost result — does NOT trigger fallback
