# Feature Specification: Policy-Compatible Cost Output

**Feature Branch**: `594-policy-cost-output`
**Created**: 2026-02-16
**Status**: Draft
**Input**: GitHub Issue #604 — Enable cost-based policy enforcement by adding structured cost metadata to analyzer diagnostics and a cost threshold enforcement mode.

## Clarifications

### Session 2026-02-16

- Q: Should the cost summary file be written to the global FinFocus home directory or the project-local `.finfocus/` directory? → A: Project-local (`$PROJECT/.finfocus/last-cost-summary.json`) with fallback to global (`~/.finfocus/last-cost-summary.json`) when no project context is available.
- Q: Should enforcement mode be configurable via environment variable in addition to config file? → A: Yes, add `FINFOCUS_ENFORCEMENT` env var accepting `advisory` or `mandatory` values, alongside the threshold env var.
- Q: What detail level should the threshold-exceeded diagnostic message include? → A: Moderate by default ("Stack cost $7,500/mo exceeds threshold $5,000/mo"), designed to be configurable in future iterations.
- Q: Should the cost summary file include a schema version for forward compatibility? → A: Yes, include a `schema_version` field (starting at `"1"`) to enable consumers to detect incompatible changes in future releases.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Cost Threshold Enforcement (Priority: P1)

As a platform engineering team lead, I want to set a maximum monthly cost threshold so that deployments exceeding the budget are blocked automatically during `pulumi preview`, preventing costly infrastructure from being provisioned without approval.

**Why this priority**: Cost overruns are the primary pain point this feature addresses. Without enforcement, cost estimates are informational only and easy to ignore. Threshold enforcement turns FinFocus from a passive advisor into an active cost governance tool.

**Independent Test**: Can be fully tested by configuring a cost threshold, running a preview with resources that exceed it, and verifying the preview is blocked (mandatory mode) or warned (advisory mode).

**Acceptance Scenarios**:

1. **Given** a cost threshold of $5,000/month is configured with mandatory enforcement, **When** an operator runs `pulumi preview` on a stack estimated at $7,500/month, **Then** the preview reports a MANDATORY diagnostic that blocks the deployment with a message indicating the threshold was exceeded.
2. **Given** a cost threshold of $5,000/month is configured with advisory enforcement (default), **When** an operator runs `pulumi preview` on a stack estimated at $7,500/month, **Then** the preview reports a WARNING diagnostic but does not block the deployment.
3. **Given** no cost threshold is configured, **When** an operator runs `pulumi preview`, **Then** all diagnostics remain ADVISORY (current behavior is unchanged).
4. **Given** a cost threshold is configured via environment variable, **When** an operator runs `pulumi preview`, **Then** the environment variable value takes precedence over the configuration file.
5. **Given** a cost threshold of $5,000/month is configured, **When** an operator runs `pulumi preview` on a stack estimated at $3,000/month, **Then** the preview shows an ADVISORY diagnostic confirming the stack is within budget.

---

### User Story 2 - Structured Cost Summary File (Priority: P2)

As a DevOps engineer integrating FinFocus with external policy packs and CI/CD pipelines, I want a structured cost summary file written after each analyzer run so that external tools can read cost data programmatically and make policy decisions.

**Why this priority**: The summary file bridges the gap between FinFocus and external policy systems (including Pulumi CrossGuard policy packs). It enables integration without requiring Pulumi platform changes and supports a broad ecosystem of cost governance tools.

**Independent Test**: Can be fully tested by running a preview and verifying the summary file is written with the expected schema, then reading it from an external script or policy pack.

**Acceptance Scenarios**:

1. **Given** the analyzer completes a stack analysis, **When** the analysis finishes successfully, **Then** a cost summary file is written to the project-local `.finfocus/` directory (or global FinFocus home directory if no project context) containing the stack name, project name, total monthly cost, currency, resource count, and per-resource cost breakdown.
2. **Given** the analyzer runs multiple times, **When** each analysis completes, **Then** the summary file is overwritten with the latest results (only the most recent analysis is retained).
3. **Given** the analyzer encounters errors for some resources, **When** the analysis completes, **Then** the summary file still contains cost data for resources that succeeded, and errored resources are excluded from the total.
4. **Given** a CI/CD pipeline runs `pulumi preview` with FinFocus analyzer, **When** the preview completes, **Then** the pipeline can read the summary file and use the total cost for downstream decisions (e.g., Slack notification, approval gate).

---

### User Story 3 - Machine-Parseable Diagnostic Metadata (Priority: P3)

As a tooling developer building integrations on top of Pulumi preview output, I want machine-parseable cost data embedded in analyzer diagnostics so that I can extract structured cost information without parsing human-readable text.

**Why this priority**: While the summary file (P2) covers most external integration needs, embedded diagnostic metadata enables tighter integration with tools that process Pulumi diagnostic output directly (log scrapers, dashboards, custom policy packs that read diagnostic descriptions).

**Independent Test**: Can be fully tested by running a preview and parsing the diagnostic description output to extract the structured metadata block, then validating the embedded data schema.

**Acceptance Scenarios**:

1. **Given** the analyzer produces a cost diagnostic for a resource, **When** the diagnostic description is generated, **Then** the description contains both a human-readable cost estimate and a structured metadata block that can be parsed independently.
2. **Given** a diagnostic contains embedded metadata, **When** the metadata block is extracted and parsed, **Then** it contains the monthly cost, currency, and adapter source.
3. **Given** the human-readable diagnostic text, **When** a user reads the diagnostic output, **Then** the metadata block does not interfere with readability (it is appended as a non-visible comment or clearly separated).

---

### Edge Cases

- What happens when cost calculation fails for all resources? The summary file should still be written with zero total cost and an empty resource list, and no threshold diagnostic should be emitted.
- What happens when the cost threshold is set to zero? It should be treated as "no threshold configured" to avoid blocking all deployments.
- What happens when the cost threshold is negative? It should be treated as invalid and ignored with a warning log.
- What happens when the summary file cannot be written (permissions, disk full)? The analyzer should log a warning but not fail the preview — cost estimation should continue normally.
- What happens when mixed currencies are detected across resources? The summary file should report the dominant currency with a flag indicating mixed currencies, and threshold enforcement should be skipped with a warning.
- What happens when the enforcement mode value is unrecognized? It should default to advisory mode with a warning log.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST write a structured cost summary file to the project-local `.finfocus/` directory after each AnalyzeStack call, containing a schema version, timestamp, stack name, project name, total monthly cost, currency, resource count, and per-resource cost details. When no project context is available, the system MUST fall back to the global FinFocus home directory.
- **FR-002**: System MUST support a configurable maximum monthly cost threshold via the configuration file under the analyzer section.
- **FR-003**: System MUST support overriding the maximum monthly cost threshold via an environment variable (`FINFOCUS_MAX_MONTHLY_COST`).
- **FR-003a**: System MUST support overriding the enforcement mode via an environment variable (`FINFOCUS_ENFORCEMENT`) accepting values `advisory` or `mandatory`.
- **FR-004**: System MUST support two enforcement modes for cost thresholds: advisory (default) and mandatory. Both modes are configurable via configuration file or environment variable.
- **FR-005**: When the total estimated cost exceeds the configured threshold in advisory mode, the system MUST emit an ADVISORY diagnostic with elevated severity (HIGH) that does not block the deployment. The diagnostic message MUST include both the actual estimated cost and the configured threshold (e.g., "Stack cost $7,500/mo exceeds threshold $5,000/mo").
- **FR-006**: When the total estimated cost exceeds the configured threshold in mandatory mode, the system MUST emit a MANDATORY-level diagnostic that blocks the deployment. The diagnostic message MUST include both the actual estimated cost and the configured threshold.
- **FR-007**: When the total estimated cost is within the configured threshold, the system MUST emit an ADVISORY diagnostic confirming the stack is within budget, regardless of enforcement mode. Exception: when all resource cost calculations have failed (total cost is unknown), no threshold diagnostic SHALL be emitted.
- **FR-008**: When no cost threshold is configured, the system MUST preserve current behavior (all diagnostics are ADVISORY).
- **FR-009**: System MUST embed machine-parseable cost metadata in per-resource diagnostic descriptions alongside the existing human-readable text.
- **FR-010**: The structured cost summary file MUST be overwritten on each analyzer run (only the latest result is retained).
- **FR-011**: System MUST handle summary file write failures gracefully — log a warning and continue without failing the preview.
- **FR-012**: System MUST treat a cost threshold of zero or negative as "no threshold configured" and log a warning.
- **FR-013**: When mixed currencies are detected across resources, the system MUST skip threshold enforcement and log a warning.

### Key Entities

- **Cost Summary**: A structured representation of a stack's total and per-resource costs produced after each analyzer run. Contains a schema version (starting at "1"), timestamp, stack/project identifiers, total monthly cost, currency, resource count, and individual resource cost entries.
- **Cost Threshold**: A configurable maximum monthly cost limit with an associated enforcement mode (advisory or mandatory) that determines whether exceeding the threshold produces a warning or blocks the deployment.
- **Diagnostic Metadata**: A machine-parseable block embedded within human-readable diagnostic descriptions containing per-resource cost, currency, and adapter source information.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After every successful analyzer run, a valid structured cost summary is available for consumption by external tools within the project-local `.finfocus/` directory (or global FinFocus home directory when no project context exists).
- **SC-002**: Operators can configure a cost threshold and enforcement mode using either a configuration file or environment variable, and the system enforces it correctly on the next preview.
- **SC-003**: When cost threshold enforcement is set to mandatory and the estimated cost exceeds the threshold, the deployment is blocked 100% of the time.
- **SC-004**: When no threshold is configured, system behavior is identical to current behavior — no diagnostics change in content or severity.
- **SC-005**: External tools can parse the diagnostic metadata from diagnostic descriptions without relying on regex over human-readable text.
- **SC-006**: All existing unit tests continue to pass without modification (backward compatibility).
- **SC-007**: Cost summary file write failures do not cause preview failures or data loss for the operator.

## Assumptions

- The cost summary file is written to the project-local `.finfocus/` directory (e.g., `$PROJECT/.finfocus/last-cost-summary.json`), consistent with other project-scoped artifacts like dismissals and project config. When no project context is detected, it falls back to the global FinFocus home directory (`~/.finfocus/`).
- The environment variable override for cost threshold takes the form `FINFOCUS_MAX_MONTHLY_COST` and accepts a numeric value representing the maximum monthly cost in the default currency. The enforcement mode override uses `FINFOCUS_ENFORCEMENT` accepting `advisory` or `mandatory`. Environment variables take precedence over configuration file values.
- The enforcement mode configuration defaults to "advisory" when not explicitly set, preserving backward compatibility.
- The diagnostic metadata format uses an HTML comment syntax to remain invisible in most rendering contexts while being extractable by tooling.
- Project-local configuration (`$PROJECT/.finfocus/config.yaml`) can override the analyzer threshold, following the existing two-tier config resolution pattern.

## Dependencies

- Depends on Issue #604 architectural decisions (B1 Integration Design Doc).
- Uses existing FinFocus configuration system for threshold and enforcement mode settings.
- Uses existing analyzer diagnostic infrastructure for enforcement level changes.
- Uses existing FinFocus home directory resolution for summary file location.

## Scope Boundaries

### In Scope

- Writing a structured cost summary file after AnalyzeStack
- Cost threshold configuration via config file and environment variable
- Advisory and mandatory enforcement modes for cost thresholds
- Machine-parseable metadata in diagnostic descriptions
- Graceful handling of write failures, mixed currencies, and invalid threshold values

### Out of Scope

- Pulumi CrossGuard policy pack implementation (external — consumes the summary file)
- Per-resource cost thresholds (only stack-level total threshold)
- Historical cost threshold tracking or trend analysis
- Cost summary file rotation or retention policies
- Notification integrations (Slack, email) for threshold violations
- Multi-currency threshold enforcement (single currency only)
