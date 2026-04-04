# Data Model: BatchCost RPC Consumer

**Feature**: 608-batch-cost-consumer
**Date**: 2026-03-31

## Proto → Internal Type Mappings

This feature bridges proto types (from finfocus-spec v0.6.0) to internal engine types. No new persistent entities are introduced.

### Request Flow: Engine → Adapter → Plugin

```text
Engine ResourceDescriptor[] ──→ pbc.BatchCostRequest ──→ Plugin
                                 │
                                 ├── resources: []*pbc.ResourceDescriptor
                                 ├── query_type: CostQueryType (PROJECTED or ACTUAL)
                                 ├── start/end: *timestamppb.Timestamp (ACTUAL only)
                                 └── dry_run: false (out of scope)
```

**ResourceDescriptor mapping** (existing in adapter, reused):

| Engine Field  | Proto Field      | Notes                              |
| ------------- | ---------------- | ---------------------------------- |
| `ID`          | `id`             | Pulumi URN                         |
| `Provider`    | `provider`       | e.g., "aws", "azure", "gcp"       |
| `Type`        | `resource_type`  | Pulumi type token                  |
| `SKU`         | `sku`            | Resolved via `resolveSKUAndRegion` |
| `Region`      | `region`         | Resolved via `resolveSKUAndRegion` |
| `Properties`  | `tags`           | Key-value properties map           |

### Response Flow: Plugin → Adapter → Engine

```text
Plugin ──→ pbc.BatchCostResponse ──→ Engine CostResult[] / ActualCostResult[]
            │
            ├── results: []*ResourceCostResult (1:1 with request resources)
            │    ├── resource: *ResourceDescriptor (echo back)
            │    └── oneof result:
            │         ├── cost_data: *CostData
            │         │    └── oneof data:
            │         │         ├── projected_cost: *GetProjectedCostResponse → CostResult
            │         │         ├── actual_cost: *ActualCostData → ActualCostResult
            │         │         ├── estimate: *EstimateCostResponse (not used)
            │         │         └── dry_run_result: *DryRunResponse (out of scope)
            │         └── error: *ResourceError → structured error
            └── max_batch_size: int32 (chunking hint)
```

### CostData → Internal Type Mapping

| CostData Variant                | Internal Type        | Mapping Function (existing)       |
| ------------------------------- | -------------------- | --------------------------------- |
| `projected_cost` (`GetProjectedCostResponse`) | `CostResult` | Reuse from `clientAdapter.GetProjectedCost` |
| `actual_cost` (`ActualCostData`)| `ActualCostResult`   | Reuse from `clientAdapter.GetActualCost` |
| `estimate` (`EstimateCostResponse`) | N/A             | Not used in batch scope           |
| `dry_run_result` (`DryRunResponse`) | N/A             | Out of scope                      |

### ResourceError → Error Mapping

| ResourceError Field           | Engine Handling                                         |
| ----------------------------- | ------------------------------------------------------- |
| `code` (gRPC status)          | Log at WARN with resource context                       |
| `message`                     | Include in error message returned to caller             |
| `resource_type_unsupported`   | Skip resource (same as per-resource validation skip)    |

### CostQueryType → Method Mapping

| CostQueryType         | Used For             | Date Fields    |
| --------------------- | -------------------- | -------------- |
| `COST_QUERY_TYPE_PROJECTED` | `GetProjectedCost` path | Not set     |
| `COST_QUERY_TYPE_ACTUAL`    | `GetActualCost` path    | `start`, `end` set |
| `COST_QUERY_TYPE_ESTIMATE`  | Not used in this feature | N/A          |
| `COST_QUERY_TYPE_UNSPECIFIED` | Defaults to ESTIMATE (not used) | N/A |

## Engine Internal Types (new)

### pluginBatch (internal to engine_batch.go)

Groups resources targeted at a single plugin for batch processing.

| Field        | Type                   | Purpose                                   |
| ------------ | ---------------------- | ----------------------------------------- |
| `plugin`     | `*pluginhost.Client`   | Target plugin client                       |
| `resources`  | `[]indexedResource`    | Resources with their original indices      |
| `hasBatch`   | `bool`                 | Whether plugin supports batch capability   |

### indexedResource (internal to engine_batch.go)

Preserves the original index for result reassembly.

| Field      | Type                 | Purpose                            |
| ---------- | -------------------- | ---------------------------------- |
| `index`    | `int`                | Original position in input slice   |
| `resource` | `ResourceDescriptor` | The resource descriptor            |

### batchResult (internal to engine_batch.go)

Result from a batch call, mapped back to engine types.

| Field    | Type           | Purpose                                   |
| -------- | -------------- | ----------------------------------------- |
| `index`  | `int`          | Original position in input slice          |
| `result` | `*CostResult`  | Cost result (nil if error)                |
| `err`    | `error`        | Error (nil if success)                    |

## State Transitions

No new state machines. The batch path is stateless request-response:

```text
Resources → [cache pre-check] → [group by plugin] → [chunk by max_batch_size]
         → [batch RPC call] → [map results] → [cache store] → [merge with cached]
         → [fallback for nil results] → Final results
```

## Cache Behavior

| Scenario                        | Cache Action                                    |
| ------------------------------- | ----------------------------------------------- |
| Resource already cached         | Excluded from batch; cached result used directly |
| Batch result for resource       | Cached independently via existing key/store      |
| Batch result with ExpiresAt     | Uses `SetWithTTL` with plugin TTL hint           |
| Batch result is placeholder     | Not cached (existing `hasOnlyPlaceholderResults` check) |
| Batch-level error → fallback    | Per-resource results cached normally             |
