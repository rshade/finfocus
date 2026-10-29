# Feature Specification: Recommendation Lifecycle History on the Web

**Feature Branch**: `issue-1858`
**Created**: 2026-10-29
**Status**: Draft
**Input**: User description: "Expose a recommendation's lifecycle history through the browser session so it matches `cost recommendations history`, without drawing that history in the page yet."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Read the same lifecycle the command line shows (Priority: P1)

A FinOps engineer who already opened Recommendations in the browser asks for
the lifecycle of one recommendation that appeared in that list. The session
returns the same events, oldest first, that
`finfocus cost recommendations history <id>` would show for the same dismissal
record and the same connected plugins: action, reason, note, time, expiry, and
actor when one was recorded.

**Why this priority**: Without this, the only way to see whether someone
already dismissed, snoozed, or reopened a recommendation is to leave the
browser.

**Independent Test**: For one recommendation that is in the latest browser
list, the returned events and their order match the command-line history for
that id.

**Acceptance Scenarios**:

1. **Given** a recommendation that was dismissed locally and also has a plugin
   lifecycle event, **When** the browser asks for that recommendation's
   history, **Then** both events appear, oldest first, with the same action,
   reason, note, time, expiry, and actor as the command line.
2. **Given** a recommendation with no recorded events, **When** the browser
   asks for its history, **Then** the answer is an empty event list, not an
   error.
3. **Given** the latest list has not been loaded yet, **When** the browser
   asks for an id that a normal recommendations load would include, **Then**
   the session loads that list once and then returns the history.

---

### User Story 2 - Refuse ids that were not listed (Priority: P1)

The history request only answers for a recommendation that was in the last
successful recommendations list for this session. An unknown id, or a missing
id, is refused. The session does not ask plugins about an arbitrary id.

**Why this priority**: A recommendation id is also the dismissal id. Plugins
may mint a new id on every recommendations call, so an open history lookup
would query plugins for ids the user never saw.

**Independent Test**: After a successful list, an id that was not in it is
refused, and no history lookup runs. A request with no id is refused before
any lookup.

**Acceptance Scenarios**:

1. **Given** a loaded recommendations list, **When** the browser asks for an
   id that is not in it, **Then** the request is refused and plugins are not
   asked for history.
2. **Given** any session, **When** the browser omits the recommendation id,
   **Then** the request is refused.
3. **Given** two tabs, **When** a later list replaces the previous one,
   **Then** history is answered only for ids in the later list.

---

### User Story 3 - Keep local history when a plugin cannot answer (Priority: P2)

A plugin that does not implement lifecycle history contributes nothing. A
plugin that fails contributes no events and is named in a warning. Local
events are still returned. The warning does not include the plugin's own
error text. The command-line table and JSON stay as they are today.

**Why this priority**: One unavailable plugin must not hide a dismissal the
team already recorded.

**Independent Test**: With a local dismissal and one failing plugin, the
events still match the command line and the response names that plugin in a
warning. A plugin that does not implement history adds no warning.

**Acceptance Scenarios**:

1. **Given** a local dismissal and a plugin that fails, **When** history is
   requested, **Then** the local event is returned and the response warns that
   the plugin's history is unavailable.
2. **Given** a plugin that does not implement history, **When** history is
   requested, **Then** only the events from other sources are returned and
   that plugin is not warned about.
3. **Given** the history lookup itself cannot run, **When** the browser asks,
   **Then** the failure is reported without events and without a file path or
   plugin error text.

---

### Edge Cases

- A recommendation row with no recommendation id has no lifecycle to show and
  is treated as not found.
- The same optional disambiguators as recommendation detail (key, type, and
  description) select which listed row is meant.
- Cancelling the browser request stops the lookup and does not return a
  partial event list.
- Free-text reason, note, and actor are kept. Credential-like field names and
  Pulumi secret values do not appear in the response.
- The browser still cannot dismiss, snooze, or reopen a recommendation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The browser session MUST return the lifecycle events for one
  recommendation in the same order, and with the same action, reason, note,
  time, expiry, and actor, as the command-line history for that id.
- **FR-002**: The session MUST use the plugins already opened for the
  dashboard. It MUST NOT open a second plugin set for history.
- **FR-003**: The session MUST read the same local dismissal record the
  command line reads, and MUST NOT change that record.
- **FR-004**: The session MUST refuse a missing recommendation id.
- **FR-005**: The session MUST refuse an id that is not in the last successful
  recommendations list for the session. With no list yet, it MUST load that
  list once, using the same default as recommendation detail, and then apply
  the same rule.
- **FR-006**: A refused or not-yet-listed id MUST NOT be sent to plugins.
- **FR-007**: A plugin that does not implement history MUST contribute no
  events and no warning.
- **FR-008**: A plugin that fails MUST NOT drop local or other plugins'
  events. The response MUST name that plugin in a warning and MUST NOT include
  the plugin's error text.
- **FR-009**: A failure of the history lookup itself MUST NOT return events
  and MUST NOT include paths or plugin error text.
- **FR-010**: The session MUST NOT sort or merge events itself. It returns the
  result of the existing history lookup.
- **FR-011**: Reason, note, and actor text MUST be preserved. Credential-like
  names and Pulumi secret values MUST NOT appear in the response.
- **FR-012**: The page MUST NOT gain dismiss, snooze, or reopen actions.

### Key Entities

- **Recommendation**: An item from the latest recommendations list. Its id is
  the dismissal id and the history id.
- **Lifecycle event**: One dismiss, snooze, reopen, or other recorded action,
  with reason, note, time, expiry, and an optional actor.
- **History warning**: A notice that one connected plugin could not provide
  history. It names the plugin and nothing from the plugin's error.

## Web UI Fit *(mandatory)*

- **Disposition**: Extends the Recommendations detail panel.
- **Web data**: The same lifecycle events as command-line history (time,
  action, actor, reason, note, expiry), in that order, plus a warning when a
  connected plugin could not contribute. The page computes nothing.
- **Layout fit**: A captioned table in the existing recommendation detail
  panel, under the current description, savings, and scorer signals. Columns:
  time, action, actor, reason, expiry. The note stays with the reason when
  present. Warnings use the existing alert line. No new view and no new
  toolbar control.
- **Additions**: A warning line when a plugin could not provide history.
  Otherwise none.
- **Render evidence**: 2026-10-29, `~/tailscale-phase-2/initial-30`, stack
  `dev`. Overview showed 30 resources, actual MTD $55.06 and projected
  $141.03. Recommendations listed 12 rows. The detail panel for
  `windowsClientInstance` showed the description, Action Modify, potential
  savings $5.84 USD, reasoning, and resource metadata. It had no lifecycle
  table. `GET /api/recommendations/history` for that listed id returned 200
  with an empty event list and no warnings. A missing id was 400 and an
  unlisted id was 404.
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility) for the detail-panel history section. No engine, API,
  redaction, or data-shape work.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a recommendation in the latest browser list, the returned
  lifecycle matches the command-line history event for event, including order.
- **SC-002**: An omitted id is refused, and an id absent from the latest list
  is refused without a plugin history lookup.
- **SC-003**: With one failing plugin and a local dismissal, the local event
  is still returned and that plugin is named in exactly one warning that does
  not contain the plugin's error text.
- **SC-004**: A person can still not dismiss, snooze, or reopen a
  recommendation from the browser.

## Assumptions

- The command-line history output does not gain a warning column. Warnings are
  part of the browser response only. The events stay the same on both sides.
- "Latest list" follows the recommendation detail rule, including its
  one-time load when nothing has been listed yet and the rule that the last
  successful list wins across tabs.
- A row with an empty recommendation id is not a history target.
- Rendering the history table is a later UX change. This feature only makes
  the data available.
