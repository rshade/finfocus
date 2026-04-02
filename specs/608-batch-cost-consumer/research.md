# Research: BatchCost RPC Consumer

**Feature**: 608-batch-cost-consumer
**Date**: 2026-03-31

## R1: Proto Types Available in finfocus-spec v0.6.0

**Decision**: All required proto types are available and generated in finfocus-spec v0.6.0 (already in go.mod).

**Rationale**: Codebase exploration confirmed:
- `BatchCostRequest` — fields: `Resources []*ResourceDescriptor`, `QueryType CostQueryType`, `Start/End *timestamppb.Timestamp`, `DryRun bool`
- `BatchCostResponse` — fields: `Results []*ResourceCostResult`, `MaxBatchSize int32`
- `ResourceCostResult` — fields: `Resource *ResourceDescriptor`, oneof result: `CostData` | `ResourceError`
- `CostData` — oneof data: `ActualCost *ActualCostData` | `ProjectedCost *GetProjectedCostResponse` | `Estimate *EstimateCostResponse` | `DryRunResult *DryRunResponse`
- `ResourceError` — fields: `Code int32`, `Message string`, `ResourceTypeUnsupported bool`
- `CostQueryType` enum: `UNSPECIFIED (0)`, `ESTIMATE (1)`, `ACTUAL (2)`, `PROJECTED (3)`
- gRPC method: `BatchCost(ctx, *BatchCostRequest, ...CallOption) (*BatchCostResponse, error)` on `CostSourceServiceClient`

**Alternatives considered**: None — spec dependency is already upgraded.

## R2: Capability Detection Infrastructure

**Decision**: The `PLUGIN_CAPABILITY_BATCH_COST` capability is fully wired end-to-end. No infrastructure changes needed.

**Rationale**: Confirmed existing wiring:
- `pluginhost/host.go`: `ConvertCapabilities` maps `PLUGIN_CAPABILITY_BATCH_COST` → `"batch_cost"`
- `router/features.go`: `FeatureBatchCost Feature = "BatchCost"`, in `ValidFeatures()`, `methodToFeature["BatchCost"]`
- `router/router.go`: `capabilityEnumFromFeature(FeatureBatchCost)` → `PLUGIN_CAPABILITY_BATCH_COST`
- `pluginhost/host.go`: `HasCapability("batch_cost")` — exact string match
- Router's `matchesCapabilities` does normalized set lookup (handles both `"BatchCost"` and `"batch_cost"`)

**Alternatives considered**: None — infrastructure is complete.

## R3: Engine Worker Pool Pattern

**Decision**: Integrate batch path at the top of `GetProjectedCost` and `GetActualCostWithOptions`, before the worker pool setup.

**Rationale**: Current pattern in both methods:
1. Setup: job channel, results channel, WaitGroup
2. Spawn workers (capped by `getWorkerCount`)
3. Each worker: `selectPluginMatchesForResource` → iterate matches → call plugin → collect result
4. Close channels, sort by index, flatten

The batch path should intercept before step 1: group resources by batch-capable plugin, execute batch calls, collect results. Resources not handled by batch (non-batch plugins, fallback resources) go through the existing worker pool.

**Alternatives considered**: Modifying the worker loop internals — rejected because it would entangle batch logic with the per-resource path, making both harder to test and maintain.

## R4: Cache Integration Pattern

**Decision**: Pre-check cache per resource before building batch requests. Cache individual results from batch responses using existing key/store functions.

**Rationale**: Existing cache pattern:
- Pre-check: `tryProjectedCostCache(ctx, resource)` → returns `[]CostResult` or nil
- Key generation: `BuildProjectedKey(provider, type, region, sku)` / `BuildActualKey(...)`
- Store: `storeProjectedCostCache(ctx, resource, results)` → marshals, checks ExpiresAt, calls `Set`/`SetWithTTL`
- TTL: `CalculatePluginTTL(expiresAt, defaultTTL)` with min 60s, max 604800s (7 days)

Batch results are individual `CostResult` per resource — same shape as per-resource results. Same cache functions apply directly.

**Alternatives considered**: Batch-level cache (cache the entire batch response as one entry) — rejected because it would make partial invalidation impossible and wouldn't interop with per-resource cache lookups.

## R5: Existing Default Batch Size Constant

**Decision**: Use `batchProcessingThreshold = 100` already defined at `engine.go:47`.

**Rationale**: The constant is already in the engine package, named appropriately, and set to the same value specified in the issue (100). No need to create a new constant.

**Alternatives considered**: New constant `DefaultBatchSize` — rejected because the existing constant already serves this purpose and avoiding duplication is cleaner.

## R6: Plugin List Display

**Decision**: No changes needed to `plugin_list.go`.

**Rationale**: The `--verbose` output's Capabilities column reads from `client.Metadata.Capabilities`, which is populated by `ConvertCapabilities` at plugin startup. When a plugin reports `PLUGIN_CAPABILITY_BATCH_COST`, `"batch_cost"` automatically appears in the list. JSON output via `PluginJSONEntry.Capabilities` also flows through automatically.

**Alternatives considered**: Adding a dedicated "Batch" column — rejected as over-engineering for a capability that's one of many.

## R7: Batch-Level Error Fallback Strategy

**Decision**: On batch-level gRPC errors (`codes.Unimplemented`, `codes.Unavailable`, `codes.Internal`), fall back to per-resource queries for ALL resources that were in that batch.

**Rationale**: A batch-level failure means the plugin couldn't process the request at all. The safest recovery is to retry each resource individually through the existing path, which handles timeouts, retries, and fallback chains per-resource.

**Alternatives considered**:
- Retry the batch — rejected because the same error would likely recur.
- Partial fallback (only retry failed resources) — not applicable; batch-level errors mean no results at all.
