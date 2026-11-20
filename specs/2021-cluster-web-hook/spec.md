# Feature Specification: Cluster Allocation Web Hook

**Feature Branch**: `2021-cluster-web-hook`
**Created**: 2026-11-19
**Status**: Implemented
**Input**: Issue #1861: expose `finfocus cost cluster` (Kubernetes cost
allocation, run-rate and historical windows, grouped by namespace,
controller, pod, node, Pulumi stack, or label) through a web API hook for
`finfocus --web` (Constitution Principle VII). The Cluster view is a UX-only
follow-up.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Query cluster allocation from the dashboard (Priority: P1)

A platform engineer runs `finfocus --web` with a usage-source plugin and an
allocator plugin installed. The dashboard asks the session for the
cluster's cost allocation, with the same context, namespace, selector,
grouping, and window choices the CLI accepts, and receives the report
`cost cluster --output json` would print.

**Why this priority**: Without it no cluster allocation reaches the
dashboard, and Principle VII blocks every later cluster feature (the
capacity split view, idle-capacity scoring) until this hook exists.

**Independent Test**: With fake usage-source and allocator plugins, run the
same request through the CLI's JSON renderer and through the dashboard
endpoint, and compare the documents field by field.

**Acceptance Scenarios**:

1. **Given** a usage source, an allocator, and no window, **When** the
   dashboard requests allocation grouped by namespace, **Then** the
   response holds the run-rate report (mode, period, currency, total, idle,
   groups, capacity, priced resources, policy, warnings) with the same
   values as the CLI JSON.
2. **Given** a start and end date, **When** the dashboard requests
   allocation, **Then** the response is the historical report for that
   window, as `--from` and `--to` produce.
3. **Given** a namespace or a label selector, **When** allocation is
   requested, **Then** the response is scoped the way the CLI scopes it:
   workload groups only and no idle amount.
4. **Given** a grouping of controller, pod, node, Pulumi stack, or
   `label:<key>`, **When** allocation is requested, **Then** the groups
   match the CLI's groups for that grouping.

---

### User Story 2 - Explain why allocation is unavailable (Priority: P1)

A user whose installation lacks a usage-source plugin, or an allocator
plugin, opens the cluster page. The page can tell them which capability is
missing and how to install it, instead of showing a server error.

**Why this priority**: Most installations have no cluster plugins. A page
that fails with a generic error there is worse than no page.

**Independent Test**: Request allocation with each capability missing in
turn and check the status and the named capability.

**Acceptance Scenarios**:

1. **Given** no plugin with the usage-stats capability, **When** allocation
   is requested, **Then** the answer is a successful response whose status
   is "unavailable" and that names the usage-stats capability and the
   CLI's install hint.
2. **Given** a usage source but no allocator, **When** allocation is
   requested, **Then** the status is "unavailable" and names the allocation
   capability.

---

### User Story 3 - Same error text as the CLI for bad input (Priority: P2)

A user who enters an unknown grouping, a malformed selector, a reversed
window, an unknown plugin name, or a policy the allocator rejects sees the
explanation the CLI gives for the equivalent flags.

**Why this priority**: The view's toolbar shows these messages as written;
a valid query is useful first.

**Independent Test**: Send each invalid input and compare the returned
message with the CLI's exit-2 message for the equivalent flags.

**Acceptance Scenarios**:

1. **Given** a grouping of `daily`, **When** allocation is requested,
   **Then** it is rejected as invalid input with the CLI's group-by message.
2. **Given** a selector without `=`, a window whose end precedes its start,
   or an end without a start, **When** allocation is requested, **Then** it
   is rejected with the CLI's message for that case.
3. **Given** a usage-source or allocator name that matches no capable
   plugin, or several capable allocators and no name, **When** allocation
   is requested, **Then** it is rejected with the CLI's selection message.
4. **Given** policy text the allocator rejects, **When** allocation is
   requested, **Then** it is rejected as invalid input and no usage is
   gathered.

---

### User Story 4 - Show the effective allocation policy (Priority: P3)

A user can ask the dashboard for the allocator's effective policy, its
source, and its digest without running an allocation, as
`cost cluster --show-policy` does.

**Why this priority**: The policy panel explains the numbers, but the
numbers come first.

**Independent Test**: Request the policy with no policy text and with
policy text, and compare the source, digest, and effective policy with the
CLI's `--show-policy --output json`.

**Acceptance Scenarios**:

1. **Given** an allocator and no policy file or text, **When** the policy
   is requested, **Then** the source is "built-in defaults" and the digest
   and effective policy are the allocator's.
2. **Given** no allocator, **When** the policy is requested, **Then** the
   status is "unavailable" and names the allocation capability. No usage
   source is needed.

### Edge Cases

- A usage source that reports partial data marks the report incomplete; the
  response carries the incomplete flag, every warning, and the same
  explanation lines the CLI prints under its table.
- A node that cannot be priced appears in the priced list with its note, as
  in the CLI JSON.
- A report whose rows do not sum to the priced total fails the conservation
  check and is never returned, the same as on the CLI.
- The idle group stays at `$0` where the allocator reports it so; the hook
  adds no rows and changes no amount.
- A plugin that times out, fails, or returns an unexpected error is reported
  as unavailable data with a generic message; plugin text does not reach the
  page.
- A plugin reload between queries uses the replacement plugin set and
  engine, as the other query endpoints do.
- Two concurrent requests are serialized through the session's query gate.
- A request for a query before the session's engine exists is told the data
  is still loading.
- The browser never names a file for the server to read. Policy text from
  the page replaces the discovered policy file; with none, the project
  `allocation.hujson`, then the global one, then the allocator's built-in
  defaults apply, as for the CLI without `--policy`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The session MUST answer a cluster allocation query taking
  context, namespace, selectors, grouping, window start and end,
  usage-source name, allocator name, and policy text, with the CLI defaults
  when a value is omitted (current context, grouping by namespace, run
  rate, automatic plugin choice, discovered policy).
- **FR-002**: The CLI command and the web query MUST build the report
  through one shared function, free of command-line state. `cost cluster`
  output (table, JSON, NDJSON, `--show-policy`) and exit codes MUST NOT
  change.
- **FR-003**: The response MUST carry the CLI JSON document for the same
  inputs: mode, period, currency, grouping, total, idle (absent when
  scoped), scope flags, incomplete flag, groups, capacity groups, priced
  resources, policy (source, digest, effective), and warnings.
- **FR-004**: The response MUST carry display strings computed on the
  server: total, idle and its share, mode line, policy line, each group's
  and capacity group's amounts and share, summarized warnings, and the
  incomplete explanation lines. The browser MUST NOT compute allocation,
  grouping, shares, or money formatting.
- **FR-005**: Every input the CLI rejects with exit 2 MUST be rejected by
  the web query as invalid input with the CLI's message text.
- **FR-006**: A missing usage-stats or allocation plugin MUST be answered
  as a successful response with status "unavailable" that names the
  missing capability, never as a server error.
- **FR-007**: The web query MUST use the session's open plugins and query
  engine, including its usage profile. It MUST NOT open plugins itself.
- **FR-008**: Plugin failures other than those in FR-005 and FR-006 MUST
  return a generic unavailable error, never plugin text.
- **FR-009**: The session MUST answer an effective-policy query that needs
  only an allocator and returns the source, digest, and effective policy
  `--show-policy` reports.
- **FR-010**: The server MUST NOT open a file path supplied by the page.
  Policy arrives as text; its source is reported as supplied by the
  request.
- **FR-011**: The new endpoints MUST follow the dashboard's existing rules
  for state-reading POST requests (same origin, JSON body, session cookie)
  and its payload redaction.

### Key Entities

- **Cluster query**: context, namespace, selectors (`key=value`), grouping,
  window start and end (date or timestamp text), usage-source name,
  allocator name, policy text.
- **Cluster report**: the CLI JSON document, unchanged in shape.
- **Cluster display**: formatted amounts, shares, mode, policy line,
  warnings, and incomplete explanation, in report order.
- **Unavailable answer**: status, the missing capability, and the CLI's
  message.
- **Policy answer**: source, digest, effective policy, and a short digest
  for display.

## Web UI Fit *(mandatory)*

- **Disposition**: New view, "Cluster", reached from the header view links.
  It has its own controls and its own data, so it is a view built from the
  shared slots rather than an Overview extension. It sits beside the
  Overview's existing cluster expansion (spec 624), which stays as is. This
  issue ships only the hook; the view is the UX follow-up.
- **Web data**: the report's groups (with notes and Pulumi URNs), capacity
  groups, total, idle, scope, incomplete flag, warnings, priced resources,
  and policy, with the display strings for each amount, exactly as the CLI
  JSON reports them. An "unavailable" answer names the plugin capability to
  install.
- **Layout fit**: toolbar with context, namespace, selector, group-by,
  window start and end, and a Run button (plugin choice fields only when
  several are installed); status line for loading, unavailable, and
  validation messages; totals bar with total, idle and its share, and the
  mode line; captioned group table (group, CPU, memory, total, share,
  notes) with numeric columns aligned like the Overview table and the idle
  group last; a capacity table when node capacity types are labeled;
  warnings and incomplete explanation below the tables.
- **Additions**: a policy panel showing source, short digest, and the
  effective policy as formatted text, loaded on demand.
- **Render evidence**: 2026-11-19, Overview with cluster expansion rendered
  with Playwright against the `oc-e2e` kind cluster and the
  `state-cluster-expansion` fixture; both endpoints matched the CLI JSON
  there (details in `quickstart.md`).
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility). No engine, API, redaction, or data-shape work. Filed as
  #2280.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For every run-rate and windowed case in the parity test, the
  dashboard response and the CLI JSON report identical values, groups,
  capacity, priced resources, policy, and warnings.
- **SC-002**: 100% of the invalid-input cases return the CLI's message
  text.
- **SC-003**: Existing `cost cluster` tests and output fixtures pass
  unchanged.
- **SC-004**: A cluster query on a loaded session opens no new plugin
  process.
- **SC-005**: With either cluster capability missing, 100% of queries
  answer "unavailable" with the capability named, and none answers a
  server error.

## Assumptions

- The kubeconfig context and the plugins come from the dashboard's own
  process and installation, as they do for the CLI run from the same
  directory; the stack selected in the dashboard does not change them.
- The JSON document already carries the capacity split from #1840, so the
  hook carries it with no extra work.
- The effective policy is passed through the dashboard's payload redaction
  like every other response; a policy key that looks like a credential name
  is dropped there. Allocation policies hold weights and label names, so
  parity holds for real policies.
- Policy file paths stay a CLI option. The page reads a file the user picks
  and sends its text, as the forecast view does for spend history.
- The dashboard never shows a server path, so a discovered policy file is
  named by its tier (project or global) rather than its path. This is the one
  place the web answer may differ from the CLI JSON.

## Dependencies

- `specs/613-cost-cluster/` and its successors (614-623) for the command.
- `specs/624-overview-cluster-expansion/` for the existing Overview
  expansion, which this feature does not change.
- `specs/1737-forecast-web-hook/` for the query-hook pattern.
