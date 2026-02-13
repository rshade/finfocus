# Tasks: Automatic Pulumi Integration for Cost Commands

**Input**: Design documents from `/specs/509-pulumi-auto-detect/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Create the `internal/pulumi/` package with all detection and execution functions, extend the ingestion layer with bytes-based parsing, and add the `--stack` flag. These are required by ALL user stories.

### Sentinel Error Types

- [x] T001 Create sentinel error types (ErrPulumiNotFound, ErrNoProject, ErrNoCurrentStack, ErrPreviewFailed, ErrExportFailed) with actionable user messages in `internal/pulumi/errors.go`

### Unit Tests for Pulumi Package (TDD: write tests BEFORE implementation)

- [x] T002 [P] Write unit tests for `FindBinary()` (binary found, ErrPulumiNotFound, error message includes install URL) using TestHelperProcess exec mock pattern in `internal/pulumi/pulumi_test.go`
- [x] T003 [P] Write unit tests for `FindProject()` (Pulumi.yaml in cwd, Pulumi.yml in cwd, Pulumi.yaml in parent dir, no project found, filesystem root reached) in `internal/pulumi/pulumi_test.go`
- [x] T004 [P] Write unit tests for `GetCurrentStack()` (current stack found, no current stack with available list, empty stack list, malformed JSON) using TestHelperProcess exec mock pattern in `internal/pulumi/pulumi_test.go`
- [x] T005 [P] Write unit tests for `Preview()` (success returns bytes, non-zero exit returns ErrPreviewFailed with stderr, context cancellation, --stack flag passed when non-empty) using TestHelperProcess exec mock pattern in `internal/pulumi/pulumi_test.go`
- [x] T006 [P] Write unit tests for `StackExport()` (success returns bytes, non-zero exit returns ErrExportFailed with stderr, --stack flag passed when non-empty) using TestHelperProcess exec mock pattern in `internal/pulumi/pulumi_test.go`

### Detection Functions (implement after tests exist and fail)

- [x] T007 Implement `FindBinary()` using `exec.LookPath("pulumi")` returning path or ErrPulumiNotFound with install URL in `internal/pulumi/pulumi.go`
- [x] T008 Implement `FindProject(dir string)` walking up directory tree looking for `Pulumi.yaml` or `Pulumi.yml`, returning project dir or ErrNoProject in `internal/pulumi/pulumi.go`
- [x] T009 Implement `GetCurrentStack(ctx, projectDir)` running `pulumi stack ls --json`, parsing StackInfo entries, returning current stack name or ErrNoCurrentStack with available stack list in `internal/pulumi/pulumi.go`

### Execution Functions (implement after tests exist and fail)

- [x] T010 Implement `Preview(ctx, PreviewOptions)` running `pulumi preview --json [--stack=X]` with 5-minute timeout, environment passthrough via `os.Environ()`, stderr capture for error reporting, returning stdout bytes in `internal/pulumi/pulumi.go`
- [x] T011 Implement `StackExport(ctx, ExportOptions)` running `pulumi stack export [--stack=X]` with 60-second timeout, environment passthrough, stderr capture, returning stdout bytes in `internal/pulumi/pulumi.go`

### Ingestion Layer Extensions

- [x] T012 [P] Write unit tests for `ParsePulumiPlan()` (valid JSON, invalid JSON, empty bytes) and verify `LoadPulumiPlan()` still works unchanged (regression) in `internal/ingest/pulumi_plan_test.go`
- [x] T013 [P] Write unit tests for `ParseStackExport()` (valid JSON, invalid JSON, empty bytes) and verify `LoadStackExport()` still works unchanged (regression) in `internal/ingest/state_test.go`
- [x] T014 [P] Add `ParsePulumiPlan(data []byte)` and `ParsePulumiPlanWithContext(ctx, data []byte)` functions in `internal/ingest/pulumi_plan.go`, then refactor `LoadPulumiPlan` and `LoadPulumiPlanWithContext` to read file and delegate to the new Parse variants
- [x] T015 [P] Add `ParseStackExport(data []byte)` and `ParseStackExportWithContext(ctx, data []byte)` functions in `internal/ingest/state.go`, then refactor `LoadStackExport` and `LoadStackExportWithContext` to read file and delegate to the new Parse variants

### CLI Flag Setup

- [x] T016 Add `--stack` persistent flag (string, optional, default "") to the `cost` parent command in `internal/cli/root.go` with description "Pulumi stack name for auto-detection (ignored with --pulumi-json/--pulumi-state)"

**Checkpoint**: Foundation ready — all detection, execution, parsing functions exist with tests. User story implementation can begin.

---

## Phase 2: User Story 1 — Zero-Flag Projected Cost Estimation (Priority: P1) MVP

**Goal**: Developers run `finfocus cost projected` with no flags and get projected costs automatically.

**Independent Test**: Run `finfocus cost projected` in a Pulumi project directory and verify cost output appears.

### Tests for User Story 1 (TDD)

- [x] T017 [US1] Write test `TestCostProjectedWithoutPulumiJson` verifying command does not error on missing `--pulumi-json` flag (flag is no longer required) in `internal/cli/cost_projected_test.go`
- [x] T018 [US1] Write test `TestCostProjectedFlagHelp` verifying help text says "optional" (not "required") for `--pulumi-json` in `internal/cli/cost_projected_test.go`

### Implementation for User Story 1

- [x] T019 [US1] Remove `cmd.MarkFlagRequired("pulumi-json")` from `NewCostProjectedCmd()` and update the flag description to indicate it is optional (auto-detected from Pulumi project if omitted) in `internal/cli/cost_projected.go`
- [x] T020 [US1] Implement `resolveResourcesFromPulumi(ctx, stack, mode)` shared helper that orchestrates: FindBinary → FindProject(".") → resolve stack (use --stack flag or GetCurrentStack) → Preview or StackExport → ParsePulumiPlan or ParseStackExport → MapResources/MapStateResources in `internal/cli/common_execution.go`
- [x] T021 [US1] Add auto-detection fallback logic in `executeCostProjected()`: if `params.planPath == ""` then call `resolveResourcesFromPulumi(ctx, stackFlag, "preview")`, log INFO "Running pulumi preview --json (this may take a moment)...", and continue with existing engine flow in `internal/cli/cost_projected.go`
- [x] T022 [US1] Update long description and examples for `cost projected` command per contracts/cli-interface.md help text in `internal/cli/cost_projected.go`

**Checkpoint**: `finfocus cost projected` works with no flags inside a Pulumi project. Existing `--pulumi-json` path unchanged.

---

## Phase 3: User Story 2 — Zero-Flag Actual Cost Retrieval (Priority: P1)

**Goal**: Developers run `finfocus cost actual` with no flags and get actual costs automatically with auto-detected date range.

**Independent Test**: Run `finfocus cost actual` in a Pulumi project directory and verify actual cost output with auto-detected dates.

### Tests for User Story 2 (TDD)

- [x] T023 [US2] Write test `TestCostActualWithoutInputFlags` verifying `validateActualInputFlags()` allows neither `--pulumi-json` nor `--pulumi-state` (no error when both omitted) in `internal/cli/cost_actual_test.go`
- [x] T024 [US2] Write test verifying mutual exclusivity still enforced (both flags provided returns error) in `internal/cli/cost_actual_test.go`

### Implementation for User Story 2

- [x] T025 [US2] Modify `validateActualInputFlags()` to return nil when neither `--pulumi-json` nor `--pulumi-state` is provided (currently errors), preserving mutual exclusivity check in `internal/cli/cost_actual.go`
- [x] T026 [US2] Add auto-detection fallback in `executeCostActual()`: if neither `planPath` nor `statePath` provided, call `resolveResourcesFromPulumi(ctx, stackFlag, "export")`, log INFO "Running pulumi stack export...", auto-detect `--from` date from state timestamps (existing `resolveFromDate` logic), and continue with existing engine flow in `internal/cli/cost_actual.go`
- [x] T027 [US2] Update long description and examples for `cost actual` command per contracts/cli-interface.md help text in `internal/cli/cost_actual.go`

**Checkpoint**: `finfocus cost actual` works with no flags. Auto-detects state and date range. Existing `--pulumi-json`/`--pulumi-state` paths unchanged.

---

## Phase 4: User Story 3 — Explicit Stack Selection (Priority: P2)

**Goal**: The `--stack` flag allows targeting a specific stack without changing the active stack.

**Independent Test**: Run `finfocus cost projected --stack production` and verify it uses the specified stack.

### Tests for User Story 3 (TDD)

- [x] T028 [US3] Write test `TestStackFlagExists` verifying `--stack` flag is present and inherited on `cost` parent command in `internal/cli/cost_projected_test.go` or `internal/cli/cost_actual_test.go`
- [x] T029 [US3] Write test verifying `--stack` flag value is passed through to `resolveResourcesFromPulumi()` when no file flags provided in `internal/cli/cost_projected_test.go`
- [x] T030 [US3] Write test verifying `--stack` flag is ignored when `--pulumi-json` is provided (file takes precedence) in `internal/cli/cost_projected_test.go`

### Implementation for User Story 3

- [x] T031 [US3] Wire `--stack` flag retrieval in `executeCostProjected()` — read from `cmd.Flags().GetString("stack")` and pass to `resolveResourcesFromPulumi()` in `internal/cli/cost_projected.go`
- [x] T032 [US3] Wire `--stack` flag retrieval in `executeCostActual()` — same pattern as projected in `internal/cli/cost_actual.go`

**Checkpoint**: `--stack production` targets the specified stack for both projected and actual commands.

---

## Phase 5: User Story 4 — Clear Error Guidance (Priority: P2)

**Goal**: All error states produce actionable messages with remediation steps.

**Independent Test**: Run commands in invalid states and verify error messages include install URLs, stack lists, and `--pulumi-json` suggestions.

### Tests for User Story 4 (TDD)

- [x] T033 [US4] Write test verifying ErrPulumiNotFound error message contains install URL `https://www.pulumi.com/docs/install/` and suggests `--pulumi-json` in `internal/pulumi/pulumi_test.go`
- [x] T034 [US4] Write test verifying ErrNoProject error message suggests `--pulumi-json` and `cd to project directory` in `internal/pulumi/pulumi_test.go`
- [x] T035 [US4] Write test verifying ErrNoCurrentStack error message includes available stack names and suggests `--stack` in `internal/pulumi/pulumi_test.go`
- [x] T036 [US4] Write test verifying ErrPreviewFailed and ErrExportFailed include the captured stderr output from the Pulumi CLI in `internal/pulumi/pulumi_test.go`

### Implementation for User Story 4

Error types and messages are implemented in Phase 1 (T001). These tests validate the error message quality. If any test fails, update error messages in `internal/pulumi/errors.go` to include the required actionable content.

**Checkpoint**: All error paths produce user-friendly messages with clear remediation steps.

---

## Phase 6: User Story 5 — Full Backward Compatibility (Priority: P1)

**Goal**: All existing flag-based workflows produce identical results.

**Independent Test**: Run `make test` and verify all existing tests pass without modification.

### Validation for User Story 5

- [x] T037 [US5] Run `make test` and verify all existing tests pass without modification — no regressions from flag changes in `cost_projected.go`, `cost_actual.go`, or ingestion refactoring
- [x] T038 [US5] Run `make lint` and verify no new lint errors introduced across all modified files
- [x] T039 [US5] Verify existing test cases in `internal/cli/cost_projected_test.go` that test `--pulumi-json plan.json` still pass with identical behavior
- [x] T040 [US5] Verify existing test cases in `internal/cli/cost_actual_test.go` that test `--pulumi-json` and `--pulumi-state` paths still pass with identical behavior

**Checkpoint**: Zero regressions confirmed. All existing workflows unchanged.

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, integration testing, final quality gates

- [x] T041 [P] Update user guide with Pulumi auto-detection section (requirements, usage, troubleshooting) in `docs/guides/`
- [x] T042 [P] Update CLI reference documentation for modified `cost projected` and `cost actual` commands and new `--stack` flag in `docs/reference/`
- [x] T043 Create integration test `TestPulumiAutoDetection` verifying end-to-end flow (detect project → resolve stack → preview/export → parse → resource extraction) using a fixture Pulumi project, with `t.Skip` if `pulumi` not available in `test/integration/pulumi_auto_test.go`
- [x] T044 Run `make test` and `make lint` on complete implementation, verify 80%+ coverage on `internal/pulumi/` package
- [x] T045 Validate quickstart.md scenarios work end-to-end against a real Pulumi project

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Foundational)**: No dependencies — start immediately
- **Phase 2 (US1 - Projected)**: Depends on Phase 1 completion
- **Phase 3 (US2 - Actual)**: Depends on Phase 1 completion. Can run in parallel with Phase 2.
- **Phase 4 (US3 - Stack)**: Depends on Phase 2 and Phase 3 (both commands must have auto-detection wired)
- **Phase 5 (US4 - Errors)**: Depends on Phase 1 (error types must exist). Can start after Phase 1.
- **Phase 6 (US5 - Backward Compat)**: Depends on Phase 2 and Phase 3 (all code changes must be complete)
- **Phase 7 (Polish)**: Depends on all previous phases

### User Story Dependencies

- **US1 (Projected)**: Depends only on Foundational phase — no other story dependencies
- **US2 (Actual)**: Depends only on Foundational phase — independent from US1
- **US3 (Stack Selection)**: Depends on US1 and US2 (flag must be wired into both commands)
- **US4 (Error Guidance)**: Depends only on Foundational phase (error types) — independent from US1/US2
- **US5 (Backward Compat)**: Validation only — depends on US1 and US2 being complete

### Within Each Phase

- Tests MUST be written and FAIL before implementation begins
- Error types before detection functions
- Detection functions before execution functions
- Ingestion extensions before CLI integration
- Core implementation before help text updates

### Parallel Opportunities

**Within Phase 1**:

- T002, T003, T004, T005, T006 (all pulumi package tests) can run in parallel
- T012 and T013 (ingestion tests) can run in parallel
- T014 and T015 (ingestion extensions) can run in parallel

**Across Phases**:

- Phase 2 (US1) and Phase 3 (US2) can run in parallel after Phase 1 completes
- Phase 5 (US4 - Errors) can run in parallel with Phase 2 and Phase 3, but NOT Phase 1 (shared `pulumi_test.go` file)
- T041 and T042 (docs) can run in parallel with each other and with Phase 6

---

## Parallel Example: Phase 1 (Foundational)

```text
# Sequential: Error types first
T001: Create sentinel error types in internal/pulumi/errors.go

# TDD: Write tests first (parallel — same file but different test functions)
T002-T006: Unit tests in internal/pulumi/pulumi_test.go

# Then implement to make tests pass (sequential — same file)
T007: Implement FindBinary() in internal/pulumi/pulumi.go
T008: Implement FindProject() in internal/pulumi/pulumi.go
T009: Implement GetCurrentStack() in internal/pulumi/pulumi.go
T010: Implement Preview() in internal/pulumi/pulumi.go
T011: Implement StackExport() in internal/pulumi/pulumi.go

# Parallel: Ingestion tests first (TDD), then extensions (different files)
T012: Plan parsing tests in internal/ingest/pulumi_plan_test.go
T013: State parsing tests in internal/ingest/state_test.go
T014: ParsePulumiPlan in internal/ingest/pulumi_plan.go
T015: ParseStackExport in internal/ingest/state.go

# After all above:
T016: --stack flag in internal/cli/root.go
```

## Parallel Example: US1 + US2 (after Phase 1)

```text
# These two phases can run in parallel (different files):

# Developer A: US1
T017-T018: Tests in internal/cli/cost_projected_test.go
T019-T022: Implementation in internal/cli/cost_projected.go + common_execution.go

# Developer B: US2
T023-T024: Tests in internal/cli/cost_actual_test.go
T025-T027: Implementation in internal/cli/cost_actual.go
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational (T001-T016)
2. Complete Phase 2: US1 Projected Auto-Detect (T017-T022)
3. **STOP and VALIDATE**: Run `finfocus cost projected` in a Pulumi project
4. Deploy/demo if ready

### Incremental Delivery

1. Phase 1 (Foundational) → Package and ingestion ready
2. Phase 2 (US1 - Projected) → Test independently → MVP!
3. Phase 3 (US2 - Actual) → Test independently → Both commands work
4. Phase 4 (US3 - Stack) → Test independently → Multi-stack support
5. Phase 5 (US4 - Errors) → Test error quality → Production-ready errors
6. Phase 6 (US5 - Backward Compat) → Validate → Regression-free
7. Phase 7 (Polish) → Docs + integration tests → Ship-ready

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Run `make lint` and `make test` after each phase checkpoint
- Stop at any checkpoint to validate story independently
- T007-T011 touch the same file (`pulumi.go`) — implement sequentially within that file
