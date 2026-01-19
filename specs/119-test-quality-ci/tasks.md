# Tasks: Test Quality and CI/CD Improvements

**Input**: Design documents from `/specs/118-test-quality-ci/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

**Tests**: This feature IS the test infrastructure - test quality is the deliverable. Constitution Principle II (TDD) applies to any new test helpers/utilities.

**Completeness**: Per Constitution Principle VI, all tasks MUST be fully implemented. No stub functions, placeholders, or TODO comments.

**Documentation**: Per Constitution Principle IV, CLAUDE.md MUST be updated with any new test patterns discovered.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5 from spec.md)
- Include exact file paths in descriptions

## Path Conventions

- **Go project**: `internal/`, `cmd/`, `test/` at repository root
- **Workflows**: `.github/workflows/`
- **Test fixtures**: `test/fixtures/`

---

## Phase 1: Setup (Verification & Preparation)

**Purpose**: Verify existing infrastructure and understand current state

- [X] T001 Verify existing fuzz test structure in `internal/ingest/fuzz_test.go`
- [X] T002 [P] Verify benchmark test structure in `test/benchmarks/parse_bench_test.go`
- [X] T003 [P] Verify existing state fixtures in `test/fixtures/state/`
- [X] T004 [P] Review existing workflow patterns in `.github/workflows/nightly.yml`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before user story implementation

**⚠️ CRITICAL**: Phase 2 is minimal for this feature - most work is independent per user story

- [X] T005 Ensure `make build` produces working binary for E2E tests
- [X] T006 [P] Verify testify assertions are available (`require`, `assert` packages)

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Fuzz Test Coverage for Pulumi Plan Parsing (Priority: P1) 🎯 MVP

**Goal**: Add `newState` seed corpus entries to fuzz tests for realistic Pulumi plan coverage

**Independent Test**: Run `go test -fuzz=FuzzJSON -fuzztime=30s ./internal/ingest` and confirm seed corpus includes `newState` structures

### Implementation for User Story 1

- [X] T007 [US1] Add `newState` create operation seed entry to `internal/ingest/fuzz_test.go`
- [X] T008 [US1] Add `newState` update operation seed entry (with `oldState`) to `internal/ingest/fuzz_test.go`
- [X] T009 [US1] Add malformed `newState` seed entry for graceful failure testing in `internal/ingest/fuzz_test.go`
- [X] T010 [US1] Run fuzz test for 30 seconds to verify no panics: `go test -fuzz=FuzzJSON -fuzztime=30s ./internal/ingest`
- [X] T011 [US1] Document fuzz corpus additions in test file comments

**Checkpoint**: User Story 1 complete - fuzz tests now cover `newState` structures

---

## Phase 4: User Story 2 - Benchmark Accuracy for Performance Testing (Priority: P1)

**Goal**: Fix benchmark JSON structure to use valid `steps` format instead of legacy `resourceChanges`

**Independent Test**: Run `go test -bench=. ./test/benchmarks/...` and confirm benchmarks complete successfully with realistic Pulumi structures

### Implementation for User Story 2

- [X] T012 [P] [US2] Update JSON generation in `test/benchmarks/parse_bench_test.go` to use `steps` array format
- [X] T013 [US2] Update resource format to include `op`, `urn`, `type` fields matching Pulumi structure in `test/benchmarks/parse_bench_test.go`
- [X] T014 [US2] Add `newState` wrapper with `inputs` to benchmark resources in `test/benchmarks/parse_bench_test.go`
- [X] T015 [US2] Run benchmarks to verify non-zero resource extraction: `go test -bench=. -benchmem ./test/benchmarks/...`
- [X] T016 [US2] Document benchmark JSON format change in test file comments

**Checkpoint**: User Story 2 complete - benchmarks now use valid Pulumi JSON format

---

## Phase 5: User Story 3 - Cross-Repository Integration Testing (Priority: P2)

**Goal**: Create automated CI workflow that tests Core and Plugin integration together with nightly schedule

**Independent Test**: Trigger workflow manually via `workflow_dispatch` and confirm both repos build and integrate successfully

### Implementation for User Story 3

- [X] T017 [US3] Create `.github/workflows/cross-repo-integration.yml` with workflow structure
- [X] T018 [US3] Add `workflow_dispatch` trigger with `plugin_ref` and `core_ref` inputs in `.github/workflows/cross-repo-integration.yml`
- [X] T019 [US3] Add nightly schedule cron (`0 2 * * *`) to `.github/workflows/cross-repo-integration.yml`
- [X] T020 [US3] Add job to checkout finfocus-core repository in `.github/workflows/cross-repo-integration.yml`
- [X] T021 [US3] Add job to checkout finfocus-plugin-aws-public repository in `.github/workflows/cross-repo-integration.yml`
- [X] T022 [US3] Add Go setup step with caching in `.github/workflows/cross-repo-integration.yml`
- [X] T023 [US3] Add plugin build and install steps in `.github/workflows/cross-repo-integration.yml`
- [X] T024 [US3] Add `finfocus plugin list` verification step in `.github/workflows/cross-repo-integration.yml`
- [X] T025 [US3] Add `finfocus cost projected` integration test step in `.github/workflows/cross-repo-integration.yml`
- [X] T026 [US3] Add failure notification job using `actions/github-script@v8` in `.github/workflows/cross-repo-integration.yml`
- [X] T027 [US3] Set proper permissions (`contents: read`, `issues: write`) in `.github/workflows/cross-repo-integration.yml`
- [X] T028 [US3] Validate workflow syntax: `actionlint .github/workflows/cross-repo-integration.yml`

**Checkpoint**: User Story 3 complete - cross-repo workflow ready for nightly integration testing

---

## Phase 6: User Story 4 - E2E Test for Actual Cost Command (Priority: P2)

**Goal**: Implement E2E tests for `cost actual` command using real Pulumi state files

**Independent Test**: Run `go test -v -tags=e2e ./test/e2e/... -run TestE2E_ActualCost` with plugin installed

### Implementation for User Story 4

- [X] T029 [US4] Create `test/e2e/actual_cost_test.go` with test structure and imports
- [X] T030 [US4] Implement `TestE2E_ActualCost_WithTimestamps` using `test/fixtures/state/valid-state.json` in `test/e2e/actual_cost_test.go`
- [X] T031 [US4] Implement `TestE2E_ActualCost_ImportedResources` using `test/fixtures/state/imported-resources.json` in `test/e2e/actual_cost_test.go`
- [X] T031a [US4] Implement `TestE2E_ActualCost_MissingTimestamps` using `test/fixtures/state/no-timestamps.json` to verify resources are skipped in `test/e2e/actual_cost_test.go`
- [X] T032 [US4] Add JSON output validation for `resource_type`, `actual_cost`, `currency` fields in `test/e2e/actual_cost_test.go`
- [X] T033 [US4] Add test skip logic when aws-public plugin is not installed in `test/e2e/actual_cost_test.go`
- [X] T034 [US4] Run E2E test to verify: `go test -v -tags=e2e ./test/e2e/... -run TestE2E_ActualCost`

**Checkpoint**: User Story 4 complete - actual cost command has E2E test coverage

---

## Phase 7: User Story 5 - Test Error Handling Improvements (Priority: P3)

**Goal**: Fix error handling patterns in test files to use proper testify assertions

**Independent Test**: Code review confirms `require.NoError(t, err)` follows all `filepath.Abs` and similar fallible operations

### Implementation for User Story 5

- [X] T035 [US5] Fix `filepath.Abs` error handling in `test/e2e/gcp_test.go` using `require.NoError(t, err)`
- [X] T036 [US5] Add NDJSON schema validation for `resource_type` field in `test/e2e/output_ndjson_test.go`
- [X] T037 [US5] Add NDJSON schema validation for `currency` field in `test/e2e/output_ndjson_test.go`
- [X] T038 [US5] Scan for other `_, _` error ignore patterns: `grep -r '_, _' test/`
- [X] T039 [US5] Fix any additional error handling issues found in scan

**Checkpoint**: User Story 5 complete - all test files properly handle errors

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and documentation updates

- [X] T040 Run full test suite: `make test`
- [X] T041 [P] Run linting: `make lint`
- [X] T042 [P] Run fuzz tests for extended duration: `go test -fuzz=FuzzJSON -fuzztime=1m ./internal/ingest`
- [X] T043 Update CLAUDE.md with new test patterns if any discovered
- [X] T044 Run quickstart.md validation commands
- [X] T045 Verify all success criteria from spec.md are met

**Checkpoint**: Phase 8 complete - all validation passes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational phase completion
  - US1 and US2 (both P1): Can proceed in parallel
  - US3 and US4 (both P2): Can proceed in parallel after P1 or concurrently
  - US5 (P3): Can proceed after P2 or concurrently
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1 - Fuzz Tests)**: No dependencies on other stories - standalone
- **User Story 2 (P1 - Benchmarks)**: No dependencies on other stories - standalone
- **User Story 3 (P2 - Cross-Repo CI)**: No dependencies on US1/US2 - standalone workflow
- **User Story 4 (P2 - ActualCost E2E)**: Requires plugin installed, but no code dependencies on other stories
- **User Story 5 (P3 - Error Handling)**: No dependencies - cleanup/polish work

### Within Each User Story

- Implementation tasks should be executed in order (file edits before verification)
- Verification tasks (running tests) come after all implementation tasks

### Parallel Opportunities

**Phase 1 (Setup)**:

- T002, T003, T004 can run in parallel (different files to review)

**Phase 3-7 (User Stories)**:

- US1 and US2 can run completely in parallel (different files: `fuzz_test.go` vs `parse_bench_test.go`)
- US3, US4, US5 can all run in parallel (different files: workflow, E2E test, existing test fixes)
- Within US3: Tasks T017-T027 must be sequential (building one workflow file)

**Phase 8 (Polish)**:

- T040, T041, T042 can run in parallel

---

## Parallel Example: User Stories 1 & 2 Together

```bash
# US1: Fuzz test improvements (internal/ingest/fuzz_test.go)
Task: T007 "Add newState create operation seed entry"
Task: T008 "Add newState update operation seed entry"
Task: T009 "Add malformed newState seed entry"

# US2: Benchmark fixes (test/benchmarks/parse_bench_test.go) - CAN RUN IN PARALLEL
Task: T012 "Update JSON generation to use steps array format"
Task: T013 "Update resource format to include op, urn, type fields"
Task: T014 "Add newState wrapper with inputs"
```

---

## Implementation Strategy

### MVP First (User Stories 1 & 2 Only)

1. Complete Phase 1: Setup (verification)
2. Complete Phase 2: Foundational (minimal)
3. Complete Phase 3: User Story 1 - Fuzz Tests
4. Complete Phase 4: User Story 2 - Benchmarks
5. **STOP and VALIDATE**: Run `make test` and `make lint`
6. Both P1 stories deliver immediate value for test quality

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 (Fuzz) + User Story 2 (Benchmarks) → Test independently → Core test quality improved (MVP!)
3. Add User Story 3 (Cross-Repo CI) → Test independently → CI/CD improved
4. Add User Story 4 (ActualCost E2E) → Test independently → E2E coverage expanded
5. Add User Story 5 (Error Handling) → Test independently → Code quality polish
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (Fuzz Tests)
   - Developer B: User Story 2 (Benchmarks)
   - Developer C: User Story 3 (Cross-Repo CI)
3. After initial stories:
   - Developer A: User Story 4 (ActualCost E2E)
   - Developer B: User Story 5 (Error Handling)
4. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story (US1-US5) for traceability
- Each user story is independently completable and testable
- US1 and US2 are both P1 priority - complete both for MVP
- US3 creates a new workflow file - all tasks within it must be sequential
- US4 depends on having the aws-public plugin installed for verification
- US5 is polish work that can be deferred if needed
- Stop at any checkpoint to validate story independently
- Run `make lint` and `make test` before claiming any phase complete
