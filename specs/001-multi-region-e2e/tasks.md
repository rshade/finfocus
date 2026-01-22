---
description: 'Task list template for feature implementation'
---

# Tasks: Add multi-region E2E testing support

**Input**: Design documents from `/specs/001-multi-region-e2e/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/
**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).
**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.
**Documentation**: Per Constitution Principle IV, documentation (README, docs/) MUST be updated concurrently with implementation to prevent drift.
**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `src/`, `tests/` at repository root
- **Web app**: `backend/src/`, `frontend/src/`
- **Mobile**: `api/src/` or `ios/src/` or `android/src/`
- Paths shown below assume single project - adjust based on plan.md structure

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Create multi-region test fixture directory structure in test/e2e/fixtures/multi-region/
- [x] T002 [P] Configure linting and formatting for new test files

---

**Note**: Phase 2 (Foundational) tasks have been merged into User Story phases. Data models, validation logic, and configuration are created as needed within each user story to avoid upfront work that may not be used.

**Checkpoint**: Setup complete - user story implementation can begin

---

## Phase 3: User Story 1 - Run E2E Tests Across Multiple Regions (Priority: P1) 🎯 MVP

**Goal**: Validate both projected and actual cost calculations across different AWS regions with varying pricing

**Independent Test**: Can be fully tested by running the E2E test suite with region parameters and verifying both projected and actual cost outputs match expected regional rates

### Implementation for User Story 1

- [x] T003 [P] [US1] Create us-east-1 Pulumi.yaml with YAML runtime and 8 resources (2 EC2, 2 EBS, 2 Network, 2 RDS) in test/e2e/fixtures/multi-region/us-east-1/Pulumi.yaml
- [x] T004 [P] [US1] Create eu-west-1 Pulumi.yaml with YAML runtime and 8 resources in test/e2e/fixtures/multi-region/eu-west-1/Pulumi.yaml
- [x] T005 [P] [US1] Create ap-northeast-1 Pulumi.yaml with YAML runtime and 8 resources in test/e2e/fixtures/multi-region/ap-northeast-1/Pulumi.yaml
- [x] T006 [P] [US1] Generate expected-costs.json for us-east-1 in test/e2e/fixtures/multi-region/us-east-1/expected-costs.json (source: AWS pricing pages, apply ±5% tolerance)
- [x] T007 [P] [US1] Generate expected-costs.json for eu-west-1 in test/e2e/fixtures/multi-region/eu-west-1/expected-costs.json (source: AWS pricing pages, apply ±5% tolerance)
- [x] T008 [P] [US1] Generate expected-costs.json for ap-northeast-1 in test/e2e/fixtures/multi-region/ap-northeast-1/expected-costs.json (source: AWS pricing pages, apply ±5% tolerance)
- [x] T009 [US1] Implement multi-region projected cost test in test/e2e/multi_region_projected_test.go
- [x] T010 [US1] Implement multi-region actual cost test in test/e2e/multi_region_actual_test.go
- [x] T011 [US1] Add region-specific plugin loading validation in test/e2e/multi_region_projected_test.go
- [x] T012 [US1] Integrate cost validation with ±5% tolerance in test/e2e/multi_region_projected_test.go
- [x] T013 [US1] Add network failure retry logic in test/e2e/multi_region_actual_test.go
- [x] T014 [US1] Implement strict failure semantics for missing pricing data in test/e2e/multi_region_helpers.go
- [x] T015 [US1] Update test/e2e/README.md with multi-region testing documentation

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Validate Region-Specific Plugin Loading (Priority: P2)

**Goal**: Ensure that region-specific plugin binaries are loaded correctly during E2E tests

**Independent Test**: Can be tested by verifying which plugin version is loaded and used for cost calculations in each region

### Implementation for User Story 2

- [x] T016 [P] [US2] Add plugin version tracking in test execution logs in test/e2e/multi_region_helpers.go
- [x] T017 [US2] Implement plugin loading verification in test/e2e/multi_region_projected_test.go
- [x] T018 [US2] Add region-specific plugin binary validation in test/e2e/multi_region_actual_test.go
- [x] T019 [US2] Extend test configuration to include expected plugin versions in test/e2e/config.go

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Test Fallback Behavior (Priority: P3)

**Goal**: Test fallback behavior when region-specific plugins are unavailable

**Independent Test**: Can be tested by simulating missing region-specific plugins and verifying fallback to public pricing data

### Implementation for User Story 3

- [x] T020 [P] [US3] Create fallback test scenarios in test/e2e/multi_region_fallback_test.go
- [x] T021 [US3] Implement plugin unavailability simulation in test/e2e/multi_region_fallback_test.go
- [x] T022 [US3] Add fallback validation logic for public pricing in test/e2e/multi_region_fallback_test.go

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [x] T023 Add performance validation for <5 minutes per region in test/e2e/multi_region_projected_test.go
- [x] T024 [P] Add documentation for multi-region E2E testing in docs/testing/multi-region-e2e.md
- [x] T025 Code cleanup and refactoring in test/e2e/ multi-region files
- [x] T026 [P] Run quickstart.md validation steps
- [x] T027 Update CLAUDE.md with new Active Technologies section

---

## Phase 7: User Story 4 - Unified Multi-Region Fixture Testing (Priority: P3)

**Goal**: Test cost calculations for a single Pulumi program that deploys resources across multiple AWS regions using YAML runtime with explicit provider aliases

**Independent Test**: Can be tested by running the unified fixture through the projected cost command and validating per-region cost attribution and aggregate totals within ±5% tolerance

**Key Differences from Per-Region Fixtures (US1-3)**:

- **Structure**: Per-region has 3 separate `Pulumi.yaml` files (one per region), Unified has 1 `Pulumi.yaml` with all regions
- **Providers**: Per-region uses single implicit AWS provider, Unified uses explicit provider aliases per region
- **Validation**: Per-region validates per-region only, Unified validates per-resource + aggregate total

### Implementation for User Story 4

- [x] T028 [US4] Run discovery test to determine plugin behavior with multi-region plans - execute `pulumi preview --json` on unified fixture and analyze region detection in test/e2e/fixtures/multi-region/unified/
- [x] T029 [P] [US4] Create unified fixture directory structure in test/e2e/fixtures/multi-region/unified/
- [x] T030 [US4] Create Pulumi.yaml with YAML runtime defining 3 explicit AWS providers (us-east-1, eu-west-1, ap-northeast-1) and 3 EC2 instances in test/e2e/fixtures/multi-region/unified/Pulumi.yaml
- [x] T031 [US4] Generate expected-costs.json with per-resource costs and aggregate validation bounds in test/e2e/fixtures/multi-region/unified/expected-costs.json
- [x] T032 [US4] Add UnifiedExpectedCosts and AggregateExpectation structs to test/e2e/multi_region_helpers.go
- [x] T033 [US4] Implement ValidateUnifiedFixtureCosts function with per-resource and aggregate validation in test/e2e/validator.go
- [x] T034 [US4] Create test function TestMultiRegion_Unified_Projected with per-resource cost assertions, region attribution validation, and aggregate total validation in test/e2e/multi_region_projected_test.go
- [x] T035 [US4] Update docs/testing/multi-region-e2e.md with unified fixture section including troubleshooting for region detection issues

**Checkpoint**: Unified multi-region fixture testing should validate:

1. Each resource's cost reflects its region-specific pricing
2. Aggregate total equals sum of individual region costs within ±5% tolerance
3. Plugin correctly extracts regions from provider configuration in plan JSON

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **User Story 1 (Phase 3)**: Depends on Setup - core multi-region testing
- **User Story 2 (Phase 4)**: Can start after Setup - plugin loading verification
- **User Story 3 (Phase 5)**: Can start after Setup - fallback behavior tests
- **Polish (Phase 6)**: Depends on US1-3 being complete
- **User Story 4 (Phase 7)**: Can start after Setup - unified multi-region fixture (independent of US1-3)

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Setup - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Setup - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Setup - May integrate with US1/US2 but should be independently testable
- **User Story 4 (P3)**: Can start after Setup - Tests unified multi-region fixtures with YAML runtime (independent track)

### Within Each User Story

- Tests (if included) MUST be written and FAIL before implementation
- Models before services
- Services before endpoints
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Models within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all Pulumi.yaml fixture creation tasks for User Story 1 together:
Task: "Create us-east-1 Pulumi.yaml with YAML runtime in test/e2e/fixtures/multi-region/us-east-1/Pulumi.yaml"
Task: "Create eu-west-1 Pulumi.yaml with YAML runtime in test/e2e/fixtures/multi-region/eu-west-1/Pulumi.yaml"
Task: "Create ap-northeast-1 Pulumi.yaml with YAML runtime in test/e2e/fixtures/multi-region/ap-northeast-1/Pulumi.yaml"

# Launch all expected-costs.json generation tasks together:
Task: "Generate expected-costs.json for us-east-1 in test/e2e/fixtures/multi-region/us-east-1/expected-costs.json"
Task: "Generate expected-costs.json for eu-west-1 in test/e2e/fixtures/multi-region/eu-west-1/expected-costs.json"
Task: "Generate expected-costs.json for ap-northeast-1 in test/e2e/fixtures/multi-region/ap-northeast-1/expected-costs.json"
```

---

## Parallel Example: User Story 4

```bash
# After T035 discovery completes, these can run in parallel:
Task: "Create unified fixture directory structure in test/e2e/fixtures/multi-region/unified/"
Task: "Add UnifiedExpectedCosts and AggregateExpectation structs to test/e2e/multi_region_helpers.go"

# After directory exists, these depend on T036:
Task: "Create Pulumi.yaml with YAML runtime in test/e2e/fixtures/multi-region/unified/Pulumi.yaml"
Task: "Generate expected-costs.json in test/e2e/fixtures/multi-region/unified/expected-costs.json"

# After structs exist, validation can proceed:
Task: "Implement ValidateUnifiedFixtureCosts in test/e2e/validator.go"

# Test function depends on all above:
Task: "Create TestMultiRegion_Unified_Projected in test/e2e/multi_region_projected_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
