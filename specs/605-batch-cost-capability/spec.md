# Feature Specification: Recognize Batch Cost Capability

**Feature Branch**: `605-batch-cost-capability`
**Created**: 2026-03-03
**Status**: Draft
**Input**: User description: "GitHub Issue #848 — Recognize PLUGIN_CAPABILITY_BATCH_COST in capability routing and plugin list"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Batch Cost Capability in Plugin List (Priority: P1)

As a FinFocus user managing multiple plugins, I want `plugin list` to display
which plugins support batch cost queries so I can understand each plugin's
capabilities and verify batch support is active.

**Why this priority**: Users need visibility into plugin capabilities to
understand what features are available. Without this, batch cost support is
invisible and users cannot troubleshoot or verify plugin configuration.

**Independent Test**: Can be fully tested by running `finfocus plugin list`
with a plugin that reports batch cost capability and verifying the output
includes the "BATCH_COST" label.

**Acceptance Scenarios**:

1. **Given** a plugin that reports batch cost capability, **When** the user
   runs `finfocus plugin list`, **Then** "BATCH_COST" appears in the
   capabilities column for that plugin.
2. **Given** a plugin that reports batch cost capability, **When** the user
   runs `finfocus plugin list --output json`, **Then** "BATCH_COST" appears
   in the capabilities array for that plugin.
3. **Given** a plugin that does NOT report batch cost capability, **When** the
   user runs `finfocus plugin list`, **Then** "BATCH_COST" does NOT appear in
   the capabilities column for that plugin.

---

### User Story 2 - Router Recognizes Batch Cost Capability (Priority: P2)

As a FinFocus user with multiple plugins configured for the same resource
types, I want the router to recognize which plugins support batch cost queries
so that batch-capable plugins can be preferred when processing large resource
sets.

**Why this priority**: Routing awareness enables future optimization where
batch-capable plugins handle bulk queries more efficiently. This is a
foundation for batch cost processing but does not change current routing
behavior.

**Independent Test**: Can be tested by verifying the router detects batch cost
capability from plugin metadata and makes it available for selection logic.

**Acceptance Scenarios**:

1. **Given** a plugin that reports batch cost capability, **When** the router
   evaluates plugin capabilities, **Then** batch cost capability is detected
   and available for routing decisions.
2. **Given** two plugins matching the same resource type where one supports
   batch cost and one does not, **When** the router selects a plugin, **Then**
   both plugins remain valid candidates (batch cost is a hint, not a hard
   filter).

---

### Edge Cases

- What happens when a plugin reports an unknown or unrecognized capability
  value? The system should handle it gracefully without crashing and display
  the raw numeric value or skip it.
- What happens when the upstream spec dependency has not been upgraded and the
  batch cost enum value is not yet defined? The system should still function
  correctly for all existing capabilities.
- What happens when a plugin reports batch cost capability alongside all other
  existing capabilities? All capabilities should display correctly without
  truncation or formatting issues.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST recognize the batch cost capability value and map it
  to the display name "BATCH_COST".
- **FR-002**: The `plugin list` table output MUST include "BATCH_COST" in the
  capabilities column for any plugin that reports this capability.
- **FR-003**: The `plugin list --output json` output MUST include "BATCH_COST"
  in the capabilities array for any plugin that reports this capability.
- **FR-004**: The router MUST be able to detect whether a plugin supports
  batch cost capability when evaluating plugin selection.
- **FR-005**: The capability mapping MUST be consistent between table output,
  JSON output, and router detection (single source of truth for the name).
- **FR-006**: System MUST handle the batch cost capability value without
  errors even if no plugins currently report it.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All existing plugin capabilities continue to display correctly
  with no regressions in table or JSON output.
- **SC-002**: "BATCH_COST" capability appears in plugin list output (both
  table and JSON formats) when reported by a plugin.
- **SC-003**: Router correctly identifies batch-cost-capable plugins without
  altering existing plugin selection behavior or priority ordering.
- **SC-004**: All unit tests pass and code passes linting validation
  (`make test && make lint`).

## Assumptions

- The finfocus-spec dependency will be upgraded to v0.5.7 (issue #844) before
  this feature is implemented, making the `PLUGIN_CAPABILITY_BATCH_COST = 12`
  enum value available in generated code.
- Batch cost capability is an optimization hint for routing, not a hard
  requirement. Existing routing behavior remains unchanged for this feature.
- The capability display name follows the existing convention of uppercase
  snake case without the `PLUGIN_CAPABILITY_` prefix (e.g., "BATCH_COST").

## Dependencies

- **Blocked by**: Issue #844 (upgrade finfocus-spec to v0.5.7)
- **Related to**: Issue #846 (BatchCost RPC consumer uses this capability for
  detection)
