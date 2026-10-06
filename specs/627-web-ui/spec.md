# Feature Specification: Web UI (Browser-Based SPA)

**Feature Branch**: `627-web-ui`  
**Created**: 2026-10-05  
**Status**: Draft  
**Input**: User description: "build a javascript spa off the command `finfocus --web` that allows all the same tools as the tui."

## Clarifications

### Session 2026-10-06

- Q: How should the local web server protect itself against other local processes and malicious websites calling it? → A: Per-session token — server generates a random token at startup; the printed/opened URL contains it and every request must carry it.
- Q: When no plan source is given at launch, how should the user supply one from inside the web UI? → A: Flags only — no in-UI sourcing; the session resolves sources exactly as the equivalent CLI command would, run in the same folder as the Pulumi program (the web UI reads Pulumi sources only; see FR-012a for Terraform).
- Q: What accessibility bar should the web UI meet? → A: Keyboard + AA contrast — full keyboard operability, visible focus, WCAG AA color contrast, semantic markup for screen readers; no formal audit commitment.
- Q: Should the web UI offer on-demand data refresh, and if so, how much? → A: Match TUI exactly — on-demand preview trigger in the overview (the TUI's `p` key equivalent) plus estimate recalculation; no general data refresh, relaunch for fresh actuals.
- Q: Where does the web UI attach to the CLI? → A: To the main program. `finfocus --web` is a flag on the root `finfocus` command, run from the Pulumi project directory, and it auto-detects the project, stack, and state the way `finfocus overview` does in that directory. It is not a flag on every command and not a `web` subcommand.
- Q: Does the web UI promise TUI parity for the group-by selector and the include-dismissed toggle? → A: No. Both are web-only enhancements beyond the TUI (the TUI fixes grouping at launch and has no include-dismissed toggle; both are CLI flags today). They re-fetch through the shared code path. The spec no longer claims parity for them.
- Q: Is a usability-test target part of this feature's success criteria? → A: No. The former SC-005 is withdrawn; usability testing is a later follow-up (see Follow-ups).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Launch a Browser-Based Cost Dashboard (Priority: P1)

A FinOps practitioner runs `finfocus --web` from their Pulumi project
directory and the CLI starts a local web server, then opens (or prints a link
to) a single-page application in their browser. The dashboard presents the same cost overview experience as the
terminal TUI: a resource table with actual month-to-date cost, projected
monthly cost, delta, drift, recommendations, and warnings, with progressive
loading as data is enriched.

**Why this priority**: This is the core of the feature. Without the overview
dashboard reachable from a browser, nothing else in the feature delivers
value. It is the smallest independently shippable slice.

**Independent Test**: Run `finfocus --web` against a fixture Pulumi plan,
open the served page in a browser, and verify the overview table loads with
the same rows, totals, and summary data the TUI would show for the same
input.

**Acceptance Scenarios**:

1. **Given** finfocus is installed and the working directory is a Pulumi
   project, **When** the user runs `finfocus --web`, **Then** a local web server starts and the user is given a URL (and the
   browser opens automatically unless suppressed) that loads the SPA.
2. **Given** a Pulumi plan or stack source is provided, **When** the
   dashboard finishes loading, **Then** the resource table shows the same
   columns and values as the TUI overview (resource, type, status, actual
   MTD, projected, delta, drift, recommendations, warnings).
3. **Given** data is still loading, **When** the user views the dashboard,
   **Then** they see progressive loading feedback equivalent to the TUI's
   phased loading checklist and per-row enrichment.
4. **Given** no plan or state input is supplied, **When** the user launches
   `--web` from the folder containing their Pulumi program, **Then** the
   session auto-detects the project, stack, and state exactly as
   `finfocus overview` does from that working directory (including its normal
   error behavior when nothing can be resolved); there is no in-UI source
   picker.
5. **Given** an explicit source (a plan JSON or a state file) is passed
   alongside `--web`, **When** the session starts, **Then** it uses that
   source instead of auto-detection, as `finfocus overview` does.

---

### User Story 2 - Explore Actual Costs and Recommendations (Priority: P2)

A user navigates within the SPA to the actual-cost view and the
recommendations view. They can filter, sort, and drill into per-resource
detail — including cost breakdowns, sustainability metrics, and
recommendation scorer signals — with the capabilities the TUI offers, plus two
web-only additions (an in-view group-by selector and an include-dismissed
toggle, see FR-008a and FR-009a).

**Why this priority**: Cost investigation is the second most common
workflow after the overview. It reuses the same data pipeline as Story 1
and can be shipped independently once the overview shell exists.

**Independent Test**: From the running SPA, open the actual-cost view with
a fixture dataset; filter and sort the table, open a resource detail, and
confirm the values match `finfocus cost actual` output for the same input.
Repeat for the recommendations view against `finfocus cost recommendations`.

**Acceptance Scenarios**:

1. **Given** the SPA is running with cost data loaded, **When** the user
   opens the actual-cost view, **Then** they see the resource cost table
   matching the TUI's cost view for the grouping the session was launched
   with.
2. **Given** the actual-cost table, **When** the user filters by text or
   cycles sort fields, **Then** the visible rows update the same way the
   TUI's filter and sort keys behave.
3. **Given** the recommendations view, **When** the user selects a
   recommendation, **Then** they see the full detail (action type, savings,
   description, scorer signals) the TUI detail view shows.
4. **Given** dismissed recommendations exist, **When** the user toggles
   "include dismissed" (a web-only addition; the TUI has no such toggle),
   **Then** dismissed items appear, matching the `--include-dismissed` CLI
   behavior.
5. **Given** the actual-cost view, **When** the user picks a different
   group-by option (a web-only addition; the TUI fixes grouping at launch),
   **Then** the view re-fetches and shows the same grouping that
   `finfocus cost actual --group-by <option>` produces.

---

### User Story 3 - What-If Cost Estimation in the Browser (Priority: P3)

A user opens the estimate view, edits resource properties, and watches
projected monthly cost recalculate live — the browser equivalent of
`finfocus cost estimate --interactive`, including pricing-mode selection.

**Why this priority**: Valuable but the least-used of the TUI experiences;
it depends on the same engine data and can land after the read-only views.

**Independent Test**: In the SPA estimate view, change an editable property
of a fixture resource and confirm the per-property delta and total monthly
delta update, matching `finfocus cost estimate --interactive` results for
the same edit.

**Acceptance Scenarios**:

1. **Given** the estimate view is open, **When** the user edits a resource
   property and commits the change, **Then** the baseline-vs-modified
   comparison and per-property deltas recalculate without a page reload.
2. **Given** multiple pricing modes are available from the plugin, **When**
   the user switches pricing mode, **Then** the estimate recalculates using
   the selected mode.

---

### Edge Cases

- **Port already in use**: The launch must fail with a clear message (or
  pick an alternate port and report it), not a stack trace.
- **Non-interactive/headless environment**: When no browser can be opened
  (CI, SSH session), the server still starts and prints the URL; it must
  not hang waiting for a browser.
- **Plugin unavailable or errors mid-load**: The affected view shows the
  same partial results and inline error indicators the TUI shows, and other
  views remain usable.
- **Encrypted Pulumi stack**: The UI must prompt for the passphrase within
  the browser, equivalent to the TUI's inline prompt (never echoing it).
- **Server shutdown**: Stopping the CLI (Ctrl+C) shuts the server down
  cleanly; a closed browser tab does not kill the server.
- **Concurrent sessions**: Two browser tabs open against the same server
  show consistent data.
- **Large stacks**: Stacks with hundreds of resources remain navigable;
  the UI must paginate or virtualize long lists as the TUI does.
- **Missing or wrong token**: A request without a valid token gets a plain
  rejection page or status that tells the user to use the URL printed in the
  terminal; no data is returned.
- **Second tab or reload**: Opening the printed URL again (new tab, reload,
  another browser profile) works for as long as the server runs, and the page
  recovers the current load state instead of restarting the load.
- **Preview already running**: Triggering the on-demand preview while one is
  in progress does not start a second `pulumi preview`; the user sees the
  running one's elapsed time.
- **Wrong passphrase**: A rejected passphrase shows an inline error and lets
  the user retry; the passphrase is never echoed, stored, or logged.
- **Terraform-only directory**: `--web` does not read Terraform state (see
  FR-012a). In a directory with no Pulumi project and no explicit source, it
  fails with the same error `finfocus overview` gives, and no server starts.
- **Web-only flags without `--web`**: `--port`, `--no-browser`, and the source
  flags FR-012 lists, given without `--web`, are rejected with a clear
  message, never silently ignored.
- **Another finfocus web session**: Two `finfocus --web` processes on the same
  machine (different ports) do not log each other out or share a session.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The main `finfocus` program MUST accept a `--web` flag, defined
  on the root command only (like `--mcp`), that starts a local web server
  serving the single-page application. `--web` MUST NOT be available on any
  subcommand, and there is no `web` subcommand. `--web` and `--mcp` are
  mutually exclusive.
- **FR-002**: On launch, the system MUST print the server URL and attempt to
  open the user's default browser, with `--no-browser` to suppress auto-open.
- **FR-003**: The server MUST bind to the loopback interface (FR-016a) and
  MUST allow the user to choose a port with `--port`, failing clearly when the
  port is unavailable.
- **FR-004**: The SPA MUST provide the same four interactive experiences as
  the TUI: overview dashboard, actual-cost view, recommendations view, and
  interactive estimate (what-if) view.
- **FR-005**: The overview dashboard MUST show the same resource table
  columns, summary status, budget-health footer, and cluster expand/collapse
  behavior as the TUI overview, and MUST offer an on-demand preview trigger
  equivalent to the TUI's `p` key (running a live preview in state-only
  mode, with elapsed-time feedback).
- **FR-006**: Every list view MUST support text filtering, sort cycling
  across the same fields as the TUI, drill-down to per-resource detail, and
  pagination or virtualization for large result sets.
- **FR-007**: The system MUST show progressive loading feedback (load phases
  and per-resource enrichment progress) equivalent to the TUI's loading
  checklist and live row updates.
- **FR-008**: The actual-cost view MUST display cost breakdowns, trends, and
  sustainability metrics as the TUI does, for the grouping the session was
  launched with (default: as `finfocus cost actual` with no `--group-by`).
- **FR-008a**: The actual-cost view MUST also offer an in-view group-by
  selector. This is a web-only addition beyond the TUI: choosing an option
  re-fetches through the shared code path. The CLI's `cost actual --group-by`
  accepts `resource`, `type`, `provider`, `date` (a deprecated alias of
  `daily`), `daily`, `monthly`, and a tag filter written `tag:key=value`
  (which filters rather than groups). The web UI offers `resource`, `type`,
  `provider`, `daily`, and `monthly` as group-by choices, plus no grouping,
  and a tag filter input that takes the same `key=value` expression. It does
  not offer `date`, because it is a deprecated alias that duplicates `daily`.
  Every offered choice MUST produce the same result as the matching CLI
  invocation, and an unsupported value MUST be rejected, not ignored.
- **FR-009**: The recommendations view MUST display summary counts, savings
  totals, and scorer signals in detail, and MUST respect the dismissal store
  by hiding dismissed recommendations by default, as the TUI does.
- **FR-009a**: The recommendations view MUST also offer an include-dismissed
  toggle. This is a web-only addition beyond the TUI (the TUI has no such
  toggle; today it is the CLI's `--include-dismissed` flag). Toggling it
  re-fetches through the shared code path and MUST show the same items
  `finfocus cost recommendations --include-dismissed` shows. The web UI MUST
  NOT dismiss or undismiss anything.
- **FR-010**: The estimate view MUST allow in-place property editing with
  live recalculation, baseline-vs-modified comparison, and pricing-mode
  selection, matching the TUI's interactive estimate.
- **FR-011**: The SPA MUST obtain all data through the same cost engine and
  plugin pipeline the CLI/TUI uses — no separate data source or duplicated
  pricing logic.
- **FR-011a**: Any API layer introduced to serve the SPA MUST be a thin
  transport over the exact same Go functions the TUI calls. It MUST NOT
  introduce new, parallel implementations of cost calculation, aggregation,
  grouping, filtering, sorting, estimation, or recommendation logic. If a
  behavior does not exist in the shared code path, it MUST be added there
  (benefiting both TUI and web) — never reimplemented for the web.
- **FR-011b**: The SPA client MUST be a renderer and input collector only.
  All cost math, currency/percent formatting rules, aggregation, filtering,
  sorting, and recalculation MUST execute in the shared Go code path, so a
  given input produces identical numbers in CLI, TUI, and web.
- **FR-012**: `--web` MUST be run from the Pulumi project directory and, with
  no source flags, MUST auto-detect the project, stack, and state exactly as
  `finfocus overview` does from the working directory (the root command
  already does this for the overview dashboard). Overrides for the cases
  auto-detection does not cover MUST be accepted on the root command, next to
  `--web`: `--pulumi-json`, `--pulumi-state`, `--stack`, `--from`, `--to`,
  `--adapter`, and `--filter`, with the meaning and mutual-exclusion rules they
  have on `finfocus overview`. These flags are defined on the root command only
  (never as persistent flags, so no subcommand inherits them), are meaningful
  only with `--web`, and MUST be rejected with a clear message when given
  without it. `--project-dir` keeps its existing meaning (configuration
  resolution) and does not change where source auto-detection looks; that stays
  the working directory, as for `finfocus overview`. The SPA MUST NOT offer its
  own file picker or upload mechanism.
- **FR-012a**: Terraform projects are out of scope for this feature.
  Auto-detection is Pulumi-only, and the overview, recommendations, and
  estimate experiences have no Terraform input path in the CLI today
  (`--terraform-state` exists only on `cost projected`, `cost actual` with
  `--from`, and `cost forecast`). `--web` MUST NOT accept `--terraform-state`.
  Supporting Terraform in the web UI depends on first giving those CLI
  experiences a Terraform path (see Follow-ups).
- **FR-013**: When a browser cannot be opened (headless/CI), the system MUST
  still start the server and print the URL without blocking.
- **FR-014**: When a Pulumi stack is encrypted, the SPA MUST provide a
  masked passphrase prompt equivalent to the TUI's inline prompt.
- **FR-015**: The server MUST shut down cleanly on interrupt, and MUST NOT
  expose data beyond the local machine by default.
- **FR-016**: At startup the server MUST generate a random per-session token.
  The printed/auto-opened URL MUST contain the token, and the server MUST
  reject any request that does not carry a valid token. The token stays
  valid until the server stops (it is not single-use, so a second tab or a
  reload keeps working) and MUST NOT be logged by finfocus. The server MUST
  also reject requests whose `Host` or `Origin` is not the server's own
  loopback address, and MUST reject state-changing requests that do not come
  from the SPA itself. This protects against other local processes, other
  localhost ports, DNS rebinding, and websites open in the user's browser.
- **FR-016a**: The server MUST listen on the loopback interface only. Unlike
  `finfocus mcp-server --transport=http`, which accepts a non-loopback
  `--addr` only with `--allow-non-loopback`, `--web` has no option to bind a
  non-loopback address.
- **FR-017**: The SPA MUST be fully operable by keyboard alone (every
  interaction available without a pointer, with visible focus indication),
  MUST meet WCAG AA color-contrast thresholds in both light and dark
  palettes, and MUST use semantic markup so screen readers can navigate
  tables, details, and forms. A formal accessibility audit is not required.
- **FR-018**: Nothing the web UI serves may expose more than the equivalent
  CLI JSON output or TUI view already shows. Pulumi secrets, credential-like
  property names, and budget notification destinations (webhook URLs, Slack
  channels, headers) MUST be omitted or redacted by the same shared rules the
  CLI and plugin requests use, including in resource detail, property diffs,
  and the estimate view's editable properties. The passphrase MUST reach the
  `pulumi` subprocess the same way the TUI passes it (subprocess environment
  only) and MUST NOT appear in any response, event, or log line.
- **FR-019**: The web UI MUST stay read-only toward infrastructure and MUST
  NOT add persistent state. The only side effects it can trigger are the
  ones the TUI already has: the on-demand `pulumi preview`, plugin queries,
  and the existing optional cost cache and history reads and writes. Deleting
  every local store MUST NOT stop `--web` from working.

### Key Entities *(include if feature involves data)*

- **Web Session**: A running server instance tied to one CLI invocation;
  holds the loaded plan/stack source, filters, and date range shared by all
  connected browser clients.
- **Overview Row / Cost Result / Recommendation / Estimate Result**: The
  existing engine result types the TUI renders today; the SPA renders the
  same entities without modification of their meaning.
- **Dismissal Record**: The existing local dismissal decisions governing
  which recommendations are hidden by default.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can go from running the launch command to viewing a
  populated cost dashboard in the browser in under 30 seconds for a typical
  stack (≤250 resources).
- **SC-002**: 100% of the data shown in the TUI for a given input (rows,
  totals, breakdowns, recommendations, estimates) is also shown in the web
  UI for the same input — verified by side-by-side comparison on at least
  three fixture stacks.
- **SC-002a**: For every fixture stack tested, every number and formatted
  value shown in the web UI equals the value the TUI and plain CLI show for
  the same input, with no rounding or formatting difference, and no
  web-specific calculation code exists (FR-011a/FR-011b).
- **SC-006**: Requests without a valid token, with a foreign `Host`, or with
  a foreign `Origin` are rejected in 100% of automated test cases, and no
  secret, credential value, or notification URL appears in any response of
  the fixture suite (FR-016, FR-018).
- **SC-003**: Every interactive TUI capability in scope (filter, sort,
  drill-down, cluster expand/collapse, property edit with recalculation,
  pricing-mode switch) is completable in the browser with no terminal
  interaction beyond the launch command, and the web-only additions
  (group-by selector, include-dismissed toggle) return the same results as
  the matching CLI flags (FR-008a, FR-009a).
- **SC-004**: The dashboard remains responsive (filter/sort actions complete
  in under 1 second) for stacks of up to 1,000 resources.

## Assumptions

- **Scope is the four TUI experiences plus two web-only additions, not full
  CLI parity**: "All the same tools as the TUI" means the four interactive
  TUI experiences (overview, cost actual, recommendations, interactive
  estimate) and the interactions available inside them. The group-by
  selector (FR-008a) and the include-dismissed toggle (FR-009a) go beyond the
  TUI and are documented as web-only. CLI-only surfaces that have no TUI screen (plugin
  management, config editing, budget definition, JSON/NDJSON export, the
  dismiss/undismiss subcommands) are out of scope for this feature; the web
  UI is read-oriented with the exception of the estimate view's in-session
  edits, which are not persisted.
- **Local-first, single user**: The server binds to localhost and serves
  the invoking user only, guarded by a per-session token (FR-016). No
  multi-user access or remote exposure is required for this feature; those
  would be separate features.
- **Boundaries**: The web UI is another presentation layer like the TUI
  (CONTEXT.md "Presentation"). It is a foreground process that lives for one
  CLI invocation, adds no required persistent state, no provider logic, and
  no write access to infrastructure (FR-019). It is separate from the MCP
  server: it exposes no tool surface and is not part of the MCP tool
  allow-list.
- **Complement, not replacement**: The TUI remains fully functional;
  `--web` is an additional way to experience the same data.
- **Browser availability**: A modern evergreen browser is assumed; the
  feature does not target terminal-only environments (the TUI already
  serves those).
- **Data freshness**: Views reflect the data loaded at session start,
  plus the same on-demand refresh the TUI offers: a user-triggered
  `pulumi preview` in the overview (equivalent to the TUI's `p` key, with
  elapsed-time feedback) and recalculation in the estimate view. There is
  no timed auto-refresh and no general re-query of plugins; users relaunch
  for fresh actual costs.
- **Single source of logic**: There is exactly one implementation of every
  cost behavior, and it lives in the existing Go engine/TUI-facing code.
  The web layer adds transport and rendering only; any missing capability
  is added to the shared path, not to the web layer.

## Follow-ups

- **Usability testing**: A usability study of the three core journeys (view
  the overview, investigate a recommendation, run a what-if estimate) is a
  later follow-up. It is not a success criterion for this feature; the former
  SC-005 is withdrawn and its ID is not reused.
- **Terraform support**: Needs a Terraform input path for the overview,
  recommendations, and estimate experiences in the CLI first (FR-012a).
