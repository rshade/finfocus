# Tasks: Plugin Install Version Fallback

**Input**: Design documents from `/specs/116-plugin-install-fallback/`
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

- **Single project**: Go project at repository root
- Source: `internal/cli/`, `internal/registry/`
- Tests: `test/unit/`, `test/integration/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and branch setup

- [x] T001 Verify branch `116-plugin-install-fallback` exists and is checked out
- [x] T002 [P] Verify Go 1.25.7 and Cobra v1.10.2 are available

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational (MANDATORY - TDD Required)

- [x] T003 [P] Unit test for FallbackInfo struct in `test/unit/registry/fallback_test.go`
- [x] T004 [P] Unit test for extended InstallOptions fields in `test/unit/registry/fallback_test.go`
- [x] T005 [P] Unit test for extended InstallResult fields in `test/unit/registry/fallback_test.go`

### Implementation for Foundational

- [x] T006 Create FallbackInfo struct in `internal/registry/github.go`
  - Fields: Release, Asset, WasFallback, RequestedVersion, FallbackReason
  - Per data-model.md specification
- [x] T007 [P] Extend InstallOptions struct in `internal/registry/installer.go`
  - Add FallbackToLatest bool field
  - Add NoFallback bool field
- [x] T008 [P] Extend InstallResult struct in `internal/registry/installer.go`
  - Add WasFallback bool field
  - Add RequestedVersion string field
- [x] T009 Create `FindReleaseWithFallbackInfo()` method in `internal/registry/github.go`
  - Wraps existing `FindReleaseWithAsset()` logic
  - Returns FallbackInfo with metadata about fallback occurrence
  - Preserves backward compatibility

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Interactive Fallback (Priority: P1) MVP

**Goal**: When a user interactively installs a plugin version that exists but lacks platform assets, prompt them to accept the latest stable version with compatible assets.

**Independent Test**: Run `finfocus plugin install <plugin>@<version-without-assets>` in a terminal and verify prompt appears with Y/n choice (default: No/abort).

### Tests for User Story 1 (MANDATORY - TDD Required)

- [x] T010 [P] [US1] Unit test for PromptResult struct in `test/unit/cli/prompt_test.go`
- [x] T011 [P] [US1] Unit test for ConfirmFallback function in `test/unit/cli/prompt_test.go`
  - Test with mock stdin for "y", "Y", "n", "N", "", Ctrl+C cases
  - Test non-TTY mode returns immediately with Accepted=false
- [x] T012 [P] [US1] Unit test for fallback prompt trigger in `test/unit/cli/plugin_install_fallback_test.go`
  - Mock scenario: version exists, no assets, TTY mode
  - Verify prompt is called with correct version info
- [x] T013 [US1] Integration test for interactive fallback in `test/integration/plugin/install_fallback_test.go`
  - Test with mock plugin server returning no assets for requested version
  - Verify installer returns "no asset found" error (prompt logic in CLI layer)

### Implementation for User Story 1

- [x] T014 [US1] Create PromptResult struct in `internal/cli/prompt.go`
  - Fields: Accepted, TimedOut, Cancelled (per data-model.md)
- [x] T015 [US1] Create ConfirmFallback function in `internal/cli/prompt.go`
  - Use `bufio.Scanner` for stdin reading (per research.md decision)
  - Use `tui.IsTTY()` for interactive detection
  - Support "Y/n" format with "n" as default (per clarification)
  - Handle empty input as "No" (abort)
  - Return PromptResult with Accepted=false on non-TTY
- [x] T016 [US1] Integrate fallback logic in `internal/cli/plugin_install.go`
  - Detect when version exists but lacks assets via FallbackInfo
  - Check if interactive (TTY) and no flags set
  - Call ConfirmFallback with version info
  - Display warning message per contracts/cli-interface.md
  - On acceptance, proceed with fallback installation
  - On decline, exit with "Installation aborted." message and exit code 1
- [x] T017 [US1] Update install output to show fallback info
  - Show "Installing PLUGIN@FALLBACK-VERSION (fallback from REQUESTED)..."
  - Show "Version: v0.1.2 (requested: v0.1.3)" in success output

**Checkpoint**: User Story 1 complete - interactive fallback with prompt works independently

---

## Phase 4: User Story 2 - Automated Fallback (Priority: P2)

**Goal**: CI/CD pipelines can use `--fallback-to-latest` flag to automatically accept fallback to the latest stable version without user interaction.

**Independent Test**: Run `finfocus plugin install <plugin>@<version-without-assets> --fallback-to-latest` in non-TTY environment and verify automatic fallback without prompt.

### Tests for User Story 2 (MANDATORY - TDD Required)

- [x] T018 [P] [US2] Unit test for --fallback-to-latest flag parsing in `test/unit/cli/plugin_install_fallback_test.go`
  - Verify flag is registered correctly
  - Verify flag value is passed to InstallOptions.FallbackToLatest
- [x] T019 [P] [US2] Unit test for automatic fallback behavior in `test/unit/cli/plugin_install_fallback_test.go`
  - Tests flag parsing, defaults, and usage descriptions
  - Verify flag can be set and interacts correctly with mutual exclusivity
- [x] T020 [US2] Integration test for automated fallback in `test/integration/plugin/install_fallback_test.go`
  - Test FindReleaseWithFallbackInfo correctly finds fallback version
  - Verify installer returns error for no-asset versions (CLI handles fallback)

### Implementation for User Story 2

- [x] T021 [US2] Add --fallback-to-latest flag to install command in `internal/cli/plugin_install.go`
  - Type: bool, default: false
  - Description: "Automatically install latest stable version if requested version lacks assets"
- [x] T022 [US2] Wire --fallback-to-latest flag to InstallOptions in `internal/cli/plugin_install.go`
  - Set InstallOptions.FallbackToLatest from flag value
- [x] T023 [US2] Implement automatic fallback logic in `internal/cli/plugin_install.go`
  - When FallbackToLatest=true and version lacks assets:
    - Skip prompt
    - Show warning message (same as interactive)
    - Proceed with fallback installation
    - Show fallback info in success output

**Checkpoint**: User Story 2 complete - automated fallback with --fallback-to-latest works independently

---

## Phase 5: User Story 3 - Explicit Fallback Disable (Priority: P3)

**Goal**: Users requiring strict version pinning can use `--no-fallback` flag to fail immediately when the requested version lacks platform assets.

**Independent Test**: Run `finfocus plugin install <plugin>@<version-without-assets> --no-fallback` and verify immediate failure with error message.

### Tests for User Story 3 (MANDATORY - TDD Required)

- [x] T024 [P] [US3] Unit test for --no-fallback flag parsing in `test/unit/cli/plugin_install_fallback_test.go`
  - Verify flag is registered correctly
  - Verify flag value is passed to InstallOptions.NoFallback
- [x] T025 [P] [US3] Unit test for mutual exclusivity in `test/unit/cli/plugin_install_fallback_test.go`
  - Verify error when both --fallback-to-latest and --no-fallback are set
  - Verify Cobra's mutual exclusion error message
- [x] T026 [P] [US3] Unit test for immediate failure behavior in `test/unit/cli/plugin_install_fallback_test.go`
  - Tests flag parsing, defaults, and usage descriptions for --no-fallback
  - Verify flag correctly signals strict mode
- [x] T027 [US3] Integration test for explicit disable in `test/integration/plugin/install_fallback_test.go`
  - Test with --no-fallback flag via NoFallback option
  - Verify error message matches: "no asset found"

### Implementation for User Story 3

- [x] T028 [US3] Add --no-fallback flag to install command in `internal/cli/plugin_install.go`
  - Type: bool, default: false
  - Description: "Disable fallback behavior entirely (fail if requested version lacks assets)"
- [x] T029 [US3] Configure mutual exclusivity in `internal/cli/plugin_install.go`
  - Use `cmd.MarkFlagsMutuallyExclusive("fallback-to-latest", "no-fallback")`
  - Per research.md, Cobra v1.10.2 supports this
- [x] T030 [US3] Wire --no-fallback flag to InstallOptions in `internal/cli/plugin_install.go`
  - Set InstallOptions.NoFallback from flag value
- [x] T031 [US3] Implement immediate failure logic in `internal/cli/plugin_install.go`
  - When NoFallback=true and version lacks assets:
    - Skip prompt
    - Return existing error: "no asset found for PLATFORM. Available: []"
    - Exit with code 1

**Checkpoint**: User Story 3 complete - all three user stories work independently

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [x] T032 [P] Update CLI help text in `internal/cli/plugin_install.go`
  - Add examples for --fallback-to-latest and --no-fallback
  - Match help text from contracts/cli-interface.md
- [x] T033 [P] Update docs/reference/cli/plugin-install.md (if exists)
  - N/A - documentation file does not exist
- [x] T034 [P] Run `make lint` and fix any issues
- [x] T035 [P] Run `make test` and ensure 80% coverage minimum
- [x] T036 Run quickstart.md validation
  - Verified help text matches documented examples
  - Full manual testing requires plugin with fallback scenario
- [x] T037 Final code review and cleanup
  - No debug logging present
  - Error messages consistent with existing patterns
  - Backward compatible - no breaking changes to existing commands

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-5)**: All depend on Foundational phase completion
  - US1, US2, US3 can proceed sequentially in priority order
  - US2 depends on US1 fallback logic (integration point)
  - US3 depends on flag infrastructure from US2
- **Polish (Phase 6)**: Depends on all user stories being complete

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Models/structs before functions
- Functions before CLI integration
- Core implementation before output formatting
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- All tests for a user story marked [P] can run in parallel
- Polish tasks marked [P] can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test interactive fallback independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational -> Foundation ready
2. Add User Story 1 -> Test independently -> Interactive fallback works (MVP!)
3. Add User Story 2 -> Test independently -> CI/CD automated fallback works
4. Add User Story 3 -> Test independently -> Strict version pinning works
5. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
