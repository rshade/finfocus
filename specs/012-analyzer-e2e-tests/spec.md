# Feature Specification: Analyzer E2E Tests

**Feature Branch**: `012-analyzer-e2e-tests`
**Created**: 2025-12-08
**Status**: Draft
**Input**: User description: "Add E2E tests for Pulumi Analyzer plugin integration - verifying the complete workflow from `pulumi preview` to cost diagnostic output with real Pulumi CLI"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Verify Analyzer Handshake Protocol (Priority: P1)

As a FinFocus maintainer, I need to verify that the analyzer plugin successfully completes the handshake protocol with the Pulumi CLI, ensuring the gRPC server starts, prints the port number to stdout correctly, and establishes connection with the Pulumi engine.

**Why this priority**: The handshake is the foundation of all analyzer functionality. If handshake fails, no cost diagnostics can be delivered. This must work before any other feature can be validated.

**Independent Test**: Can be fully tested by configuring a Pulumi project with the analyzer, running `pulumi preview`, and verifying the analyzer process starts without protocol errors. Delivers confidence that the basic integration works.

**Acceptance Scenarios**:

1. **Given** a Pulumi project configured with the FinFocus analyzer, **When** `pulumi preview` is executed, **Then** the analyzer server should start and complete the handshake without errors.
2. **Given** the analyzer is starting, **When** the Pulumi engine connects, **Then** the server should print only the port number to stdout (no other text).
3. **Given** the analyzer is running, **When** the preview completes, **Then** the analyzer should gracefully shut down.

---

### User Story 2 - Verify Cost Diagnostics in Preview Output (Priority: P1)

As a FinFocus user running `pulumi preview`, I need to see cost estimation diagnostics in the preview output for each resource being created, so I can understand the cost implications before deploying.

**Why this priority**: This is the core value proposition of the analyzer - showing costs during preview. Without visible diagnostics, the feature provides no user benefit.

**Independent Test**: Can be tested by running `pulumi preview` on a project with known resources (e.g., EC2 instance) and verifying cost diagnostics appear in the output.

**Acceptance Scenarios**:

1. **Given** a Pulumi project with a priced resource (e.g., AWS EC2 instance), **When** `pulumi preview` runs with the analyzer, **Then** the preview output should include a cost diagnostic message for that resource.
2. **Given** multiple resources in the preview, **When** the analyzer processes them, **Then** each resource should have an individual cost estimate displayed.
3. **Given** a resource type without pricing data, **When** the analyzer processes it, **Then** a diagnostic should appear indicating no pricing information is available.

---

### User Story 3 - Verify Stack Cost Summary (Priority: P2)

As a FinFocus user, I need to see a total estimated monthly cost summary for the entire stack at the end of the preview, so I can quickly understand overall cost impact.

**Why this priority**: Summary provides aggregate value after individual diagnostics. Important but not critical for basic functionality validation.

**Independent Test**: Can be tested by verifying the preview output contains a summary line showing total estimated cost across all resources.

**Acceptance Scenarios**:

1. **Given** a Pulumi preview with multiple priced resources, **When** the preview completes, **Then** a stack summary diagnostic should show total estimated monthly cost.
2. **Given** a preview with zero-cost resources only, **When** the preview completes, **Then** the summary should show $0.00 total.

---

### User Story 4 - Verify Graceful Degradation (Priority: P2)

As a FinFocus maintainer, I need the analyzer to handle errors gracefully without blocking or failing the Pulumi preview, ensuring cost calculation issues never prevent infrastructure deployment.

**Why this priority**: Important for production reliability but secondary to core functionality verification.

**Independent Test**: Can be tested by configuring scenarios where pricing lookup fails and verifying preview still completes successfully with warning diagnostics.

**Acceptance Scenarios**:

1. **Given** a resource type that causes cost calculation to fail, **When** `pulumi preview` runs, **Then** the preview should complete successfully (not error out).
2. **Given** a cost calculation error occurs, **When** the analyzer generates diagnostics, **Then** a warning diagnostic should appear instead of an error that blocks deployment.
3. **Given** no plugins are installed, **When** the analyzer runs, **Then** it should operate in spec-only mode and continue without failing.

---

### User Story 5 - Verify Test Environment Portability (Priority: P3)

As a CI/CD pipeline maintainer, I need the E2E tests to run reliably in CI without requiring cloud provider credentials, using only local Pulumi backend and mock providers.

**Why this priority**: Important for CI/CD automation but can use existing patterns from current E2E test infrastructure.

**Independent Test**: Can be tested by running the E2E tests in a fresh environment with only Pulumi CLI installed and verifying tests pass or skip gracefully.

**Acceptance Scenarios**:

1. **Given** Pulumi CLI is not installed, **When** E2E tests run, **Then** tests should skip gracefully with an informative message.
2. **Given** a CI environment without AWS credentials, **When** E2E tests run, **Then** tests should complete using local backend and mock resources.

---

### Edge Cases

- What happens when the analyzer server cannot bind to a port? The preview should fail with a clear error message from the analyzer.
- How does the system handle analyzer process crashes during preview? Pulumi should report the analyzer failure but complete the preview.
- What happens when analyzer produces malformed diagnostics? Pulumi should handle gracefully and continue the preview.
- How does the system behave when the analyzer takes too long to respond? Timeout handling should be tested.
- What happens when running on Windows vs Unix? The binary location and execution should work cross-platform.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST build the `finfocus` binary before running E2E tests using the existing TestMain pattern.
- **FR-002**: System MUST create test Pulumi projects as fixtures with predictable resources for validation.
- **FR-003**: System MUST use `pulumi login --local` to avoid cloud backend dependencies.
- **FR-004**: System MUST configure the analyzer plugin in test project's `Pulumi.yaml` using the `analyzers` configuration.
- **FR-005**: System MUST execute `pulumi preview` and capture both stdout and stderr output for validation.
- **FR-006**: System MUST verify cost diagnostics appear in the preview output by parsing the output.
- **FR-007**: System MUST verify stack summary diagnostic shows total estimated cost.
- **FR-008**: System MUST verify graceful degradation when cost calculation fails (no preview failures).
- **FR-009**: System MUST skip tests gracefully when Pulumi CLI is not installed.
- **FR-010**: Tests MUST use the `//go:build e2e` build tag to allow skipping in quick test runs.
- **FR-011**: Tests MUST integrate with the existing `nightly.yml` workflow structure.
- **FR-012**: System MUST clean up all test resources (stacks, temp directories) after tests complete.

### Key Entities

- **Test Fixture Project**: A minimal Pulumi project (YAML-based) with predictable resources for testing. Contains `Pulumi.yaml` with analyzer configuration and resource definitions.
- **Analyzer Configuration**: The `analyzers` section in `Pulumi.yaml` pointing to the built `finfocus` binary with path and version.
- **Diagnostic Output**: The cost estimation messages that appear in `pulumi preview` output, including per-resource costs and status.
- **Stack Summary**: The aggregate cost summary diagnostic at the end of preview showing total monthly cost.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: E2E tests complete successfully within the existing 90-minute nightly test timeout.
- **SC-002**: Tests can run in CI environment using only Pulumi CLI and local backend (no cloud credentials required for analyzer-specific tests).
- **SC-003**: Tests detect regressions in analyzer handshake protocol (tests fail if handshake breaks).
- **SC-004**: Tests detect regressions in diagnostic output format (tests fail if diagnostics stop appearing).
- **SC-005**: Tests skip gracefully when Pulumi CLI is not available (no hard failures on missing dependencies).
- **SC-006**: Tests pass with `make test-e2e` following existing E2E test patterns.
- **SC-007**: Test execution adds no more than 15 minutes to overall nightly test run time.

## Assumptions

- Pulumi CLI v3.x or later is available in CI environment (already installed per `nightly.yml`).
- YAML-based Pulumi projects are used for test fixtures to avoid additional language runtime dependencies.
- The `finfocus` binary is built before tests run (existing TestMain pattern).
- Real AWS resources (t3.micro EC2) are used to validate actual cost accuracy.
- AWS credentials are configured in both local environment and `nightly.yml`.
- The `aws-public` plugin is installed using existing `PluginManager` infrastructure.
- Existing E2E test infrastructure (`test/e2e/`) provides reusable patterns for test setup and teardown.
- The analyzer configuration in `Pulumi.yaml` uses the `plugins.analyzers` section format.
- The test environment has write access to temp directories for local Pulumi state.

## Dependencies

- Pulumi CLI must be installed in CI environment (already configured in `nightly.yml`).
- AWS credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY) in CI environment.
- Built `finfocus` binary with analyzer functionality.
- `aws-public` plugin for accurate pricing data.
- Existing E2E test framework in `test/e2e/`.
- Go 1.25.7+ for test execution.

## Out of Scope

- Performance benchmarking of analyzer - separate concern from functional E2E testing.
- Testing analyzer with TypeScript/Python/Go Pulumi programs - YAML provides sufficient coverage for integration testing.
- Multi-cloud testing (Azure/GCP) - AWS provides sufficient coverage for analyzer integration validation.
