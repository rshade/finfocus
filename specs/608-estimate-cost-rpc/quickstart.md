# Quickstart: EstimateCost RPC Consumer

**Feature**: 608-estimate-cost-rpc
**Date**: 2026-03-31

## What Changed

The `cost estimate` command now calls plugins via the `EstimateCost` gRPC RPC
when available. Previously, the engine always used a fallback strategy (calling
`GetProjectedCost` twice). Now, plugins that implement `EstimateCost` are
called directly for more accurate estimates.

## User-Facing Behavior

**No CLI changes.** The `finfocus cost estimate` command works the same way.
The difference is internal — the engine prefers the `EstimateCost` RPC over
the fallback path.

### With a Plugin That Implements EstimateCost

```bash
finfocus cost estimate \
  --provider aws \
  --resource-type ec2:Instance \
  --property instanceType=m5.large
```

The engine calls the plugin's `EstimateCost` RPC twice (baseline + modified)
and computes the delta. Output is the same format.

### With a Plugin That Does NOT Implement EstimateCost

```bash
finfocus cost estimate \
  --provider aws \
  --resource-type ec2:Instance \
  --property instanceType=m5.large
```

The engine falls back to double `GetProjectedCost` calls. The
`UsedFallback` flag in JSON output indicates which path was taken.

## For Plugin Developers

If your plugin already implements `EstimateCost` in the
`CostSourceService` gRPC service, no changes are needed. The core will
now actually call it.

If your plugin does NOT implement `EstimateCost`, it continues to work via
the fallback path. The core handles `Unimplemented` gracefully.

## Files Modified

| File | Change |
|------|--------|
| `internal/proto/adapter.go` | Added `EstimateCost` to `CostSourceClient` interface; implemented `clientAdapter.EstimateCost`; replaced `BuildEstimateCostRequest` stub; removed `ErrEstimateCostNotSupported` and dead internal types |
| `internal/engine/estimate.go` | Replaced `tryEstimateCostRPC` stub with real two-call implementation |
| `internal/proto/adapter_test.go` | Updated mocks, added tests |
| `internal/engine/estimate_test.go` | Added RPC success/fallback/error tests |

## Verification

```bash
make test   # All tests pass
make lint   # No lint errors
```
