# Tasks: Analyzer Install/Uninstall

**Input**: Design documents from `/specs/590-analyzer-install/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go monorepo**: `internal/`, `cmd/`, `pkg/`, `test/` at repository root
- Core install logic: `internal/analyzer/install.go`
- CLI commands: `internal/cli/analyzer_install.go`, `internal/cli/analyzer_uninstall.go`
- Tests: `*_test.go` alongside source files
- Integration tests: `test/integration/`

---

## Phase 1: Setup

**Purpose**: Verify project structure and confirm existing patterns

- [x] T001 Verify existing analyzer package structure and binary name detection in cmd/finfocus/main.go and internal/analyzer/server.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, constants, and shared helper functions that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational Phase (TDD Required)

- [x] T002 Write tests for InstallOptions, InstallResult types and constants in internal/analyzer/install_test.go
- [x] T003 Write tests for ResolvePulumiPluginDir with precedence: --target-dir > $PULUMI_HOME/plugins/ > ~/.pulumi/plugins/ in internal/analyzer/install_test.go
- [x] T004 [P] Write tests for IsInstalled scanning analyzer-finfocus-v* directories in internal/analyzer/install_test.go
- [x] T005 [P] Write tests for InstalledVersion parsing version from directory name pattern in internal/analyzer/install_test.go
- [x] T006 [P] Write tests for NeedsUpdate comparing installed vs current binary version in internal/analyzer/install_test.go

### Implementation for Foundational Phase

- [x] T007 Define InstallOptions and InstallResult types with analyzerDirPrefix and analyzerBinaryName constants in internal/analyzer/install.go
- [x] T008 Implement ResolvePulumiPluginDir with precedence: override > $PULUMI_HOME/plugins/ > ~/.pulumi/plugins/ in internal/analyzer/install.go
- [x] T009 [P] Implement IsInstalled to scan for analyzer-finfocus-v* directories in internal/analyzer/install.go
- [x] T010 [P] Implement InstalledVersion to parse version from directory name pattern analyzer-finfocus-v{version} in internal/analyzer/install.go
- [x] T011 [P] Implement NeedsUpdate to compare installed version against version.GetVersion() in internal/analyzer/install.go

**Checkpoint**: All helper functions are implemented and tested. Install/Uninstall phases can begin.

---

## Phase 3: User Story 1 - First-Time Analyzer Installation (Priority: P1) MVP

**Goal**: A developer runs `finfocus analyzer install` and the Pulumi analyzer is set up automatically, replacing the 4-step manual process.

**Independent Test**: Run `finfocus analyzer install` on a clean system and verify that the analyzer binary exists in the Pulumi plugin directory with the correct name (`pulumi-analyzer-finfocus`) and permissions.

**Includes**: US3 (Force Reinstall) and US4 (Custom Target Dir) implementation, since the Install function handles all scenarios per Constitution Principle VI (no stubs). US3/US4 specific test scenarios are in their own phases.

### Tests for User Story 1 (TDD Required)

- [x] T012 [P] [US1] Write tests for Install fresh install: resolve binary via os.Executable, create versioned dir, symlink on Unix / copy on Windows in internal/analyzer/install_test.go
- [x] T013 [P] [US1] Write tests for Install already-installed detection: same version no-op, different version suggest --force, --force replaces in internal/analyzer/install_test.go
- [x] T014 [P] [US1] Write tests for Install edge cases: missing binary path, directory creation failure, symlink cross-device fallback to copy in internal/analyzer/install_test.go
- [x] T015 [P] [US1] Write tests for CLI install command output formatting and flag parsing in internal/cli/analyzer_install_test.go

### Implementation for User Story 1

- [x] T016 [US1] Implement Install function with complete logic: resolve binary path via os.Executable+EvalSymlinks, check IsInstalled, handle force/no-force, create versioned directory, symlink (Unix) or copy (Windows) with cross-device fallback in internal/analyzer/install.go
- [x] T017 [US1] Create CLI install command NewAnalyzerInstallCmd with --force and --target-dir flags, RunE calling analyzer.Install, success/already-installed/upgrade output formatting in internal/cli/analyzer_install.go
- [x] T018 [US1] Register install subcommand via AddCommand in internal/cli/analyzer.go

**Checkpoint**: `finfocus analyzer install` works for fresh installs, reports status for existing installs, and supports --force and --target-dir. All US1 acceptance scenarios pass.

---

## Phase 4: User Story 2 - Analyzer Uninstall (Priority: P1)

**Goal**: A developer runs `finfocus analyzer uninstall` and all installed analyzer versions are cleanly removed from the Pulumi plugin directory.

**Independent Test**: Install the analyzer, then run `finfocus analyzer uninstall` and verify the analyzer-finfocus-v* directory is removed.

### Tests for User Story 2 (TDD Required)

- [x] T019 [P] [US2] Write tests for Uninstall removing all analyzer-finfocus-v* directories in internal/analyzer/install_test.go
- [x] T020 [P] [US2] Write tests for Uninstall no-op when analyzer is not installed in internal/analyzer/install_test.go
- [x] T021 [P] [US2] Write tests for CLI uninstall command output formatting and flag parsing in internal/cli/analyzer_uninstall_test.go

### Implementation for User Story 2

- [x] T022 [US2] Implement Uninstall function to scan and remove all analyzer-finfocus-v* directories from plugin dir in internal/analyzer/install.go
- [x] T023 [US2] Create CLI uninstall command NewAnalyzerUninstallCmd with --target-dir flag, RunE calling analyzer.Uninstall, success/no-op output formatting in internal/cli/analyzer_uninstall.go
- [x] T024 [US2] Register uninstall subcommand via AddCommand in internal/cli/analyzer.go

**Checkpoint**: `finfocus analyzer uninstall` removes installed analyzers and reports results. Both install and uninstall work independently.

---

## Phase 5: User Story 3 - Force Reinstall / Upgrade (Priority: P2)

**Goal**: Verify that `finfocus analyzer install --force` correctly replaces an existing installation and that version mismatch detection works with clear user messaging.

**Independent Test**: Install version A, then run `finfocus analyzer install --force` and verify the installed version matches the current binary.

**Note**: The Install function and --force flag are already implemented in Phase 3. This phase adds specific test coverage for US3 acceptance scenarios.

### Tests for User Story 3

- [x] T025 [P] [US3] Write test for upgrade workflow: install v1, simulate version change, verify install without --force suggests upgrade in internal/analyzer/install_test.go
- [x] T026 [P] [US3] Write test for force replacement: install v1, force install v2, verify old directory removed and new directory created in internal/analyzer/install_test.go
- [x] T027 [US3] Write CLI test for version mismatch messaging and --force flag behavior in internal/cli/analyzer_install_test.go

**Checkpoint**: Force reinstall and upgrade workflows are fully validated with explicit test coverage.

---

## Phase 6: User Story 4 - Custom Target Directory (Priority: P3)

**Goal**: Verify that `finfocus analyzer install --target-dir /custom/path` installs to the specified location instead of the default Pulumi plugin directory.

**Independent Test**: Run `finfocus analyzer install --target-dir /tmp/test-plugins` and verify the binary appears at the specified location.

**Note**: The --target-dir flag and ResolvePulumiPluginDir override are already implemented. This phase adds specific test coverage for US4 acceptance scenarios.

### Tests for User Story 4

- [x] T028 [P] [US4] Write test for install with custom target-dir: verify binary placed in specified directory in internal/analyzer/install_test.go
- [x] T029 [P] [US4] Write test for uninstall with custom target-dir: verify removal from specified directory in internal/analyzer/install_test.go
- [x] T030 [US4] Write CLI test for --target-dir flag propagation in both install and uninstall commands in internal/cli/analyzer_install_test.go

**Checkpoint**: Custom target directory works for both install and uninstall commands.

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Integration testing, validation, and final quality checks

- [x] T031 [P] Create integration test for full install-verify-uninstall cycle using t.TempDir() in test/integration/analyzer_install_test.go
- [x] T032 Run make lint and fix any linting issues across all new files
- [x] T033 Run make test and verify 80%+ coverage for internal/analyzer/install.go and CLI commands
- [x] T034 Validate all quickstart.md scenarios execute correctly against the implementation

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 - BLOCKS all user stories
- **US1 Install (Phase 3)**: Depends on Phase 2 completion
- **US2 Uninstall (Phase 4)**: Depends on Phase 2 completion; can run in parallel with Phase 3
- **US3 Force Reinstall (Phase 5)**: Depends on Phase 3 (Install must be implemented first)
- **US4 Custom Target Dir (Phase 6)**: Depends on Phases 3 and 4 (both commands must exist)
- **Polish (Phase 7)**: Depends on all previous phases

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **US2 (P1)**: Can start after Foundational (Phase 2) - Independent of US1 (parallel opportunity)
- **US3 (P2)**: Depends on US1 completion (extends Install behavior)
- **US4 (P3)**: Depends on US1 and US2 completion (tests both commands)

### Within Each User Story

- Tests MUST be written and FAIL before implementation (TDD)
- Helper functions before main functions
- Core logic before CLI commands
- CLI command before registration

### Parallel Opportunities

- **Phase 2**: T004, T005, T006 can run in parallel (different helper function tests)
- **Phase 2**: T009, T010, T011 can run in parallel (different helper function implementations)
- **Phase 3**: T012, T013, T014, T015 can run in parallel (different test files/functions)
- **Phase 4**: T019, T020, T021 can run in parallel (different test files/functions)
- **Phase 3 and Phase 4**: Can be developed in parallel by different developers (independent user stories)
- **Phase 5**: T025, T026 can run in parallel (different test scenarios)
- **Phase 6**: T028, T029 can run in parallel (different test scenarios)

---

## Parallel Example: Phase 2 (Foundational)

```text
# Launch all helper function tests together:
Task: "Write tests for IsInstalled in internal/analyzer/install_test.go"
Task: "Write tests for InstalledVersion in internal/analyzer/install_test.go"
Task: "Write tests for NeedsUpdate in internal/analyzer/install_test.go"

# Then launch all helper function implementations together:
Task: "Implement IsInstalled in internal/analyzer/install.go"
Task: "Implement InstalledVersion in internal/analyzer/install.go"
Task: "Implement NeedsUpdate in internal/analyzer/install.go"
```

## Parallel Example: US1 + US2

```text
# After Phase 2 completes, both stories can start simultaneously:

# Developer A (US1 - Install):
Task: "Write tests for Install function in internal/analyzer/install_test.go"
Task: "Write tests for CLI install command in internal/cli/analyzer_install_test.go"
Task: "Implement Install function in internal/analyzer/install.go"
Task: "Create CLI install command in internal/cli/analyzer_install.go"

# Developer B (US2 - Uninstall):
Task: "Write tests for Uninstall function in internal/analyzer/install_test.go"
Task: "Write tests for CLI uninstall command in internal/cli/analyzer_uninstall_test.go"
Task: "Implement Uninstall function in internal/analyzer/install.go"
Task: "Create CLI uninstall command in internal/cli/analyzer_uninstall.go"
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Setup (verify structure)
2. Complete Phase 2: Foundational (types, helpers, tests)
3. Complete Phase 3: US1 Install (core value proposition)
4. **STOP and VALIDATE**: Test `finfocus analyzer install` independently
5. This alone replaces the 4-step manual process

### Incremental Delivery

1. Complete Setup + Foundational -> Foundation ready
2. Add US1 Install -> Test independently -> MVP complete
3. Add US2 Uninstall -> Test independently -> Install/Uninstall pair complete
4. Add US3 Force Reinstall -> Test independently -> Upgrade workflow complete
5. Add US4 Custom Target Dir -> Test independently -> Full feature complete
6. Each story adds value without breaking previous stories

### Suggested MVP Scope

US1 (First-Time Install) alone delivers the core value: replacing 4 manual steps with 1 command. US2 (Uninstall) should follow immediately as it completes the pair.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Binary naming follows main.go detection: `pulumi-analyzer-finfocus` (not `pulumi-analyzer-cost`)
- Directory naming follows Pulumi convention: `analyzer-finfocus-v{version}/`
- Version from `pkg/version.GetVersion()` - set at build time via ldflags
- ResolvePulumiPluginDir follows existing ResolveConfigDir pattern from internal/config/config.go
- Platform detection via `runtime.GOOS == "windows"` for symlink vs copy strategy
- No new external dependencies required
