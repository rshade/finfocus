# Feature Specification: Recommendation History

**Feature Branch**: `1729-recommendation-history`
**Created**: 2026-10-19
**Status**: Implemented
**Input**: User description: "Merge plugin lifecycle history into cost recommendations history"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Read plugin lifecycle history (Priority: P1)

A person asks for the lifecycle history of one recommendation and sees events
that installed plugins recorded, including who performed the action when the
plugin supplies that. This works even when the recommendation was never
dismissed on this machine.

**Why this priority**: Local dismissal history cannot show server-side or
shared lifecycle events. That is the gap this feature closes.

**Independent Test**: With a plugin that returns one event naming an actor,
and no local dismissal record, `cost recommendations history <id>` shows that
event and the actor in table, JSON, and NDJSON output.

**Acceptance Scenarios**:

1. **Given** a plugin has lifecycle events for a recommendation and this
   machine has no dismissal record for it, **When** the user requests that
   recommendation's history, **Then** those plugin events are shown.
2. **Given** an event includes an actor, **When** history is shown as a table,
   **Then** the actor is visible, and **When** it is shown as JSON or NDJSON,
   **Then** the actor is included on that event.
3. **Given** an event has no actor, **When** history is shown, **Then** the
   event is still shown and the actor is blank in the table and omitted from
   structured output.

---

### User Story 2 - One chronological history (Priority: P2)

A person sees local dismissal events and plugin events as one list, oldest
first, without the same event appearing twice and without a distinct change
being dropped.

**Why this priority**: A merged list is only useful when duplicates and
ordering are predictable.

**Independent Test**: A local dismiss event and a plugin copy of that same
event, plus a second plugin event with a different actor, produce two rows:
one combined row that keeps the actor, and the distinct event.

**Acceptance Scenarios**:

1. **Given** local and plugin events for the same recommendation, **When**
   history is requested, **Then** the result is ordered from oldest to newest.
2. **Given** a local event and a plugin event that share action, reason, note,
   time, and expiry, and only one of them names an actor, **When** history is
   requested, **Then** one event is shown and it keeps that actor.
3. **Given** two events that differ in actor, action, reason, note, time, or
   expiry, **When** history is requested, **Then** both are shown.
4. **Given** several events share the same time, **When** history is requested
   twice, **Then** their relative order is the same both times.

---

### User Story 3 - Local history survives plugin trouble (Priority: P3)

A person can still read the local dismissal history when plugins are missing,
old, failing, or return a bad event. Canceling the command stops the query.
Reading history does not change the saved dismissal file, and a file written
before actors existed still loads.

**Why this priority**: History must stay available in the setups that work
today.

**Independent Test**: A local dismissal file is shown unchanged when every
plugin reports that history is unsupported, when a plugin call fails, and
when one returned event has an unreadable time beside a valid event.

**Acceptance Scenarios**:

1. **Given** no plugins are installed, **When** the user requests history,
   **Then** the local record is shown and no Pulumi plan is required.
2. **Given** a plugin does not support history or returns no events, **When**
   the user requests history, **Then** the local record is shown and the
   command succeeds.
3. **Given** a plugin fails for a reason other than cancellation, **When** the
   user requests history, **Then** the failure is reported as a diagnostic and
   the local record is still shown.
4. **Given** the user cancels the command, **When** a plugin query is in
   progress, **Then** the command stops and reports the cancellation.
5. **Given** one plugin event has an unreadable time or expiry, **When**
   history is requested, **Then** that event is omitted and every valid local
   and plugin event is still shown.
6. **Given** a dismissal file has no actor field, **When** it is loaded and
   history is requested, **Then** the file loads and is not rewritten.

### Edge Cases

- An empty recommendation id is rejected before any plugin is queried.
- A missing event time is kept and listed after events that have a time.
- An event that is present but has an unreadable time or expiry is omitted.
- A nil or empty plugin result does not erase local events.
- Two plugins can contribute events. Matching events collapse as in story 2.
  Different actors from different plugins are both kept.
- Opening plugins can fail. The command then shows local history and records
  the failure as a diagnostic.
- The command closes plugin connections when it finishes, including when it
  fails after they were opened.
- Help text describes combined history. It does not say the command is
  limited to local state.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The history command MUST show lifecycle events recorded locally
  and events returned by installed plugins for the requested recommendation.
- **FR-002**: The command MUST show plugin events when the recommendation has
  no local dismissal record.
- **FR-003**: Each shown event MUST keep the action, reason, note, time,
  expiry, and actor text supplied with it. Action and reason MUST NOT be
  reduced to a fixed list of values.
- **FR-004**: When two events share action, reason, note, time, and expiry,
  the history MUST show one copy for each distinct non-empty actor. An empty
  actor MUST yield to a copy that names an actor. Identical copies MUST
  appear once.
- **FR-005**: Events that differ in action, reason, note, time, expiry, or
  non-empty actor MUST all be shown.
- **FR-006**: Events MUST be ordered from oldest to newest. Events with no
  time follow events that have a time. Repeating the query MUST keep the same
  order when the inputs are unchanged.
- **FR-007**: An event with an unreadable time or expiry MUST be omitted.
  That omission MUST NOT drop other events or the local record.
- **FR-008**: Plugins that do not support history, return no events, cannot
  be opened, or fail for a reason other than cancellation MUST leave local
  history usable. Those conditions MUST be diagnostics, not a failed command.
- **FR-009**: Canceling the command MUST stop plugin queries and MUST surface
  the cancellation instead of a successful history list.
- **FR-010**: Requesting history MUST NOT change the local dismissal file or
  the in-memory dismissal record.
- **FR-011**: A dismissal file written without actors MUST remain readable
  without a migration. Events without an actor MUST stay valid.
- **FR-012**: Table, JSON, and NDJSON output MUST remain supported. The table
  MUST include an actor column. Structured output MUST include an actor only
  when one is present.
- **FR-013**: The command MUST NOT require a Pulumi plan. With no plugins
  available it MUST show local history only.
- **FR-014**: An empty recommendation id MUST be rejected.
- **FR-015**: Command help and the command reference MUST describe combined
  local and plugin history.

### Key Entities

- **Lifecycle event**: One recorded change to a recommendation. Attributes are
  action, reason, note, time, optional expiry, and optional actor.
- **Local dismissal record**: The on-disk history for recommendations
  dismissed, snoozed, or restored on this machine.
- **Plugin history**: Lifecycle events an installed plugin keeps for a
  recommendation id. A plugin may have none, and older plugins may not offer
  history at all.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a recommendation with only plugin history, the command shows
  every valid plugin event and its actor on the first request.
- **SC-002**: A mixed local and plugin history is oldest-first, contains one
  row for a duplicated event, and keeps a named actor on that row.
- **SC-003**: With plugins unavailable or unsupported, the local events shown
  match the saved record and the dismissal file bytes are unchanged.
- **SC-004**: A person can tell from the command help and the command
  reference that history includes plugin events when plugins provide them and
  still works from the local record alone.

## Assumptions

- The published plugin contract for recommendation history is already part of
  the release this repository depends on. This feature uses that contract and
  does not define a new one.
- Two events match when action, reason, note, time, and expiry are the same
  instant. The actor is not part of that match.
- A missing time is not unreadable. An unreadable time or expiry is one that
  is present but is not a valid instant.
- "Installed plugins" means the plugins the product already discovers. The
  command does not take a plan file and does not gain a required plugin flag.
- Diagnostics use the existing logging path. They do not change the history
  printed on standard output or the success of the command, except for
  cancellation and for an empty recommendation id.
- Earlier dismissal work described this command as local-only. This feature
  replaces that behavior for the history command only. Dismiss, snooze, and
  undismiss are unchanged.
- No web page for recommendation history is in scope.
