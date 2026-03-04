# Tasks: Recognize Batch Cost Capability

**Input**: Design documents from `/specs/605-batch-cost-capability/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness),
all tasks MUST be fully implemented. Stub functions, placeholders, and TODO
comments are strictly forbidden.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

## User Story 1 — Already Implemented

> **US1 (Plugin List Display)** requires no new code. Research confirmed that
> `ConvertCapabilities()` in `internal/pluginhost/host.go:194-195` already maps
> `PLUGIN_CAPABILITY_BATCH_COST` → `"batch_cost"`, and `plugin list` display
> is capability-agnostic. Existing test `TestConvertCapabilities` in
> `internal/pluginhost/host_test.go` already covers batch\_cost. FR-001,
> FR-002, FR-003 are satisfied by existing code.

---

## Phase 1: User Story 2 — Router Recognizes Batch Cost Capability (Priority: P2)

**Goal**: Add `FeatureBatchCost` to the router's feature/capability mapping so
the router can detect and match plugins that support batch cost queries.

**Independent Test**: Run `go test -v -run TestBatchCost ./internal/router/...`
and verify new feature constant is recognized, maps correctly to proto enum,
and normalizes in both PascalCase and snake\_case forms.

### Tests for User Story 2 (TDD — write first)

- [X] T001 [P] [US2] Add `TestIsValidFeature` case for `"BatchCost"` and update `TestValidFeatures` count from 6 to 7 with `FeatureBatchCost` in expected slice in `internal/router/features_test.go`
- [X] T002 [P] [US2] Add `TestFeatureFromMethod` case for `"BatchCost"` → `FeatureBatchCost` in `internal/router/features_test.go`
- [X] T003 [P] [US2] Add test cases for `capabilityEnumFromFeature` (BatchCost → PLUGIN_CAPABILITY_BATCH_COST) and `capabilityEnumFromString` ("BatchCost" and "batch_cost") in `internal/router/router_test.go`

### Implementation for User Story 2

- [X] T004 [US2] Add `FeatureBatchCost Feature = "BatchCost"` constant with godoc, append to `ValidFeatures()` return slice, and add `"BatchCost": FeatureBatchCost` to `methodToFeature` map in `internal/router/features.go`
- [X] T005 [US2] Add `case FeatureBatchCost` → `PLUGIN_CAPABILITY_BATCH_COST` in `capabilityEnumFromFeature()` and add `case "BatchCost", "batch_cost"` → `PLUGIN_CAPABILITY_BATCH_COST` in `capabilityEnumFromString()` in `internal/router/router.go`

**Checkpoint**: Router recognizes BatchCost as a valid feature and correctly
maps it between Feature constants, proto enums, and display strings.

---

## Phase 2: Validation

**Purpose**: Verify all changes pass quality gates

- [X] T006 Run `make test` to verify all unit tests pass with no regressions
- [X] T007 Run `make lint` to verify linting passes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (US2)**: No setup or foundational prerequisites — changes are
  additive to existing router code
- **Phase 2 (Validation)**: Depends on Phase 1 completion

### Within Phase 1

- T001, T002, T003 (tests) can run in parallel — different test functions
- T004 must complete before T005 (T005 references the constant from T004)
- T001+T002 target `features_test.go`, T003 targets `router_test.go`
- Tests should be written first (TDD), then implementation (T004, T005)

### Parallel Opportunities

- T001, T002, T003 are all [P] — write all test cases in parallel
- T004 and T005 are sequential (same dependency chain)

---

## Parallel Example: User Story 2

```text
# Write all tests in parallel (TDD):
T001: Update TestIsValidFeature + TestValidFeatures in features_test.go
T002: Add TestFeatureFromMethod case in features_test.go
T003: Add capabilityEnum mapping tests in router_test.go

# Then implement sequentially:
T004: Add constant + ValidFeatures() + methodToFeature in features.go
T005: Add switch cases in router.go
```

---

## Implementation Strategy

### MVP (this feature is small enough for single delivery)

1. Write tests (T001-T003) — verify they fail
2. Implement (T004-T005) — verify tests pass
3. Validate (T006-T007) — `make test && make lint`

---

## Notes

- US1 requires zero code changes (already implemented in pluginhost)
- All production changes are in `internal/router/` (2 files)
- All test changes are in `internal/router/` (2 files)
- Follow the `FeatureBudgets` pattern exactly — it was the most recent addition
- The `sync.Once` cache in `validFeatureNameSet()` is initialized lazily, so
  adding to `ValidFeatures()` is safe without any cache invalidation
