# Feature Specification: Budget Threshold Exit Codes

**Feature Branch**: `124-budget-exit-codes`
**Created**: 2026-01-24
**Status**: Draft
**Input**: User description: "Add configurable exit codes when budget thresholds are exceeded, enabling CI/CD pipeline integration for automated cost governance."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - CI/CD Pipeline Fails on Budget Exceeded (Priority: P1)

A CI/CD operator configures their pipeline to fail when cost estimates exceed the configured budget, preventing deployments that would bust the budget.

**Why this priority**: This is the core value proposition—enabling automated cost governance in CI/CD pipelines. Without this, there's no machine-readable signal for pipelines to act on.

**Independent Test**: Can be fully tested by running `finfocus cost projected` with a Pulumi plan that exceeds budget, verifying the command exits with a non-zero code, and confirming the exit code value matches configuration.

**Acceptance Scenarios**:

1. **Given** a budget of $100 is configured with `exit_on_threshold: true`, **When** the projected cost is $150, **Then** the command exits with exit code 1 (default).
2. **Given** a budget of $100 is configured with `exit_on_threshold: true` and `exit_code: 2`, **When** the projected cost is $150, **Then** the command exits with exit code 2.
3. **Given** a budget of $100 is configured with `exit_on_threshold: true`, **When** the projected cost is $80, **Then** the command exits with exit code 0 (no threshold exceeded).
4. **Given** no budget is configured, **When** the cost command runs, **Then** the command exits with exit code 0 regardless of cost.

---

### User Story 2 - Environment-Based Exit Code Configuration (Priority: P2)

A platform engineer sets different exit behaviors for different environments using environment variables, allowing production to be strict while development is lenient.

**Why this priority**: Multi-environment support is essential for real-world CI/CD adoption where different environments have different policies.

**Independent Test**: Can be tested by setting environment variables and verifying exit code behavior matches the environment configuration without any config file.

**Acceptance Scenarios**:

1. **Given** `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD=true` is set, **When** budget is exceeded, **Then** the command exits with non-zero code.
2. **Given** `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD=false` is set (overriding config file with `exit_on_threshold: true`), **When** budget is exceeded, **Then** the command exits with code 0.
3. **Given** `FINFOCUS_BUDGET_EXIT_CODE=3` is set, **When** budget is exceeded with exit enabled, **Then** the command exits with code 3.
4. **Given** both environment variable and config file specify exit code, **When** budget is exceeded, **Then** environment variable takes precedence.

---

### User Story 3 - Warning Threshold Exit Behavior (Priority: P2)

A FinOps manager configures warning thresholds (e.g., 80%) to trigger non-zero exit codes, enabling early warning in pipelines before budget is fully consumed.

**Why this priority**: Early warning prevents surprises and allows teams to take action before hitting hard budget limits.

**Independent Test**: Can be tested by configuring an 80% warning threshold, running cost command with spend at 85%, and verifying exit code behavior.

**Acceptance Scenarios**:

1. **Given** a budget of $100 with 80% warning alert and `exit_on_threshold: true`, **When** projected cost is $85, **Then** the command exits with configured exit code.
2. **Given** a budget of $100 with 80% warning alert and `exit_on_threshold: true`, **When** projected cost is $70, **Then** the command exits with code 0 (below warning threshold).
3. **Given** multiple thresholds (50%, 80%, 100%) with `exit_on_threshold: true`, **When** any threshold is exceeded, **Then** the command exits with the configured exit code.

---

### User Story 4 - CLI Flag Overrides (Priority: P3)

An operator overrides the configured exit behavior at runtime using CLI flags for one-off checks or testing.

**Why this priority**: Runtime flexibility is valuable but less common than persistent configuration.

**Independent Test**: Can be tested by passing CLI flags and verifying they override config file settings for that invocation.

**Acceptance Scenarios**:

1. **Given** config file has `exit_on_threshold: false`, **When** `--exit-on-threshold` flag is passed, **Then** the command exits with non-zero code on budget exceeded.
2. **Given** config file has `exit_code: 1`, **When** `--exit-code 5` flag is passed, **Then** the command exits with code 5 on budget exceeded.
3. **Given** environment variable `FINFOCUS_BUDGET_EXIT_CODE=3` is set, **When** `--exit-code 7` flag is passed, **Then** CLI flag takes precedence and command exits with code 7.

---

### Edge Cases

- What happens when exit code is set to 0 explicitly? (Allowed—disables exit behavior even with `exit_on_threshold: true`)
- What happens when exit code is set to a negative number? (Rejected as invalid during configuration validation)
- What happens when exit code exceeds 255 (maximum Unix exit code)? (Clamped to 255 or rejected during validation)
- What happens when both `exit_on_threshold: false` and `--exit-on-threshold` flag are used? (CLI flag wins per precedence rules)
- What happens when budget evaluation fails due to currency mismatch? (Exit with code 1 for error, not configured exit code)
- What happens when the cost calculation itself encounters errors? (Continue to output, evaluate budget if possible, then exit with appropriate code)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support `exit_on_threshold` configuration option (boolean) to enable non-zero exit codes when budget thresholds are exceeded.
- **FR-002**: System MUST support `exit_code` configuration option (integer) to customize the exit code value, defaulting to 1.
- **FR-003**: System MUST evaluate budget thresholds after rendering cost output, ensuring users always see the cost data before the process exits.
- **FR-004**: System MUST support environment variable overrides: `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD` and `FINFOCUS_BUDGET_EXIT_CODE`.
- **FR-005**: System MUST apply configuration precedence: CLI flags > environment variables > config file > defaults.
- **FR-006**: System MUST exit with code 0 when no budget is configured, regardless of cost.
- **FR-007**: System MUST exit with code 0 when cost is below all configured thresholds.
- **FR-008**: System MUST exit with the configured exit code when any threshold is exceeded (warning, budget, or forecast).
- **FR-009**: System MUST exit with code 1 when an error occurs during budget evaluation (distinct from threshold exceeded).
- **FR-010**: System MUST validate exit codes are within valid range (0-255) and reject invalid configurations.
- **FR-011**: System MUST support CLI flags `--exit-on-threshold` and `--exit-code` for runtime override.
- **FR-012**: System MUST log the exit code reason when `--debug` is enabled for troubleshooting.

### Key Entities *(include if feature involves data)*

- **Exit Code Configuration**: Settings controlling when and how the CLI returns non-zero exit codes. Contains `exit_on_threshold` (boolean), `exit_code` (integer 0-255).
- **Budget Evaluation Result**: Extended result from budget evaluation that includes exit code decision. Contains `ThresholdExceeded` (boolean), `ShouldExit` (boolean), `ExitCode` (integer), `Reason` (string for debugging).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: CI/CD pipelines can detect budget threshold violations without parsing command output (exit code alone is sufficient).
- **SC-002**: Users can configure exit behavior through any of three methods (config file, environment variable, CLI flag) with consistent results.
- **SC-003**: Cost output is always displayed before exit, ensuring users never lose visibility into cost data due to exit behavior.
- **SC-004**: Exit code configuration errors are reported clearly with actionable messages identifying the invalid setting.
- **SC-005**: All existing cost command behavior remains unchanged when exit behavior is not configured (zero breaking changes for existing users).
- **SC-006**: Exit behavior integrates seamlessly with existing budget status rendering (budget status box appears before exit).

## Assumptions

- **A-001**: The existing budget configuration structure (`BudgetConfig` in `config/budget.go`) will be extended with new fields rather than creating a separate configuration structure.
- **A-002**: Exit code behavior applies to both `cost projected` and `cost actual` commands equally.
- **A-003**: The exit code for error conditions (code 1) is not configurable—only threshold-exceeded exit codes are configurable.
- **A-004**: Exit codes are evaluated after all output is rendered, including budget status display.
- **A-005**: The feature depends on Issue #217 (MVP budget alerts) being implemented, as exit behavior relies on threshold evaluation.
