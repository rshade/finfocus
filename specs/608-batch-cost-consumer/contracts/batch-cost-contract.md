# Contract: BatchCost Internal Interface

**Feature**: 608-batch-cost-consumer
**Date**: 2026-03-31

## 1. CostSourceClient Interface Addition

```go
// In internal/proto/adapter.go — add to CostSourceClient interface:

BatchCost(ctx context.Context, in *pbc.BatchCostRequest,
    opts ...grpc.CallOption) (*pbc.BatchCostResponse, error)
```

**Contract**: Sends a batch of resource descriptors to a plugin and returns per-resource results. The response `Results` slice MUST be in the same order as the request `Resources` slice. The caller is responsible for chunking by `max_batch_size`.

## 2. clientAdapter.BatchCost Implementation

```go
// In internal/proto/adapter.go — add to clientAdapter:

func (c *clientAdapter) BatchCost(ctx context.Context, in *pbc.BatchCostRequest,
    opts ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
    return c.client.BatchCost(ctx, in, opts...)
}
```

**Contract**: Direct pass-through to the generated gRPC client. No mapping needed on the request side — `BatchCostRequest` uses `ResourceDescriptor` directly.

## 3. Response Mapping Functions

### mapBatchProjectedResults

```go
// In internal/proto/adapter.go:

func mapBatchProjectedResults(resp *pbc.BatchCostResponse) ([]batchMappedResult, error)
```

**Input**: `BatchCostResponse` from a projected cost batch call.
**Output**: Slice of `batchMappedResult` (one per `ResourceCostResult`), each containing:
- `CostResult` (populated from `CostData.GetProjectedCost()`) or
- `error` (from `ResourceError`) or
- `skip bool` (when `ResourceTypeUnsupported == true`)

**Contract**:
- Result count MUST equal `len(resp.Results)`
- For `CostData.GetProjectedCost()` → map to `CostResult` using same logic as existing `clientAdapter.GetProjectedCost`
- For `ResourceError` with `ResourceTypeUnsupported == true` → set `skip = true`
- For `ResourceError` without `ResourceTypeUnsupported` → return structured error with resource type and message
- For nil/empty `CostData` → return nil `CostResult` (triggers fallback)

### mapBatchActualResults

```go
// In internal/proto/adapter.go:

func mapBatchActualResults(resp *pbc.BatchCostResponse) ([]batchMappedResult, error)
```

**Input**: `BatchCostResponse` from an actual cost batch call.
**Output**: Same structure as `mapBatchProjectedResults` but maps `CostData.GetActualCost()` to `ActualCostResult`.

**Contract**: Same as projected mapping but uses `ActualCostData` → `ActualCostResult` mapping logic from existing `clientAdapter.GetActualCost`.

## 4. Engine Batch Functions

### groupResourcesByPlugin

```go
// In internal/engine/engine_batch.go:

func (e *Engine) groupResourcesByPlugin(
    ctx context.Context,
    resources []ResourceDescriptor,
    feature string,
) map[string]*pluginBatch
```

**Input**: Resources to process and the feature string (`"BatchCost"`).
**Output**: Map from plugin name to `pluginBatch` struct containing the plugin client, its matched resources with original indices, and whether it supports batch.

**Contract**:
- Calls `selectPluginMatchesForResource(ctx, resource, feature)` per resource
- Groups by primary (highest-priority) match's plugin name
- Sets `hasBatch` based on `client.HasCapability("batch_cost")`
- Resources matching no plugin are returned in a special "unmatched" group

### executeBatchForPlugin

```go
// In internal/engine/engine_batch.go:

func (e *Engine) executeBatchForPlugin(
    ctx context.Context,
    plugin *pluginhost.Client,
    resources []indexedResource,
    queryType pbc.CostQueryType,
    opts batchOptions,
) ([]batchResult, error)
```

**Input**: Target plugin, resources with indices, query type, and options (date range for actual).
**Output**: Per-resource results mapped back to original indices, or batch-level error.

**Contract**:
- Chunks resources by `batchProcessingThreshold` (or smaller if `max_batch_size` from prior response indicates)
- Sends `BatchCost` RPC per chunk
- Maps results via `mapBatchProjectedResults` or `mapBatchActualResults`
- On batch-level gRPC error → returns the error (caller handles fallback)
- On partial failure → returns mixed results (some with errors, some with data)

### chunkResources

```go
// In internal/engine/engine_batch.go:

func chunkResources(resources []indexedResource, chunkSize int) [][]indexedResource
```

**Input**: Resources and maximum chunk size.
**Output**: Slices of resources, each at most `chunkSize` elements.

**Contract**:
- If `chunkSize <= 0`, uses `batchProcessingThreshold` (100)
- Last chunk may be smaller than `chunkSize`
- Preserves order

## 5. Error Types

| Error Scenario | Handling |
|---------------|----------|
| Batch RPC returns `codes.Unimplemented` | Batch-level error → caller falls back to per-resource |
| Batch RPC returns `codes.Unavailable` | Batch-level error → caller falls back to per-resource |
| Batch RPC returns `codes.Internal` | Batch-level error → caller falls back to per-resource |
| Batch RPC returns `codes.DeadlineExceeded` | Batch-level error → fall back if time remains, propagate if not |
| `ResourceError` with `ResourceTypeUnsupported` | Skip resource (log at WARN) |
| `ResourceError` without `ResourceTypeUnsupported` | Report per-resource error (log at WARN) |
| Response `Results` count != request `Resources` count | Log error, fall back to per-resource |
