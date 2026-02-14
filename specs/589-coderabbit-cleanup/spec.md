# Feature Specification: CodeRabbit Cleanup from Pulumi Auto-Detect PR

**Feature Branch**: `589-coderabbit-cleanup`
**Created**: 2026-02-13
**Status**: Draft
**Input**: Follow-up items identified by CodeRabbit review on the `509-pulumi-auto-detect` PR. These are out-of-scope cleanup tasks grouped by category for incremental resolution.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consistent Codebase Documentation (Priority: P1)

As a developer reading FinFocus source code, I encounter duplicated, stuttered, or incomplete doc comments on recently added functions. Each function should have exactly one concise Go-style doc comment so I can quickly understand what the function does without wading through repetitive text.

**Why this priority**: Doc comments are the primary onboarding surface for contributors. Duplicated or truncated comments cause confusion and erode trust in code quality.

**Independent Test**: Can be verified by running `go doc` on each affected function and confirming exactly one clean comment appears per function, with no stuttered or truncated sentences.

**Acceptance Scenarios**:

1. **Given** the function `resolveSKUAndRegion` in `internal/proto/adapter.go`, **When** I read its doc comment, **Then** there is a single concise paragraph describing its behavior with no duplicated sentences.
2. **Given** the function `MapStateResource` in `internal/ingest/state.go`, **When** I read its doc comment, **Then** it clearly states that user-declared inputs take precedence over provider-computed outputs on conflict.
3. **Given** the `mockPbcCostSourceServiceClient` in `internal/proto/adapter_test.go`, **When** I read its doc comment, **Then** it accurately states that all methods return empty success responses (not "panic").

---

### User Story 2 - Code Quality and Correctness Improvements (Priority: P1)

As a developer maintaining the CLI layer, I need small logic fixes, missing validation, and error handling improvements so the codebase is correct and defensive against edge cases.

**Why this priority**: These are correctness issues that could lead to subtle bugs or confusing error messages in production use.

**Independent Test**: Can be verified by running unit tests that cover simplified conditionals, error handling on flag parsing, audit logging paths, metadata validation, registry enrichment, and error message content.

**Acceptance Scenarios**:

1. **Given** the `resolveFromDate` function in `cost_actual.go`, **When** the conditional for auto-detecting from state timestamps is evaluated, **Then** the condition is simplified to `params.planPath == ""` (removing the redundant disjunction).
2. **Given** the `GetString("stack")` call in `cost_actual.go`, **When** the flag retrieval returns an error, **Then** the error is captured and returned to the caller.
3. **Given** the auto-detect path in `cost_projected.go`, **When** `resolveResourcesFromPulumi` fails, **Then** an audit entry is recorded before returning the error.
4. **Given** a metadata flag entry `"=value"` passed to `parseMetadataFlags`, **When** the function processes it, **Then** it produces a warning and skips the entry rather than inserting an empty-key entry into the map.
5. **Given** a plugin version directory with existing metadata that lacks a `"region"` key, **When** the registry scans for plugins, **Then** the region parsed from the binary name is added to the existing metadata.
6. **Given** the `NotFoundError` function in `internal/pulumi/errors.go`, **When** it constructs the user-facing error message, **Then** the message references both `--pulumi-json` and `--pulumi-state` options.

---

### User Story 3 - Structured Logging Consistency (Priority: P2)

As an operator debugging FinFocus in production, I need every log line to include both `component` and `operation` fields so I can filter and correlate log events across the full request lifecycle.

**Why this priority**: Missing structured fields degrade observability. This is important but does not affect correctness.

**Independent Test**: Can be verified by inspecting log output at debug level and confirming every affected log call includes both `Str("component", ...)` and `Str("operation", ...)` fields.

**Acceptance Scenarios**:

1. **Given** the debug log calls in `common_execution.go` for project_dir and stack detection, **When** they fire, **Then** both include `Str("component", "pulumi")` and `Str("operation", "detect_project")` (or analogous operation name).
2. **Given** the debug and warning log calls in `engine.go` for state-based estimation and no actual cost, **When** they fire, **Then** both include a `Str("operation", ...)` field alongside the existing `component` field.
3. **Given** the error log call in `pulumi_plan.go` for parse failure, **When** it fires, **Then** it includes `Str("operation", "parse_plan")` alongside the existing `component` field.

---

### User Story 4 - Test Reliability Improvements (Priority: P2)

As a CI pipeline, I need auto-detection tests to be isolated from the host filesystem and context cancellation tests to be deterministic so test results are stable regardless of working directory or timing.

**Why this priority**: Flaky tests waste developer time and reduce trust in CI signals.

**Independent Test**: Can be verified by running the affected tests repeatedly in a directory containing `Pulumi.yaml` and confirming they pass consistently.

**Acceptance Scenarios**:

1. **Given** the auto-detection tests in `cost_actual_test.go` (T023, line 257, `TestCostActualWithoutInputFlags`, `TestStackFlagExistsOnActual`), **When** they run, **Then** each test uses `t.TempDir()` with `os.Chdir()` to isolate from the host working directory.
2. **Given** the context cancellation tests in `pulumi_test.go` (`TestPreview_ContextCancellation`, `TestStackExport_ContextCancellation`), **When** they run, **Then** they use explicit `cancel()` calls before invoking the function under test instead of `time.Sleep` for timing.

---

### User Story 5 - Intentional Flag Optionality Documentation (Priority: P3)

As a developer reviewing the CLI flags, I need the `--pulumi-json` flag's intentional optionality to be clearly documented in code comments so I understand it was deliberately made optional for auto-detection support.

**Why this priority**: Without documentation, a future contributor might re-add `MarkFlagRequired` thinking it was accidentally removed.

**Independent Test**: Can be verified by reading the code comment near the flag definition and confirming it explains the intentional decision.

**Acceptance Scenarios**:

1. **Given** the `--pulumi-json` flag registration in `cost_projected.go`, **When** I read the surrounding code, **Then** a comment explains that the flag is intentionally optional to support Pulumi project auto-detection.

---

### User Story 6 - Architecture Refactor: Replace Global Runner (Priority: P3)

As a developer writing concurrent tests for the Pulumi integration, I need the mutable package-level `Runner` variable replaced with a `PulumiClient` struct that embeds a `CommandRunner` so tests can run in parallel without shared mutable state.

**Why this priority**: This is a larger refactor that improves test isolation and code structure but does not affect runtime behavior. It may warrant its own PR.

**Independent Test**: Can be verified by running all Pulumi-related tests with `-race` and `-count=10` and confirming no race conditions.

**Acceptance Scenarios**:

1. **Given** the `internal/pulumi` package, **When** I inspect it, **Then** the global `Runner` variable no longer exists.
2. **Given** the `PulumiClient` struct, **When** I call `Preview` or `StackExport`, **Then** they are methods on the struct (not package-level functions).
3. **Given** all call sites of the former package-level functions, **When** I inspect them, **Then** they use the new `PulumiClient` struct.

---

### User Story 7 - CI/Tooling: Prettier Formatting (Priority: P3)

As a CI pipeline, I need docs markdown files to pass Prettier formatting checks so documentation PRs do not fail on formatting.

**Why this priority**: Low risk, low effort, but prevents CI failures on documentation changes.

**Independent Test**: Can be verified by running `npx prettier --check` on the affected docs files and confirming no formatting errors.

**Acceptance Scenarios**:

1. **Given** the file `docs/guides/routing.md`, **When** Prettier formatting is applied, **Then** no formatting changes remain and CI passes.

---

### Edge Cases

- What happens when `parseMetadataFlags` receives an entry with only whitespace before the `=` (e.g., `"  =value"`)? The trimmed key is empty and should produce a warning.
- What happens when `registry.go` enrichment encounters metadata with a `"region"` key already set? The existing value should be preserved (not overwritten).
- What happens when all auto-detection steps fail in `cost_projected.go` but the audit context was never initialized? The audit entry should still be recorded using the same pattern as `loadAndMapResources`.
- What happens when `os.Chdir` in tests fails? Tests should call `require.NoError(t, err)` to fail fast.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST consolidate all duplicated Go doc comments into single, concise comments per function (11 functions across 5 files)
- **FR-002**: System MUST complete the truncated `MapStateResource` doc comment to clarify that user-declared inputs take precedence over provider-computed outputs on conflict
- **FR-003**: System MUST correct the `mockPbcCostSourceServiceClient` doc comment to accurately state all methods return empty success responses
- **FR-004**: System MUST add `Str("component", ...)` and `Str("operation", ...)` to all log calls that are missing them (4 locations across 3 files)
- **FR-005**: System MUST simplify the `resolveFromDate` conditional from `params.statePath != "" || (params.planPath == "" && params.statePath == "")` to `params.planPath == ""`
- **FR-006**: System MUST handle the error from `cmd.Flags().GetString("stack")` in `cost_actual.go` instead of discarding it
- **FR-007**: System MUST record an audit entry when `resolveResourcesFromPulumi` fails in `cost_projected.go`
- **FR-008**: System MUST document the intentional optionality of `--pulumi-json` flag in `cost_projected.go`
- **FR-009**: System MUST validate that metadata keys are non-empty after trimming in `parseMetadataFlags` and produce a warning for empty keys
- **FR-010**: System MUST enrich plugin metadata with region from binary name when metadata exists but lacks a `"region"` key
- **FR-011**: System MUST update `NotFoundError` in `errors.go` to reference both `--pulumi-json` and `--pulumi-state` options
- **FR-012**: System MUST isolate auto-detection tests using `t.TempDir()` and `os.Chdir()` for directory isolation
- **FR-013**: System MUST replace `time.Sleep` in context cancellation tests with explicit `cancel()` calls
- **FR-014**: System MUST replace the global `Runner` variable in `internal/pulumi/pulumi.go` with a `PulumiClient` struct
- **FR-015**: System MUST apply Prettier formatting to affected docs markdown files

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All affected functions have exactly one doc comment each with no duplicated sentences, verified by `go doc` output inspection
- **SC-002**: All log calls in the 4 identified locations include both `component` and `operation` fields, verified by structured log output at debug level
- **SC-003**: All 7 code quality items (3a-3g) pass their respective acceptance criteria, verified by targeted unit tests
- **SC-004**: Auto-detection tests pass reliably when run from a directory containing `Pulumi.yaml`, verified by running tests 10 times in succession
- **SC-005**: Context cancellation tests pass deterministically without timing-dependent `time.Sleep`, verified by running with `-count=100`
- **SC-006**: `make lint` and `make test` pass with zero new warnings or failures
- **SC-007**: Test coverage does not decrease from baseline after changes

## Assumptions

- The simplified conditional `params.planPath == ""` is logically equivalent to the original due to mutual-exclusivity enforced by `validateActualInputFlags`
- The `--pulumi-json` flag optionality is an intentional design decision (not a bug) and only requires documentation
- The global `Runner` refactor (Group 5) may be deferred to a separate PR if it proves too large for this cleanup branch
- Prettier is available via `npx prettier` in the CI environment
- The audit context pattern used in `loadAndMapResources` is the correct pattern to mirror for the auto-detect path
