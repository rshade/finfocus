# Data Model: Batch Cost Capability

**Date**: 2026-03-03
**Feature**: 605-batch-cost-capability

## Overview

No new data entities are introduced. This feature adds a single constant value
to the existing `Feature` type and maps it through existing capability
resolution functions.

## Existing Entities (unchanged)

### Feature (type alias: `string`)

- **Location**: `internal/router/features.go`
- **Purpose**: Represents a plugin capability type used for routing decisions
- **Current values**: ProjectedCosts, ActualCosts, Recommendations, Carbon,
  DryRun, Budgets
- **Addition**: `BatchCost` — represents the batch cost query capability

### PluginCapability (proto enum)

- **Location**: finfocus-spec (external dependency)
- **Purpose**: Wire-format enum for plugin capability advertisement
- **Relevant value**: `PLUGIN_CAPABILITY_BATCH_COST = 12`
- **String form**: `"batch_cost"` (from `ConvertCapabilities()`)

### PluginMetadata (struct)

- **Location**: `internal/proto/types.go`
- **Purpose**: Holds plugin info including capabilities string slice
- **Impact**: No changes — `Capabilities []string` already carries `"batch_cost"`
  when converted from proto

## Relationships

```text
Proto Enum (BATCH_COST=12) → ConvertCapabilities() → "batch_cost" string
                                                          ↓
Feature constant (BatchCost) ← capabilityEnumFromString() ← normalizedCapabilitySet()
                            → capabilityEnumFromFeature() → Proto Enum (for matching)
```
