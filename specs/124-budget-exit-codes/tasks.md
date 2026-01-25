# Tasks: Budget Threshold Exit Codes

**Input**: Design documents from `/specs/124-budget-exit-codes/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV, documentation (README, docs/) MUST be updated concurrently with implementation to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Go project**: `internal/` for packages, `test/` for tests
- Paths shown below match the finfocus repository structure

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Branch preparation and structure verification

- [ ] T001 Verify branch `124-budget-exit-codes` is checked out and up-to-date with main
- [ ] T002 [P] Verify all dependencies are installed (`go mod download`)
- [ ] T003 [P] Run existing tests to establish baseline (`make test`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core configuration and error types that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational (MANDATORY - TDD Required) ⚠️

- [ ] T004 [P] Unit test for `ErrExitCodeOutOfRange` error type in `internal/config/budget_test.go`
- [ ] T005 [P] Unit test for `BudgetConfig.GetExitCode()` method in `internal/config/budget_test.go`
- [ ] T006 [P] Unit test for `BudgetConfig.ShouldExitOnThreshold()` method in `internal/config/budget_test.go`
- [ ] T007 [P] Unit test for exit code validation (0-255 range) in `internal/config/budget_test.go`

### Implementation for Foundational

- [ ] T008 Add `ErrExitCodeOutOfRange` error variable in `internal/config/budget.go`
- [ ] T009 Add `ExitOnThreshold` and `ExitCode` fields to `BudgetConfig` struct in `internal/config/budget.go`
- [ ] T010 Implement `GetExitCode()` method (returns 1 if not set) in `internal/config/budget.go`
- [ ] T011 Implement `ShouldExitOnThreshold()` method in `internal/config/budget.go`
- [ ] T012 Add exit code range validation (0-255) to `BudgetConfig.Validate()` in `internal/config/budget.go`

**Checkpoint**: Foundation ready - BudgetConfig can store and validate exit settings

---

## Phase 3: User Story 1 - CI/CD Pipeline Fails on Budget Exceeded (Priority: P1) 🎯 MVP

**Goal**: When budget threshold is exceeded with `exit_on_threshold: true`, CLI exits with configured code

**Independent Test**: Configure `exit_on_threshold: true` in config, run `cost projected` with high-cost plan, verify exit code matches configured value

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T013 [P] [US1] Unit test for `BudgetStatus.ShouldExit()` returns false when disabled in `internal/engine/budget_cli_test.go`
- [ ] T014 [P] [US1] Unit test for `BudgetStatus.ShouldExit()` returns false when no thresholds exceeded in `internal/engine/budget_cli_test.go`
- [ ] T015 [P] [US1] Unit test for `BudgetStatus.ShouldExit()` returns true when enabled AND threshold exceeded in `internal/engine/budget_cli_test.go`
- [ ] T016 [P] [US1] Unit test for `BudgetStatus.GetExitCode()` returns 0 when should not exit in `internal/engine/budget_cli_test.go`
- [ ] T017 [P] [US1] Unit test for `BudgetStatus.GetExitCode()` returns configured code when should exit in `internal/engine/budget_cli_test.go`
- [ ] T018 [P] [US1] Unit test for `BudgetStatus.ExitReason()` returns empty string when no exit in `internal/engine/budget_cli_test.go`
- [ ] T019 [P] [US1] Unit test for `BudgetStatus.ExitReason()` returns descriptive message on exit in `internal/engine/budget_cli_test.go`
- [ ] T020 [US1] Integration test for config file with `exit_on_threshold: true` in `test/integration/budget_exit_test.go`
- [ ] T020a [US1] Unit test for exit code 1 when budget evaluation error occurs (FR-009) in `internal/engine/budget_cli_test.go`

### Implementation for User Story 1

- [ ] T021 [US1] Implement `BudgetStatus.ShouldExit()` method in `internal/engine/budget_cli.go`
- [ ] T022 [US1] Implement `BudgetStatus.GetExitCode()` method in `internal/engine/budget_cli.go`
- [ ] T023 [US1] Implement `BudgetStatus.ExitReason()` method in `internal/engine/budget_cli.go`
- [ ] T024 [US1] Add `checkBudgetExit()` helper function in `internal/cli/cost_budget.go`
- [ ] T025 [US1] Call `checkBudgetExit()` after `renderBudgetIfConfigured()` in `internal/cli/cost_projected.go`
- [ ] T026 [US1] Call `checkBudgetExit()` after `renderBudgetIfConfigured()` in `internal/cli/cost_actual.go`
- [ ] T027 [US1] Add debug logging for exit code evaluation in `internal/cli/cost_budget.go`
- [ ] T027a [US1] Handle budget evaluation errors with exit code 1 (FR-009) in `internal/cli/cost_budget.go`

**Checkpoint**: MVP complete - exit codes work from config file

---

## Phase 4: User Story 2 - Environment-Based Exit Code Configuration (Priority: P2)

**Goal**: DevOps teams can configure exit behavior via environment variables without modifying config files

**Independent Test**: Set `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD=true` and `FINFOCUS_BUDGET_EXIT_CODE=3`, run `cost projected`, verify exit code is 3

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [ ] T028 [P] [US2] Unit test for `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD=true` enables exit in `internal/config/config_test.go`
- [ ] T029 [P] [US2] Unit test for `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD=1` enables exit in `internal/config/config_test.go`
- [ ] T030 [P] [US2] Unit test for `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD=false` disables exit in `internal/config/config_test.go`
- [ ] T031 [P] [US2] Unit test for `FINFOCUS_BUDGET_EXIT_CODE=42` sets exit code in `internal/config/config_test.go`
- [ ] T032 [P] [US2] Unit test for env var overriding config file value in `internal/config/config_test.go`
- [ ] T033 [US2] Integration test for environment variable configuration in `test/integration/budget_exit_test.go`

### Implementation for User Story 2

- [ ] T034 [US2] Add `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD` handling in `applyEnvironmentOverrides()` in `internal/config/config.go`
- [ ] T035 [US2] Add `FINFOCUS_BUDGET_EXIT_CODE` handling in `applyEnvironmentOverrides()` in `internal/config/config.go`

**Checkpoint**: Environment variable configuration works

---

## Phase 5: User Story 3 - Warning Threshold Exit Behavior (Priority: P2)

**Goal**: Explicit `exit_code: 0` provides warning logs without pipeline failure

**Independent Test**: Configure `exit_on_threshold: true` and `exit_code: 0`, trigger threshold, verify exit code is 0 with logged warning

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [ ] T036 [P] [US3] Unit test for `exit_code: 0` with `exit_on_threshold: true` returns exit 0 in `internal/engine/budget_cli_test.go`
- [ ] T037 [P] [US3] Unit test for warning log when `exit_code: 0` and threshold exceeded in `internal/cli/cost_budget_test.go`
- [ ] T038 [US3] Integration test for warning-only mode in `test/integration/budget_exit_test.go`

### Implementation for User Story 3

- [ ] T039 [US3] Add warning log output when `exit_code: 0` and threshold exceeded in `internal/cli/cost_budget.go`
- [ ] T040 [US3] Update `ExitReason()` to indicate warning-only mode in `internal/engine/budget_cli.go`

**Checkpoint**: Warning-only mode works without failing pipelines

---

## Phase 6: User Story 4 - CLI Flag Overrides (Priority: P3)

**Goal**: Operators can override exit behavior for single runs without changing config or environment

**Independent Test**: Run `cost projected --exit-on-threshold --exit-code 5 --pulumi-json plan.json`, verify exit code is 5 when threshold exceeded

### Tests for User Story 4 (MANDATORY - TDD Required) ⚠️

- [ ] T041 [P] [US4] Unit test for `--exit-on-threshold` flag parsing in `internal/cli/cost_test.go`
- [ ] T042 [P] [US4] Unit test for `--exit-code` flag parsing in `internal/cli/cost_test.go`
- [ ] T043 [P] [US4] Unit test for CLI flags overriding environment variables in `internal/cli/cost_test.go`
- [ ] T044 [P] [US4] Unit test for CLI flags overriding config file values in `internal/cli/cost_test.go`
- [ ] T045 [US4] Integration test for CLI flag overrides in `test/integration/budget_exit_test.go`

### Implementation for User Story 4

- [ ] T046 [US4] Add `--exit-on-threshold` persistent flag to `cost` command in `internal/cli/cost.go`
- [ ] T047 [US4] Add `--exit-code` persistent flag to `cost` command in `internal/cli/cost.go`
- [ ] T048 [US4] Apply CLI flag values to BudgetConfig in `cost_projected.go` before evaluation
- [ ] T049 [US4] Apply CLI flag values to BudgetConfig in `cost_actual.go` before evaluation

**Checkpoint**: Full configuration precedence (CLI > Env > Config > Default) works

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, edge cases, and final validation

- [ ] T050 [P] Update docs/reference/cli.md with new `--exit-on-threshold` and `--exit-code` flags
- [ ] T051 [P] Update docs/reference/configuration.md with new `exit_on_threshold` and `exit_code` fields
- [ ] T052 [P] Update docs/deployment/ci-cd-integration.md with exit code usage examples
- [ ] T053 [P] Add edge case test for exit code 255 in `internal/config/budget_test.go`
- [ ] T054 [P] Add edge case test for invalid exit code 256 in `internal/config/budget_test.go`
- [ ] T055 [P] Add edge case test for negative exit code -1 in `internal/config/budget_test.go`
- [ ] T056 Verify 80% minimum test coverage for modified files
- [ ] T057 Run `make lint` and fix any issues
- [ ] T058 Run `make test` and verify all tests pass
- [ ] T059 Validate quickstart.md examples manually

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion
  - User stories can then proceed in priority order (P1 → P2 → P2 → P3)
  - US1 (MVP) must complete before US2-4 for incremental delivery
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories - **MVP**
- **User Story 2 (P2)**: Can start after US1 complete - Extends config loading
- **User Story 3 (P2)**: Can start after US1 complete - Extends exit behavior
- **User Story 4 (P3)**: Can start after US2 complete - Needs env var layer for precedence testing

### Within Each Phase

- Tests MUST be written and FAIL before implementation
- Config changes before engine changes
- Engine changes before CLI changes
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tests marked [P] can run in parallel
- All tests within a user story marked [P] can run in parallel
- Documentation updates in Phase 7 marked [P] can run in parallel

---

## Parallel Example: Foundational Phase Tests

```bash
# Launch all foundational tests together:
Task: "Unit test for ErrExitCodeOutOfRange error type"
Task: "Unit test for BudgetConfig.GetExitCode() method"
Task: "Unit test for BudgetConfig.ShouldExitOnThreshold() method"
Task: "Unit test for exit code validation (0-255 range)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test exit codes work from config file
5. Merge to main if ready - MVP delivered!

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → MVP delivered!
3. Add User Story 2 → Environment variable support
4. Add User Story 3 → Warning-only mode
5. Add User Story 4 → CLI flag overrides
6. Complete Phase 7 → Documentation and edge cases
7. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Exit codes must be 0-255 (Unix standard)
- Configuration precedence: CLI flags > Environment Variables > Config File > Defaults
