# Feature Specification: Implement EstimateCost RPC Consumer (Remove Stub)

**Feature Branch**: `608-estimate-cost-rpc`
**Created**: 2026-03-30
**Status**: Draft
**Input**: GitHub Issue #847: Replace `tryEstimateCostRPC` and `BuildEstimateCostRequest` stubs with real gRPC implementations

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Plugin-Powered What-If Estimation (Priority: P1)

A developer runs `finfocus cost estimate` with property overrides and the
engine calls the plugin's `EstimateCost` RPC directly. The plugin returns
baseline cost, modified cost, and per-property deltas — all computed
server-side with the plugin's pricing data.

**Why this priority**: This is the core value — enabling plugins to provide
accurate, server-side cost estimates rather than relying on local YAML specs or
the less precise double-`GetProjectedCost` fallback. It unblocks the entire
estimation pipeline for plugins that implement the RPC.

**Independent Test**: Can be fully tested by calling `EstimateCost` on the
engine with a mock plugin that implements the RPC, verifying that the response
contains baseline, modified, and delta data from the plugin (not from the
fallback path).

**Acceptance Scenarios**:

1. **Given** a plugin that implements `EstimateCost`, **When** the engine calls
   `tryEstimateCostRPC` with a valid request containing resource descriptor and
   property overrides, **Then** the engine returns an `EstimateResult` with
   baseline and modified costs from the plugin, and `UsedFallback` is false.

2. **Given** a plugin that returns per-property deltas in its response,
   **When** the engine processes the `EstimateCostResponse`, **Then** the
   `EstimateResult.Deltas` slice contains one `CostDelta` per overridden
   property with correct cost change values.

3. **Given** a plugin that returns a valid `EstimateCostResponse`, **When**
   the engine converts the response to `CostResult`, **Then** `ExpiresAt` is
   nil (the `EstimateCostResponse` proto lacks `expires_at`) and the existing
   cache machinery uses default TTL behavior.

---

### User Story 2 - Graceful Fallback to Double-GetProjectedCost (Priority: P2)

A developer runs `finfocus cost estimate` but the plugin does not implement
`EstimateCost` (returns `Unimplemented`). The engine transparently falls back
to the existing double-`GetProjectedCost` strategy.

**Why this priority**: Backward compatibility is essential. Existing plugins
that do not implement the new RPC must continue to work without degradation.
The existing fallback path must remain functional.

**Independent Test**: Can be fully tested by calling `EstimateCost` with a mock
plugin that returns `Unimplemented` and verifying the engine produces a result
with `UsedFallback = true`.

**Acceptance Scenarios**:

1. **Given** a plugin that returns `Unimplemented` for `EstimateCost`, **When**
   the engine calls `tryEstimateCostRPC`, **Then** the engine falls through to
   `estimateCostFallback` and returns a valid `EstimateResult` with
   `UsedFallback = true`.

2. **Given** multiple plugins where the first returns `Unimplemented` and the
   second implements the RPC, **When** the engine iterates plugins, **Then** the
   engine uses the second plugin's RPC response (not the fallback).

---

### User Story 3 - Adapter Builds Valid Proto Requests (Priority: P1)

The adapter layer correctly converts engine-level `EstimateRequest` into the
gRPC-level `EstimateCostRequest` proto message, populating resource descriptor,
property overrides, and usage profile.

**Why this priority**: Without correct request construction, the RPC call sends
empty or malformed data to plugins. This is a prerequisite for Story 1.

**Independent Test**: Can be fully tested by calling `BuildEstimateCostRequest`
with various input combinations and verifying the resulting proto message has
correct fields populated.

**Acceptance Scenarios**:

1. **Given** a resource descriptor with provider, type, SKU, and region,
   **When** `BuildEstimateCostRequest` is called, **Then** the returned proto
   request has all fields populated correctly.

2. **Given** properties with multiple key-value pairs, **When**
   `BuildEstimateCostRequest` is called, **Then** the proto request's
   `Attributes` struct contains all entries as structpb values.

3. **Given** a nil resource descriptor, **When** `BuildEstimateCostRequest` is
   called, **Then** an appropriate validation error is returned.

---

### User Story 4 - Interface Completeness (Priority: P1)

The `CostSourceClient` interface includes `EstimateCost` so that all adapter
and mock implementations can delegate to the gRPC method.

**Why this priority**: The interface is the contract that binds the engine to
plugin communication. Without this method, the adapter cannot call the RPC.

**Independent Test**: Can be fully tested by verifying that `clientAdapter`
satisfies `CostSourceClient` at compile time and that the method delegates to
the underlying gRPC client.

**Acceptance Scenarios**:

1. **Given** the `CostSourceClient` interface, **When** compiled, **Then** it
   includes an `EstimateCost` method with the correct signature.

2. **Given** a `clientAdapter` instance, **When** `EstimateCost` is called,
   **Then** it delegates to the underlying `pbc.CostSourceServiceClient`.

---

### Edge Cases

- What happens when the plugin returns a nil `EstimateCostResponse`? The
  engine must return an error for the nil response.
- What happens when `PropertyOverrides` is empty? The engine should reject
  the request at validation (no meaningful comparison without overrides).
- What happens when plugin returns deltas that don't match the requested
  overrides? The engine should use whatever the plugin provides without
  revalidating delta keys.
- What happens when multiple plugins are available and the first times out?
  The engine already handles this — it logs and tries the next plugin.
- What happens when the plugin returns a cost in a different currency than
  expected? The engine should pass through the plugin's currency without
  conversion (consistent with existing behavior).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST add `EstimateCost` method to the `CostSourceClient`
  interface with the correct gRPC signature.
- **FR-002**: System MUST implement `clientAdapter.EstimateCost` to delegate to
  the underlying `pbc.CostSourceServiceClient.EstimateCost`.
- **FR-003**: System MUST replace `BuildEstimateCostRequest` stub with real
  request construction that maps `ResourceDescriptor` and property overrides to
  the proto `EstimateCostRequest` message.
- **FR-004**: System MUST replace `tryEstimateCostRPC` stub with real
  implementation that builds the proto request, calls the adapter, and converts
  the response to `EstimateResult`.
- **FR-005**: System MUST remove the `ErrEstimateCostNotSupported` sentinel
  error from the adapter package.
- **FR-006**: System MUST preserve the existing fallback behavior — when a
  plugin returns `Unimplemented`, the engine falls through to
  `estimateCostFallback`.
- **FR-007**: System MUST handle the absence of `expires_at` on
  `EstimateCostResponse` by leaving `CostResult.ExpiresAt` nil, allowing the
  existing cache machinery to use default TTL behavior.
- **FR-008**: System MUST validate the `EstimateCostResponse` — return errors
  for nil response or negative `CostMonthly`, and default `Currency` to "USD"
  when empty.
- **FR-009**: System MUST update all mock implementations to satisfy the
  expanded `CostSourceClient` interface.

### Key Entities

- **EstimateCostRequest (proto)**: gRPC request containing resource descriptor,
  property overrides, and optional usage profile. Sent to plugin.
- **EstimateCostResponse (proto)**: gRPC response containing baseline cost,
  modified cost, and per-property deltas. Received from plugin.
- **EstimateRequest (engine)**: Internal engine-level request structure
  (already exists in `types.go`).
- **EstimateResult (engine)**: Internal engine-level result structure (already
  exists in `types.go`).
- **CostSourceClient (adapter)**: Interface that wraps the gRPC client — must
  gain `EstimateCost` method.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users running `cost estimate` with a plugin that implements
  `EstimateCost` receive per-property cost deltas directly from the plugin
  without using the fallback path.
- **SC-002**: Users running `cost estimate` with a plugin that does NOT
  implement `EstimateCost` experience no change in behavior — the existing
  fallback continues to work.
- **SC-003**: All existing tests continue to pass after the stub is replaced.
- **SC-004**: New tests achieve 80% or higher coverage on changed code,
  including success path, unimplemented fallback, validation errors, and
  timeout handling.
- **SC-005**: The `ErrEstimateCostNotSupported` sentinel error is fully removed
  from the codebase with no references remaining.
- **SC-006**: `make test && make lint` pass with zero errors.

## Assumptions

- finfocus-spec v0.6.0 (already in `go.mod`) includes the `EstimateCost` RPC
  definition with `EstimateCostRequest` and `EstimateCostResponse` proto
  messages.
- The existing `estimateCostFallback` function is correct and does not need
  modification.
- The existing engine loop in `EstimateCost` (lines 68-132 of `estimate.go`)
  already handles `Unimplemented`, timeout, and cancellation — only
  `tryEstimateCostRPC` needs replacement.
- Plugin capability checking (via `Supports` RPC) is handled at the router
  level, not within `tryEstimateCostRPC` itself.
- No CLI changes are needed — the `cost estimate` command already calls
  `engine.EstimateCost`.

## Scope Boundaries

**In scope**:

- Adapter interface expansion (`CostSourceClient.EstimateCost`)
- Adapter implementation (`clientAdapter.EstimateCost`)
- Request builder (`BuildEstimateCostRequest`)
- Engine RPC caller (`tryEstimateCostRPC`)
- Sentinel error removal
- Unit tests for all new code paths

**Out of scope**:

- CLI changes (already wired)
- Plugin-side implementation (plugins implement their own `EstimateCost`)
- New capabilities or features beyond replacing the stub
- Cache layer changes (TTL handling reuses existing patterns)
- Router capability checks (already handled by router)

## Dependencies

- **Blocked by**: Issue #844 (upgrade finfocus-spec to v0.5.7) — however,
  finfocus-spec v0.6.0 is already in `go.mod`, so this dependency may already
  be satisfied.
- **Related**: Issue #846 (BatchCost supports `COST_QUERY_TYPE_ESTIMATE`)
- **Related**: Issue #845 (cache `expires_at` handling)
