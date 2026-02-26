# Tasks: Analyzer Quality Cluster

**Input**: Design documents from `/specs/603-analyzer-quality/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Go Test Path Conventions

Unit tests for Go projects MUST be colocated with source code, not placed in `test/unit/`.

- **Unit tests**: `internal/[package]/[name]_test.go` (colocated with source)
  - Black-box (public API): `package foo_test`
  - White-box (unexported access): `package foo`
  - Run with: `go test ./internal/...`
- **Integration tests**: `test/integration/` (cross-component, requires running plugins)
  - Run with: `go test ./test/integration/...`
- **E2E tests**: `test/e2e/` (requires built binary and external credentials)
  - Run with: `go test -tags e2e ./test/e2e/...`

---

## Phase 1: US1 — Accurate Stack Cost Summary (P1, #746)

**Goal**: Fix the $0.00 stack summary bug so `AnalyzeStack` reports the correct
total of all per-resource costs cached during `Analyze()` calls.

**Independent Test**: Run `go test ./internal/analyzer/...` and verify the
stack summary diagnostic message matches the sum of individually cached costs.

### Tests for US1 (TDD — Write First)

- [X] T001 [P] [US1] Add test `TestStackSummaryDiagnostic_MatchesBuildCostSummary` verifying `StackSummaryDiagnostic` and `BuildCostSummary` produce consistent totals for the same input in `internal/analyzer/diagnostics_test.go`
- [X] T002 [P] [US1] Add test `TestStackSummaryDiagnostic_ExcludesErrors` verifying error resources (ERROR:/VALIDATION: prefix) are excluded from the summary total but successful resources are counted in `internal/analyzer/diagnostics_test.go`
- [X] T003 [P] [US1] Add test `TestAnalyze_CachesErrorCosts` verifying that when `GetProjectedCost` returns an error, a zero-cost error result is still cached for `AnalyzeStack` visibility in `internal/analyzer/server_test.go`
- [X] T004 [P] [US1] Add test `TestAnalyzeStack_MixedSuccessAndError` verifying that a stack with 3 successful and 2 error resources reports correct total and "3 resources analyzed" in `internal/analyzer/server_test.go`

### Implementation for US1

- [X] T005 [US1] Refactor `StackSummaryDiagnostic()` in `internal/analyzer/diagnostics.go` to use `BuildCostSummary()` for aggregation, formatting the message from the `CostSummary` struct fields (`TotalMonthlyCost`, `ResourceCount`, `Currency`) instead of duplicating counting logic
- [X] T006 [US1] In `Analyze()` in `internal/analyzer/server.go`, when `calcErr != nil` (line ~203), cache a zero-cost error result with `Notes: "ERROR: " + calcErr.Error()` before returning the `WarningDiagnostic`, so the resource is visible to `AnalyzeStack`
- [X] T007 [US1] Verify all existing `internal/analyzer/` tests pass with `go test ./internal/analyzer/...` and update any tests broken by the counting logic change

**Checkpoint**: Stack summary now accurately reflects per-resource costs. Run
`go test ./internal/analyzer/... -run "TestStackSummary|TestAnalyze"` to verify.

---

## Phase 2: US2 — Policy Pack Directory Setup (P2, #755)

**Goal**: `finfocus analyzer install` automatically creates `~/.finfocus/analyzer/`
with `PulumiPolicy.yaml` and a correctly-named binary reference so
`pulumi preview --policy-pack` works immediately.

**Independent Test**: Run `finfocus analyzer install` in a temp directory, then
verify `PulumiPolicy.yaml` exists with `runtime: finfocus` and the binary
reference is a valid symlink or copy.

### Tests for US2 (TDD — Write First)

- [X] T008 [P] [US2] Add test `TestResolvePolicyPackDir` verifying default path `~/.finfocus/analyzer/` and `FINFOCUS_HOME` override in new file `internal/analyzer/policypack_test.go`
- [X] T009 [P] [US2] Add test `TestWritePulumiPolicyYAML` verifying YAML content has `name: finfocus`, `runtime: finfocus`, and `description` field in `internal/analyzer/policypack_test.go`
- [X] T010 [P] [US2] Add test `TestSetupPolicyPack_CreatesDirectory` verifying directory creation, binary symlink/copy, and idempotent re-setup in `internal/analyzer/policypack_test.go`
- [X] T011 [P] [US2] Add test `TestSetupPolicyPack_WindowsCopy` verifying file copy is used on Windows (mock `runtime.GOOS` or test the `copyFile` path) in `internal/analyzer/policypack_test.go`
- [X] T012 [P] [US2] Add test `TestInstall_SetsPolicyPackResult` verifying `InstallResult` has `PolicyPackDir` and `PolicyPackMethod` populated after a successful install in `internal/analyzer/install_test.go`

### Implementation for US2

- [X] T013 [P] [US2] Create `internal/analyzer/policypack.go` with `ResolvePolicyPackDir() (string, error)` returning `~/.finfocus/analyzer/` (with `FINFOCUS_HOME` override), `PolicyPackConfig` struct, `WritePulumiPolicyYAML(dir string) error`, and `SetupPolicyPack(ctx, execPath string) (dir, method string, err error)` that creates the directory, writes YAML, and creates the binary reference via `linkOrCopy()`
- [X] T014 [US2] Add `PolicyPackDir string` and `PolicyPackMethod string` fields to `InstallResult` in `internal/analyzer/install.go`
- [X] T015 [US2] Integrate `SetupPolicyPack()` call into `Install()` in `internal/analyzer/install.go` after the Pulumi plugin install succeeds, storing results in `InstallResult.PolicyPackDir` and `InstallResult.PolicyPackMethod`
- [X] T016 [US2] Update CLI output in `internal/cli/analyzer_install.go` to print `Policy pack: <dir>` and `Policy pack method: <method>` after a successful install

**Checkpoint**: `finfocus analyzer install` creates both the Pulumi plugin directory
and the policy pack directory. Run `go test ./internal/analyzer/... -run "TestPolicyPack|TestInstall"`.

---

## Phase 3: US3 — Force Reinstall Syncs Policy Pack (P3, #754)

**Goal**: `finfocus analyzer install --force` updates both the Pulumi plugin
binary and the policy pack binary when the policy pack directory exists.

**Depends on**: Phase 2 (US2) must be complete.

**Independent Test**: Install version A, force reinstall version B, verify
both binary locations reflect version B.

### Tests for US3 (TDD — Write First)

- [X] T017 [P] [US3] Add test `TestInstall_ForceSyncsPolicyPack` verifying `--force` updates the policy pack binary when the policy pack directory exists in `internal/analyzer/install_test.go`
- [X] T018 [P] [US3] Add test `TestInstall_ForceSkipsMissingPolicyPack` verifying `--force` does not error when the policy pack directory does not exist in `internal/analyzer/install_test.go`
- [X] T019 [P] [US3] Add test `TestInstall_ForceSyncFailureWarns` verifying that a policy pack sync failure produces a warning log but does not fail the overall install in `internal/analyzer/install_test.go`

### Implementation for US3

- [X] T020 [US3] Add `syncPolicyPackBinary(ctx context.Context, execPath, policyPackDir string) error` helper in `internal/analyzer/install.go` that removes the old binary reference and creates a new one via `linkOrCopy()`
- [X] T021 [US3] In `Install()` in `internal/analyzer/install.go`, after the force reinstall succeeds, call `syncPolicyPackBinary()` if the policy pack directory exists; log a warning on failure (FR-009)

**Checkpoint**: `--force` keeps both binary locations in sync. Run
`go test ./internal/analyzer/... -run "TestInstall_Force"`.

---

## Phase 4: US4 — Post-Install PATH Instructions (P4, #756)

**Goal**: After a successful install, print clear PATH export instructions
and a sample `pulumi preview --policy-pack` command so users know the next step.

**Depends on**: Phase 2 (US2) must be complete.

**Independent Test**: Run `finfocus analyzer install` and verify the output
includes PATH instructions for fresh installs but not for no-ops or JSON mode.

### Tests for US4 (TDD — Write First)

- [X] T022 [P] [US4] Add test `TestAnalyzerInstallCmd_PrintsPATHInstructions` verifying PATH export and `pulumi preview --policy-pack` instructions appear in output for fresh installs in `internal/cli/analyzer_install_test.go`
- [X] T023 [P] [US4] Add test `TestAnalyzerInstallCmd_NoPATHOnNoOp` verifying PATH instructions are absent when the install is a no-op (`already_current`) in `internal/cli/analyzer_install_test.go`

### Implementation for US4

- [X] T024 [US4] In `internal/cli/analyzer_install.go`, after the `ActionInstalled` case, print PATH instructions referencing `result.PolicyPackDir` with `export PATH="$HOME/.finfocus/analyzer:$PATH"` and `pulumi preview --policy-pack ~/.finfocus/analyzer` (FR-010, FR-011)

**Checkpoint**: Fresh installs print next-step instructions. Run
`go test ./internal/cli/... -run "TestAnalyzerInstall"`.

---

## Phase 5: US5 — Analyzer Check Command (P5, #757)

**Goal**: `finfocus analyzer check` verifies policy pack directory,
`PulumiPolicy.yaml`, binary in PATH, and gRPC server responsiveness, reporting
pass/fail with actionable remediation.

**Depends on**: Phase 2 (US2) must be complete.

**Independent Test**: Run `finfocus analyzer check` in a correctly configured
environment (all pass) and a misconfigured environment (failures with remediation).

### Tests for US5 (TDD — Write First)

- [X] T025 [P] [US5] Add test `TestCheckPolicyPackDir_Pass` and `TestCheckPolicyPackDir_Fail` verifying directory existence check returns correct status and remediation in new file `internal/analyzer/check_test.go`
- [X] T026 [P] [US5] Add test `TestCheckPulumiPolicyYAML_Pass` and `TestCheckPulumiPolicyYAML_Fail` verifying YAML validation returns correct status and message in `internal/analyzer/check_test.go`
- [X] T027 [P] [US5] Add test `TestCheckBinaryInPATH_Fail` verifying PATH check returns fail status with `export PATH=...` remediation in `internal/analyzer/check_test.go`
- [X] T028 [P] [US5] Add test `TestRunChecks_SkipCascade` verifying that when policy pack dir check fails, subsequent checks are skipped with status `skip` in `internal/analyzer/check_test.go`
- [X] T029 [P] [US5] Add test `TestAnalyzerCheckCmd_AllPass` and `TestAnalyzerCheckCmd_JSONOutput` verifying CLI output format for table and JSON modes in new file `internal/cli/analyzer_check_test.go`

### Implementation for US5

- [X] T030 [P] [US5] Create `internal/analyzer/check.go` with `CheckResult` and `CheckReport` types (from data-model.md), and implement `checkPolicyPackDir() CheckResult` verifying `~/.finfocus/analyzer/` exists
- [X] T031 [P] [US5] Implement `checkPulumiPolicyYAML(dir string) CheckResult` in `internal/analyzer/check.go` that reads and validates `PulumiPolicy.yaml` has `runtime: finfocus`
- [X] T032 [US5] Implement `checkBinaryInPATH() CheckResult` in `internal/analyzer/check.go` using `exec.LookPath("pulumi-analyzer-policy-finfocus")`
- [X] T033 [US5] Implement `checkGRPCSmokeTest(ctx context.Context) CheckResult` in `internal/analyzer/check.go` that starts `finfocus analyzer serve` subprocess, reads port from stdout, makes `GetAnalyzerInfo` gRPC call with 5-second timeout, and cleans up
- [X] T034 [US5] Implement `RunChecks(ctx context.Context) (*CheckReport, error)` in `internal/analyzer/check.go` that executes checks sequentially with skip cascading (fail on policy_pack_dir skips downstream checks)
- [X] T035 [US5] Create `internal/cli/analyzer_check.go` with `NewAnalyzerCheckCmd() *cobra.Command` supporting `--output` flag (table/json), printing pass/fail indicators for table mode, JSON-marshaled `CheckReport` for json mode, and exiting with code 0 on all-pass or 1 on any failure
- [X] T036 [US5] Wire `NewAnalyzerCheckCmd()` into `NewAnalyzerCmd()` in `internal/cli/analyzer.go` by adding `cmd.AddCommand(NewAnalyzerCheckCmd())`

**Checkpoint**: `finfocus analyzer check` works in both pass and fail scenarios.
Run `go test ./internal/analyzer/... -run "TestCheck"` and
`go test ./internal/cli/... -run "TestAnalyzerCheck"`.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Documentation updates, full validation, coverage verification.

- [X] T037 [P] Update analyzer section in `CLAUDE.md` to document the `analyzer check` command, policy pack directory setup in `analyzer install`, and `--force` sync behavior
- [X] T038 [P] Update `internal/cli/analyzer.go` command examples to include `finfocus analyzer check` and `finfocus analyzer install` with policy pack output
- [X] T039 Run `make test` to verify all tests pass across the entire project
- [X] T040 Run `make lint` to verify all linting passes (including markdownlint)
- [X] T041 Verify 80%+ test coverage on new code with `go test -coverprofile=coverage.out ./internal/analyzer/... && go tool cover -func=coverage.out`

---

## Dependencies and Execution Order

### Phase Dependencies

```text
Phase 1 (US1 - #746) ─────────────────────────────────────→ Phase 6 (Polish)
                                                              ↑
Phase 2 (US2 - #755) ──┬── Phase 3 (US3 - #754) ──────────→ │
                        ├── Phase 4 (US4 - #756) ──────────→ │
                        └── Phase 5 (US5 - #757) ──────────→ │
```

### User Story Dependencies

- **US1 (P1, #746)**: No dependencies — can start immediately
- **US2 (P2, #755)**: No dependencies — can start immediately (parallel with US1)
- **US3 (P3, #754)**: Depends on US2 — needs policy pack infrastructure
- **US4 (P4, #756)**: Depends on US2 — needs `PolicyPackDir` in `InstallResult`
- **US5 (P5, #757)**: Depends on US2 — needs policy pack structure to check against

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation tasks follow dependency order (types → logic → integration)
- Story complete when all tests pass and acceptance scenarios verified

### Parallel Opportunities

**Phase 1 and Phase 2 run in parallel** (no cross-dependencies):

- T001-T004 (US1 tests) run in parallel with T008-T012 (US2 tests)
- T005-T006 (US1 impl) run in parallel with T013-T016 (US2 impl)

**Phase 3, 4, and 5 run in parallel** (all depend only on Phase 2):

- T017-T021 (US3) run in parallel with T022-T024 (US4) and T025-T036 (US5)

**Within each phase**, tasks marked [P] can run in parallel.

---

## Parallel Example: US1 and US2

```text
# Launch US1 tests and US2 tests simultaneously:
T001: TestStackSummaryDiagnostic_MatchesBuildCostSummary (diagnostics_test.go)
T002: TestStackSummaryDiagnostic_ExcludesErrors (diagnostics_test.go)
T003: TestAnalyze_CachesErrorCosts (server_test.go)
T004: TestAnalyzeStack_MixedSuccessAndError (server_test.go)
  ↕ parallel with ↕
T008: TestResolvePolicyPackDir (policypack_test.go)
T009: TestWritePulumiPolicyYAML (policypack_test.go)
T010: TestSetupPolicyPack_CreatesDirectory (policypack_test.go)
T011: TestSetupPolicyPack_WindowsCopy (policypack_test.go)
T012: TestInstall_SetsPolicyPackResult (install_test.go)
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1 (US1): Fix $0.00 stack summary
2. **STOP and VALIDATE**: Verify stack summary is accurate
3. This alone resolves the highest-priority user-facing bug

### Incremental Delivery

1. US1 (P1): Fix $0.00 summary → highest impact, standalone
2. US2 (P2): Policy pack setup → enables the primary workflow
3. US3 (P3): Force sync → correctness on upgrades
4. US4 (P4): PATH instructions → UX polish
5. US5 (P5): Check command → diagnostic tooling
6. Each story adds value without breaking previous stories

### Parallel Strategy

With capacity for parallel work:

1. **Parallel**: US1 + US2 simultaneously (no cross-dependencies)
2. **Parallel**: US3 + US4 + US5 simultaneously (all depend only on US2)
3. Polish phase after all stories complete

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing
- Run `make lint` and `make test` before claiming any story complete
- US1 and US2 have zero cross-dependencies and are ideal parallel candidates
