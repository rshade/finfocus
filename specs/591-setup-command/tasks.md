# Tasks: finfocus setup — One-Command Bootstrap

**Input**: Design documents from `/specs/591-setup-command/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, quickstart.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create types, command scaffold, and register with root command

- [x] T001 Create `internal/cli/setup.go` with StepStatus type (success/warning/skipped/error), StepResult struct (Name, Status, Message, Critical, Error), SetupOptions struct (SkipAnalyzer, SkipPlugins, NonInteractive), and SetupResult struct (Steps, HasErrors, HasWarnings) per data-model.md
- [x] T002 Create NewSetupCmd() Cobra command in `internal/cli/setup.go` with `--non-interactive`, `--skip-analyzer`, `--skip-plugins` flags and RunE that delegates to runSetup()
- [x] T003 Register NewSetupCmd() in `internal/cli/root.go` by adding it to the cmd.AddCommand() call at line 94

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core orchestration and formatting infrastructure that all user stories depend on

- [x] T004 Implement formatStatus() helper in `internal/cli/setup.go` that returns TTY markers (unicode checkmark/warning/dash/cross) or non-TTY markers ([OK]/[WARN]/[SKIP]/[ERR]) based on SetupOptions.NonInteractive
- [x] T005 Implement runSetup() orchestrator in `internal/cli/setup.go` with collect-and-continue pattern: execute all 6 steps sequentially, collect StepResults into SetupResult, print each step's status line via cmd.Printf(), and return error if any critical step failed
- [x] T006 Create `internal/cli/setup_test.go` with test helpers: newTestSetupCmd() that creates a testable Cobra command with bytes.Buffer output, and assertStepResult() for validating step outcomes

**Checkpoint**: Foundation ready — user story implementation can now begin

---

## Phase 3: User Story 1 + 2 — First-Time Setup + Idempotent Re-Run (Priority: P1)

**Goal**: Complete bootstrap from clean system; safe to re-run without data loss

**Independent Test**: Run `finfocus setup` on a clean t.TempDir(), verify all directories/config/artifacts exist. Run again, verify no errors and existing config preserved.

### Tests for US1 + US2 (MANDATORY — TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T007 [P] [US1] Write test for stepDisplayVersion() verifying output contains version string and Go runtime version in `internal/cli/setup_test.go`
- [x] T008 [P] [US1] Write tests for stepDetectPulumi() covering both found (verify version in output) and not-found (verify warning status, non-fatal) cases in `internal/cli/setup_test.go`
- [x] T009 [P] [US1] Write test for stepCreateDirectories() verifying base, plugins, cache, and logs directories are created with correct permissions in `internal/cli/setup_test.go`
- [x] T010 [P] [US1] Write test for stepInitConfig() verifying default config.yaml is written when absent in `internal/cli/setup_test.go`
- [x] T011 [P] [US1] Write test for stepInstallAnalyzer() verifying it calls analyzer.Install() and maps ActionInstalled/ActionAlreadyCurrent/error to correct StepResult in `internal/cli/setup_test.go`
- [x] T012 [P] [US1] Write test for stepInstallPlugins() verifying it iterates defaultPlugins slice and handles install success/failure per plugin in `internal/cli/setup_test.go`
- [x] T013 [P] [US2] Write idempotency tests: directories already exist (success, not error), config already exists (preserved, not overwritten), analyzer already current (ActionAlreadyCurrent mapped to success) in `internal/cli/setup_test.go`
- [x] T014 [US2] Write integration test: execute full setup via newTestSetupCmd() on t.TempDir(), then execute again on same dir, verify zero errors and identical filesystem state in `internal/cli/setup_test.go`

### Implementation for US1 + US2

- [x] T015 [US1] Implement stepDisplayVersion() in `internal/cli/setup.go` using version.GetVersion() and runtime.Version()
- [x] T016 [US1] Implement stepDetectPulumi() in `internal/cli/setup.go` using pulumi.FindBinary() for PATH detection and exec.CommandContext for `pulumi version` output; return warning StepResult if not found
- [x] T017 [US1] Implement stepCreateDirectories() in `internal/cli/setup.go` using config.ResolveConfigDir() for base path, os.MkdirAll for base (0700), plugins (0750), cache (0700), and logs (0700) directories; return separate StepResult per directory with critical=true
- [x] T018 [US1] Implement stepInitConfig() in `internal/cli/setup.go` using os.Stat to check existence, config.New() for defaults, config.Save() only if file absent; return critical StepResult
- [x] T019 [US1] Implement stepInstallAnalyzer() in `internal/cli/setup.go` calling analyzer.Install(ctx, analyzer.InstallOptions{}) and mapping result.Action to StepResult with critical=false
- [x] T020 [US1] Implement stepInstallPlugins() in `internal/cli/setup.go` iterating defaultPlugins (var defaultPlugins = []string{"aws-public"}), calling registry.Installer.Install() for each with FallbackToLatest=true, collecting per-plugin StepResults with critical=false
- [x] T021 [US1] Implement printSummary() in `internal/cli/setup.go` printing completion message with next-steps hint via cmd.Printf(); implement exit code logic in runSetup() returning error when SetupResult.HasErrors is true

**Checkpoint**: US1 + US2 fully functional — first-time setup works, re-runs are safe

---

## Phase 4: User Story 3 + 4 — Non-Interactive + Skip Flags (Priority: P2)

**Goal**: Support CI/CD pipelines with TTY-independent output and selective step skipping

**Independent Test**: Run with `--non-interactive` and verify ASCII markers in output; run with `--skip-analyzer --skip-plugins` and verify only directories + config created

### Tests for US3 + US4 (MANDATORY — TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T022 [P] [US3] Write test for non-interactive output: execute setup with NonInteractive=true, verify output contains [OK]/[WARN] markers instead of unicode symbols in `internal/cli/setup_test.go`
- [x] T023 [P] [US3] Write test for TTY auto-detection: verify that when stdin is not a terminal, NonInteractive is set automatically in `internal/cli/setup_test.go`
- [x] T024 [P] [US4] Write test for --skip-analyzer: execute setup with SkipAnalyzer=true, verify analyzer step has skipped status and no analyzer.Install() call in `internal/cli/setup_test.go`
- [x] T025 [P] [US4] Write test for --skip-plugins: execute setup with SkipPlugins=true, verify plugin step has skipped status in `internal/cli/setup_test.go`
- [x] T026 [US4] Write test for combined skip flags: execute setup with both SkipAnalyzer=true and SkipPlugins=true, verify only directories and config steps execute in `internal/cli/setup_test.go`

### Implementation for US3 + US4

- [x] T027 [US3] Add TTY auto-detection to runSetup() in `internal/cli/setup.go`: check isTerminal(os.Stdin) and set opts.NonInteractive=true if not a terminal; pass NonInteractive through to formatStatus() for all step output

**Checkpoint**: US3 + US4 complete — CI/CD and selective setup work correctly

---

## Phase 5: User Story 5 — Custom Home Directory (Priority: P3)

**Goal**: Support FINFOCUS_HOME and PULUMI_HOME environment variable overrides for enterprise deployments

**Independent Test**: Set FINFOCUS_HOME to a temp dir, run setup, verify all resources created under that path

### Tests for US5 (MANDATORY — TDD Required)

- [x] T028 [P] [US5] Write test for FINFOCUS_HOME override: set env var to t.TempDir(), execute setup, verify directories and config created under custom path in `internal/cli/setup_test.go`
- [x] T029 [P] [US5] Write test for PULUMI_HOME fallback: set PULUMI_HOME (without FINFOCUS_HOME), execute setup, verify resources created under PULUMI_HOME/finfocus/ in `internal/cli/setup_test.go`

**Checkpoint**: US5 complete — custom home directory works for enterprise deployments

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Edge cases, quality gates, and documentation

- [x] T030 Write edge case tests in `internal/cli/setup_test.go`: permission denied on directory creation (verify error StepResult with actionable message), partial setup (directories exist but config missing), exit code 0 with warnings, exit code 1 with critical failure
- [x] T031 Run `make lint` and fix all linting issues across `internal/cli/setup.go` and `internal/cli/setup_test.go`
- [x] T032 Run `make test` and verify all tests pass including existing tests (no regressions)
- [x] T033 Verify 80%+ test coverage for `internal/cli/setup.go` using `go test -coverprofile=coverage.out ./internal/cli/... && go tool cover -func=coverage.out`
- [x] T034 Validate quickstart.md scenarios: run `finfocus setup` on clean dir, verify output matches quickstart.md examples

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (types must exist for orchestrator)
- **US1+US2 (Phase 3)**: Depends on Phase 2 (orchestrator must exist)
- **US3+US4 (Phase 4)**: Depends on Phase 3 (step functions must exist for flag/TTY tests)
- **US5 (Phase 5)**: Depends on Phase 3 (step functions must exist for env var tests)
- **Polish (Phase 6)**: Depends on all previous phases

### User Story Dependencies

- **US1+US2 (P1)**: Can start after Phase 2 — no dependencies on other stories
- **US3+US4 (P2)**: Depends on US1 implementation (steps must exist to test with flags)
- **US5 (P3)**: Depends on US1 implementation (steps must exist to test with env vars)
- **US4 and US5 can run in parallel** after US1 is complete

### Within Each User Story

- Tests MUST be written and FAIL before implementation begins
- Step functions can be implemented in any order within a story
- Story complete before moving to next priority

### Parallel Opportunities

- All test tasks within a phase marked [P] can run in parallel (independent test functions)
- Phase 4 (US3+US4) and Phase 5 (US5) can run in parallel after Phase 3 completes
- Polish tasks T031 and T032 can run in parallel

---

## Parallel Example: US1 Tests

```bash
# Launch all US1 test tasks together (independent test functions):
T007: "Write test for stepDisplayVersion() in setup_test.go"
T008: "Write tests for stepDetectPulumi() in setup_test.go"
T009: "Write test for stepCreateDirectories() in setup_test.go"
T010: "Write test for stepInitConfig() in setup_test.go"
T011: "Write test for stepInstallAnalyzer() in setup_test.go"
T012: "Write test for stepInstallPlugins() in setup_test.go"
```

---

## Implementation Strategy

### MVP First (US1+US2 Only)

1. Complete Phase 1: Setup (types + command)
2. Complete Phase 2: Foundational (orchestrator + formatting)
3. Complete Phase 3: US1+US2 (all 6 steps + idempotency)
4. **STOP and VALIDATE**: Run `make lint && make test`, verify coverage
5. At this point, `finfocus setup` works end-to-end

### Incremental Delivery

1. Phase 1+2: Foundation ready
2. Phase 3 (US1+US2): Full setup works, idempotent (MVP!)
3. Phase 4 (US3+US4): CI/CD and selective setup
4. Phase 5 (US5): Enterprise custom paths
5. Phase 6: Polish and coverage enforcement

---

## Notes

- All source code in 2 files: `internal/cli/setup.go` and `internal/cli/setup_test.go` (plus 1 line change in `root.go`)
- [P] tasks = independent test functions, no file-level conflicts
- [Story] label maps task to specific user story for traceability
- US1 and US2 are merged in Phase 3 because idempotency is inherent to each step function (not separate logic)
- US3 and US4 are merged in Phase 4 because both are flag-driven behavior modifiers
- Skip flags (US4) require no additional implementation — the step functions from US1 already check SetupOptions; Phase 4 adds test coverage
- Commit after each phase or logical group
- Stop at any checkpoint to validate independently
