# Tasks: CodeRabbit Cleanup from Pulumi Auto-Detect PR

**Input**: Design documents from `/specs/589-coderabbit-cleanup/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Establish baseline before making changes

- [x] T001 Record baseline test coverage with `go test -coverprofile=coverage.out ./...` and note percentage
- [x] T002 Run `make lint` and `make test` to confirm clean starting state

**Checkpoint**: Baseline established, all tests and lint passing before any edits

---

## Phase 2: User Story 1 - Consistent Codebase Documentation (Priority: P1)

**Goal**: Consolidate all duplicated/stuttered/incomplete doc comments into single concise Go-style comments across 11 functions in 5 files.

**Independent Test**: Run `go doc` on each affected function and confirm exactly one clean comment per function.

### Implementation for User Story 1

- [x] T003 [P] [US1] Consolidate `resolveSKUAndRegion` doc comment in `internal/proto/adapter.go:707-724` — merge two paragraphs into single concise comment describing provider-specific SKU/region extraction with fallback behavior
- [x] T004 [P] [US1] Consolidate `enrichTagsWithSKUAndRegion` doc comment in `internal/proto/adapter.go:871-882` — remove initial fragment, keep the detailed block starting with parameter descriptions
- [x] T005 [P] [US1] Remove repeated first line from `toStringMap` doc comment in `internal/proto/adapter.go:856-858` — keep single line: "toStringMap converts a map[string]interface{} to a map[string]string."
- [x] T006 [P] [US1] Collapse `FindBinary` doc comment in `internal/pulumi/pulumi.go:69-71` — single sentence: "FindBinary returns the full path to the pulumi executable by searching the system PATH."
- [x] T007 [P] [US1] Remove duplicated description block from `FindProject` doc comment in `internal/pulumi/pulumi.go:80-87` — keep one concise paragraph describing upward directory search for Pulumi.yaml/Pulumi.yml
- [x] T008 [P] [US1] Collapse `GetCurrentStack` doc comment in `internal/pulumi/pulumi.go:112-117` — single concise paragraph describing stack detection via `pulumi stack ls --json`
- [x] T009 [P] [US1] Remove duplicated one-liner from `Preview` doc comment in `internal/pulumi/pulumi.go:224-228` — keep the detailed block starting with "Preview runs `pulumi preview --json`"
- [x] T010 [P] [US1] Remove duplicated one-liner from `StackExport` doc comment in `internal/pulumi/pulumi.go:242-245` — keep the detailed block starting with "StackExport runs `pulumi stack export`"
- [x] T011 [P] [US1] Remove stuttered prefix from `resolveAWSSKU` doc comment in `internal/skus/aws.go:22-27` — keep single block starting with "resolveAWSSKU returns the well-known AWS SKU name"
- [x] T012 [P] [US1] Remove shorter stub from `extractPulumiSegment` doc comment in `internal/skus/aws.go:36-42` — keep the full comment starting with "extractPulumiSegment extracts the module/resource segment"
- [x] T013 [P] [US1] Complete truncated `MapStateResource` doc comment in `internal/ingest/state.go:168-172` — fix sentence ending "while user-declared from the resource type." to clarify that user-declared inputs take precedence over provider-computed outputs on conflict
- [x] T014 [P] [US1] Correct `mockPbcCostSourceServiceClient` doc comment in `internal/proto/adapter_test.go:2990-2991` — change "the rest panic" to "the rest return empty success responses"

**Checkpoint**: All 11 functions have exactly one concise doc comment. Verify with `go doc` for each.

---

## Phase 3: User Story 2 - Code Quality and Correctness (Priority: P1)

**Goal**: Apply 7 small logic fixes, validation improvements, and error handling corrections across 5 files.

**Independent Test**: Run `make test` and verify all existing plus new test cases pass for each fix.

### Implementation for User Story 2

- [x] T015 [P] [US2] Simplify `resolveFromDate` conditional in `internal/cli/cost_actual.go:565-566` — replace `params.statePath != "" || (params.planPath == "" && params.statePath == "")` with `params.planPath == ""`
- [x] T016 [P] [US2] Handle error from `cmd.Flags().GetString("stack")` in `internal/cli/cost_actual.go:539` — replace `stackFlag, _ := cmd.Flags().GetString("stack")` with proper error capture and return
- [x] T017 [P] [US2] Add audit entry on auto-detect failure in `internal/cli/cost_projected.go:159` — call `audit.logFailure(ctx, err)` before returning error when `resolveResourcesFromPulumi` fails, matching the `loadAndMapResources` pattern
- [x] T018 [P] [US2] Validate empty metadata keys in `internal/cli/plugin_install.go:470-488` — after `strings.TrimSpace(parts[0])`, check if key is empty; if so, append warning `"ignored metadata entry %q: empty key"` and `continue`
- [x] T019 [P] [US2] Fix registry region enrichment in `internal/registry/registry.go:72-77` — when `meta != nil` but lacks `"region"` key, parse region from binary name and add it to existing metadata map
- [x] T020 [P] [US2] Update `NotFoundError` message in `internal/pulumi/errors.go:36` — change `"provide --pulumi-json"` to `"provide --pulumi-json or --pulumi-state"`
- [x] T021 [US2] Add unit test for `parseMetadataFlags` empty key validation in `internal/cli/plugin_install_internal_test.go` — test that `"=value"` and `"  =value"` produce warnings and are skipped
- [x] T022 [US2] Add unit test for registry region enrichment with non-nil metadata in `internal/registry/registry_test.go` — test that metadata with existing keys but no `"region"` gets region enriched from binary name
- [x] T023 [US2] Add unit test for `NotFoundError` message content in `internal/pulumi/pulumi_test.go` — assert error message contains both `--pulumi-json` and `--pulumi-state`

**Checkpoint**: All 7 correctness fixes applied. `make test` passes with new test cases covering edge cases.

---

## Phase 4: User Story 3 - Structured Logging Consistency (Priority: P2)

**Goal**: Add missing `component` and/or `operation` fields to 4 log call sites across 3 files.

**Independent Test**: Run affected code paths with `FINFOCUS_LOG_LEVEL=debug` and verify all log lines include both fields.

### Implementation for User Story 3

- [x] T024 [P] [US3] Add `Str("component", "pulumi")` and `Str("operation", "detect_project")` to debug log calls in `internal/cli/common_execution.go:215`
- [x] T025 [P] [US3] Add `Str("operation", "get_actual_cost")` to debug log call in `internal/engine/engine.go:1093-1098` (state-based estimation fallback)
- [x] T026 [P] [US3] Add `Str("operation", "get_actual_cost")` to warn log call in `internal/engine/engine.go:1113-1117` (no actual cost data)
- [x] T027 [P] [US3] Add `Str("operation", "parse_plan")` to error log call in `internal/ingest/pulumi_plan.go:73-78`

**Checkpoint**: All 4 log call sites include both `component` and `operation` fields. Consistent with project logging patterns.

---

## Phase 5: User Story 4 - Test Reliability Improvements (Priority: P2)

**Goal**: Isolate auto-detection tests from host filesystem and make context cancellation tests deterministic.

**Independent Test**: Run affected tests 10 times in succession from a directory containing `Pulumi.yaml` and confirm all pass consistently.

### Implementation for User Story 4

- [x] T028 [P] [US4] Add directory isolation to "no flags triggers auto-detection (T023)" subtest in `internal/cli/cost_actual_test.go` — added `isolate` field to test struct with `os.Chdir(t.TempDir())` and `t.Cleanup` restore
- [x] T029 [P] [US4] Add directory isolation to "neither pulumi-json nor pulumi-state" subtest in `internal/cli/cost_actual_test.go` — same pattern as T028
- [x] T030 [P] [US4] Add directory isolation to `TestCostActualWithoutInputFlags` in `internal/cli/cost_actual_test.go` — same pattern as T028
- [x] T031 [P] [US4] Add directory isolation to `TestStackFlagExistsOnActual` in `internal/cli/cost_actual_test.go` — same pattern as T028
- [x] T032 [P] [US4] Replace `time.Sleep(60ms)` with `context.WithDeadline` (past deadline) in `TestPreview_ContextCancellation` in `internal/pulumi/pulumi_test.go` — deterministic DeadlineExceeded without sleep
- [x] T033 [P] [US4] Replace `time.Sleep(60ms)` with `context.WithDeadline` (past deadline) in `TestStackExport_ContextCancellation` in `internal/pulumi/pulumi_test.go` — deterministic DeadlineExceeded without sleep

**Checkpoint**: Tests pass deterministically with `-count=100` and from directories containing Pulumi.yaml.

---

## Phase 6: User Story 5 - Flag Optionality Documentation (Priority: P3)

**Goal**: Document the intentional optionality of `--pulumi-json` flag for auto-detection support.

**Independent Test**: Read the code comment near the flag definition and confirm it explains the design decision.

### Implementation for User Story 5

- [x] T034 [US5] Add inline comment above `--pulumi-json` flag registration in `internal/cli/cost_projected.go:81` explaining the flag is intentionally optional to support Pulumi project auto-detection (no `MarkFlagRequired`)

**Checkpoint**: Code comment clearly documents the intentional design decision.

---

## Phase 7: User Story 6 - Architecture Refactor: PulumiClient Struct (Priority: P3)

**Goal**: Replace the global mutable `Runner` variable with a `PulumiClient` struct for concurrent test safety.

**Independent Test**: Run `go test -race -count=10 ./internal/pulumi/...` and all Pulumi-related tests pass with no race conditions.

### Implementation for User Story 6

- [ ] T035 [US6] Create `PulumiClient` struct with `runner` field, `NewClient()` and `NewClientWithRunner()` constructors in `internal/pulumi/pulumi.go` — keep `FindBinary` and `FindProject` as package-level functions
- [ ] T036 [US6] Convert `runPulumiCommand` to method on `*PulumiClient` in `internal/pulumi/pulumi.go` — change `Runner.Run(ctx, ...)` to `c.runner.Run(ctx, ...)`
- [ ] T037 [US6] Convert `GetCurrentStack` to method on `*PulumiClient` in `internal/pulumi/pulumi.go` — update internal `Runner.Run` call to use `c.runner.Run`
- [ ] T038 [US6] Convert `Preview` and `StackExport` to methods on `*PulumiClient` in `internal/pulumi/pulumi.go` — update to call `c.runPulumiCommand`
- [ ] T039 [US6] Remove global `Runner` variable from `internal/pulumi/pulumi.go:67`
- [ ] T040 [US6] Update `detectPulumiProject` in `internal/cli/common_execution.go` to create and use `*pulumi.PulumiClient` for `GetCurrentStack` calls
- [ ] T041 [US6] Update `resolveResourcesFromPulumi` in `internal/cli/common_execution.go` to use `*pulumi.PulumiClient` for `Preview` and `StackExport` calls
- [ ] T042 [US6] Update `loadOverviewFromAutoDetect` and `resolveOverviewPlan` in `internal/cli/overview.go` to use `*pulumi.PulumiClient`
- [ ] T043 [US6] Update all unit tests in `internal/pulumi/pulumi_test.go` to use `NewClientWithRunner(mock)` instead of global `Runner` assignment
- [ ] T044 [US6] Update integration tests in `test/integration/pulumi_auto_test.go` to use `*pulumi.PulumiClient`
- [ ] T045 [US6] Run `go test -race -count=10 ./internal/pulumi/... ./internal/cli/... ./test/integration/...` to verify no race conditions

**Checkpoint**: Global `Runner` removed. All call sites use struct methods. Race detector passes.

---

## Phase 8: User Story 7 - CI/Tooling: Prettier Formatting (Priority: P3)

**Goal**: Apply Prettier formatting to docs markdown files to prevent CI failures.

**Independent Test**: Run `npx prettier --check docs/guides/routing.md` and confirm no changes needed.

### Implementation for User Story 7

- [x] T046 [P] [US7] Run `npx prettier --check docs/guides/routing.md` — already formatted, no changes needed

**Checkpoint**: Prettier check passes with no formatting changes remaining.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Final validation across all changes

- [x] T047 Run `make test` to verify all tests pass after all changes
- [x] T048 Run `make lint` to verify no lint errors (use extended timeout)
- [x] T049 Verify test coverage has not decreased from baseline recorded in T001 — increased from 68.3% to 70.6%
- [x] T050 Run markdownlint on modified markdown files — no errors

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **US1 (Phase 2)**: Depends on Setup — pure comment edits, no code logic changes
- **US2 (Phase 3)**: Depends on Setup — independent correctness fixes
- **US3 (Phase 4)**: Depends on Setup — independent logging additions
- **US4 (Phase 5)**: Depends on Setup — independent test improvements
- **US5 (Phase 6)**: Depends on Setup — single comment addition
- **US6 (Phase 7)**: Depends on US1-US5 completion (modifies same files, largest blast radius)
- **US7 (Phase 8)**: Depends on Setup — independent docs formatting
- **Polish (Phase 9)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Independent — comment-only changes, no code logic affected
- **US2 (P1)**: Independent — each fix touches different functions/files
- **US3 (P2)**: Independent — logging field additions, no logic changes
- **US4 (P2)**: Independent — test-only changes, no production code affected
- **US5 (P3)**: Independent — single comment addition
- **US6 (P3)**: Depends on US1-US5 — refactors `internal/pulumi/pulumi.go` which is also modified by US1 (doc comments) and US4 (test fixes); should run last
- **US7 (P3)**: Independent — docs-only change

### Parallel Opportunities

- **US1, US2, US3, US4, US5, US7** can all run in parallel (different files or non-overlapping changes)
- Within US1: All T003-T014 can run in parallel (different functions/files)
- Within US2: T015-T020 can run in parallel (different files); T021-T023 depend on their respective implementation tasks
- Within US3: All T024-T027 can run in parallel (different files)
- Within US4: All T028-T033 can run in parallel (different test files)
- **US6 must run last** due to shared file modifications with US1 and US4

---

## Parallel Example: User Stories 1-5 + 7

```text
# These can all launch concurrently after Setup:

# US1 (doc comments) — all parallel, different files:
Task T003: Consolidate resolveSKUAndRegion doc in internal/proto/adapter.go
Task T006: Collapse FindBinary doc in internal/pulumi/pulumi.go
Task T011: Remove stuttered resolveAWSSKU doc in internal/skus/aws.go
Task T013: Complete MapStateResource doc in internal/ingest/state.go

# US2 (correctness) — all parallel, different files:
Task T015: Simplify conditional in internal/cli/cost_actual.go
Task T017: Add audit entry in internal/cli/cost_projected.go
Task T018: Validate empty keys in internal/cli/plugin_install.go
Task T019: Fix region enrichment in internal/registry/registry.go
Task T020: Update NotFoundError in internal/pulumi/errors.go

# US3 (logging) — all parallel, different files:
Task T024: Add fields in internal/cli/common_execution.go
Task T025: Add operation in internal/engine/engine.go
Task T027: Add operation in internal/ingest/pulumi_plan.go

# US7 (prettier) — independent:
Task T046: Format docs/guides/routing.md
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (baseline)
2. Complete Phase 2: US1 — Doc comment cleanup
3. Complete Phase 3: US2 — Code quality fixes
4. **STOP and VALIDATE**: `make test` and `make lint` pass
5. These two P1 stories deliver the highest-value cleanup items

### Incremental Delivery

1. Setup + US1 + US2 → Core cleanup complete (P1 stories)
2. Add US3 + US4 + US5 → Observability + test reliability (P2/P3 stories)
3. Add US7 → CI formatting fix
4. Add US6 → Architecture refactor (optional, may defer to separate PR)
5. Polish → Final validation

---

## Notes

- All tasks within US1 are parallelizable — they edit different functions/files
- US6 (PulumiClient refactor) has the largest blast radius and should run last
- US6 may be deferred to a separate PR per spec assumptions
- Line numbers in task descriptions are approximate — verify current positions before editing
- Use `require.NoError` (not manual `if err != nil`) per project testify standards
