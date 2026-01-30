# Implementation Plan: Pre-Flight Request Validation

**Branch**: `107-preflight-validation` | **Date**: 2025-12-29 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/107-preflight-validation/spec.md`

## Summary

Integrate `pluginsdk.ValidateProjectedCostRequest()` and `pluginsdk.ValidateActualCostRequest()` from finfocus-spec v0.4.11+ for pre-flight request validation in `internal/proto/adapter.go`. This catches malformed requests before gRPC calls to plugins, providing actionable error messages and reducing plugin round-trip latency for invalid requests.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: github.com/rshade/finfocus-spec/sdk/go/pluginsdk v0.4.11+, zerolog v1.34.0
**Storage**: N/A (validation is stateless)
**Testing**: go test with table-driven tests, mock clients
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI tool
**Performance Goals**: <100ns per validation call (zero-allocation happy path)
**Constraints**: No breaking changes to existing API signatures
**Scale/Scope**: Validates 1-1000 resources per request

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in Core that uses shared pluginsdk validation - not a provider integration
- [x] **Test-Driven Development**: Unit tests planned for all validation integration points (80%+ coverage target)
- [x] **Cross-Platform Compatibility**: Pure Go code, no platform-specific behavior
- [x] **Documentation as Code**: Will update CLAUDE.md with validation patterns
- [x] **Protocol Stability**: Uses existing pluginsdk functions, no protocol changes
- [x] **Implementation Completeness**: All validation paths fully implemented, no stubs or TODOs
- [x] **Quality Gates**: Will run make lint && make test before completion
- [x] **Multi-Repo Coordination**: Depends on pluginsdk v0.4.11+ (already integrated)

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/107-preflight-validation/
├── plan.md              # This file
├── research.md          # Phase 0 output (minimal - dependencies resolved)
├── data-model.md        # Phase 1 output (existing types, no new entities)
├── quickstart.md        # Phase 1 output (usage examples)
├── contracts/           # Phase 1 output (N/A - internal API, no external contracts)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/proto/
├── adapter.go           # Primary integration point - add validation calls
└── adapter_test.go      # Add validation test cases

internal/logging/
└── logging.go           # Existing - use FromContext for WARN logs
```

**Structure Decision**: Minimal footprint - validation is added to existing functions in `adapter.go`. No new files needed. Test cases added to existing `adapter_test.go`.

## Complexity Tracking

No Constitution violations - this is a straightforward integration of existing pluginsdk functions.

## Design Decisions

### D1: Validation Location

**Decision**: Validate in `clientAdapter.GetProjectedCost()` and `clientAdapter.GetActualCost()` methods (adapter layer), not in the higher-level `GetProjectedCostWithErrors`/`GetActualCostWithErrors` functions.

**Rationale**:

- The adapter methods construct the protobuf request (`pbc.GetProjectedCostRequest`), which is what `pluginsdk.ValidateProjectedCostRequest()` validates
- Validation after request construction, before gRPC call maximizes coverage
- Higher-level functions deal with internal types, not protobuf types

### D2: Error Handling Strategy

**Decision**: On validation failure, return an error from the adapter method (consistent with existing error handling).

**Rationale**:

- The existing pattern in `GetProjectedCostWithErrors` already handles errors from `client.GetProjectedCost()` calls
- Validation errors will be captured in `ErrorDetail` with "pre-flight validation failed" prefix
- Maintains consistency with existing error tracking infrastructure

### D3: Logging Integration

**Decision**: Log validation failures at WARN level using `logging.FromContext(ctx)` before returning the error.

**Rationale**:

- Consistent with existing logging patterns in the codebase
- WARN level is appropriate for user-fixable issues
- Context propagation ensures trace_id correlation

### D4: Notes Prefix for Validation Errors

**Decision**: Use `"VALIDATION: %v"` prefix in Notes field to distinguish from plugin errors (which use `"ERROR: %v"`).

**Rationale**:

- Meets SC-002: "Validation errors are distinguished from plugin errors"
- Clear visual distinction in output
- Users can grep/filter on prefix

## Integration Points

### GetProjectedCost (adapter.go:427-491)

```go
// Current: constructs pbc.GetProjectedCostRequest then calls c.client.GetProjectedCost
// Change: Add validation after request construction, before gRPC call

func (c *clientAdapter) GetProjectedCost(...) (*GetProjectedCostResponse, error) {
    for _, resource := range in.Resources {
        sku, region := resolveSKUAndRegion(resource.Provider, resource.Properties)

        req := &pbc.GetProjectedCostRequest{
            Resource: &pbc.ResourceDescriptor{...},
        }

        // NEW: Pre-flight validation
        if err := pluginsdk.ValidateProjectedCostRequest(req); err != nil {
            log := logging.FromContext(ctx)
            log.Warn().Ctx(ctx).
                Str("resource_type", resource.Type).
                Err(err).
                Msg("pre-flight validation failed")
            // Skip this resource, append placeholder result
            results = append(results, &CostResult{
                Currency:    "USD",
                MonthlyCost: 0,
                Notes:       fmt.Sprintf("VALIDATION: %v", err),
            })
            continue
        }

        resp, err := c.client.GetProjectedCost(ctx, req, opts...)
        // ... existing handling
    }
}
```

### GetActualCost (adapter.go:493-560)

```go
// Similar pattern for actual cost validation
func (c *clientAdapter) GetActualCost(...) (*GetActualCostResponse, error) {
    for _, resourceID := range in.ResourceIDs {
        req := &pbc.GetActualCostRequest{
            ResourceId: resourceID,
            Start:      timestamppb.New(time.Unix(in.StartTime, 0)),
            End:        timestamppb.New(time.Unix(in.EndTime, 0)),
            Tags:       make(map[string]string),
        }

        // NEW: Pre-flight validation
        if err := pluginsdk.ValidateActualCostRequest(req); err != nil {
            log := logging.FromContext(ctx)
            log.Warn().Ctx(ctx).
                Str("resource_id", resourceID).
                Err(err).
                Msg("pre-flight validation failed for actual cost")
            // Skip this resource, append placeholder result
            results = append(results, &ActualCostResult{
                Currency:   "USD",
                TotalCost:  0,
            })
            continue
        }

        resp, err := c.client.GetActualCost(ctx, req, opts...)
        // ... existing handling
    }
}
```

## Test Strategy

### Unit Tests to Add (adapter_test.go)

1. **TestGetProjectedCost_ValidationFailure_EmptyProvider**
   - Input: Resource with empty Provider field
   - Expected: VALIDATION note, no gRPC call made

2. **TestGetProjectedCost_ValidationFailure_EmptySKU**
   - Input: Resource with empty SKU (no instanceType in properties)
   - Expected: VALIDATION note, no gRPC call made

3. **TestGetProjectedCost_ValidationFailure_EmptyRegion**
   - Input: Resource with empty region (no region/availabilityZone)
   - Expected: VALIDATION note, no gRPC call made

4. **TestGetProjectedCost_ValidationFailure_MixedValidInvalid**
   - Input: [valid resource, invalid resource, valid resource]
   - Expected: Valid resources call plugin, invalid gets VALIDATION note

5. **TestGetActualCost_ValidationFailure_EmptyResourceID**
   - Input: Empty resource ID
   - Expected: VALIDATION note, no gRPC call made

6. **TestGetActualCost_ValidationFailure_InvalidTimeRange**
   - Input: EndTime before StartTime
   - Expected: VALIDATION note, no gRPC call made

### Test Coverage Target

- `clientAdapter.GetProjectedCost`: 95% (critical path)
- `clientAdapter.GetActualCost`: 95% (critical path)
- Overall `internal/proto`: 85%+

## Implementation Sequence

1. **Add import** for `pluginsdk` package (already importing `mapping` subpackage)
2. **Add validation to GetProjectedCost** with logging and placeholder results
3. **Add validation to GetActualCost** with logging and placeholder results
4. **Add unit tests** for all validation failure scenarios
5. **Run make lint && make test** to verify quality gates
6. **Update CLAUDE.md** with validation pattern documentation
