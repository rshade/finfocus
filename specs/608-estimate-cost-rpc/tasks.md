# Tasks: EstimateCost RPC Consumer

**Input**: Design documents from `/specs/608-estimate-cost-rpc/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness),
all tasks MUST be fully implemented. Stub functions, placeholders, and TODO
comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity),
documentation (CHANGELOG) MUST be updated concurrently with implementation.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Verify proto types are available and the build compiles cleanly
before making changes.

- [X] T001 Verify `pbc.EstimateCostRequest` and `pbc.EstimateCostResponse` exist in finfocus-spec v0.6.0 by running `go doc` or grep on the generated proto package
- [X] T002 Verify `structpb.NewStruct` is available by checking the `google.golang.org/protobuf/types/known/structpb` import compiles in `internal/proto/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Expand the `CostSourceClient` interface and update ALL mock
implementations. This MUST complete before any user story work because adding
a method to an interface is a compile-breaking change.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T003 Add `EstimateCost` method to the `CostSourceClient` interface in `internal/proto/adapter.go` with signature: `EstimateCost(ctx context.Context, in *pbc.EstimateCostRequest, opts ...grpc.CallOption) (*pbc.EstimateCostResponse, error)`
- [X] T004 Implement `clientAdapter.EstimateCost` in `internal/proto/adapter.go` that delegates to `c.client.EstimateCost(ctx, in, opts...)`
- [X] T005 [P] Add `EstimateCost` stub method to `mockCostSourceClient` in `internal/proto/adapter_test.go` (function-based mock pattern: add `estimateCostFunc` field + method that calls it or returns `Unimplemented`)
- [X] T006 [P] Add `EstimateCost` stub method to `mockCostSourceClient` in `internal/engine/budget_engine_test.go` (returns `Unimplemented` status error)
- [X] T007 [P] Add `EstimateCost` stub method to `mockCostSourceClient` in `test/integration/budget_health_test.go` (returns `Unimplemented` status error)
- [X] T008 Run `go build ./...` to confirm all mock implementations satisfy the expanded interface

**Checkpoint**: Project compiles with expanded interface. All existing tests still pass.

---

## Phase 3: User Story 3 — Adapter Builds Valid Proto Requests (Priority: P1, Prerequisite for US1)

**Goal**: Replace the `BuildEstimateCostRequest` stub with real request
construction that maps `ResourceDescriptor` + properties to
`pbc.EstimateCostRequest`.

**Independent Test**: Call `BuildEstimateCostRequest` with various input
combinations and verify the resulting proto message has correct fields.

### Tests for User Story 3 (MANDATORY — TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T009 [P] [US3] Write test `TestBuildEstimateCostRequest_ValidResource` in `internal/proto/adapter_test.go` — given a `ResourceDescriptor` with Type and Properties, verify returned `pbc.EstimateCostRequest` has `ResourceType` and `Attributes` populated correctly
- [X] T010 [P] [US3] Write test `TestBuildEstimateCostRequest_NilResource` in `internal/proto/adapter_test.go` — given nil resource, verify error is returned
- [X] T011 [P] [US3] Write test `TestBuildEstimateCostRequest_EmptyType` in `internal/proto/adapter_test.go` — given resource with empty Type, verify error is returned
- [X] T012 [P] [US3] Write test `TestBuildEstimateCostRequest_NilProperties` in `internal/proto/adapter_test.go` — given resource with nil Properties, verify request is built with empty Attributes struct

### Implementation for User Story 3

- [X] T013 [US3] Replace `BuildEstimateCostRequest` stub in `internal/proto/adapter.go` — change signature to accept `(resource *ResourceDescriptor, properties map[string]interface{}) (*pbc.EstimateCostRequest, error)`, add `structpb` import, validate inputs, use `structpb.NewStruct(properties)` to build Attributes
- [X] T014 [US3] Remove dead internal types from `internal/proto/adapter.go`: `EstimateCostRequest` (line ~1277), `EstimateCostResponse` (line ~1289), `CostDelta` (line ~1301)
- [X] T015 [US3] Remove `ErrEstimateCostNotSupported` sentinel error from `internal/proto/adapter.go` (line ~24) and verify no remaining references

**Checkpoint**: `BuildEstimateCostRequest` produces valid `pbc.EstimateCostRequest` messages. All T009-T012 tests pass.

---

## Phase 4: User Story 1 — Plugin-Powered What-If Estimation (Priority: P1)

**Goal**: Replace the `tryEstimateCostRPC` stub with real implementation that
calls the plugin's `EstimateCost` RPC twice (baseline + modified), converts
responses to `CostResult`, and computes deltas.

**Independent Test**: Call `EstimateCost` on the engine with a mock plugin that
implements the RPC, verifying that the response contains baseline, modified,
and delta data from the plugin (not from the fallback path).

### Tests for User Story 1 (MANDATORY — TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T016 [P] [US1] Write test `TestTryEstimateCostRPC_Success` in `internal/engine/estimate_test.go` — mock plugin returns valid `EstimateCostResponse` for both calls, verify `EstimateResult` has correct baseline/modified costs, `TotalChange` is the delta, and `UsedFallback` is false
- [X] T017 [P] [US1] Write test `TestTryEstimateCostRPC_SinglePropertyDelta` in `internal/engine/estimate_test.go` — single override produces one `CostDelta` entry with correct property name and cost change
- [X] T018 [P] [US1] Write test `TestTryEstimateCostRPC_MultiPropertyCombinedDelta` in `internal/engine/estimate_test.go` — multiple overrides produce a "combined" delta (same as fallback behavior)
- [X] T019 [P] [US1] Write test `TestTryEstimateCostRPC_NilResponse` in `internal/engine/estimate_test.go` — plugin returns nil response, verify error is returned
- [X] T020 [P] [US1] Write test `TestTryEstimateCostRPC_NegativeCost` in `internal/engine/estimate_test.go` — plugin returns negative `CostMonthly`, verify error is returned
- [X] T021 [P] [US1] Write test `TestTryEstimateCostRPC_EmptyCurrency` in `internal/engine/estimate_test.go` — plugin returns empty currency, verify default "USD" is used
- [X] T021a [P] [US1] Write test `TestTryEstimateCostRPC_CurrencyPassthrough` in `internal/engine/estimate_test.go` — plugin returns non-USD currency (e.g., "EUR"), verify currency is preserved without conversion
- [X] T021b [P] [US1] Write test `TestTryEstimateCostRPC_NilExpiresAt` in `internal/engine/estimate_test.go` — verify response conversion leaves `CostResult.ExpiresAt` nil since `EstimateCostResponse` proto lacks `expires_at`
- [X] T021c [P] [US1] Write test `TestEstimateCost_EmptyOverrides` in `internal/engine/estimate_test.go` — call `EstimateCost` with empty `PropertyOverrides` map, verify validation rejects the request (no meaningful comparison without overrides)

### Implementation for User Story 1

- [X] T022 [US1] Implement `tryEstimateCostRPC` in `internal/engine/estimate.go` — build baseline request via `proto.BuildEstimateCostRequest(resource, resource.Properties)`, call `client.API.EstimateCost(ctx, baselineReq)`, deep copy + merge overrides into properties, build modified request, call `client.API.EstimateCost(ctx, modifiedReq)`, convert both responses to `CostResult`, compute `TotalChange` and per-property deltas, return `EstimateResult` with `UsedFallback = false`
- [X] T023 [US1] Implement response-to-CostResult conversion helper in `internal/engine/estimate.go` — map `CostMonthly` → `MonthlyCost`, derive `HourlyCost` (÷730), copy `Currency` (default "USD"), append `PricingCategory`/`SpotRisk` to Notes, validate non-nil response and non-negative cost

**Checkpoint**: Engine calls plugin's `EstimateCost` RPC for what-if analysis. All T016-T021 tests pass. `UsedFallback` is false when RPC succeeds.

---

## Phase 5: User Story 2 — Graceful Fallback to Double-GetProjectedCost (Priority: P2)

**Goal**: Verify that when a plugin returns `Unimplemented` for `EstimateCost`,
the engine transparently falls back to the existing double-`GetProjectedCost`
strategy with `UsedFallback = true`.

**Independent Test**: Call `EstimateCost` with a mock plugin that returns
`Unimplemented` and verify the engine produces a result with
`UsedFallback = true`.

### Tests for User Story 2 (MANDATORY — TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T024 [P] [US2] Write test `TestEstimateCost_FallbackOnUnimplemented` in `internal/engine/estimate_test.go` — mock plugin returns `Unimplemented` for `EstimateCost`, verify engine still returns valid `EstimateResult` via fallback with `UsedFallback = true`
- [X] T025 [P] [US2] Write test `TestEstimateCost_MultiPlugin_FirstUnimplemented` in `internal/engine/estimate_test.go` — first plugin returns `Unimplemented`, second implements RPC, verify engine uses second plugin's response with `UsedFallback = false`

### Implementation for User Story 2

- [X] T026 [US2] Verify `tryEstimateCostRPC` correctly returns `Unimplemented` status error so the existing engine loop (estimate.go:80-118) can handle fallback — no new implementation code expected, this should already work from T022; confirm with tests

**Checkpoint**: Plugins that don't implement `EstimateCost` fall through to the existing fallback. `UsedFallback = true` in fallback results.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Cleanup, validation, and documentation.

- [X] T027 [P] Update CHANGELOG.md with entry for EstimateCost RPC consumer implementation (handled by release-please from conventional commit)
- [X] T028 Run `make test` to verify all tests pass (unit + existing integration)
- [X] T029 Run `make lint` to verify no lint errors
- [X] T030 Verify no remaining references to `ErrEstimateCostNotSupported` in the codebase via grep
- [X] T031 Verify no remaining references to dead internal types (`proto.EstimateCostRequest`, `proto.EstimateCostResponse`, `proto.CostDelta`) via grep
- [X] T032 Run quickstart.md validation — run `finfocus cost estimate --help` to verify command is wired; with recorder plugin installed, run a cost estimate to verify fallback path executes without error

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories
- **User Story 3 (Phase 3)**: Depends on Foundational — BLOCKS User Story 1
- **User Story 1 (Phase 4)**: Depends on User Story 3 (`BuildEstimateCostRequest` is needed by `tryEstimateCostRPC`)
- **User Story 2 (Phase 5)**: Depends on User Story 1 (verifies fallback behavior of the real implementation)
- **Polish (Phase 6)**: Depends on all user stories being complete

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Interface changes before implementation
- Request building before RPC calling
- Story complete before moving to next priority

### Parallel Opportunities

- T005, T006, T007 can run in parallel (different mock files)
- T009, T010, T011, T012 can run in parallel (same file but independent test functions)
- T016-T021c can run in parallel (independent test functions)
- T024, T025 can run in parallel (independent test functions)
- T027 can run in parallel with T028-T032

---

## Parallel Example: User Story 3 (Request Builder)

```bash
# Launch all tests together:
Task: "TestBuildEstimateCostRequest_ValidResource in internal/proto/adapter_test.go"
Task: "TestBuildEstimateCostRequest_NilResource in internal/proto/adapter_test.go"
Task: "TestBuildEstimateCostRequest_EmptyType in internal/proto/adapter_test.go"
Task: "TestBuildEstimateCostRequest_NilProperties in internal/proto/adapter_test.go"
```

## Parallel Example: User Story 1 (RPC Implementation)

```bash
# Launch all tests together:
Task: "TestTryEstimateCostRPC_Success in internal/engine/estimate_test.go"
Task: "TestTryEstimateCostRPC_SinglePropertyDelta in internal/engine/estimate_test.go"
Task: "TestTryEstimateCostRPC_NilResponse in internal/engine/estimate_test.go"
```

---

## Implementation Strategy

### MVP First (User Stories 3 + 1)

1. Complete Phase 1: Setup (verify proto types available)
2. Complete Phase 2: Foundational (interface + mocks compile)
3. Complete Phase 3: User Story 3 — `BuildEstimateCostRequest` works
4. Complete Phase 4: User Story 1 — `tryEstimateCostRPC` works
5. **STOP and VALIDATE**: `make test && make lint` pass

### Incremental Delivery

1. Setup + Foundational → Project compiles with expanded interface
2. User Story 3 → Request builder works → Tests pass
3. User Story 1 → Full RPC pipeline works → Tests pass (MVP!)
4. User Story 2 → Fallback verified → Tests pass
5. Polish → CHANGELOG, final validation

### Key Files Modified

| File | Tasks | Change Summary |
|------|-------|---------------|
| `internal/proto/adapter.go` | T003, T004, T013, T014, T015 | Interface + clientAdapter + request builder + dead code removal |
| `internal/proto/adapter_test.go` | T005, T009-T012 | Mock update + request builder tests |
| `internal/engine/estimate.go` | T022, T023 | Real `tryEstimateCostRPC` + response conversion |
| `internal/engine/estimate_test.go` | T016-T021c, T024-T025 | RPC success/fallback/error/edge-case tests |
| `internal/engine/budget_engine_test.go` | T006 | Mock `EstimateCost` stub |
| `test/integration/budget_health_test.go` | T007 | Mock `EstimateCost` stub |

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- User Story 4 (Interface Completeness) is handled in Phase 2 as foundational work (no story label per template rules)
- User Story 3 (Request Builder) is ordered before User Story 1 because US1 depends on it
- The existing `estimateCostFallback` function is NOT modified
- ~150-200 lines of new code + tests (per plan.md estimate)
- FR-007 (expires_at) is a confirmed no-op: proto lacks the field, ExpiresAt stays nil, covered by T021b
- FR-008 (response validation) covers nil response (T019), negative cost (T020), empty currency (T021)
