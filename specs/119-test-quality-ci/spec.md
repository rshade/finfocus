# Feature Specification: Test Quality and CI/CD Improvements

**Feature Branch**: `118-test-quality-ci`
**Created**: 2026-01-19
**Status**: Draft
**Input**: Implement test quality improvements and CI/CD enhancements addressing GitHub issues #326, #334, and #236

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Fuzz Test Coverage for Pulumi Plan Parsing (Priority: P1)

As a developer maintaining the JSON parser, I need fuzz tests to cover real Pulumi plan structures including `newState` nesting, so that edge cases and malformed input are detected before reaching production.

**Why this priority**: Fuzz testing is a critical safety net for parsing untrusted input. The `newState` structure is how Pulumi actually provides resource details, making this the highest priority gap.

**Independent Test**: Can be verified by running `go test -fuzz=FuzzJSON -fuzztime=30s ./internal/ingest` and confirming the seed corpus includes `newState` structures.

**Acceptance Scenarios**:

1. **Given** the fuzz test corpus, **When** a Pulumi plan with `newState.inputs` is provided, **Then** the parser extracts resource properties correctly
2. **Given** the fuzz test corpus, **When** a Pulumi plan with both `oldState` and `newState` (update operation) is provided, **Then** the parser handles the transition correctly
3. **Given** the fuzz test corpus, **When** malformed JSON with partial `newState` is provided, **Then** the parser fails gracefully without panic

---

### User Story 2 - Benchmark Accuracy for Performance Testing (Priority: P1)

As a developer running performance benchmarks, I need the benchmark tests to use valid Pulumi JSON structures (`steps` instead of legacy `resourceChanges`), so that benchmark results reflect actual parsing performance.

**Why this priority**: Invalid benchmark data produces meaningless results, wasting CI resources and giving false confidence about performance characteristics.

**Independent Test**: Can be verified by running `go test -bench=. ./test/benchmarks/...` and confirming benchmarks complete successfully with realistic Pulumi structures.

**Acceptance Scenarios**:

1. **Given** the benchmark generates test JSON, **When** the benchmark runs, **Then** the JSON uses `steps` array with proper `op`, `urn`, and `type` fields
2. **Given** the benchmark parses generated JSON, **When** parsing completes, **Then** resources are successfully extracted (not zero)

---

### User Story 3 - Cross-Repository Integration Testing (Priority: P2)

As a release engineer, I need automated CI workflow that tests Core and Plugin integration together, so that breaking changes between repositories are caught before release.

**Why this priority**: Cross-repo integration failures are currently only caught manually. Automated nightly testing prevents version incompatibility issues from reaching users.

**Independent Test**: Can be verified by triggering the workflow manually via `workflow_dispatch` and confirming both repos build and integrate successfully.

**Acceptance Scenarios**:

1. **Given** the cross-repo workflow is triggered, **When** both Core and Plugin repositories are checked out, **Then** both build successfully
2. **Given** the plugin is installed, **When** `finfocus plugin list` runs, **Then** the plugin appears in the list
3. **Given** the plugin is validated, **When** `finfocus cost projected` runs with a sample plan, **Then** cost results are returned without errors
4. **Given** a nightly schedule, **When** 2 AM UTC arrives, **Then** the workflow runs automatically

---

### User Story 4 - E2E Test for Actual Cost Command (Priority: P2)

As a QA engineer, I need E2E tests for the `cost actual` command using real Pulumi state files, so that the actual cost pipeline is validated end-to-end with the AWS plugin.

**Why this priority**: The aws-public plugin now implements `GetActualCost`, removing the previous blocker. This enables complete validation of the actual cost feature.

**Independent Test**: Can be verified by running `go test -v -tags=e2e ./test/e2e/... -run TestE2E_ActualCost` with the plugin installed.

**Acceptance Scenarios**:

1. **Given** a state file with timestamped resources, **When** `cost actual` command runs, **Then** JSON output contains cost data for each resource
2. **Given** a state file with imported (external) resources, **When** `cost actual` command runs, **Then** the command completes without error
3. **Given** a resource with `created` and `modified` timestamps, **When** actual cost is calculated, **Then** the cost reflects the runtime period

---

### User Story 5 - Test Error Handling Improvements (Priority: P3)

As a developer writing tests, I need all test files to properly handle errors from standard library functions, so that test failures are clear and actionable rather than cryptic nil pointer panics.

**Why this priority**: Proper error handling in tests improves debugging experience and prevents false positives when tests fail for environmental reasons.

**Independent Test**: Can be verified by code review confirming `require.NoError(t, err)` follows all `filepath.Abs` and similar fallible operations.

**Acceptance Scenarios**:

1. **Given** a test uses `filepath.Abs`, **When** the path resolution fails, **Then** the test fails with a clear error message (not nil pointer panic)
2. **Given** an NDJSON output test, **When** validating output lines, **Then** schema correctness is verified (not just JSON syntax)

---

### Edge Cases

- What happens when the plugin binary location differs between CI and local development?
- How does the cross-repo workflow handle plugin build failures gracefully?
- What happens when the state file contains resources from unsupported providers?
- How does the actual cost test handle resources with missing timestamps? → Skip resource and log warning
- What happens when the benchmark generates more resources than memory allows?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Fuzz test corpus MUST include seed entries with `newState` structure matching actual Pulumi preview output
- **FR-002**: Fuzz test corpus MUST include update operations with both `oldState` and `newState`
- **FR-003**: Benchmark tests MUST use `steps` array (not legacy `resourceChanges`) for JSON generation
- **FR-004**: Benchmark resource format MUST include `op`, `urn`, and `type` fields matching Pulumi structure
- **FR-005**: GCP E2E test MUST check error return from `filepath.Abs` using testify assertions
- **FR-006**: NDJSON test MUST validate schema fields (`resource_type`, `currency`) for cost result lines
- **FR-007**: Cross-repo workflow MUST support manual triggering via `workflow_dispatch` with customizable refs
- **FR-008**: Cross-repo workflow MUST run on nightly schedule (2 AM UTC)
- **FR-009**: Cross-repo workflow MUST checkout both finfocus-core and finfocus-plugin-aws-public
- **FR-010**: Cross-repo workflow MUST build and install plugin before running integration tests
- **FR-011**: ActualCost E2E test MUST use state fixtures with `created` and `modified` timestamps
- **FR-012**: ActualCost E2E test MUST validate JSON output contains `resource_type`, `actual_cost`, and `currency` fields
- **FR-013**: ActualCost E2E test MUST include test case for imported (external=true) resources
- **FR-014**: Cross-repo workflow MUST create a GitHub issue automatically when the workflow fails
- **FR-015**: ActualCost calculation MUST skip resources with missing `created` timestamps and log a warning

### Key Entities

- **Pulumi Plan JSON**: Preview output containing `steps` array with resource operations, each step having `op`, `urn`, `newState`, and optionally `oldState`
- **Pulumi State JSON**: Stack export containing `deployment.resources` array with `created`, `modified`, `inputs`, and `outputs` for each resource
- **Cost Result**: Output object containing `resource_type`, `monthly_cost` or `actual_cost`, `currency`, and optional `notes`
- **Test Fixture**: Static JSON file in `test/fixtures/` representing realistic Pulumi output for deterministic testing (state fixtures in `test/fixtures/state/`)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Fuzz tests discover parsing edge cases when run for 30+ seconds (`go test -fuzz=FuzzJSON -fuzztime=30s`)
- **SC-002**: Benchmarks complete successfully and report meaningful operations per second (> 0 ns/op)
- **SC-003**: All E2E tests pass in CI environment with plugin installed
- **SC-004**: Cross-repo workflow completes in under 10 minutes for typical runs
- **SC-005**: No test files contain ignored errors from `filepath.Abs` or similar fallible operations
- **SC-006**: NDJSON output validation catches schema violations (missing required fields)
- **SC-007**: ActualCost E2E test validates end-to-end flow from state file to cost output

## Assumptions

- The aws-public plugin's `GetActualCost` implementation uses runtime x list price calculation as documented
- GitHub Actions `workflow_dispatch` is available for manual workflow triggering
- The `actions/checkout@v6` and `actions/setup-go@v6` actions are available and stable
- Plugin binary naming conventions follow existing patterns (`finfocus-plugin-*` or `bin/*`)
- Go 1.25.7 fuzz testing features are stable and compatible with CI runners
- Nightly schedule cron (`0 2 * * *`) runs at 2 AM UTC as expected by GitHub Actions

## Clarifications

### Session 2026-01-19

- Q: How should the team be notified when the nightly cross-repo workflow fails? → A: Create GitHub issue automatically on failure (existing pattern)
- Q: How should ActualCost E2E test handle resources with missing `created` timestamps? → A: Skip resource and log warning (evaluate in production)

## Out of Scope

- Performance optimization of the parser itself (this feature only validates existing behavior)
- Adding new fuzz test targets beyond JSON and YAML parsing
- Plugin functionality changes (plugin is assumed to work correctly)
- Cost accuracy validation (E2E tests validate structure, not cost correctness)
- Multi-provider cross-repo testing (initially only aws-public plugin)
