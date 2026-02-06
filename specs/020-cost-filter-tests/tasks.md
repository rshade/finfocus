---
description: 'Task list for integration tests for --filter flag'
---

# Tasks: Integration Tests for --filter Flag

**Input**: Design documents from `/specs/020-cost-filter-tests/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **CLI tool**: `test/integration/cli/`, `test/fixtures/plans/` at repository root
- Paths follow existing project structure

## Phase 1: Setup (Project Readiness)

**Purpose**: Ensure development environment is ready for test implementation

- [x] T001 Verify existing test infrastructure works in test/integration/helpers/cli_helper.go
- [x] T002 Confirm Go 1.25.7 and testify dependencies are available
- [x] T003 Test existing integration test execution with `make test`

---

## Phase 2: Foundational (Test Infrastructure)

**Purpose**: Core testing infrastructure that MUST be complete before ANY user story tests can be implemented

**⚠️ CRITICAL**: No user story test work can begin until this phase is complete

- [x] T004 Create multi-resource test fixture in test/fixtures/plans/multi-resource-plan.json with 10-20 resources across AWS, Azure, GCP
- [x] T005 Verify existing CLIHelper properly captures command output and handles temporary files
- [x] T006 Test cost projected command execution without filters (baseline functionality)
- [x] T007 Test cost actual command execution with --group-by tag filtering (baseline functionality)

**Checkpoint**: Test infrastructure ready - user story test implementation can now begin

---

## Phase 3: User Story 1 - Projected Cost Filtering by Type and Provider (Priority: P1) 🎯 MVP

**Goal**: Implement integration tests for filtering projected costs by resource type and provider

**Independent Test**: Run `cost projected --filter "type=aws:ec2/instance"` against test fixture and verify only EC2 instances appear in output

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T008 [P] [US1] Create test file structure in test/integration/cli/filter_test.go with basic setup
- [x] T009 [P] [US1] Write TestProjectedCost_FilterByType test function for exact type matching
- [x] T010 [P] [US1] Write TestProjectedCost_FilterByTypeSubstring test function for partial type matching
- [x] T011 [P] [US1] Write TestProjectedCost_FilterByProvider test function for provider filtering

### Implementation for User Story 1

- [x] T012 [US1] Implement test logic for type filtering validation in test/integration/cli/filter_test.go
- [x] T013 [US1] Implement test logic for provider filtering validation in test/integration/cli/filter_test.go
- [x] T014 [US1] Add test assertions for output format validation (JSON structure)
- [x] T015 [US1] Add resource count validation for filtered results

**Checkpoint**: At this point, User Story 1 tests should pass and validate projected cost filtering independently

---

## Phase 4: User Story 2 - Actual Cost Filtering by Tags (Priority: P1)

**Goal**: Implement integration tests for filtering actual costs by tags using --group-by

**Independent Test**: Run `cost actual --group-by "tag:env=prod"` and verify only resources with env=prod tag are included

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [x] T016 [P] [US2] Write TestActualCost_FilterByTag test function for single tag filtering
- [x] T017 [P] [US2] Write TestActualCost_FilterByTagAndType test function for combined filtering

### Implementation for User Story 2

- [x] T018 [US2] Implement tag filtering test logic in test/integration/cli/filter_test.go
- [x] T019 [US2] Add mock server setup for cost actual command testing
- [x] T020 [US2] Implement result validation for tag-filtered outputs
- [x] T021 [US2] Add assertions for tag-based resource filtering

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Robust Edge Case Handling (Priority: P2)

**Goal**: Implement tests for edge cases including no matches, invalid syntax, and case sensitivity

**Independent Test**: Run filters that should return no results and verify graceful handling

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [x] T022 [P] [US3] Write TestProjectedCost_FilterNoMatch test for empty result handling
- [x] T023 [P] [US3] Write TestProjectedCost_FilterInvalidSyntax test for error message validation
- [x] T024 [P] [US3] Write TestFilter_CaseSensitivity test for case-sensitive behavior

### Implementation for User Story 3

- [x] T025 [US3] Implement no-match scenario validation in test/integration/cli/filter_test.go
- [x] T026 [US3] Add invalid syntax error message checking
- [x] T027 [US3] Implement case sensitivity test assertions
- [x] T028 [US3] Add special character handling tests for filter strings

**Checkpoint**: Edge case handling should now work across all filter operations

---

## Phase 6: User Story 4 - Consistent Output Formats (Priority: P2)

**Goal**: Implement tests ensuring filtering works consistently across table, JSON, and NDJSON outputs

**Independent Test**: Run same filter with different --output flags and verify consistent filtering

### Tests for User Story 4 (MANDATORY - TDD Required) ⚠️

- [x] T029 [P] [US4] Write TestFilter_AllOutputFormats test for table output filtering
- [x] T030 [P] [US4] Write TestFilter_AllOutputFormats test for JSON output filtering
- [x] T031 [x] T031 [P] [US4] Write TestFilter_AllOutputFormats test for NDJSON output filtering

### Implementation for User Story 4

- [x] T032 [US4] Implement output format validation logic in test/integration/cli/filter_test.go
- [x] T033 [US4] Add JSON parsing and structure validation for filtered results
- [x] T034 [US4] Add NDJSON line-by-line validation for filtered results
- [x] T035 [US4] Implement table output parsing and filtering verification

**Checkpoint**: All user stories should now work with consistent output format handling

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final improvements and validation across all implemented tests

- [x] T036 [P] Run complete test suite and verify no regressions with `make test`
- [x] T037 [P] Run linting and verify code quality with `make lint`
- [x] T038 Validate test coverage meets 80% requirement for filter functionality
- [x] T039 [P] Update documentation in README.md if needed for new test capabilities
- [x] T040 Execute quickstart.md validation steps
- [x] T041 [P] Add any missing edge case tests identified during implementation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion
  - User stories can proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2)
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - No dependencies on other stories
- **User Story 2 (P1)**: Can start after Foundational - No dependencies on other stories
- **User Story 3 (P2)**: Can start after Foundational - May reference US1/US2 test patterns
- **User Story 4 (P2)**: Can start after Foundational - May reference US1/US2 test patterns

### Within Each User Story

- Tests MUST be written and FAIL before implementation (TDD requirement)
- Test file creation before individual test functions
- Basic filtering tests before edge cases
- Output format tests can be parallel within story

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel
- All tests for a user story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different developers

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Create test file structure in test/integration/cli/filter_test.go with basic setup"
Task: "Write TestProjectedCost_FilterByType test function for exact type matching"
Task: "Write TestProjectedCost_FilterByTypeSubstring test function for partial type matching"
Task: "Write TestProjectedCost_FilterByProvider test function for provider filtering"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Verify projected cost filtering works end-to-end

### Incremental Delivery

1. Complete Setup + Foundational → Test infrastructure ready
2. Add User Story 1 → Test projected cost type/provider filtering → Validate MVP
3. Add User Story 2 → Test actual cost tag filtering → Validate cost commands
4. Add User Story 3 → Test edge cases → Validate robustness
5. Add User Story 4 → Test output formats → Validate completeness
6. Each story adds test coverage without breaking previous tests

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Stories 1 & 3 (projected costs)
   - Developer B: User Stories 2 & 4 (actual costs and outputs)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different test functions, no file conflicts
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing (TDD requirement)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
