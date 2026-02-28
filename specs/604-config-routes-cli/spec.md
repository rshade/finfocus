# Feature Specification: Config Routes CLI Commands

**Feature Branch**: `604-config-routes-cli`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "Add config routes list and config routes test CLI commands for visibility into plugin routing configuration and debugging plugin selection behavior"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Routing Configuration (Priority: P1)

As an operator managing multiple cost plugins, I want to see a summary of my
routing configuration so I can verify which plugins are configured to handle
which resource types without manually reading YAML files.

**Why this priority**: This is the foundational capability. Without visibility
into the current routing state, users cannot verify their configuration or
diagnose issues. This is a prerequisite for effective routing debugging.

**Independent Test**: Can be fully tested by running `finfocus config routes list`
with a configured routing section and verifying the output matches the YAML
configuration. Delivers immediate value by replacing manual YAML file reading.

**Acceptance Scenarios**:

1. **Given** a config file with routing rules defined, **When** the user runs
   `config routes list`, **Then** they see a table showing each plugin's
   priority, features, patterns, and fallback status.
2. **Given** no routing configuration exists (automatic mode), **When** the
   user runs `config routes list`, **Then** they see a message indicating
   automatic provider-based routing is active.
3. **Given** a config file with routing rules, **When** the user runs
   `config routes list --output json`, **Then** they receive the routing
   configuration as structured JSON suitable for scripting.

---

### User Story 2 - Test Plugin Selection for a Resource Type (Priority: P2)

As an operator debugging why a specific plugin is or isn't handling a particular
resource, I want to simulate the plugin selection process for a given resource
type and see the full match chain with explanations.

**Why this priority**: This is the primary debugging tool. While viewing
configuration (P1) shows what is configured, testing selection shows what
would actually happen at runtime. This requires understanding of P1 to be
meaningful.

**Independent Test**: Can be fully tested by running
`finfocus config routes test aws:ec2:Instance` and verifying the output shows
the correct plugin match order, match reasons, and per-feature assignments
based on the routing configuration.

**Acceptance Scenarios**:

1. **Given** routing rules that match a resource type, **When** the user runs
   `config routes test aws:ec2:Instance`, **Then** they see a ranked list of
   plugins that would handle this resource, with match reasons and priorities.
2. **Given** routing rules with region-specific patterns, **When** the user
   runs `config routes test aws:ec2:Instance us-east-1`, **Then** the
   selection factors region into the matching logic.
3. **Given** routing rules covering multiple feature types, **When** the user
   runs `config routes test aws:ec2:Instance`, **Then** they see which plugin
   handles each feature (ProjectedCosts, ActualCosts, Recommendations).
4. **Given** no routing configuration (automatic mode), **When** the user runs
   `config routes test aws:ec2:Instance`, **Then** they see a message
   explaining that automatic provider-based selection is active and all
   plugins matching the provider would be used.

---

### User Story 3 - Machine-Readable Output for Scripting (Priority: P3)

As a DevOps engineer integrating FinFocus into CI/CD pipelines, I want to
retrieve routing information in JSON format so I can programmatically validate
routing configuration as part of automated checks.

**Why this priority**: JSON output is a convenience feature for automation.
The core value is delivered by P1 and P2 in human-readable format. JSON
output extends that value to scripted environments.

**Independent Test**: Can be fully tested by running both commands with
`--output json` and parsing the output as valid JSON. Delivers value for
CI/CD integration and configuration management scripts.

**Acceptance Scenarios**:

1. **Given** any routing configuration state, **When** the user runs
   `config routes list --output json`, **Then** valid JSON is written to
   stdout that can be parsed by standard tools (e.g., `jq`).
2. **Given** any routing configuration state, **When** the user runs
   `config routes test <type> --output json`, **Then** valid JSON is written
   to stdout containing the match chain and per-feature assignments.

---

### Edge Cases

- What happens when the routing config exists but contains zero plugins?
  The list command displays the empty routing table with a note that no
  plugins are configured.
- What happens when the resource type argument to `config routes test` is
  malformed (e.g., empty string, no colons)?
  The command accepts any string as a resource type and shows matches (or no
  matches) based on pattern rules. No strict format validation is imposed
  since different providers may use different naming conventions.
- What happens when a configured routing plugin is not installed?
  The list command displays the configuration as-is (it reads config, not
  plugin binaries). The test command simulates selection based on config
  alone, noting which plugins are referenced.
- What happens with project-local config overriding global routing?
  The list command shows the effective (merged) routing configuration and
  indicates whether it came from project-local or global config.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `config routes` parent command that groups
  routing-related subcommands.
- **FR-002**: System MUST provide a `config routes list` subcommand that
  displays the effective routing configuration as a formatted table.
- **FR-003**: The list subcommand MUST display for each routing rule: plugin
  name, priority, enabled features, resource patterns, and fallback status.
- **FR-004**: The list subcommand MUST indicate the configuration source path
  (which config file the routing rules came from).
- **FR-005**: The list subcommand MUST display a clear message when no routing
  configuration exists, explaining that automatic provider-based routing is
  active.
- **FR-006**: System MUST provide a `config routes test <resource-type>
  [region]` subcommand that simulates plugin selection.
- **FR-007**: The test subcommand MUST display a ranked list of plugins that
  would match the given resource type, including match reason and priority.
- **FR-008**: The test subcommand MUST display per-feature plugin assignments
  showing which plugin would handle each feature type (ProjectedCosts,
  ActualCosts, Recommendations, etc.).
- **FR-009**: When a region argument is provided, the test subcommand MUST
  factor region into the plugin selection simulation.
- **FR-010**: The test subcommand MUST work from configuration alone without
  requiring actual plugin binaries to be installed or running.
- **FR-011**: Both subcommands MUST support `--output json` flag for
  machine-readable output.
- **FR-012**: Both subcommands MUST be read-only operations that do not modify
  any configuration or system state.
- **FR-013**: Both subcommands MUST include help text with usage examples.
- **FR-014**: Both subcommands MUST follow existing CLI conventions (use
  `RunE` for error handling, use `cmd.Printf()` for output).

### Key Entities

- **Routing Rule**: A declaration binding a plugin name to resource type
  patterns, features, priority, and fallback behavior. Configured in
  `config.yaml` under the `routing.plugins` section.
- **Plugin Match**: The result of evaluating a routing rule against a resource
  type, containing the matched plugin, its priority, the reason for the match,
  and whether the match came from config or automatic detection.
- **Feature Assignment**: The mapping of each feature type (ProjectedCosts,
  ActualCosts, Recommendations) to the highest-priority plugin that supports
  it for a given resource type.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view the full routing configuration in under 5 seconds
  by running a single command, eliminating the need to manually read YAML
  files.
- **SC-002**: Users can determine which plugin handles a specific resource type
  in a single command invocation, reducing debugging time from minutes
  (reading config + understanding algorithm) to seconds.
- **SC-003**: Both commands produce valid, parseable JSON when `--output json`
  is specified, enabling integration into automated configuration validation
  workflows.
- **SC-004**: Users who have no routing configured receive clear guidance about
  the default automatic routing behavior, reducing confusion about plugin
  selection.
- **SC-005**: All unit tests pass with at least 80% coverage for the new
  command implementations.

## Assumptions

- The `config routes test` command simulates selection purely from configuration
  data. It does not launch plugins or verify plugin availability at runtime.
  This is intentional to allow testing routing changes before installing
  plugins.
- The priority sort order follows the existing convention: higher priority
  number = higher precedence (the router sorts descending by priority value).
- The table output format follows existing CLI patterns (tabwriter-based
  formatting used elsewhere in the codebase).
- Both commands use the existing two-tier configuration resolution (project-local
  overrides global) without introducing new config merging behavior.
- Region is treated as an optional argument, not a flag, consistent with the
  positional argument pattern described in the issue.

## Scope Boundaries

**In scope**:

- `config routes list` command with table and JSON output
- `config routes test <resource-type> [region]` command with table and JSON output
- Unit tests for both commands
- Help text with usage examples

**Out of scope**:

- `config routes add` / `config routes remove` (future CRUD operations)
- Modifications to the router itself
- Changes to the routing configuration schema
- Interactive routing configuration wizard
