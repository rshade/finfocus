# Tasks: Cost Estimate Command for What-If Scenario Modeling

**Input**: Design documents from `/specs/223-cost-estimate/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

This is a Go CLI project with the following structure:

- **CLI**: `internal/cli/`
- **Engine**: `internal/engine/`
- **Proto/Adapter**: `internal/proto/`
- **TUI**: `internal/tui/`
- **Tests**: `internal/*/` (alongside implementation), `test/integration/`, `test/fixtures/`

---

## Phase 1: Setup

**Purpose**: Create test fixtures and foundational types needed by all user stories

- [X] T001 Create test fixture directory structure at test/fixtures/estimate/
- [X] T002 [P] Create single-resource test fixture at test/fixtures/estimate/single-resource.json
- [X] T003 [P] Create plan-with-modify test fixture at test/fixtures/estimate/plan-with-modify.json

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and engine infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Add EstimateResult and CostDelta types to internal/engine/types.go
- [X] T005 [P] Write unit tests for EstimateResult and CostDelta types in internal/engine/types_test.go
- [X] T006 Create estimate.go with EstimateRequest type and Engine.EstimateCost method signature in internal/engine/estimate.go
- [X] T007 [P] Write unit tests for EstimateCost method in internal/engine/estimate_test.go
- [X] T008 Implement EstimateCost method with fallback logic (try EstimateCost RPC, fall back to double GetProjectedCost) in internal/engine/estimate.go
- [X] T009 Add EstimateCost request builder and validation to internal/proto/adapter.go
- [X] T010 [P] Write unit tests for EstimateCost adapter functions in internal/proto/adapter_test.go

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Quick Property Changes (Priority: P1) MVP

**Goal**: Developers can quickly evaluate cost impact of changing a single resource configuration without modifying Pulumi code

**Independent Test**: Run `finfocus cost estimate --provider aws --resource-type ec2:Instance --property instanceType=m5.large` and verify baseline/modified cost output

### Tests for User Story 1 (MANDATORY - TDD Required)

- [X] T011 [P] [US1] Write unit tests for NewCostEstimateCmd() flag parsing in internal/cli/cost_estimate_test.go
- [X] T012 [P] [US1] Write unit tests for parsePropertyOverrides() function in internal/cli/cost_estimate_test.go
- [X] T013 [P] [US1] Write unit tests for validateEstimateFlags() mutual exclusivity in internal/cli/cost_estimate_test.go
- [X] T014 [P] [US1] Write unit tests for single-resource execution flow including "no properties specified shows baseline only" edge case in internal/cli/cost_estimate_test.go

### Implementation for User Story 1

- [X] T015 [US1] Create costEstimateParams struct and NewCostEstimateCmd() with all flags (--provider, --resource-type, --property, --pulumi-json, --modify, --interactive, --output, --region, --adapter) in internal/cli/cost_estimate.go
- [X] T016 [US1] Implement parsePropertyOverrides() to parse --property key=value flags in internal/cli/cost_estimate.go
- [X] T017 [US1] Implement validateEstimateFlags() for mutual exclusivity validation in internal/cli/cost_estimate.go
- [X] T018 [US1] Implement executeCostEstimate() for single-resource mode calling Engine.EstimateCost in internal/cli/cost_estimate.go
- [X] T019 [US1] Add estimate command registration to cost command group in internal/cli/cost.go
- [X] T020 [US1] Implement RenderEstimateResult() for table output with color-coded deltas in internal/engine/estimate.go
- [X] T021 [US1] Implement RenderEstimateResult() for JSON and NDJSON output formats in internal/engine/estimate.go

**Checkpoint**: User Story 1 complete - single-resource estimation works independently

---

## Phase 4: User Story 2 - Batch Modifications on Existing Plans (Priority: P2)

**Goal**: Developers can estimate cost impact of modifying resources in an existing Pulumi preview JSON file

**Independent Test**: Run `finfocus cost estimate --pulumi-json plan.json --modify "web-server:instanceType=m5.large"` and verify modified costs

### Tests for User Story 2 (MANDATORY - TDD Required)

- [X] T022 [P] [US2] Write unit tests for parseModifications() function in internal/cli/cost_estimate_test.go
- [X] T023 [P] [US2] Write unit tests for plan-based execution flow in internal/cli/cost_estimate_test.go
- [X] T024 [P] [US2] Write unit tests for resource-not-found error handling in internal/cli/cost_estimate_test.go

### Implementation for User Story 2

- [X] T025 [US2] Implement parseModifications() to parse --modify resource:key=value flags in internal/cli/cost_estimate.go
- [X] T026 [US2] Implement plan-based mode in executeCostEstimate() loading plan via ingest package in internal/cli/cost_estimate.go
- [X] T027 [US2] Implement applyModificationsToResources() to merge overrides into plan resources in internal/cli/cost_estimate.go
- [X] T028 [US2] Implement resource-not-found error handling with clear error messages in internal/cli/cost_estimate.go
- [X] T029 [US2] Implement multi-resource delta rendering for plan-based results in internal/engine/estimate.go

**Checkpoint**: User Story 2 complete - plan-based estimation works independently

---

## Phase 5: User Story 3 - Interactive TUI Exploration (Priority: P3)

**Goal**: Developers can interactively explore property configurations with live cost updates

**Independent Test**: Run `finfocus cost estimate --interactive`, modify a property, and verify cost display updates

### Tests for User Story 3 (MANDATORY - TDD Required)

- [X] T030 [P] [US3] Write unit tests for EstimateModel initialization in internal/tui/estimate_model_test.go
- [X] T031 [P] [US3] Write unit tests for EstimateModel Update() message handling in internal/tui/estimate_model_test.go
- [X] T032 [P] [US3] Write unit tests for delta visualization component in internal/tui/delta_view_test.go

### Implementation for User Story 3

- [X] T033 [US3] Create EstimateModel with property editor state in internal/tui/estimate_model.go
- [X] T034 [US3] Implement Init(), Update(), View() for EstimateModel in internal/tui/estimate_model.go
- [X] T035 [US3] Create DeltaView component for cost delta visualization in internal/tui/delta_view.go
- [X] T036 [US3] Implement property editing with keyboard navigation in internal/tui/estimate_model.go
- [X] T037 [US3] Implement async cost recalculation on property change in internal/tui/estimate_model.go
- [X] T038 [US3] Wire --interactive flag to launch TUI in executeCostEstimate() in internal/cli/cost_estimate.go
- [X] T039 [US3] Implement clean exit on 'q' or Ctrl+C in internal/tui/estimate_model.go

**Checkpoint**: User Story 3 complete - interactive TUI works independently

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Integration tests, documentation, and final quality gates

- [X] T040 [P] Create integration test for single-resource estimation with mock plugin including "property doesn't affect pricing shows $0.00 delta" edge case in test/integration/cost_estimate_test.go
- [X] T041 [P] Create integration test for plan-based estimation with fixtures in test/integration/cost_estimate_test.go
- [X] T042 [P] Create integration test for fallback behavior when EstimateCost not implemented in test/integration/cost_estimate_test.go
- [X] T043 Update CLI documentation for cost estimate command in docs/reference/cli-commands.md
- [ ] T044 Add cost estimate examples to getting-started documentation in docs/getting-started/
- [X] T045 Run make lint and fix all linting errors
- [X] T046 Run make test and ensure 80%+ coverage for new code
- [X] T047 Validate all acceptance scenarios from spec.md pass
- [X] T048 Create performance benchmark validating SC-004 (90% of single-resource estimates complete within 5 seconds) in internal/engine/estimate_test.go

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational completion
- **User Story 2 (Phase 4)**: Depends on Foundational completion (can run parallel with US1)
- **User Story 3 (Phase 5)**: Depends on Foundational completion (can run parallel with US1/US2)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Independent of US1
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Independent of US1/US2

### Within Each User Story

- Tests MUST be written and FAIL before implementation (TDD)
- Tests before implementation tasks
- Core implementation before output formatting
- Story complete before moving to next priority

### Parallel Opportunities

- T002, T003 can run in parallel (different fixture files)
- T005, T007, T010 can run in parallel (different test files)
- T011-T014 can run in parallel (all US1 tests)
- T022-T024 can run in parallel (all US2 tests)
- T030-T032 can run in parallel (all US3 tests)
- T040-T042 can run in parallel (different integration test scenarios)

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Write unit tests for NewCostEstimateCmd() flag parsing in internal/cli/cost_estimate_test.go"
Task: "Write unit tests for parsePropertyOverrides() function in internal/cli/cost_estimate_test.go"
Task: "Write unit tests for validateEstimateFlags() mutual exclusivity in internal/cli/cost_estimate_test.go"
Task: "Write unit tests for single-resource execution flow in internal/cli/cost_estimate_test.go"
```

---

## Parallel Example: Foundational Phase

```bash
# Launch all foundational tests together:
Task: "Write unit tests for EstimateResult and CostDelta types in internal/engine/types_test.go"
Task: "Write unit tests for EstimateCost method in internal/engine/estimate_test.go"
Task: "Write unit tests for EstimateCost adapter functions in internal/proto/adapter_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (fixtures)
2. Complete Phase 2: Foundational (types, engine method)
3. Complete Phase 3: User Story 1 (single-resource estimation)
4. **STOP and VALIDATE**: Test `finfocus cost estimate --provider aws --resource-type ec2:Instance --property instanceType=m5.large`
5. Run `make lint && make test`
6. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → MVP complete!
3. Add User Story 2 → Test independently → Plan-based estimation
4. Add User Story 3 → Test independently → Interactive TUI
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. All team members complete Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (single-resource)
   - Developer B: User Story 2 (plan-based)
   - Developer C: User Story 3 (TUI)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing
- Run `make lint && make test` before marking any phase complete
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
