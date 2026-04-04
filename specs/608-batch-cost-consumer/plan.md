# Implementation Plan: BatchCost RPC Consumer

**Branch**: `608-batch-cost-consumer` | **Date**: 2026-03-31 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/608-batch-cost-consumer/spec.md`

## Summary

Implement the BatchCost RPC consumer in finfocus core to send multiple resource descriptors to a plugin in a single gRPC call instead of N individual calls. This is a transport optimization that reduces round-trips from 100 to 1-2 for large stacks. The engine detects batch capability per plugin, groups resources by target plugin, pre-checks cache to exclude cached resources, chunks by `max_batch_size`, and falls back to per-resource queries when batch is unavailable or fails.

## Technical Context

**Language/Version**: Go 1.25.8 (see `go.mod`)
**Primary Dependencies**: finfocus-spec v0.6.0 (proto definitions with `BatchCost` RPC), Cobra (CLI), gRPC, zerolog (logging)
**Storage**: BoltDB cost cache (`~/.finfocus/cache/cache.db`) — batch results cached per-resource independently
**Testing**: Go testing with testify (`require`/`assert`), table-driven tests, mock gRPC clients
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module (CLI tool)
**Performance Goals**: 100 resources → 1-2 gRPC calls instead of 100; wall-clock reduction proportional to round-trip savings
**Constraints**: Backward compatible — non-batch plugins must behave identically to current behavior
**Scale/Scope**: Stacks with 50-1000+ resources; default batch size 100 (`batchProcessingThreshold` at `engine.go:47`)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: BatchCost is a transport optimization in the orchestration layer (engine). All cost logic remains in plugins. Core sends descriptors and maps responses — no provider-specific code.
- [x] **Test-Driven Development**: Tests planned for all layers: adapter interface/mapping, engine batch path (success, partial failure, chunking, fallback, cache pre-check), capability detection. 80% minimum, 95% for critical paths. No TUI changes → no golden files needed.
- [x] **Cross-Platform Compatibility**: Pure Go, no platform-specific code. gRPC and BoltDB are already cross-platform.
- [x] **Documentation Integrity**: CLAUDE.md batch-cost-capability entry already exists. README updates for batch capability in plugin development docs.
- [x] **Protocol Stability**: Using existing `BatchCost` RPC from finfocus-spec v0.6.0. No protocol changes needed. Backward compatible via capability detection + fallback.
- [x] **Implementation Completeness**: Full implementation — no stubs, no TODOs. Batch path, fallback path, cache integration, error mapping all complete.
- [x] **Persistence Model**: Uses existing cost cache (BoltDB). Batch results cached per-resource independently via `storeProjectedCostCache`/`storeActualCostCache`. No new persistent stores.
- [x] **Quality Gates**: `make test && make lint` required. All CI checks (race detector, coverage, linting, security).
- [x] **Multi-Repo Coordination**: Depends on finfocus-spec v0.6.0 (already in go.mod). No spec changes needed. Plugin implementations will add BatchCost support independently.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/608-batch-cost-consumer/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 research findings
├── data-model.md        # Data model and type mappings
├── quickstart.md        # Implementation quickstart guide
├── contracts/           # Internal interface contracts
│   └── batch-cost-contract.md
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── proto/
│   └── adapter.go           # Add BatchCost to CostSourceClient interface + clientAdapter impl + response mapping
├── engine/
│   ├── engine.go            # Add batch path in GetProjectedCost/GetActualCost: group-by-plugin, cache pre-check, chunk, call, map results
│   ├── engine_batch.go      # NEW: batch-specific helpers (groupByPlugin, chunkResources, executeBatch, buildBatchCostRequest)
│   ├── engine_batch_test.go # NEW: batch path unit tests
│   └── types.go             # No changes expected (CostResult already sufficient)
├── pluginhost/
│   └── host.go              # No changes needed (ConvertCapabilities already handles batch_cost)
├── router/
│   └── features.go          # No changes needed (FeatureBatchCost already registered)
└── cli/
    └── plugin_list.go       # No changes needed (capabilities flow through automatically)
```

**Structure Decision**: All batch logic lives in the engine layer (`internal/engine/`) with a new `engine_batch.go` file for batch-specific helpers. The adapter layer (`internal/proto/adapter.go`) gets the interface addition and response mapping. This follows the existing pattern where engine orchestrates and adapter translates proto types.

## Complexity Tracking

No constitution violations — table not needed.

## Design Decisions

### D1: Batch path integration point

**Decision**: Modify `GetProjectedCost` and `GetActualCostWithOptions` to detect batch capability and redirect to the batch path before entering the per-resource worker loop.

**Rationale**: This preserves the existing API contract — callers don't change. The batch path is an internal optimization transparent to the engine's consumers (CLI commands, TUI).

**Alternative rejected**: New `GetBatchProjectedCost` method — would require all callers to choose between batch and non-batch, duplicating call-site logic.

### D2: Resource grouping strategy

**Decision**: Group resources by primary plugin match before batching. For each resource, call `selectPluginMatchesForResource(ctx, resource, "BatchCost")` and group by the top-priority match's plugin name.

**Rationale**: Reuses existing routing infrastructure. The router already handles pattern matching, provider matching, and priority sorting. Grouping by primary match ensures each batch goes to the right plugin.

**Alternative rejected**: Grouping by provider string — too coarse, doesn't account for routing config or multi-plugin setups.

### D3: Fallback handling for batch results

**Decision**: Two-level fallback:
1. **Batch-level**: If the batch RPC itself fails (gRPC status error), fall back to per-resource queries for ALL resources in that batch.
2. **Resource-level**: If a `ResourceCostResult` has nil/empty cost data (not an error, just no data), that individual resource falls back to the next plugin in its fallback chain.

**Rationale**: Matches existing per-resource fallback semantics. `$0.00` is a valid result (no fallback). Only nil/empty triggers fallback per the router's `ShouldFallback` contract.

### D4: Batch-then-fallback ordering

**Decision**: For each plugin group:
1. Pre-check cache → exclude cached resources
2. If plugin supports batch → send batch request
3. For resources with nil/empty results → check if fallback is enabled → queue for next plugin
4. For resources with errors → report error (no fallback unless `resource_type_unsupported`)
5. If plugin doesn't support batch → fall back to per-resource worker pool

**Rationale**: Maximizes batch efficiency while preserving fallback semantics. Cache pre-check reduces batch size. Fallback chain operates per-resource after batch results are mapped back.

### D5: Cache key reuse

**Decision**: Reuse existing `BuildProjectedKey` and `BuildActualKey` functions for batch results. Each resource's result from a batch is cached with the same key it would have gotten in the per-resource path.

**Rationale**: Ensures cache consistency between batch and per-resource paths. A resource cached via batch can be served from cache in a subsequent per-resource query and vice versa.

### D6: Response mapping architecture

**Decision**: Create two mapping functions in `internal/proto/adapter.go`:
- `mapBatchProjectedResults(resp *pbc.BatchCostResponse) []batchMappedResult` — maps `CostData.GetProjectedCost()` → `CostResult`
- `mapBatchActualResults(resp *pbc.BatchCostResponse) []batchMappedResult` — maps `CostData.GetActualCost()` → `ActualCostResult`
- Both handle `ResourceError` → structured error with resource type and message

**Rationale**: Split by query type mirrors the existing `GetProjectedCost`/`GetActualCost` adapter methods. Reuses existing mapping logic. The `CostData` oneof variants contain the same response types as individual RPCs, so mapping code can be shared.

### D7: `max_batch_size` handling

**Decision**: Use `batchProcessingThreshold` (100) as the initial chunk size. After receiving the first `BatchCostResponse`, if `MaxBatchSize > 0 && MaxBatchSize < currentChunkSize`, adjust subsequent chunk sizes downward. Never increase beyond the default.

**Rationale**: Defensive approach. Start with a known-safe default. Plugin's `max_batch_size` is a hint to go smaller, not larger. A `max_batch_size` of 0 means "no preference" (use default).
