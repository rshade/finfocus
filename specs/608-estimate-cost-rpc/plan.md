# Implementation Plan: EstimateCost RPC Consumer

**Branch**: `608-estimate-cost-rpc` | **Date**: 2026-03-31 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/608-estimate-cost-rpc/spec.md`

## Summary

Replace stubbed `tryEstimateCostRPC` and `BuildEstimateCostRequest` with real
implementations that call the plugin's `EstimateCost` gRPC RPC. The actual
proto (finfocus-spec v0.6.0) provides a simple single-resource estimation
interface (`ResourceType` + `Attributes` -> `CostMonthly`), so the "what-if"
comparison logic (baseline vs modified) lives in the engine, calling the RPC
twice per estimation.

## Technical Context

**Language/Version**: Go 1.25.8 (see `go.mod`)
**Primary Dependencies**: finfocus-spec v0.6.0 (proto definitions), gRPC, Cobra, zerolog
**Storage**: N/A (no new persistent state)
**Testing**: `go test` with testify assertions, mocked gRPC clients
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI binary with gRPC plugin architecture
**Performance Goals**: EstimateCost calls should complete within `perResourceTimeout` (existing)
**Constraints**: No new dependencies; reuses existing adapter patterns
**Scale/Scope**: 4 files modified, ~150-200 lines of new code + tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: This replaces a stub that delegates to
  plugins via gRPC. Core remains provider-agnostic; estimation logic is
  executed by the plugin.
- [x] **Test-Driven Development**: Tests planned for all new code paths:
  successful RPC, unimplemented fallback, validation errors, nil response
  handling. 80%+ coverage target. No TUI changes.
- [x] **Cross-Platform Compatibility**: No platform-specific code. Uses
  standard gRPC and protobuf types.
- [x] **Documentation Integrity**: CHANGELOG update planned. No new exported
  API surface requiring README changes. Existing `cost estimate` docs remain
  valid.
- [x] **Protocol Stability**: Consumes existing `EstimateCost` RPC from
  finfocus-spec v0.6.0. No protocol changes.
- [x] **Implementation Completeness**: Removes stubs and sentinel error.
  Constitution Principle VI is the primary driver — replacing
  `ErrEstimateCostNotSupported` with real behavior.
- [x] **Persistence Model**: No persistent state changes. No new BoltDB stores.
- [x] **Quality Gates**: `make test && make lint` required before completion.
- [x] **Multi-Repo Coordination**: Consumes finfocus-spec v0.6.0 (already in
  `go.mod`). No spec-side changes needed. Plugin implementations unaffected
  (they already implement or don't implement EstimateCost).

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/608-estimate-cost-rpc/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── estimate-rpc-mapping.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/proto/
├── adapter.go           # MODIFIED: Add EstimateCost to CostSourceClient,
│                        #   implement clientAdapter.EstimateCost,
│                        #   replace BuildEstimateCostRequest stub,
│                        #   remove ErrEstimateCostNotSupported
├── adapter_test.go      # MODIFIED: Add tests, update mock

internal/engine/
├── estimate.go          # MODIFIED: Replace tryEstimateCostRPC stub
├── estimate_test.go     # MODIFIED: Add RPC success/fallback tests
```

**Structure Decision**: This modifies existing files only — no new files needed.
The adapter pattern is established; we add one more method following the same
conventions as `GetProjectedCost`, `DismissRecommendation`, etc.

## Key Technical Decisions

### Proto Shape Mismatch

The actual `pbc.EstimateCostRequest` (v0.6.0) is simpler than the original
design document (`specs/223-cost-estimate/contracts/estimate-rpc.md`):

| Aspect | Original Design | Actual Proto (v0.6.0) |
|--------|----------------|----------------------|
| Request | ResourceDescriptor + property_overrides + UsageProfile | ResourceType + Attributes (structpb.Struct) |
| Response | baseline + modified + deltas[] | Currency + CostMonthly + PricingCategory + SpotRisk |

**Impact**: The "what-if" comparison (baseline vs modified) stays in the engine.
`tryEstimateCostRPC` calls the plugin's `EstimateCost` **twice** — once with
original properties, once with overrides applied — then computes deltas.

### Interface Method Signature

Follow the pattern of `GetBudgets`, `DryRun`, `Supports` — pass `pbc` types
directly since the proto is simple and no internal wrapper is needed:

```go
EstimateCost(ctx context.Context, in *pbc.EstimateCostRequest,
    opts ...grpc.CallOption) (*pbc.EstimateCostResponse, error)
```

### BuildEstimateCostRequest Redesign

The function signature changes to build a `pbc.EstimateCostRequest` (not the
internal `EstimateCostRequest`). It takes a `ResourceDescriptor` and builds
the proto using `structpb.NewStruct` for the Attributes field. The caller
invokes it twice (baseline + modified).

### Internal Types Cleanup

The internal `EstimateCostRequest`, `EstimateCostResponse`, and `CostDelta`
types in `adapter.go:1277-1315` become unused after the stub is removed.
They should be removed to avoid dead code.
