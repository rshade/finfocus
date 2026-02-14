# Tasks: Neo-Friendly CLI Fixes

**Input**: Design documents from `/specs/590-neo-cli-fixes/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Verify baseline project health before making changes

- [x] T001 Verify baseline by running `make test` and `make lint` to confirm all tests pass and no lint errors exist

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: No foundational tasks needed. All three user stories modify independent files and have no shared prerequisites beyond the existing codebase.

**Checkpoint**: Proceed directly to user story implementation.

---

## Phase 3: User Story 1 - Semantic Exit Codes for Budget Violations (Priority: P1) MVP

**Goal**: Propagate custom budget exit codes through `main()` so AI agents can distinguish "budget exceeded" (exit 2) from "command failed" (exit 1) without parsing error text.

**Independent Test**: Run `finfocus cost projected` with a budget config that sets `exit_code: 2` and a low `monthly_limit`. Verify `$?` equals 2 when costs exceed the limit, 0 when within budget, and 1 on evaluation error.

**Acceptance**: FR-001, FR-002 (spec.md); SC-001 (success criteria)

### Tests for User Story 1 (TDD Required)

- [x] T002 [US1] Write tests for BudgetExitError extraction via `errors.As()` in `cmd/finfocus/main_test.go`: test that `run()` returning a `*BudgetExitError{ExitCode: 2}` is correctly detected, and that non-BudgetExitError errors fall through to default exit code 1
- [x] T003 [P] [US1] Write tests for cost actual budget error return in `internal/cli/cost_budget_test.go`: verify that `checkBudgetExitFromResult` error is returned (not just logged) in the cost actual command path, matching the cost projected behavior; include test for budget exit in non-table output modes (JSON, NDJSON)

### Implementation for User Story 1

- [x] T004 [US1] Fix `cost actual` to return BudgetExitError instead of logging it in `internal/cli/cost_actual.go:247-249`: change the `log.Warn()` path to `return exitErr` to match `internal/cli/cost_projected.go:247-249`
- [x] T005 [US1] Remove the table-only output format guard (`params.output == outputFormatTable`) from the budget exit check in `internal/cli/cost_actual.go:242` so budget exit codes propagate in all output modes (JSON, NDJSON, table), matching `internal/cli/cost_projected.go:242` which has no output format guard
- [x] T006 [US1] Update `main()` to extract BudgetExitError exit code via `errors.As()` in `cmd/finfocus/main.go:62-65`: before `os.Exit(1)`, check `errors.As(err, &budgetErr)` and if matched call `os.Exit(budgetErr.ExitCode)`. Verify `BudgetExitError` is already exported from `internal/cli/cost_budget.go` (it is — uppercase name)

**Checkpoint**: Exit codes propagate correctly from both `cost projected` and `cost actual`. Process exits with custom code on budget violation, 1 on general error, 0 on success.

---

## Phase 4: User Story 2 - Structured Error Objects in JSON Output (Priority: P2)

**Goal**: Add structured error objects with stable error codes (`PLUGIN_ERROR`, `VALIDATION_ERROR`, `TIMEOUT_ERROR`, `NO_COST_DATA`) to JSON/NDJSON output so AI agents can programmatically categorize errors without string parsing.

**Independent Test**: Run `finfocus cost projected --output json` with a plan that triggers plugin errors/validation failures. Verify each result with an error has a structured `error` object with `code`, `message`, and `resourceType` fields. Verify the `notes` field has no `ERROR:` or `VALIDATION:` prefix when `error` is present.

**Acceptance**: FR-003, FR-004, FR-005, FR-009, FR-011 (spec.md); SC-002, SC-004 (success criteria)

### Tests for User Story 2 (TDD Required)

- [x] T007 [US2] Write tests for StructuredError type and JSON/NDJSON serialization in `internal/engine/types_test.go`: verify `StructuredError` marshals correctly with `code`, `message`, `resourceType` fields; verify `CostResult` with non-nil `Error` serializes the error object in both JSON and NDJSON modes; verify `CostResult` with nil `Error` omits the field
- [x] T008 [P] [US2] Write tests for error code assignment at all four error origins in `internal/proto/adapter_test.go`: verify VALIDATION_ERROR for pre-flight failures, PLUGIN_ERROR for gRPC failures, TIMEOUT_ERROR for `context.DeadlineExceeded`, and that Notes field has no prefix when StructuredError is present

### Implementation for User Story 2

- [x] T009 [US2] Add `StructuredError` struct with `Code`, `Message`, `ResourceType` fields, error code constants (`ErrCodePluginError`, `ErrCodeValidationError`, `ErrCodeTimeoutError`, `ErrCodeNoCostData`), and `Error *StructuredError` field to `CostResult` in `internal/engine/types.go`
- [x] T010 [US2] Populate StructuredError at all error origins in `internal/proto/adapter.go`: set `VALIDATION_ERROR` at pre-flight validation failures (lines ~135-141, ~238-246), set `PLUGIN_ERROR` at gRPC call failures (lines ~155-164, ~257-266), detect `context.DeadlineExceeded` and set `TIMEOUT_ERROR` instead of `PLUGIN_ERROR`, strip `ERROR:`/`VALIDATION:` prefixes from Notes when StructuredError is present (FR-011)
- [x] T011 [US2] Add `NO_COST_DATA` structured error for the "No pricing information available" path in `internal/engine/engine.go:~398`
- [x] T012 [US2] Update error detection in `internal/tui/cost_view.go:58,367` and `internal/engine/overview_enrich.go:83,116` to check `result.Error != nil` instead of `strings.HasPrefix(result.Notes, "ERROR:")` so table output and overview enrichment continue to detect errors after Notes prefix stripping (FR-009)

**Checkpoint**: JSON/NDJSON output contains structured error objects for all error conditions. Table output remains unchanged. Notes field is clean when error object is present.

---

## Phase 5: User Story 3 - Plugin List as Structured Data (Priority: P3)

**Goal**: Add `--output json` support to the `plugin list` command so AI agents can programmatically inspect installed plugin names, versions, providers, and capabilities.

**Independent Test**: Run `finfocus plugin list --output json` and verify the output is a valid JSON array. Verify each entry has `name`, `version`, `path`, `supportedProviders`, and `capabilities` fields. Verify empty plugin directory produces `[]`.

**Acceptance**: FR-006, FR-007, FR-008, FR-009, FR-010 (spec.md); SC-003, SC-004 (success criteria)

### Tests for User Story 3 (TDD Required)

- [x] T013 [US3] Write tests for plugin list JSON output in `internal/cli/plugin_list_test.go`: test JSON output with plugins present (verify array of objects with required fields), test empty plugin list produces `[]` not null (FR-008), test failed metadata retrieval includes notes field (FR-010), test table output unchanged (FR-009)

### Implementation for User Story 3

- [x] T014 [US3] Add `--output` string flag (default `"table"`, values `"table"` and `"json"`) to `NewPluginListCmd()` and implement JSON rendering path using `json.NewEncoder` in `internal/cli/plugin_list.go`: marshal `[]enrichedPluginInfo` as JSON array, handle empty list as `[]`, handle no-plugins-installed as `[]`

**Checkpoint**: `finfocus plugin list --output json` produces valid JSON array. Empty directory produces `[]`. Failed plugins appear with notes. Table output is unchanged.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, final validation, and quality gates

- [x] T015 [P] Update CLAUDE.md with structured error code constants (`ErrCodePluginError`, `ErrCodeValidationError`, `ErrCodeTimeoutError`, `ErrCodeNoCostData`) and document `plugin list --output json` support
- [x] T016 Run `make lint`, `make test`, and `make test-race` for final validation (all tests pass with race detector, zero lint errors, 80%+ coverage)
- [x] T017 Run quickstart.md verification scenarios: verify exit codes, structured errors in JSON, and plugin list JSON output

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **User Stories (Phases 3-5)**: All depend on Phase 1 baseline verification only
  - US1, US2, US3 are fully independent and can run in parallel
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent. Modifies `cmd/finfocus/main.go` and `internal/cli/cost_actual.go`
- **User Story 2 (P2)**: Independent. Modifies `internal/engine/types.go`, `internal/proto/adapter.go`, `internal/engine/engine.go`, `internal/tui/cost_view.go`, `internal/engine/overview_enrich.go`
- **User Story 3 (P3)**: Independent. Modifies `internal/cli/plugin_list.go`

No cross-story file conflicts exist. All three stories can safely run in parallel.

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation tasks within a story are sequential (same files)
- Story complete before marking checkpoint

### Parallel Opportunities

- T002 and T003 (US1 tests): parallel, different files
- T007 and T008 (US2 tests): parallel, different files
- All three user stories (Phases 3-5): parallel, no file overlap
- T015 (docs update): parallel with T016/T017

---

## Parallel Example: All User Stories

```text
# After Phase 1 baseline verification, launch all three stories simultaneously:

# Story 1 (main.go, cost_actual.go):
Task: T002 "Write exit code extraction tests in cmd/finfocus/main_test.go"
Task: T003 "Write cost actual budget error tests in internal/cli/cost_budget_test.go"
Task: T004 "Fix cost_actual.go to return BudgetExitError"
Task: T005 "Remove table-only guard from cost_actual budget check"
Task: T006 "Update main() with errors.As extraction"

# Story 2 (types.go, adapter.go, engine.go, tui/cost_view.go, overview_enrich.go):
Task: T007 "Write StructuredError tests in internal/engine/types_test.go"
Task: T008 "Write error code assignment tests in internal/proto/adapter_test.go"
Task: T009 "Add StructuredError type to internal/engine/types.go"
Task: T010 "Populate StructuredError in internal/proto/adapter.go"
Task: T011 "Add NO_COST_DATA error in internal/engine/engine.go"
Task: T012 "Update TUI/overview error detection to use result.Error"

# Story 3 (plugin_list.go):
Task: T013 "Write plugin list JSON tests in internal/cli/plugin_list_test.go"
Task: T014 "Add --output json to plugin list in internal/cli/plugin_list.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Verify baseline
2. Complete Phase 3: User Story 1 (Semantic Exit Codes)
3. **STOP and VALIDATE**: Test exit codes independently
4. This alone enables AI agents to distinguish budget violations from failures

### Incremental Delivery

1. Verify baseline → Baseline healthy
2. Add User Story 1 → Test exit codes → Budget-aware CI/CD enabled (MVP)
3. Add User Story 2 → Test JSON errors → Structured error handling enabled
4. Add User Story 3 → Test plugin JSON → Full agent introspection enabled
5. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All three stories modify independent files (zero cross-story overlap)
- Total: 17 tasks (1 setup, 5 US1, 6 US2, 2 US3, 3 polish)
- Estimated new code: ~250 lines across 8 source files
- No new packages, modules, or dependencies required
