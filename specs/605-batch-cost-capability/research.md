# Research: Batch Cost Capability

**Date**: 2026-03-03
**Feature**: 605-batch-cost-capability

## Research Questions

### RQ-1: Where does capability string conversion happen?

**Decision**: `ConvertCapabilities()` in `internal/pluginhost/host.go:164-206`
already handles `PLUGIN_CAPABILITY_BATCH_COST` → `"batch_cost"` (line 194-195).
No changes needed.

**Rationale**: The function was updated when finfocus-spec v0.5.7 was integrated
(or is ready for it — the enum case already exists in the switch statement).

**Alternatives considered**: None — single canonical location.

### RQ-2: Where are Feature constants defined?

**Decision**: `internal/router/features.go` defines Feature type and constants
(lines 14-32). Currently has 6 features: ProjectedCosts, ActualCosts,
Recommendations, Carbon, DryRun, Budgets. `FeatureBatchCost` must be added here.

**Rationale**: Follows the established pattern — one constant per plugin
capability that the router can match against.

**Alternatives considered**: None — established pattern.

### RQ-3: What functions need the new capability case?

**Decision**: Three functions in `internal/router/router.go` need updating:

1. `capabilityEnumFromFeature()` (line 482-499) — maps Feature → proto enum
2. `capabilityEnumFromString()` (line 501-518) — maps string → proto enum
   (both PascalCase "BatchCost" and snake_case "batch_cost")

Additionally in `internal/router/features.go`:

1. `ValidFeatures()` (line 36-45) — return slice of all features
2. `methodToFeature` map (line 98-108) — maps gRPC method → Feature
   (add "BatchCost" → FeatureBatchCost)

**Rationale**: All these follow the exact same pattern as existing capabilities
(Budgets was the most recent addition — follow that pattern).

**Alternatives considered**: None — must match all existing patterns.

### RQ-4: Does plugin list display need changes?

**Decision**: No. `internal/cli/plugin_list.go` uses `formatCapabilities()`
which joins whatever strings are in the plugin's `Metadata.Capabilities` slice.
Since `ConvertCapabilities()` already produces `"batch_cost"`, it will appear
automatically.

**Rationale**: The display is generic — it doesn't have a hardcoded list of
known capabilities.

**Alternatives considered**: None — display is already capability-agnostic.

### RQ-5: What is the gRPC method name for BatchCost?

**Decision**: `"BatchCost"` — confirmed from `internal/proto/adapter_test.go`
(line 3132) which defines `BatchCost()` method on the mock client.

**Rationale**: Follows proto service naming convention (PascalCase method name).

**Alternatives considered**: None — defined by proto.

### RQ-6: Does `config routes test` need changes?

**Decision**: No explicit changes needed. The `config routes test` command
iterates over `router.ValidFeatureNames()` (lines 307, 366, 404 of
`config_routes.go`). Once `FeatureBatchCost` is added to `ValidFeatures()`,
it will automatically appear in route testing output.

**Rationale**: The command dynamically pulls from `ValidFeatures()`.

**Alternatives considered**: None — design is dynamic.

## Summary

This is a purely additive change to the router's feature mapping system.
The pluginhost conversion and plugin list display already work. Only the
router package needs updating to recognize BatchCost as a routable feature.

Files requiring changes:

| File | Change | Lines |
|------|--------|-------|
| `internal/router/features.go` | Add constant, update ValidFeatures(), add method mapping | ~8 lines |
| `internal/router/router.go` | Add cases in 2 switch statements | ~6 lines |
| `internal/router/features_test.go` | Update test expectations for 7 features | ~15 lines |
| `internal/router/router_test.go` | Add test cases for new capability mapping | ~20 lines |

Total: ~49 lines across 4 files.
