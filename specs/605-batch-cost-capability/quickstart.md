# Quickstart: Batch Cost Capability

**Date**: 2026-03-03
**Feature**: 605-batch-cost-capability

## Prerequisites

- finfocus-spec v0.5.7+ dependency upgraded (issue #844)
- Go 1.25.7+

## Implementation Steps

### 1. Add Feature Constant

In `internal/router/features.go`, add the `FeatureBatchCost` constant alongside
existing features and update `ValidFeatures()`, `methodToFeature`.

### 2. Add Router Mapping Cases

In `internal/router/router.go`, add `FeatureBatchCost` cases to:

- `capabilityEnumFromFeature()` — map Feature → proto enum
- `capabilityEnumFromString()` — map "BatchCost"/"batch_cost" → proto enum

### 3. Update Tests

In `internal/router/features_test.go`:

- Update `TestValidFeatures` count from 6 → 7
- Add `FeatureBatchCost` to expected slice
- Add `"BatchCost"` test case to `TestIsValidFeature`
- Add `"BatchCost"` method mapping to `TestFeatureFromMethod`

In `internal/router/router_test.go`:

- Add test cases for `capabilityEnumFromFeature` with BatchCost
- Add test cases for `capabilityEnumFromString` with "BatchCost" and
  "batch_cost"

### 4. Verify

```bash
make test
make lint
```

## Verification

After implementation, verify with:

```bash
# Unit tests pass
go test -v ./internal/router/...

# All tests pass
make test

# Linting passes
make lint
```
