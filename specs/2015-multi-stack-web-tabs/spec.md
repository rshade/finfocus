# Feature Specification: Multi-Stack Web Dashboard with a Tab Bar

**Feature Branch**: `2015-multi-stack-web-tabs`
**Created**: 2026-11-08
**Status**: Implemented
**Input**: GitHub issue rshade/finfocus#2039 — "web: --stack=a,b,c multi-stack
web UI with tab bar". Step 4 of 4 (#2036, #2037, #2038, then #2039). Builds on
the `/s/<stack>/` routing and prefix-safe frontend (spec 2012) and the
per-stack-safe data path (spec 2013).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Open several stacks in one dashboard (Priority: P1)

An engineer runs `finfocus --web --stack=dev,prod` and gets one page with a
tab per stack along the top. Selecting a tab shows that stack's own Overview,
Cost, Recommendations, and every other existing view, labeled with the stack
name. Nothing is combined across stacks.

**Why this priority**: This is the feature. Without it the flag is only
parsed.

**Independent Test**: Start the dashboard with two stacks that hold different
resources, switch tabs, and confirm each tab shows only its own resources and
the header names the active stack.

**Acceptance Scenarios**:

1. **Given** `--stack=a,b`, **When** the page loads, **Then** it shows two
   tabs, `a` and `b`, with the first selected and its Overview loading.
2. **Given** both tabs, **When** the user selects `b`, **Then** every view
   shows `b`'s data, the header shows `b`, and the address reads
   `#/s/b/<view>`.
3. **Given** `--stack a --stack b`, **When** the page loads, **Then** the
   result is identical to `--stack=a,b`.
4. **Given** an address `#/s/b/cost`, **When** opened directly, **Then** `b`'s
   Cost view is shown with the `b` tab selected.
5. **Given** a stack named in `--stack` that the project does not have,
   **When** the command starts, **Then** it exits 2 and names the available
   stacks, before any server starts.

---

### User Story 2 - Stacks load only when opened (Priority: P1)

A stack's plugins and pipeline start the first time its tab is selected, not
at startup. Only the selected stack streams live updates; leaving a tab ends
its stream, and returning to it shows its retained data.

**Why this priority**: Starting every stack's plugins up front multiplies
startup cost and plugin processes by the number of stacks.

**Independent Test**: Start with three stacks and confirm only the first has
started plugins; select the second and confirm it starts then; confirm one
event stream is open at any moment.

**Acceptance Scenarios**:

1. **Given** three stacks, **When** the dashboard opens, **Then** only the
   selected stack's pipeline has started.
2. **Given** a stack never opened, **When** the dashboard shuts down, **Then**
   shutdown completes without error and no process remains.
3. **Given** the user leaves a tab and returns, **When** it is reselected,
   **Then** the data already loaded is shown, not reloaded from scratch.

---

### User Story 3 - A broken stack does not break the others (Priority: P1)

A stack that cannot load (no state, decrypt failure, plugin failure) shows a
tab whose view reports the error. The other tabs keep working.

**Independent Test**: Include one stack whose state cannot be read; confirm
its tab appears and shows an error while the healthy tab loads normally.

**Acceptance Scenarios**:

1. **Given** stack `bad` fails to load, **When** its tab is selected, **Then**
   the tab content states the failure in the same words a single-stack run
   uses, and `bad` is not omitted from the tab bar.
2. **Given** `bad` failed, **When** the user selects another tab, **Then** that
   stack loads and renders normally.

---

### User Story 4 - Each stack asks for its own passphrase (Priority: P2)

When a selected encrypted stack needs a passphrase, the prompt appears in that
tab and the entry unlocks only that stack.

**Independent Test**: Two encrypted stacks with different passphrases; unlock
one, switch, and confirm the other still prompts.

**Acceptance Scenarios**:

1. **Given** stack `prod` needs a passphrase, **When** the user opens its tab,
   **Then** the prompt appears there, and entering it loads `prod` only.
2. **Given** a wrong passphrase, **When** submitted, **Then** the retry
   behavior is the same as a single stack.
3. **Given** a wrong passphrase was submitted, **When** the tab bar refreshes,
   **Then** that tab shows the locked marker (a prompt is pending), not the
   failed marker, and a correct entry then loads the stack.
4. **Given** a passphrase was just submitted and the stack is loading again,
   **When** the tab bar refreshes, **Then** that tab reads loading, not failed.

---

### User Story 5 - Keyboard and screen-reader use of the tab bar (Priority: P2)

The tab bar is a tablist: arrow keys move between tabs, Home and End jump to
the first and last, the selected tab is announced, and the selected tab's
content is associated with it.

**Independent Test**: Drive the bar with the keyboard only and with an
accessibility snapshot.

**Acceptance Scenarios**:

1. **Given** focus is on a tab, **When** the user presses Right or Left,
   **Then** focus and selection move to the next or previous tab, wrapping at
   the ends.
2. **Given** a tab bar, **When** inspected, **Then** it exposes tablist, tab,
   and selected state, and the view region is the tab's panel.
3. **Given** a route change after a tab switch, **When** focus handling runs,
   **Then** it follows the existing rule that focus a user already moved is
   not stolen.

---

### User Story 6 - One stack behaves exactly as today (Priority: P1)

`finfocus --web`, `--web --stack dev`, and the TUI hand-off show no tab bar, no
new header element, and the same addresses (`#/<view>`) as before.

**Acceptance Scenarios**:

1. **Given** zero or one stack, **When** the page loads, **Then** the page is
   unchanged and no stack list is offered.
2. **Given** `--stack=a,b` with `--pulumi-json`, `--pulumi-state`, or a
   Terraform project, **When** the command starts, **Then** it exits 2 with a
   message that a stack list needs a Pulumi project's own stacks.

### Edge Cases

- Duplicate names in the list (`a,a,b`) collapse in first-appearance order. A
  list that ends with one name (`a,a`) behaves as a single stack, with no tab
  bar (FR-001, FR-013).
- An empty element in a list of two or more values (`a,,b`, `a,`, or
  `--stack a --stack ""`) or an invalid name is a usage error (exit 2). A single
  empty value (`--stack ""`) is no stack, as today.
- A name that is not a valid URL path segment (spec 2012) is a usage error.
- An address with no stack (`#/cost`) in multi-stack mode goes to the first
  stack's same view. An unknown stack in the address goes to the first stack.
- `finfocus --web --stack=a,b` through `tailscale serve` works with the
  options from #2036; the tab bar adds no absolute URLs.
- The cookie is one global cookie (spec 2012), so one sign-in covers all tabs.
- A passphrase prompt left open while switching tabs is shown again when that
  tab is reselected.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `--stack` MUST accept comma-separated values and repeats, and
  MUST trim whitespace around each name. Zero or one resulting name MUST run the existing single-stack path unchanged. A
  lone empty value counts as no name; an empty element beside another value is
  a usage error.
- **FR-002**: With two or more names, the command MUST validate each against
  the project's stacks (`pulumi.ListStacks`) before serving, exit 2 on an
  unknown name, and list the available names in the message.
- **FR-003**: A stack list MUST be rejected (exit 2) with `--pulumi-json`,
  `--pulumi-state`, or a Terraform project.
- **FR-004**: Each stack MUST get its own session, pipeline, passphrase
  channel, and plugin set, registered under `/s/<stack>/`, sharing only the
  stores and log hub (spec 2013).
- **FR-005**: A stack's pipeline MUST start on its first activation, once, no
  matter how many times it is activated. Never-activated stacks MUST shut
  down cleanly.
- **FR-006**: The server MUST offer the page the list of stacks, in the order
  given, in the shell page itself so tabs never wait on a request, and with each
  stack's load state (`idle` for not started, `loading`, `locked` for a pending
  passphrase prompt, `ready`, `failed`) from a stack list route, and a way to
  activate a stack. The routes MUST pass the same authentication and hardening
  as every other route.
- **FR-007**: The tab bar MUST live in the app shell (no iframes), route as
  `#/s/<stack>/<view>`, and send every API, stream, and import request under
  the active stack's `/s/<stack>` prefix.
- **FR-008**: Exactly one overview event stream MUST be open, for the active
  stack. Switching MUST close the old stream and discard its client state
  before the new view mounts.
- **FR-009**: A stack that fails to load MUST keep its tab and show the
  failure in its view. Failure text MUST be the safe label the single-stack
  page uses. Building a stack's runtime starts nothing and cannot fail, so
  every failure, including one at start, arrives as a pipeline error on that
  stack's own session and never aborts the server or another stack.
- **FR-010**: The header MUST show the active stack's name in multi-stack
  mode.
- **FR-011**: The tab bar MUST follow the ARIA tablist pattern with Left,
  Right, Home, and End key handling.
- **FR-012**: Passphrase prompts and answers MUST stay inside the stack that
  asked. A rejected passphrase leaves the stack `locked` (retryable), not
  `failed`.
- **FR-013**: Zero or one stack MUST render no tab bar and keep `#/<view>`
  addresses and root-relative behavior.
- **FR-014**: Shutdown MUST stop every started stack and release plugin
  processes, as the single-stack path does.
- **FR-015**: Docs (web-ui guide, CLI reference) MUST describe the list form.
  The schema golden for the root command MUST reflect the flag type.

### Key Entities

- **Stack tab**: a stack name, its load state, and its retained view data.
- **Stack list**: the ordered, de-duplicated names from `--stack`.

## Web UI Fit *(mandatory)*

- **Disposition**: Extends the app shell. This feature is the web dashboard
  work itself: it adds a tab bar and header label and reuses every existing
  view unchanged under each stack.
- **Web data**: Per-stack results already served by the existing views. The
  only new data is the stack list with load states.
- **Layout fit**: The tab bar sits in the header between the title and the view
  navigation, built from the existing header slots. Each stack's views fill the
  same heading, controls row, status line, summary strip, and captioned table
  as today.
- **Additions**: Header stack label; per-tab load state markers (failed and
  locked, each as text plus an accessible label, not color alone).
- **Render evidence**: Playwright render of two stacks from a populated
  project, recorded in the PR.
- **Follow-up issue scope**: One UX-only `web-ui` issue is filed on
  completion (Principle VII). It covers tab bar layout with many or long stack
  names (overflow and wrapping on narrow screens), tab marker copy, and
  keyboard and screen-reader polish found in the render check. It carries no
  engine, API, redaction, or data-shape work. #2039 is the feature issue, not
  this follow-up.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With N stacks, starting the dashboard starts exactly 1 stack's
  plugins; each other stack starts only when first selected.
- **SC-002**: At any moment the page holds at most 1 open overview event
  stream.
- **SC-003**: A user can reach any stack's any view with the keyboard alone.
- **SC-004**: A failing stack never prevents another stack's tab from loading
  and rendering (automated test with one failing and one healthy stack).
- **SC-005**: Single-stack runs produce the same page and addresses as the
  previous release; the existing web suites pass with only pins for the flag
  type changed.

## Assumptions

- Stack names are Pulumi simple names; qualified names are out of scope.
- No cross-stack totals or comparison; each tab is independent.
- The first listed stack is the initial tab.
- Only Pulumi projects support stack lists.
- A `failed` stack stays failed until the dashboard restarts; there is no
  in-page retry. A `locked` stack retries through its passphrase prompt.
