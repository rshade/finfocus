# Feature Specification: Wire Router into Cost Commands

**Feature Branch**: `511-wire-router`
**Created**: 2026-02-13
**Status**: Draft
**Input**: User description: "feat(cli): wire router into cost commands for region-aware plugin selection"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Region-Aware Cost Queries (Priority: P1)

As a cloud infrastructure operator with plugins installed for multiple regions (e.g., us-east-1 and us-west-2), I want cost commands to automatically select the correct region-specific plugin for each resource so that I get accurate cost data without cross-region mismatches.

**Why this priority**: This is the core value proposition. Without region-aware routing, every installed plugin is queried for every resource regardless of region, producing incorrect or zero results for region-specific resources. This directly impacts cost accuracy.

**Independent Test**: Can be fully tested by running a cost command with routing configuration specifying region preferences and verifying only the matching plugin is queried for each resource.

**Acceptance Scenarios**:

1. **Given** a user has two plugins installed (one for us-east-1, one for us-west-2) and routing configuration specifying region preferences, **When** the user runs `cost projected` for an us-east-1 resource, **Then** only the us-east-1 plugin is queried for that resource.
2. **Given** a user has routing configuration with region preferences, **When** the user runs `cost actual` for resources spanning multiple regions, **Then** each resource is routed to the appropriate regional plugin.
3. **Given** a user has routing configuration, **When** the user runs `cost recommendations`, **Then** recommendations are fetched from the region-appropriate plugin for each resource.

---

### User Story 2 - Transparent Backward Compatibility (Priority: P1)

As an existing user who has no routing configuration, I want all cost commands to continue working exactly as they do today (querying all installed plugins for every resource) so that my current workflow is not disrupted by this change.

**Why this priority**: Equal to P1 because breaking existing users is unacceptable. Zero-config users must see no behavioral change.

**Independent Test**: Can be fully tested by running any cost command without routing configuration and verifying all plugins are queried (identical to current behavior).

**Acceptance Scenarios**:

1. **Given** a user has no routing configuration in their config file, **When** the user runs any cost command, **Then** all installed plugins are queried for every resource (current behavior preserved).
2. **Given** a user has an empty routing section in their config file, **When** the user runs a cost command, **Then** behavior is identical to having no routing section at all.

---

### User Story 3 - Priority-Based Plugin Selection (Priority: P2)

As a user with multiple plugins that can handle the same resource types, I want to assign priorities to plugins so that the highest-priority plugin is queried first, with automatic fallback to lower-priority plugins if the first one fails.

**Why this priority**: Extends the core routing with prioritization and fallback, adding resilience and control for advanced users. Depends on P1 routing being active.

**Independent Test**: Can be fully tested by configuring two plugins with different priorities for the same resource type and verifying the higher-priority plugin is preferred.

**Acceptance Scenarios**:

1. **Given** two plugins configured for the same resource type with different priorities, **When** the user runs a cost command, **Then** the higher-priority plugin is queried first.
2. **Given** a higher-priority plugin fails for a resource and fallback is enabled, **When** the cost command processes that resource, **Then** the lower-priority plugin is automatically queried as a fallback.
3. **Given** a plugin with fallback explicitly disabled fails, **When** the cost command processes the resource, **Then** no fallback to other plugins occurs and the failure is reported.

---

### User Story 4 - Consistent Routing Across All Commands (Priority: P2)

As a user, I want routing to apply consistently across all cost-related commands (projected, actual, estimate, recommendations, dismiss, overview, analyzer) so that I have a single, unified routing configuration that governs all plugin interactions.

**Why this priority**: Ensures completeness. Partial routing (where some commands use routing and others do not) would create confusing, inconsistent behavior.

**Independent Test**: Can be fully tested by configuring routing and running each cost command, verifying that each command respects the routing configuration.

**Acceptance Scenarios**:

1. **Given** routing configuration exists, **When** the user runs `cost projected`, **Then** the router selects plugins based on the configuration.
2. **Given** routing configuration exists, **When** the user runs `cost actual`, **Then** the same routing logic applies.
3. **Given** routing configuration exists, **When** the user runs `cost estimate`, `cost recommendations`, `cost recommendations dismiss`, `overview`, or `analyzer serve`, **Then** each command uses the same routing rules.
4. **Given** commands that do not use plugins (e.g., `cost recommendations history`, `cost recommendations undismiss`), **When** they execute, **Then** they continue to work without routing (no plugins involved).

---

### Edge Cases

- What happens when routing configuration references a plugin that is not installed? The system should log a warning and fall back to querying available plugins.
- What happens when routing configuration contains invalid patterns (bad regex/glob)? The system should log a warning and fall back to querying all plugins (graceful degradation).
- What happens when all routed plugins fail for a resource? The system should report the failure with appropriate error information (same as current plugin-failure behavior).
- What happens when routing configuration is valid but no plugins match a resource? The system should fall back to querying all plugins (same as no-router behavior) and log a debug message.
- How does the system handle routing configuration changes between invocations? Each command invocation reads the current configuration fresh, so changes take effect immediately.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST use routing configuration to select plugins for resources in all cost commands that interact with plugins (projected, actual, estimate, recommendations, dismiss, overview, analyzer).
- **FR-002**: System MUST preserve existing behavior (query all plugins) when no routing configuration is present.
- **FR-003**: System MUST bridge the type differences between the router's plugin match representation and the engine's plugin match representation without data loss.
- **FR-004**: System MUST log a warning and gracefully degrade to querying all plugins when router creation fails (e.g., invalid patterns, missing plugins).
- **FR-005**: System MUST share the same routing logic between configuration validation (`config validate`) and runtime cost commands.
- **FR-006**: System MUST NOT introduce routing into commands that do not use plugins (e.g., history and undismiss commands that pass no plugin clients).
- **FR-007**: System MUST support feature-specific routing (e.g., a plugin configured only for "ProjectedCosts" should not be selected for "ActualCosts" queries).
- **FR-008**: System MUST add negligible overhead per resource when routing is active (no user-perceptible latency increase).

### Key Entities

- **Routing Configuration**: User-defined rules specifying which plugins handle which resources, with priority and fallback settings. Stored in the user's config file.
- **Plugin Match**: A mapping between a resource and a selected plugin, including the reason for selection (pattern match, provider match, or global) and the plugin's priority.
- **Engine Router**: The interface that the cost calculation engine uses to request plugin selection decisions from the routing system.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 9 cost command call sites that use plugins wire the router, verified by test coverage.
- **SC-002**: When no routing configuration exists, all cost commands produce identical results to the current implementation (zero behavioral regression).
- **SC-003**: When routing configuration specifies region preferences, only matching plugins are queried for each resource (verified by integration test).
- **SC-004**: Router initialization adds less than 10ms per command invocation (no user-perceptible performance impact).
- **SC-005**: Unit test coverage for the type bridge between router and engine achieves 80%+ coverage.
- **SC-006**: The system gracefully handles all edge cases (missing plugins, invalid config, empty matches) without crashing or returning misleading results.

### Assumptions

- The existing `internal/router/` package is complete and correct for plugin selection logic; this feature only needs to wire it into the runtime path.
- The `engine.Router` interface and `engine.WithRouter()` builder method are stable and do not need modification.
- The `config.New()` function reliably loads routing configuration from `~/.finfocus/config.yaml` when present.
- Commands that pass `nil` for plugin clients (history, undismiss) should not be modified, as they have no plugins to route.
