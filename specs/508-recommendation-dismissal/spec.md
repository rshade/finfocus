# Feature Specification: Recommendation Dismissal and Lifecycle Management

**Feature Branch**: `508-recommendation-dismissal`
**Created**: 2026-02-05
**Status**: Draft
**Input**: User description: "feat(cli): Add recommendation dismissal and lifecycle management - Allow users to dismiss, snooze, or acknowledge cost optimization recommendations, using the DismissRecommendation RPC from finfocus-spec v0.5.5"
**Issue**: [#464](https://github.com/rshade/finfocus/issues/464)

## Clarifications

### Session 2026-02-05

- Q: What is the dismissal ownership model -- should the CLI own dismissal state locally, delegate to plugins, or both? → A: Plugin-primary with local fallback. Call `DismissRecommendation` RPC when the plugin advertises `PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS`; always persist locally for client-side filtering and audit. Local state feeds `ExcludedRecommendationIds` on `GetRecommendationsRequest` as belt-and-suspenders filtering.
- Q: How should `--include-dismissed` present dismissed recommendations alongside active ones, given the proto has no `include_dismissed` field? → A: Merge. Fetch active recommendations from the plugin, then append locally-dismissed records with status annotations (Dismissed/Snoozed). Dismissed items show their last-known details from local state, interleaved with live active recommendations.
- Q: Should direct transitions between Dismissed and Snoozed states be allowed, or must the user undismiss first? → A: Allow direct transitions. Dismissed->Snoozed (add expiry) and Snoozed->Dismissed (remove expiry) are allowed in a single action. Re-snooze (update expiry date) is also allowed. The new state overwrites the previous dismissal record.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Dismiss a False Positive Recommendation (Priority: P1)

As an infrastructure operator, I receive a cost optimization recommendation that I know is intentional (e.g., an oversized instance kept for burst capacity). I want to dismiss this recommendation permanently so it stops appearing in my results, while recording my reasoning for audit purposes.

**Why this priority**: This is the core value proposition. Without the ability to dismiss recommendations, users are forced to see irrelevant suggestions repeatedly, reducing trust in the tool and creating noise that obscures actionable recommendations.

**Independent Test**: Can be fully tested by dismissing a single recommendation by ID with a reason, then verifying it no longer appears in default list output. Delivers immediate value by reducing noise in recommendation results.

**Acceptance Scenarios**:

1. **Given** a list of active recommendations including rec-123, **When** the user dismisses rec-123 with reason "business-constraint" and a note "Burst capacity requirement", **Then** the plugin is called via DismissRecommendation RPC (if capable), the dismissal is persisted locally, and rec-123 no longer appears in subsequent recommendation listings.
2. **Given** a recommendation rec-123 that has been dismissed, **When** the user lists recommendations with default settings, **Then** rec-123 is excluded from the output via both plugin-side filtering and local ExcludedRecommendationIds.
3. **Given** no recommendation exists with ID "nonexistent-id", **When** the user attempts to dismiss "nonexistent-id", **Then** the system displays an error indicating the recommendation was not found.
4. **Given** a recommendation rec-123 that is already dismissed, **When** the user attempts to dismiss it again, **Then** the system informs the user it is already dismissed and does not create a duplicate entry.

---

### User Story 2 - Snooze a Recommendation for Later Review (Priority: P2)

As a team lead planning quarterly infrastructure reviews, I see a valid recommendation that cannot be acted upon until next quarter. I want to snooze it until a specific date so it reappears automatically when it becomes actionable, without cluttering my current view.

**Why this priority**: Snoozing extends dismissal with time-based expiry, which is a natural workflow enhancement. It builds on the dismiss capability (P1) and addresses the common pattern of deferred action items.

**Independent Test**: Can be tested by snoozing a recommendation with a future date, verifying it disappears from listings, then verifying it reappears after the snooze date passes.

**Acceptance Scenarios**:

1. **Given** an active recommendation rec-456, **When** the user snoozes it until 2026-04-01 with reason "deferred" and note "Q2 review", **Then** the snooze is persisted (plugin via ExpiresAt + locally) and rec-456 no longer appears in default listings.
2. **Given** a snoozed recommendation rec-456 with expiry date 2026-04-01, **When** the user lists recommendations on or after 2026-04-01, **Then** rec-456 reappears as an active recommendation (auto-unsnooze).
3. **Given** a snoozed recommendation rec-456, **When** the user lists recommendations before the snooze expiry, **Then** rec-456 is excluded from default output.
4. **Given** a snooze request with a date in the past, **When** the user attempts to snooze, **Then** the system rejects the request with a clear error message.
5. **Given** a permanently dismissed recommendation rec-123, **When** the user snoozes it until 2026-06-01, **Then** the dismissal record is updated to Snoozed with the new expiry date in a single action (no undismiss required).
6. **Given** a snoozed recommendation rec-456 expiring 2026-04-01, **When** the user snoozes it again until 2026-07-01, **Then** the expiry date is updated to the new date.

---

### User Story 3 - View All Recommendations Including Dismissed (Priority: P2)

As an infrastructure manager conducting a periodic audit, I need to see all recommendations regardless of dismissal status. I want to view the complete picture including dismissed and snoozed items with their status clearly indicated, so I can review past decisions and ensure nothing critical was overlooked.

**Why this priority**: Visibility into dismissed recommendations is essential for audit and review. Without it, users lose track of what was dismissed and why, undermining accountability.

**Independent Test**: Can be tested by dismissing several recommendations, then listing with the include-dismissed option and verifying all recommendations appear with their correct status labels (Active, Dismissed, Snoozed).

**Acceptance Scenarios**:

1. **Given** 3 active, 1 dismissed, and 1 snoozed recommendation, **When** the user lists with include-dismissed enabled, **Then** all 5 recommendations appear in a merged view -- active items from the plugin and dismissed/snoozed items from local state -- each with status indicators (Active, Dismissed, Snoozed).
2. **Given** 3 active, 1 dismissed, and 1 snoozed recommendation, **When** the user lists with default settings, **Then** only the 3 active recommendations appear.
3. **Given** dismissed recommendations exist, **When** the user views the full list, **Then** each dismissed/snoozed item shows its last-known details, reason, note, and dismissal date from local state.

---

### User Story 4 - Re-enable a Dismissed Recommendation (Priority: P2)

As an operator whose infrastructure requirements have changed, I want to re-enable a previously dismissed recommendation so it appears in my active list again. For example, an instance that was kept oversized for burst capacity is no longer needed for that purpose.

**Why this priority**: Circumstances change, and users must be able to reverse dismissal decisions. This completes the lifecycle management loop.

**Independent Test**: Can be tested by dismissing a recommendation, then undismissing it, and verifying it reappears in the default recommendation listing.

**Acceptance Scenarios**:

1. **Given** a dismissed recommendation rec-123, **When** the user undismisses it, **Then** rec-123 reappears in the default recommendation listing.
2. **Given** a snoozed recommendation rec-456, **When** the user undismisses it before the snooze expiry, **Then** rec-456 immediately reappears in the default listing.
3. **Given** an active (non-dismissed) recommendation rec-789, **When** the user attempts to undismiss it, **Then** the system informs the user it is not currently dismissed.

---

### User Story 5 - View Dismissal History (Priority: P3)

As a compliance officer, I want to see the history of actions taken on a specific recommendation, including when it was dismissed, the reason given, and when it was re-enabled, so I can maintain an audit trail.

**Why this priority**: History tracking is valuable for governance and compliance but is not required for core dismiss/snooze functionality. It enhances auditability without blocking primary workflows.

**Independent Test**: Can be tested by performing a series of dismiss/undismiss actions on a recommendation, then viewing its history and verifying all actions appear chronologically.

**Acceptance Scenarios**:

1. **Given** a recommendation rec-123 that was dismissed and later undismissed, **When** the user views history for rec-123, **Then** all lifecycle events are shown in chronological order with timestamps, actions, and reasons.
2. **Given** a recommendation with no dismissal history, **When** the user views its history, **Then** the system indicates no lifecycle events exist.

---

### User Story 6 - Plugin-Delegated Dismissal (Priority: P1)

As a user with a plugin that supports recommendation dismissal (e.g., a SaaS cost management service), I expect the plugin to be the primary handler for dismissals. The plugin persists the dismissal in its own storage (e.g., a JSON exclusion file, a SaaS API call) and respects it in future GetRecommendations responses. The CLI always persists locally as well for client-side filtering.

**Why this priority**: Elevated to P1 because the plugin-primary model is the core architectural decision. Plugin-delegated dismissal ensures cross-environment consistency (CI/CD, multiple machines using the same plugin backend).

**Independent Test**: Can be tested by dismissing a recommendation with a plugin that advertises DISMISS_RECOMMENDATIONS capability and verifying the plugin receives the RPC. Then test with a plugin that lacks the capability and verify local-only dismissal succeeds.

**Acceptance Scenarios**:

1. **Given** a plugin that advertises DISMISS_RECOMMENDATIONS capability, **When** the user dismisses a recommendation, **Then** the DismissRecommendation RPC is called on the plugin AND the dismissal is persisted locally.
2. **Given** a plugin that does not advertise DISMISS_RECOMMENDATIONS capability, **When** the user dismisses a recommendation, **Then** the dismissal is stored locally only, without error.
3. **Given** a plugin RPC call fails (network error, timeout), **When** the user dismisses a recommendation, **Then** the local dismissal still succeeds and the user is warned about the plugin communication failure.
4. **Given** a plugin that supports dismiss, **When** the user lists recommendations after dismissing, **Then** the plugin's GetRecommendations response already excludes the dismissed recommendation AND the local ExcludedRecommendationIds provides redundant filtering.

---

### Edge Cases

- What happens when a user dismisses a recommendation that no longer exists in the current plugin results? For plugins with dismiss capability, the plugin validates the ID and returns an error if unknown. For local-only fallback, the dismissal is recorded locally and silently ignored during filtering.
- What happens when multiple users on different machines dismiss the same recommendation? If using a plugin with dismiss capability, the plugin's backend handles deduplication. For local-only state, each machine maintains its own local state with no cross-machine synchronization.
- What happens when the local dismissal state file is corrupted or missing? The system should treat all recommendations as active and warn the user about the state file issue.
- What happens when a snoozed recommendation's expiry date is exactly "now"? The recommendation should be treated as active (expired snooze).
- What happens when the user provides an invalid dismissal reason? The system should reject the input and display valid reason options.
- What happens when the dismissal reason is "other" but no note is provided? The system should require a note for the "other" reason.
- What happens when the local state file grows very large (thousands of dismissals)? The system should handle this gracefully without noticeable performance degradation for typical operations.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow users to dismiss a recommendation by its unique identifier with a specified reason.
- **FR-002**: System MUST support the following dismissal reasons (matching finfocus-spec v0.5.5 DismissalReason enum): not-applicable, already-implemented, business-constraint, technical-constraint, deferred, inaccurate, and other.
- **FR-003**: System MUST require a free-text note when the dismissal reason is "other".
- **FR-004**: System MUST allow users to provide an optional free-text note (custom reason) for any dismissal reason.
- **FR-005**: System MUST always persist dismissal state locally so it survives across CLI invocations, regardless of plugin capability.
- **FR-006**: System MUST exclude dismissed and snoozed recommendations from default listing output.
- **FR-007**: System MUST provide an option to include dismissed and snoozed recommendations in listing output by merging locally-stored dismissal records with active plugin results into a unified view.
- **FR-008**: System MUST allow users to snooze a recommendation until a specified future date (mapped to ExpiresAt in the DismissRecommendation RPC).
- **FR-009**: System MUST automatically re-activate snoozed recommendations when the snooze expiry date has passed.
- **FR-010**: System MUST allow users to re-enable (undismiss) a previously dismissed or snoozed recommendation.
- **FR-010a**: System MUST allow direct state transitions: Dismissed to Snoozed (add expiry), Snoozed to Dismissed (remove expiry), and Snoozed to Snoozed (update expiry date), each in a single action without requiring undismiss first.
- **FR-011**: System MUST display clear status indicators (Active, Dismissed, Snoozed) when listing all recommendations.
- **FR-012**: System MUST reject snooze requests with dates in the past.
- **FR-013**: System MUST call the DismissRecommendation RPC on the plugin when the plugin advertises PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS. The plugin is the primary owner of dismissal state when capable.
- **FR-014**: System MUST fall back to local-only dismissal when the plugin does not advertise DISMISS_RECOMMENDATIONS capability or when the plugin RPC call fails.
- **FR-015**: System MUST provide a confirmation prompt before dismissing, with an option to skip confirmation.
- **FR-016**: System MUST pass locally-dismissed recommendation IDs via ExcludedRecommendationIds on GetRecommendationsRequest for client-side filtering.
- **FR-017**: System MUST support viewing the lifecycle history of a specific recommendation from local state.
- **FR-018**: System MUST record timestamps for all dismissal lifecycle events (dismiss, snooze, undismiss).
- **FR-019**: System MUST handle a corrupted or missing local state file gracefully by treating all recommendations as active and warning the user.
- **FR-020**: System MUST check the plugin's advertised capabilities to determine whether to call DismissRecommendation RPC or use local-only fallback.

### Key Entities

- **Recommendation**: A cost optimization suggestion from a plugin, identified by a unique ID, with type, description, estimated savings, and lifecycle status (Active, Dismissed, Snoozed).
- **Dismissal Record**: A record of a user's decision to dismiss or snooze a recommendation, containing the recommendation ID, reason, optional custom reason (note), timestamp, optional expiry date, and dismissed-by identifier.
- **Dismissal Reason**: An enumerated set of predefined reasons matching finfocus-spec v0.5.5: not-applicable, already-implemented, business-constraint, technical-constraint, deferred, inaccurate, and other.
- **Lifecycle Event**: A timestamped record of an action taken on a recommendation (dismissed, snoozed, undismissed), forming the local audit history.
- **Plugin Capability**: The PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS flag advertised by a plugin indicating it supports the DismissRecommendation RPC.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can dismiss a recommendation in a single command invocation and the recommendation no longer appears in subsequent default listings.
- **SC-002**: Users can snooze a recommendation and have it automatically reappear after the specified date without manual intervention.
- **SC-003**: Users can view all recommendations (including dismissed) in a single listing to audit past decisions.
- **SC-004**: Users can re-enable a dismissed recommendation in a single command invocation.
- **SC-005**: Dismissal state persists reliably across CLI sessions -- dismissals made in one session are respected in all future sessions.
- **SC-006**: Plugin RPC failures do not prevent local dismissal from succeeding (zero user-facing errors for plugin communication issues).
- **SC-007**: All local lifecycle events are recorded with timestamps, enabling audit trail reconstruction for any recommendation.
- **SC-008**: The system handles at least 1,000 stored dismissals without noticeable degradation in listing or dismiss operations.
- **SC-009**: When a plugin supports dismiss capability, the dismissal is persisted plugin-side, ensuring cross-environment consistency (CI/CD, other machines using the same plugin backend).

## Assumptions

- Recommendation IDs are stable and unique within a given plugin's output. If a plugin regenerates IDs between runs, previously dismissed IDs may not match and dismissed recommendations may reappear.
- Local state storage provides single-machine fallback and client-side filtering. Cross-environment consistency is provided by plugins that support the DismissRecommendation RPC.
- The DismissRecommendation RPC and PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS are available in finfocus-spec v0.5.5 (confirmed).
- Snooze expiry checks happen at query time (when listing recommendations), not via background processes.
- The "history" subcommand (User Story 5) tracks only local lifecycle events. Plugin-side history requires a future spec enhancement (see Follow-Up Tickets).
- The `--include-dismissed` flag operates on local state only, since GetRecommendationsRequest does not currently have an include_dismissed field (see Follow-Up Tickets).

## Follow-Up Tickets

These items are out of scope for this feature but should be tracked as future enhancements:

- **finfocus-spec**: Add `include_dismissed` field to `GetRecommendationsRequest` so plugins can return dismissed recommendations for audit views (currently `--include-dismissed` can only use local state).
- **finfocus-spec**: Add `GetRecommendationHistory` or `GetDismissedRecommendations` RPC to enable plugin-side audit trail retrieval.
- **finfocus (this repo)**: Add `DismissRecommendation` method to the `CostSourceClient` adapter interface in `internal/proto/adapter.go` (prerequisite for this feature's implementation).
