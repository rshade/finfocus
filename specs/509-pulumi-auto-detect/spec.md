# Feature Specification: Automatic Pulumi Integration for Cost Commands

**Feature Branch**: `509-pulumi-auto-detect`
**Created**: 2026-02-11
**Status**: Draft
**Input**: User description: "When --pulumi-json and --pulumi-state flags are omitted from finfocus cost projected and finfocus cost actual, automatically detect the Pulumi project in the current directory and execute the appropriate Pulumi CLI commands (pulumi preview --json or pulumi stack export) to generate the required input data."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Zero-Flag Projected Cost Estimation (Priority: P1)

A developer working inside a Pulumi project directory wants to see projected infrastructure costs without manually running Pulumi commands and piping output to files. They simply run `finfocus cost projected` from their project directory, and the tool automatically detects the Pulumi project, identifies the current stack, runs a preview, and calculates costs.

**Why this priority**: This is the primary use case that eliminates the two-step workflow. Projected cost estimation is the most common operation and the one users perform most frequently during development iterations.

**Independent Test**: Can be fully tested by running `finfocus cost projected` inside a Pulumi project directory with a configured stack and verifying cost output is displayed.

**Acceptance Scenarios**:

1. **Given** a developer is in a directory containing a Pulumi project with an active stack, **When** they run `finfocus cost projected` with no flags, **Then** the tool runs a Pulumi preview, calculates projected costs, and displays results in the default output format.
2. **Given** a developer is in a directory containing a Pulumi project with an active stack, **When** they run `finfocus cost projected --output json`, **Then** the tool auto-detects the project and displays results in JSON format.
3. **Given** a developer provides `--pulumi-json plan.json` explicitly, **When** they run the command, **Then** the tool uses the provided file (existing behavior, unchanged).

---

### User Story 2 - Zero-Flag Actual Cost Retrieval (Priority: P1)

A developer working inside a Pulumi project directory wants to see actual historical costs for their deployed infrastructure without manually exporting state. They run `finfocus cost actual` and the tool automatically exports the current stack state, detects the deployment timeline from resource timestamps, and queries for actual costs.

**Why this priority**: Actual cost retrieval is equally important as projected costs. State-based auto-detection provides richer data (resource IDs, timestamps) that enables automatic date range detection, making the experience even smoother than projected costs.

**Independent Test**: Can be fully tested by running `finfocus cost actual` inside a Pulumi project directory with a deployed stack and verifying actual cost output with auto-detected date range.

**Acceptance Scenarios**:

1. **Given** a developer is in a directory with a Pulumi project that has a deployed stack, **When** they run `finfocus cost actual` with no flags, **Then** the tool exports the stack state, auto-detects the time range from resource timestamps, and displays actual costs.
2. **Given** a developer provides `--pulumi-state state.json` or `--pulumi-json plan.json` explicitly, **When** they run the command, **Then** the tool uses the provided file (existing behavior, unchanged).
3. **Given** a developer is in a Pulumi project with a deployed stack, **When** they run `finfocus cost actual --from 2026-01-01`, **Then** the tool uses the auto-detected state but honors the explicit date override.

---

### User Story 3 - Explicit Stack Selection (Priority: P2)

A developer working with multiple Pulumi stacks (e.g., dev, staging, production) wants to target a specific stack for cost analysis without switching their active stack. They use the `--stack` flag to specify which stack to analyze.

**Why this priority**: Multi-stack workflows are common in production environments but secondary to the core auto-detection experience. The default behavior (use current stack) handles the most common case.

**Independent Test**: Can be fully tested by running `finfocus cost projected --stack production` in a project with multiple stacks and verifying costs are calculated for the specified stack.

**Acceptance Scenarios**:

1. **Given** a Pulumi project with stacks named "dev" and "production", **When** the developer runs `finfocus cost projected --stack production`, **Then** the tool runs a preview against the "production" stack and displays its projected costs.
2. **Given** a Pulumi project with stacks named "dev" and "production", **When** the developer runs `finfocus cost actual --stack dev`, **Then** the tool exports the "dev" stack state and displays its actual costs.
3. **Given** a developer provides `--stack staging` and `--pulumi-json plan.json`, **When** they run the command, **Then** the `--stack` flag is ignored and the provided file is used.

---

### User Story 4 - Clear Error Guidance (Priority: P2)

A developer who doesn't have Pulumi installed, or who runs the command outside a Pulumi project directory, receives clear, actionable error messages explaining what's wrong and how to fix it.

**Why this priority**: Good error messages prevent frustration and support self-service troubleshooting. This is essential for adoption but secondary to the happy-path functionality.

**Independent Test**: Can be fully tested by running the command in various invalid states and verifying error messages are helpful and include remediation steps.

**Acceptance Scenarios**:

1. **Given** a developer does not have Pulumi installed, **When** they run `finfocus cost projected` without flags, **Then** the tool displays an error message that includes a link to install Pulumi and suggests using `--pulumi-json` as an alternative.
2. **Given** a developer is in a directory that is not a Pulumi project, **When** they run `finfocus cost projected` without flags, **Then** the tool displays an error message explaining no Pulumi project was found and suggests using `--pulumi-json`.
3. **Given** a developer is in a Pulumi project with no active stack and no `--stack` flag, **When** they run the command, **Then** the tool displays an error listing available stacks and suggests using `--stack`.
4. **Given** the Pulumi preview command fails (e.g., syntax errors in the program), **When** the developer runs `finfocus cost projected`, **Then** the tool surfaces the Pulumi error output so the developer can diagnose the issue.

---

### User Story 5 - Full Backward Compatibility (Priority: P1)

All existing workflows that use explicit `--pulumi-json` and `--pulumi-state` flags continue to work identically. No existing scripts, CI/CD pipelines, or documentation break.

**Why this priority**: Backward compatibility is a non-negotiable requirement. Existing users and automated pipelines must not be disrupted.

**Independent Test**: Can be fully tested by running the existing test suite and verifying all tests pass without modification. Additionally, running commands with explicit flags produces identical output to the current version.

**Acceptance Scenarios**:

1. **Given** a CI/CD pipeline runs `finfocus cost projected --pulumi-json plan.json`, **When** the updated tool is deployed, **Then** the command produces the same output as the previous version.
2. **Given** a script runs `finfocus cost actual --pulumi-state state.json --from 2026-01-01`, **When** the updated tool is deployed, **Then** the command produces the same output as the previous version.
3. **Given** a developer has existing shell scripts using `--pulumi-json`, **When** they upgrade to the new version, **Then** all scripts continue to work without changes.

---

### Edge Cases

- What happens when the developer is in a subdirectory of a Pulumi project (not the root)? The tool should walk up the directory tree to find the project root.
- What happens when a Pulumi preview takes longer than expected (large infrastructure)? The tool should have a reasonable timeout and display progress indication.
- What happens when the Pulumi backend requires authentication that has expired? The Pulumi CLI error should be surfaced to the user.
- What happens when `PULUMI_CONFIG_PASSPHRASE` is required but not set? The tool should detect the error from the Pulumi CLI and surface it rather than appearing frozen.
- What happens when both `--stack` and `--pulumi-json` are provided? The `--stack` flag should be ignored since the file takes precedence.
- What happens when the Pulumi project uses a cloud backend (Pulumi Cloud, S3) vs local file backend? The tool should work with any backend since it delegates to the Pulumi CLI.
- What happens when the specified `--stack` doesn't exist? The Pulumi CLI error should be surfaced with available stack names.
- What happens when the Pulumi preview produces valid output but with zero resources (empty stack)? The tool should handle this gracefully, displaying a message that no resources were found.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST automatically detect a Pulumi project by locating a project configuration file (`Pulumi.yaml` or `Pulumi.yml`) in the current directory or any parent directory.
- **FR-002**: The system MUST automatically determine the current active Pulumi stack when no explicit stack is specified.
- **FR-003**: The system MUST verify that the Pulumi CLI is available before attempting auto-detection, providing an actionable error with installation instructions if not found.
- **FR-004**: For `cost projected`, the system MUST execute a Pulumi preview and capture the structured output for cost calculation when no input file is provided.
- **FR-005**: For `cost actual`, the system MUST execute a Pulumi stack export and capture the state output for cost calculation when no input file is provided.
- **FR-006**: The system MUST support a `--stack` flag on cost commands to allow explicit stack selection during auto-detection.
- **FR-007**: The system MUST ignore the `--stack` flag when explicit file flags (`--pulumi-json`, `--pulumi-state`) are provided.
- **FR-008**: The system MUST preserve the user's full environment (credentials, backend configuration, encryption passphrases) when invoking Pulumi commands.
- **FR-009**: The system MUST enforce reasonable timeouts on Pulumi command execution (5 minutes for preview, 60 seconds for export) to prevent indefinite hangs.
- **FR-010**: The system MUST maintain full backward compatibility with all existing flag-based workflows producing identical results.
- **FR-011**: The system MUST display progress indication when running Pulumi commands so users know the tool is working.
- **FR-012**: The system MUST work across all supported platforms (Linux, macOS, Windows) for both binary detection and project detection.
- **FR-013**: The system MUST surface Pulumi CLI error output when preview or export commands fail, preserving the original error context.
- **FR-014**: For `cost actual` with auto-detection, the system MUST auto-detect the `--from` date from resource timestamps in the exported state (existing behavior for state-based input).
- **FR-015**: The system MUST provide a clear error hierarchy: missing binary > missing project > missing stack > command failure > parse failure.

### Key Entities

- **Pulumi Project**: A directory containing a Pulumi configuration file (`Pulumi.yaml` or `Pulumi.yml`) that defines an infrastructure-as-code project. Contains one or more stacks.
- **Pulumi Stack**: A named instance of a Pulumi project representing a specific deployment environment (e.g., dev, staging, production). Each stack has its own configuration and state.
- **Preview Output**: The structured representation of what changes Pulumi would make to infrastructure, containing resource types, properties, and operations. Used for projected cost calculation.
- **Stack State (Export)**: The current deployment state of a stack, containing all managed resources with their properties, cloud IDs, and timestamps. Used for actual cost calculation.
- **Resource Descriptor**: The internal representation of a cloud resource used by the cost engine, derived from either preview output or stack state.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers can obtain projected costs with a single command (`finfocus cost projected`) when inside a Pulumi project directory, eliminating the previous two-command workflow.
- **SC-002**: Developers can obtain actual costs with a single command (`finfocus cost actual`) when inside a Pulumi project directory with a deployed stack.
- **SC-003**: All existing test suites pass without modification, confirming zero regressions for flag-based workflows.
- **SC-004**: Error messages for missing prerequisites (no Pulumi CLI, no project, no stack) include actionable remediation steps that a developer can follow without consulting external documentation.
- **SC-005**: The auto-detection workflow produces identical cost calculation results compared to the manual two-step workflow for the same project and stack.
- **SC-006**: The feature works on all supported platforms (Linux, macOS, Windows) without platform-specific workarounds.
- **SC-007**: New code achieves 80% or higher test coverage, with error paths and edge cases tested.
- **SC-008**: The `--stack` flag allows targeting any available stack without modifying the active stack selection.

## Assumptions

- The Pulumi CLI is a user-managed dependency; FinFocus does not install or manage it.
- The user's environment (shell profile, env vars) provides all necessary credentials and configuration for Pulumi to operate (AWS keys, backend tokens, passphrases).
- Pulumi project directories always contain either `Pulumi.yaml` or `Pulumi.yml` at the project root.
- The `pulumi stack ls` command reliably reports the current stack.
- Preview and export commands produce valid, parseable output on success (exit code 0) and meaningful error messages on failure (non-zero exit code).
- The existing ingestion layer correctly parses Pulumi preview JSON and stack export JSON; only the source of that data changes (file vs CLI output).
- Timeout defaults (5 minutes for preview, 60 seconds for export) are sufficient for typical infrastructure projects; no user-configurable timeout flag is needed initially.
- Progress indication is provided via INFO-level log messages (e.g., "Running pulumi preview --json (this may take a moment)..."), not visual progress bars or spinners.

## Scope Boundaries

### In Scope

- Auto-detection of Pulumi project from current directory (with parent walk-up)
- Auto-detection of current Pulumi stack
- Execution of Pulumi preview for projected costs
- Execution of Pulumi stack export for actual costs
- New `--stack` flag for explicit stack selection
- Updated CLI help text and examples
- Error handling for all failure modes
- Unit, integration, and regression tests
- User-facing documentation updates

### Out of Scope

- Pulumi Go Automation API usage (rejected after research due to caching bugs and missing JSON output)
- `--pulumi-dir` flag for specifying project directory (users should `cd` to project)
- Pulumi Cloud API integration (only local CLI invocation)
- Caching of preview or export results between runs
- Interactive stack selection prompts (CLI tools should not prompt)
- Changes to the cost calculation engine, plugin system, or output formats
- Automatic Pulumi CLI installation or version management
- User-configurable timeout overrides for Pulumi commands

## Dependencies

- Pulumi CLI must be installed and in the user's PATH (runtime dependency, not build dependency)
- No new library dependencies required
- Existing Pulumi SDK dependency remains unchanged (used only for Analyzer protocol)
