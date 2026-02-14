# Feature Specification: Neo-Friendly CLI Fixes

**Feature Branch**: `590-neo-cli-fixes`
**Created**: 2026-02-13
**Status**: Draft
**Input**: GitHub Issue #611 - Fix CLI gaps that prevent reliable use by AI agents (Pulumi Neo)
**Milestone**: 2025-Q3 - v0.3.0 Install UX, Scale & Pulumi Integration

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Semantic Exit Codes for Budget Violations (Priority: P1)

As an AI agent (Pulumi Neo) orchestrating cost-aware deployments, I need the CLI to
return distinct exit codes for different failure types so I can programmatically
distinguish between "budget exceeded" (exit 2) and "command failed" (exit 1) without
parsing error text.

**Why this priority**: Exit codes are the most fundamental machine-readable signal.
Without correct exit codes, AI agents cannot make automated deployment decisions based
on budget outcomes. This is the simplest fix with the highest impact on agent reliability.

**Independent Test**: Can be fully tested by running a budget-exceeding cost calculation
and verifying the process exit code matches the budget configuration.

**Acceptance Scenarios**:

1. **Given** a budget threshold is configured with exit code 2, **When** projected costs
   exceed the threshold, **Then** the CLI process exits with code 2.
2. **Given** a budget threshold is configured with a custom exit code (e.g., 5), **When**
   the threshold is exceeded, **Then** the CLI process exits with that custom exit code.
3. **Given** a budget evaluation error occurs, **When** the CLI processes the error,
   **Then** the CLI exits with code 1 (general failure).
4. **Given** costs are within budget, **When** the CLI completes successfully, **Then**
   the CLI exits with code 0.

---

### User Story 2 - Structured Error Objects in JSON Output (Priority: P2)

As an AI agent consuming CLI output, I need errors to be represented as structured
objects in JSON output so I can programmatically categorize and handle different error
types (plugin failures, validation errors, timeouts) without parsing human-readable
strings.

**Why this priority**: String-based error parsing is fragile and breaks when error
messages change. Structured errors enable agents to make reliable decisions about
retries, fallbacks, and user notifications. This affects all JSON-consuming workflows.

**Independent Test**: Can be tested by triggering various error conditions (plugin
failure, validation error, timeout) and verifying the JSON output contains structured
error objects with consistent fields.

**Acceptance Scenarios**:

1. **Given** a plugin connection fails during cost calculation, **When** the user
   requests JSON output, **Then** the result includes a structured error object with
   a categorized error code and human-readable message.
2. **Given** a resource fails pre-flight validation, **When** the user requests JSON
   output, **Then** the result includes a structured error object with a validation
   error code.
3. **Given** a plugin times out, **When** the user requests JSON output, **Then** the
   result includes a structured error object with a timeout error code.
4. **Given** no cost data is available for a resource, **When** the user requests JSON
   output, **Then** the result includes a structured error object with a no-data code.
5. **Given** table output is requested, **When** errors occur, **Then** the existing
   human-readable error display is preserved unchanged.

---

### User Story 3 - Plugin List as Structured Data (Priority: P3)

As an AI agent managing plugin lifecycle, I need to retrieve the list of installed
plugins in a structured format so I can programmatically inspect plugin versions,
capabilities, and providers without parsing tabular text output.

**Why this priority**: Plugin management is a prerequisite for cost operations. Agents
need to verify plugin availability and capabilities before invoking cost commands. This
is lower priority because agents can work around it by attempting commands directly.

**Independent Test**: Can be tested by running the plugin list command with structured
output format and verifying the output is valid, parseable, and contains all required
plugin metadata fields.

**Acceptance Scenarios**:

1. **Given** plugins are installed, **When** the user requests structured output from
   the plugin list command, **Then** the output is a valid, parseable data structure
   containing an array of plugin objects.
2. **Given** plugins are installed, **When** structured output is requested, **Then**
   each plugin entry includes name, version, path, supported providers, and capabilities.
3. **Given** no plugins are installed, **When** structured output is requested, **Then**
   the output is an empty array (not null or an error message).
4. **Given** a plugin fails metadata retrieval, **When** structured output is requested,
   **Then** the plugin entry includes the failure information in a notes field alongside
   available metadata.

---

### Edge Cases

- What happens when the budget exit code is 0? The system MUST treat exit code 0 as
  "no exit action" and return success normally.
- What happens when multiple budget scopes are violated with different exit codes?
  The highest exit code should take precedence.
- What happens when a plugin returns a mix of valid results and errors in the same
  response? Each resource result should independently carry its error information.
- What happens when the structured error code is unrecognized by a consuming agent?
  Error codes MUST be stable strings that agents can match on; the message field
  provides additional context.
- What happens when table output is requested? All existing behavior MUST be preserved;
  structured errors only apply to machine-readable output formats.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST propagate custom exit codes from budget threshold violations
  to the process exit status, rather than always exiting with code 1.
- **FR-002**: The CLI MUST use a standard error detection mechanism to extract the
  budget-specific exit code from the error chain before falling back to exit code 1.
- **FR-003**: When JSON or NDJSON output is requested and a resource encounters an error,
  the output MUST include a structured error object with at minimum: an error code
  (categorized string), a human-readable message, and the affected resource type.
- **FR-004**: The system MUST define a fixed set of error codes for machine consumption:
  `PLUGIN_ERROR`, `VALIDATION_ERROR`, `TIMEOUT_ERROR`, `NO_COST_DATA`.
- **FR-005**: Error codes MUST be stable across versions (no renaming or removal without
  a major version bump).
- **FR-006**: The plugin list command MUST support a structured output format that
  produces a parseable data structure (array of plugin objects).
- **FR-007**: Each plugin object in structured output MUST include: name, version, file
  path, supported providers (array), and capabilities (array).
- **FR-008**: When no plugins are installed, the structured output MUST return an empty
  array, not null or an error.
- **FR-009**: Table output for all commands MUST remain unchanged; structured output is
  additive and opt-in.
- **FR-010**: When a plugin fails during metadata retrieval, its entry in structured
  output MUST still appear with available metadata and a notes field describing the
  failure.
- **FR-011**: The Notes field in JSON output MUST NOT contain error string prefixes
  (e.g., "ERROR:", "VALIDATION:") when a structured error object is present.

### Key Entities

- **Structured Error**: An error representation with a categorized code, human-readable
  message, and contextual resource type. Used exclusively in machine-readable output
  formats.
- **Error Code**: A stable, enumerated string identifier for error categories. Values:
  `PLUGIN_ERROR`, `VALIDATION_ERROR`, `TIMEOUT_ERROR`, `NO_COST_DATA`.
- **Plugin Info**: A structured representation of an installed plugin including name,
  version, path, providers, capabilities, and optional notes.

## Assumptions

- The overview command already supports `--output json|ndjson` (confirmed by codebase
  inspection). This spec focuses on the three remaining gaps.
- AI agents will use exit codes as the primary signal and structured JSON as the
  secondary signal for decision-making.
- Existing consumers of table output will not be affected.
- Error codes are additive; new codes may be introduced in minor versions but existing
  codes will not be removed or renamed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: AI agents can distinguish budget violations from general failures by
  inspecting the CLI exit code alone, with 100% accuracy across all budget
  configurations.
- **SC-002**: AI agents can categorize 100% of errors in JSON output by inspecting a
  structured error code field, without string parsing.
- **SC-003**: AI agents can enumerate installed plugins and their capabilities from
  structured output, with all metadata fields present for every reachable plugin.
- **SC-004**: All existing table-output workflows produce identical output before and
  after this change (zero regressions).
- **SC-005**: All unit tests pass and linting passes after implementation.
