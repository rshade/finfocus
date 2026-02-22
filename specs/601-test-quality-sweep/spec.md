# Feature Specification: Test Quality Sweep

**Feature Branch**: `601-test-quality-sweep`
**Created**: 2026-02-22
**Status**: Draft
**Input**: User description: "Batch test quality improvements across 11 GitHub issues (786, 785, 784, 782, 776, 775, 774, 743, 737, 722, 683): fix vacuous tests, close leaked resources, hermetic env isolation, deduplicate test setup, consolidate to table-driven tests, fix masked errors, fix always-skipped integration tests, verify defensive copies, and fix test data quality issues."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Fix Bug-Hiding Test Defects (Priority: P1)

As a developer, I need tests that actually verify the behavior they claim to test, so that real regressions are caught rather than silently passing.

**Why this priority**: Vacuous tests (#786), masked errors (#743), and always-skipped tests (#737) represent the highest risk -- they create a false sense of safety while bugs slip through undetected.

**Independent Test**: Can be verified by confirming that previously-vacuous assertions now exercise the intended code paths, that error-level log output is visible during integration test failures, and that previously-skipped integration tests execute in CI.

**Acceptance Scenarios**:

1. **Given** the budget_scoped_test exit code 0 test case, **When** the test runs, **Then** `ExitOnThreshold` is set to true so `validateExitCode()` exercises the exit code range validation path.
2. **Given** integration tests with global log suppression, **When** a plugin communication error occurs at WARN/ERROR level, **Then** the error is visible in test output via test-aware log routing.
3. **Given** integration tests that were always skipped (recorder, plugin version, CLI streaming, Pulumi auto), **When** CI runs the integration suite, **Then** each test either executes successfully or is gated behind an explicit build tag rather than a runtime skip that always triggers.

---

### User Story 2 - Eliminate Resource Leaks in Tests (Priority: P1)

As a developer, I need tests to properly clean up resources (clients, file handles, temp directories) so that goroutine leaks and file descriptor exhaustion do not cause flaky CI failures.

**Why this priority**: Leaked plugin clients (#785) and unclosed cache stores (#683) can cause goroutine leaks and race detector failures, directly impacting CI reliability.

**Independent Test**: Can be verified by running the affected test files with the race detector and confirming no leaked goroutines at test exit.

**Acceptance Scenarios**:

1. **Given** TestNewClient_Success creates a plugin client, **When** the test completes, **Then** `client.Close()` has been called via defer.
2. **Given** TestClient_APIUsage creates a plugin client, **When** the test completes, **Then** `client.Close()` has been called via defer.
3. **Given** the disabled operations cache test, **When** the test completes, **Then** the store has been closed via defer.

---

### User Story 3 - Ensure Hermetic Test Isolation (Priority: P1)

As a developer, I need tests to run hermetically without depending on the developer's local environment, so that tests produce consistent results regardless of the machine.

**Why this priority**: Non-hermetic tests (#784, #683) can pass locally but fail in CI (or vice versa), wasting developer time on environment-specific debugging.

**Independent Test**: Can be verified by setting `FINFOCUS_HOME` to a nonexistent path before running the affected config tests and confirming they still pass.

**Acceptance Scenarios**:

1. **Given** `stubHome(t)` is called in config tests, **When** the test runs with `FINFOCUS_HOME` set externally, **Then** `FINFOCUS_HOME` has been cleared to empty so `GetConfigDir()` uses the stubbed `HOME` path.
2. **Given** InitCache tests, **When** they run, **Then** they use an isolated temp directory rather than the real user home path.

---

### User Story 4 - Consolidate Duplicate Tests into Table-Driven (Priority: P2)

As a developer, I need duplicated test logic consolidated into table-driven test suites so that adding new test cases requires only a new table entry rather than copying boilerplate.

**Why this priority**: Issues #774, #775, #776, and #782 collectively remove hundreds of lines of duplicated scaffolding, reducing maintenance burden and making test coverage gaps easier to spot.

**Independent Test**: Can be verified by running `go test -v` on each affected package and confirming all previously-passing assertions still pass, with reduced line count.

**Acceptance Scenarios**:

1. **Given** 4 near-identical cost projected tests, **When** consolidated, **Then** a single table-driven test covers all cases with approximately 350 fewer lines.
2. **Given** 9 flat tests in pulumi_plan_test.go, **When** merged into table-driven suites, **Then** unique assertions are preserved, manual `t.Errorf`/`t.Fatalf` calls are converted to testify, and duplicate cases are removed.
3. **Given** 5 TestGetPluginInfo_* functions, **When** consolidated, **Then** a single table-driven test with struct fields covering all variation points replaces the 5 functions.
4. **Given** plugin_validate_test.go with duplicated env setup, **When** refactored, **Then** tests use the shared `setupTestEnv(t)` helper and fragile substring assertions are replaced with precise assertions.

---

### User Story 5 - Fix Test Data Quality Issues (Priority: P2)

As a developer, I need test data to be realistic and assertions to be precise, so that tests validate actual correctness rather than accidentally passing on malformed data.

**Why this priority**: Control characters in test data (#683) can mask real encoding bugs, and weak assertions allow incorrect keys to pass validation.

**Independent Test**: Can be verified by inspecting the generated test data for absence of control characters and by confirming key assertions check exact values.

**Acceptance Scenarios**:

1. **Given** performance test data generation using `string(rune(i))`, **When** fixed, **Then** string formatting produces printable strings without null bytes or control characters.
2. **Given** recommendation key assertion that only checks prefix, **When** fixed, **Then** the assertion validates the exact expected key value.
3. **Given** an unused `wantNil` field in cache store test structs, **When** fixed, **Then** the field is either used in assertions or removed entirely.

---

### User Story 6 - Verify Defensive Copy Independence (Priority: P3)

As a developer, I need the DataReadyMsg handler's defensive copy test to actually prove that the copy is independent of the source, so that future regressions in copy semantics are caught.

**Why this priority**: This is a correctness verification for an existing defensive pattern -- lower risk since the pattern is already implemented, but the test gap should be closed.

**Independent Test**: Can be verified by mutating the original `testRows` slice after the model update and asserting the model's internal state is unaffected.

**Acceptance Scenarios**:

1. **Given** TestOverviewModel_DataReadyMsg has verified state transition, **When** the original `testRows` slice is mutated after the update, **Then** the model's internal row data remains unchanged.

---

### Edge Cases

- What happens when a table-driven test case has an empty or nil input where the original flat test relied on package-level state?
- How does the system handle integration tests that require external binaries when those binaries are unavailable -- explicit skip with message vs build tag gating?
- What happens if `stubHome(t)` cleanup restores `FINFOCUS_HOME` to a value that was set by a parent test?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The budget_scoped_test exit code 0 test case MUST set `ExitOnThreshold` to true so that `validateExitCode()` exercises the exit code range validation path.
- **FR-002**: All plugin clients created in test functions MUST be closed via `defer client.Close()` after successful creation.
- **FR-003**: The `stubHome(t)` helper MUST clear `FINFOCUS_HOME` (set to empty string) in addition to `HOME` and `USERPROFILE`.
- **FR-004**: Plugin validate tests MUST use `setupTestEnv(t)` instead of duplicating environment setup, and fragile substring assertions MUST be replaced with precise assertions.
- **FR-005**: The 5 `TestGetPluginInfo_*` functions MUST be consolidated into a single table-driven test with struct fields covering all variation points.
- **FR-006**: The 9 flat tests in pulumi_plan_test.go MUST be merged into existing table-driven suites, converting manual assertions to testify.
- **FR-007**: The 4 cost projected tests MUST be consolidated into a single table-driven test, eliminating duplicated scaffolding.
- **FR-008**: Integration test log suppression MUST NOT mask WARN/ERROR level messages from plugin communication; errors MUST be routed through test-aware log output.
- **FR-009**: Always-skipped integration tests MUST either be made runnable (by building required binaries in TestMain or replacing external dependencies) or gated behind explicit build tags with documentation.
- **FR-010**: The DataReadyMsg handler test MUST verify defensive copy independence by mutating the source after copy and asserting the model state is unaffected.
- **FR-011**: Performance test data MUST use string formatting to produce printable strings without null bytes or control characters.
- **FR-012**: Cache key assertions MUST validate exact expected key values, not just prefixes.
- **FR-013**: Test struct fields MUST be connected to assertions; any genuinely unused fields MUST be removed.
- **FR-014**: InitCache tests MUST use isolated temp directories rather than potentially resolving to real user paths.
- **FR-015**: All changes MUST maintain or improve existing test coverage -- no previously-passing test logic may be lost during consolidation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 11 issues are resolved with `make test` passing and zero regressions.
- **SC-002**: `make lint` passes with zero new warnings introduced.
- **SC-003**: Net reduction of at least 300 lines of duplicated test code through table-driven consolidation.
- **SC-004**: Zero goroutine leaks detected when running affected test packages with `-race` flag.
- **SC-005**: Previously-skipped integration tests (#737) have at least one execution path that runs in CI without requiring external pre-built binaries.
- **SC-006**: All test assertions are precise -- no substring-only checks where exact values are known, no vacuous test cases that pass without exercising the code under test.
- **SC-007**: Config tests pass regardless of whether `FINFOCUS_HOME` is set in the developer's environment.

## Assumptions

- The `ptr()` helper function for creating pointers to primitive values already exists in the test files or can be trivially added.
- `setupTestEnv(t)` in plugin_validate_test.go already handles the common environment setup needed by the validate tests.
- The `zerolog.TestWriter(t)` approach is compatible with the existing integration test framework for routing log output.
- Table-driven consolidation will preserve all unique assertion logic from the original flat tests.
- Build tag gating is an acceptable alternative to fixing always-skipped tests when external dependencies cannot be eliminated.

## Scope Boundaries

### In Scope

- All 11 listed GitHub issues and their specific file changes
- Converting manual assertion patterns to testify where touched
- Removing dead code (unused struct fields) in modified files

### Out of Scope

- Adding new test coverage beyond what the 11 issues require
- Refactoring test files not mentioned in the issues
- Changes to production (non-test) code
- CI pipeline configuration changes (beyond build tag usage)
- Performance optimization of test execution time
