# Feature Specification: Resource History Store with Layered Cost Attribution

**Feature Branch**: `608-resource-history-store`
**Created**: 2026-03-30
**Status**: Draft
**Input**: GitHub Issue #934

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Accurate Full-Month Costs After Resource Replacements (Priority: P1)

A user runs `finfocus cost actual` for a complete month (e.g., March 1-31). During that month, an EC2 instance was replaced on day 15 — the old instance (`i-old123`) ran days 1-15 costing $50, and the new instance (`i-new456`) ran days 15-31 costing $50. Today, finfocus only knows about `i-new456` and reports $50 instead of the true $100.

With this feature, the system tracks all cloud identifiers a resource has ever had. When querying actual costs for a date range, it queries the billing provider for **every** cloud ID that was active during that range, producing the correct $100 total.

**Why this priority**: This is the core problem. Without this, finfocus systematically under-reports actual costs whenever resources are replaced, which is a common infrastructure operation. Inaccurate cost reporting undermines user trust in the tool.

**Independent Test**: Can be fully tested by replacing a resource mid-month, then running `cost actual` for the full month and verifying the total includes costs from both the old and new resource incarnations.

**Acceptance Scenarios**:

1. **Given** a resource was replaced mid-month (old cloud ID ran days 1-15, new cloud ID ran days 15-31), **When** the user runs `cost actual --from month-start --to month-end`, **Then** the reported cost includes billing for BOTH cloud IDs.
2. **Given** a resource was replaced multiple times in one month (3 different cloud IDs), **When** the user queries actual costs for the full month, **Then** all three incarnations' costs are included in the total.
3. **Given** a resource has never been replaced, **When** the user queries actual costs, **Then** the behavior is identical to today (no regression).

---

### User Story 2 - Automatic Resource Identity Tracking (Priority: P2)

Every time the user runs a finfocus command that loads infrastructure state (e.g., `cost actual`, `overview`), the system silently records which cloud IDs are currently associated with each resource. Over time, this builds a complete history of resource identity changes without any manual user action.

**Why this priority**: This is the data collection mechanism that enables P1. Without historical data accumulation, the system cannot know about previous resource incarnations. It must be automatic because users cannot be expected to manually register resource ID changes.

**Independent Test**: Can be tested by running finfocus multiple times across state changes and verifying the history store accumulates entries for old and new cloud IDs.

**Acceptance Scenarios**:

1. **Given** this is the first time finfocus runs with history enabled, **When** the user runs `cost actual`, **Then** the system creates a persistent history store and records all current resource-to-cloud-ID mappings.
2. **Given** finfocus has been run before and a resource's cloud ID has changed, **When** the user runs any command that loads state, **Then** the new cloud ID is recorded as a new entry while the old entry is preserved.
3. **Given** the system is recording history, **When** the same resource with the same cloud ID is observed on subsequent runs, **Then** only the "last seen" timestamp is updated (no duplicate entries).

---

### User Story 3 - Deleted Resource Cost Visibility (Priority: P3)

A user destroys a resource on day 10 of the month. When querying actual costs for the full month, the system should still report the costs that resource incurred during days 1-10, even though the resource no longer exists in the current infrastructure state.

**Why this priority**: Resources that existed and incurred costs during the queried period should be visible regardless of current state. Without this, deleting resources causes their historical costs to "disappear" from reports.

**Independent Test**: Can be tested by recording a resource, then deleting it from state, then querying actual costs for the period it was active.

**Acceptance Scenarios**:

1. **Given** a resource existed from day 1-10 of the month and was then deleted, **When** the user runs `cost actual` for the full month, **Then** the report includes the deleted resource's costs for days 1-10.
2. **Given** a resource was created on day 5 and deleted on day 20, **When** the user queries actual costs for the full month, **Then** costs for days 5-20 are included.
3. **Given** a resource was deleted more than 90 days ago (past retention window), **When** the user queries costs, **Then** the system does not include it (history entries have been cleaned up).

---

### User Story 4 - Plan and Deployment Lineage Capture (Priority: P4)

When a user runs `finfocus overview` (which triggers `pulumi preview`) or uses finfocus as a Pulumi analyzer during `pulumi up`, the system captures resource identity information from these events. Replace and delete operations in previews reveal old cloud IDs that would otherwise be lost. Analyzer events during deployments capture real-time resource provisioning data.

**Why this priority**: State snapshots (P2) only capture what exists NOW. Plan lineage and analyzer events fill gaps — especially for resources that are replaced or destroyed between finfocus runs. Currently, replace operations are skipped entirely during plan processing.

**Independent Test**: Can be tested by running a preview that includes a replace operation and verifying that both the old and new cloud IDs are recorded in the history store.

**Acceptance Scenarios**:

1. **Given** a preview contains a "replace" operation with both old and new resource state, **When** the user runs `finfocus overview`, **Then** both the old cloud ID and new cloud ID are recorded in the history store.
2. **Given** a preview contains a "delete" operation, **When** the user runs `finfocus overview`, **Then** the deleted resource's cloud ID is recorded with its last-known state.
3. **Given** finfocus is running as a Pulumi analyzer during `pulumi up` (not a dry run), **When** resources are provisioned, **Then** the system records resource observations including cloud IDs from post-provisioning events.

---

### User Story 5 - Tag-Based Cost Attribution Fallback (Priority: P5)

For resources where no cloud ID history exists (first-time users, resources created and destroyed between finfocus runs), the system can fall back to querying cloud billing services by resource tags. The user configures which tags to use for cost attribution, and the system uses these when ID-based queries cannot be performed.

**Why this priority**: This is a safety net for the "cold start" problem. New finfocus users have no history, and resources that churn between runs leave no ID trace. Tags provide an alternative attribution path. However, tags require cloud-provider-specific setup (e.g., AWS Cost Allocation Tags activation), making this opt-in.

**Independent Test**: Can be tested by configuring tag-based allocation, having a resource with no ID history, and verifying the system queries billing by tags instead.

**Acceptance Scenarios**:

1. **Given** tag-based allocation is enabled and configured with specific tags, **When** a resource has no cloud ID history for the queried period, **Then** the system queries the billing provider using the configured tags to attribute costs.
2. **Given** a resource has BOTH cloud ID history and tags, **When** the user queries actual costs, **Then** the system uses the cloud ID (primary) and does NOT also query by tags (no double-counting).
3. **Given** tag-based allocation is disabled (default), **When** a resource has no cloud ID history, **Then** the system reports only what it can find by current cloud ID (existing behavior).

---

### Edge Cases

- What happens when finfocus runs for the first time with history enabled but no prior data exists? The system should record the current state and function normally with no cost gaps beyond what already exists today.
- How does the system handle a resource whose cloud ID is reused by the cloud provider (rare but possible)? The system deduplicates by URN+cloudID combination, so a reused cloud ID under a different URN is treated as a separate resource.
- What happens when the history store file is corrupted or inaccessible? The system should degrade gracefully to current behavior (query only current cloud IDs) and log a warning, similar to how cache corruption is handled today.
- What happens during the retention cleanup if a resource's last-seen timestamp is exactly at the retention boundary? The system should use a consistent comparison (entries older than retention window are removed; entries at the boundary are kept).
- How does the system handle resources with no cloud ID in state (e.g., Pulumi component resources, provider resources)? These are skipped — only resources with physical cloud IDs are tracked.
- What happens when multiple finfocus processes run concurrently and write to the history store? The storage backend must handle concurrent access safely (same pattern as the existing cache store).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST persist a mapping of resource identities (URN to cloud ID) across runs, surviving restarts and upgrades.
- **FR-002**: System MUST automatically record resource-to-cloud-ID mappings every time infrastructure state is loaded (during `cost actual`, `overview`, or analyzer operations).
- **FR-003**: System MUST support multiple cloud IDs per resource URN, representing different incarnations over time.
- **FR-004**: System MUST record when each resource-cloud-ID pairing was first observed and last observed.
- **FR-005**: When querying actual costs for a date range, the system MUST query billing providers for ALL cloud IDs that were active during that range, not just the current cloud ID.
- **FR-006**: System MUST include costs for resources that were deleted during the queried date range, if history data exists for them.
- **FR-007**: System MUST extract old and new resource cloud IDs from infrastructure change plans (replace and delete operations), which are currently skipped.
- **FR-008**: System MUST record resource observations during analyzer events (live deployments), capturing cloud IDs from post-provisioning data when available.
- **FR-009**: System MUST provide a tag-based cost attribution fallback for resources with no cloud ID history, configurable via user settings.
- **FR-010**: System MUST use cloud ID-based queries as the primary attribution method and ONLY fall back to tag-based queries when no ID history exists (no double-counting).
- **FR-011**: System MUST automatically clean up history entries older than a configurable retention period on startup.
- **FR-012**: System MUST allow users to enable/disable history tracking and configure the retention period.
- **FR-013**: System MUST allow users to enable/disable tag-based allocation and configure which tags are used for cost attribution.
- **FR-014**: System MUST handle history store unavailability gracefully, degrading to current behavior (query only current cloud IDs) with appropriate warnings.
- **FR-015**: System MUST store history data separately from cache data, as history is persistent and important while cache is ephemeral and safe to delete.
- **FR-016**: System MUST deduplicate cost results when the same billing period is covered by overlapping cloud ID queries for the same resource.

### Key Entities

- **Resource History Entry**: Represents a single observation of a resource's cloud identity. Key attributes: resource URN, cloud ID, resource type, cloud provider, first-seen timestamp, last-seen timestamp, observation source (state snapshot, plan lineage, or analyzer event), and associated resource tags.
- **Resource Tag Index Entry**: An index mapping a specific tag key-value pair back to resource URNs and their cloud IDs. Enables efficient lookup of resources by tag for the tag-based attribution fallback.
- **History Configuration**: User-configurable settings controlling whether history tracking is enabled, the retention period (default 90 days), whether tag-based allocation is active, and which tag keys to use for attribution.

## Assumptions

- History data accumulates incrementally; no backfill mechanism is needed for periods before finfocus was installed.
- The default retention period of 90 days covers typical quarterly cost review cycles.
- Tag-based allocation is opt-in (disabled by default) because it requires provider-specific tag setup (e.g., AWS Cost Allocation Tags activation has a 24-hour delay and is not retroactive).
- Cloud providers retain billing data tagged on destroyed resources, making tag-based attribution viable even after resource deletion.
- GCP label keys do not support colons, so tag keys like `pulumi:project` require normalization to `pulumi_project` for GCP queries.
- Replace operations in Pulumi plans contain both old state (with previous cloud ID) and new state (with new cloud ID), providing the lineage data needed.
- The existing concurrent access patterns used by the cache store are sufficient for the history store's concurrency needs.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a resource replaced mid-month, `cost actual` for the full month reports costs from BOTH the old and new resource incarnations (100% of billable cloud IDs queried).
- **SC-002**: After 5 consecutive finfocus runs with resource changes between runs, the history store contains entries for all observed resource incarnations with correct first-seen/last-seen timestamps.
- **SC-003**: Costs for resources deleted during the queried date range appear in the `cost actual` output when history data exists for them.
- **SC-004**: Replace operations in infrastructure previews result in both old and new cloud IDs being captured (currently 0% are captured; target is 100%).
- **SC-005**: Tag-based fallback attribution successfully retrieves costs for resources with no cloud ID history when enabled and tags are properly configured in the cloud provider.
- **SC-006**: History retention cleanup removes 100% of entries older than the configured retention period without affecting entries within the retention window.
- **SC-007**: With history tracking enabled, finfocus command execution time increases by no more than 200ms for typical workloads (up to 500 resources).
- **SC-008**: History store corruption or unavailability does not prevent finfocus from functioning — it degrades gracefully to current behavior with a warning.
- **SC-009**: All existing tests continue to pass with history tracking enabled (no regressions).
- **SC-010**: New test coverage for history store operations achieves 80% or higher.
