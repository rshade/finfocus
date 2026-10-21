# Feature Specification: Overview Web Hint

**Feature Branch**: `1731-overview-web-hint`
**Created**: 2026-10-20
**Status**: Draft
**Input**: User description: "Web hint in the overview TUI: when bare
`finfocus` (or `finfocus overview`) opens the interactive TUI, show a footer
hint and a key binding (`w`) that tells the user the browser dashboard exists
and how to start it (`finfocus --web`). Pressing `w` quits the TUI and starts
the same web session as `finfocus --web`, including SIGINT/SIGTERM shutdown
that stops plugin children. No new prompt: agents and non-TTY runs keep
getting plain text unchanged, so no hint or key is ever shown or active in
plain/non-interactive mode. Bare `finfocus` outside a Pulumi project still
prints help."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover the browser dashboard from the terminal (Priority: P1)

A person runs `finfocus` in a Pulumi project from an interactive terminal and
lands in the overview. Today nothing tells them a browser dashboard exists.
The overview now shows a short hint in its footer naming the dashboard and the
command that starts it.

**Why this priority**: Discovery is the whole point. Without the hint, the
web experience stays invisible to anyone who never reads the docs.

**Independent Test**: Run `finfocus` in a populated project from a real
terminal and read the footer. The hint is present, names the key and the
`finfocus --web` command, and does not hide existing key help.

**Acceptance Scenarios**:

1. **Given** an interactive terminal inside a Pulumi project, **When** the
   overview opens, **Then** the footer shows a hint that the browser dashboard
   is available, the `w` key, and `finfocus --web`.
2. **Given** a narrow terminal, **When** the overview opens, **Then** the
   existing footer key help keeps priority and the hint is shortened or
   dropped rather than wrapping or overflowing the line.
3. **Given** the overview is showing an error or an empty stack, **When** the
   footer renders, **Then** the hint still appears and does not obscure the
   error.

---

### User Story 2 - Switch to the dashboard with one key (Priority: P2)

A person reading the hint presses `w`. The terminal view closes cleanly and
the same browser session that `finfocus --web` starts is launched, with the
same link printed in the terminal. Ctrl+C stops the web session and the
plugins it started, as it does for `finfocus --web`.

**Why this priority**: The hint is more useful when it can be acted on, but
the hint alone already delivers discovery, so this follows it.

**Independent Test**: Press `w` in the overview and confirm the web session
starts, the terminal is restored, and an interrupt shuts down the server and
leaves no plugin processes behind.

**Acceptance Scenarios**:

1. **Given** the interactive overview is open, **When** the person presses
   `w`, **Then** the overview exits, the terminal is restored, and the web
   session starts with the same behavior and output as `finfocus --web`.
2. **Given** the web session started from the overview, **When** the person
   interrupts it (SIGINT or SIGTERM), **Then** the server stops and plugin
   child processes are stopped, as with `finfocus --web`.
3. **Given** the overview list is shown while rows are still loading, **When**
   the person presses `w`, **Then** loading is canceled cleanly and the web
   session starts without leaving plugin processes from the overview behind.
4. **Given** the web session fails to start (for example the port is taken),
   **When** the person pressed `w`, **Then** the same error `finfocus --web`
   would give is shown and the exit code matches it.

---

### User Story 3 - Agents and scripts see no change (Priority: P1)

Agents, CI jobs, pipes, and anything without an interactive terminal get the
same plain-text overview they get today. They are never shown a hint, never
prompted, and the `w` key does nothing because there is no interactive view.

**Why this priority**: A prompt or key handler that blocks an unattended run
is a regression for the people who depend on deterministic output.

**Independent Test**: Run bare `finfocus` in a project with stdin and stdout
redirected, with `AGENT_MODE` set, and with `--plain`. Compare output with the
current release: byte-for-byte equal apart from nothing new.

**Acceptance Scenarios**:

1. **Given** stdout is not a terminal, **When** `finfocus` runs in a project,
   **Then** the output is the plain-text overview with no hint line.
2. **Given** `AGENT_MODE` is set or `--plain` is passed, **When** the overview
   runs, **Then** no hint appears and no key is read.
3. **Given** any non-interactive run, **When** it finishes, **Then** it exits
   without waiting for input and with the same exit code as before.

---

### User Story 4 - Bare finfocus outside a project is unchanged (Priority: P3)

Running `finfocus` outside a Pulumi project still prints help. This feature
does not add a picker, a prompt, or an empty dashboard there.

**Why this priority**: It records a boundary so the feature does not grow into
a launcher.

**Independent Test**: Run `finfocus` in an empty directory from a terminal and
confirm the help text is unchanged.

**Acceptance Scenarios**:

1. **Given** a directory with no Pulumi project, **When** `finfocus` runs with
   no arguments, **Then** help is printed and nothing waits for input.

---

### Edge Cases

- The person runs `finfocus overview` directly, not bare `finfocus`. The hint
  and key behave the same, because both reach the same interactive view.
- `--web` is already given on the command line. The web session starts as
  today and no overview or hint is involved.
- `w` is pressed while a detail view, filter input, or other modal is active.
  A text input receives the character as typed text; the key triggers the web
  session only from the main overview.
- The terminal is resized or very small. The hint never causes wrapping or
  hides the existing key help.
- The overview runs under the MCP server. No hint and no key apply, because
  there is no interactive terminal.
- Web startup is slow or fails after the overview has closed. The person gets
  the `finfocus --web` error and is not dropped back into the overview.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The interactive overview MUST show a footer hint that names the
  browser dashboard, the `w` key, and the command `finfocus --web`.
- **FR-002**: The hint MUST yield space to the existing footer key help and
  MUST NOT wrap or overflow at any supported terminal width; when it does not
  fit it is shortened or omitted.
- **FR-003**: Pressing `w` in the main interactive overview MUST close the
  overview, restore the terminal, and start the same web session that
  `finfocus --web` starts, with identical output, exit codes, and error text.
- **FR-004**: A web session started from the overview MUST shut down on
  SIGINT and SIGTERM and MUST stop the plugin processes it started, exactly as
  `finfocus --web` does.
- **FR-005**: Pressing `w` MUST first release everything the overview holds
  (loading work and plugin processes), including while rows are still
  loading in the list view, so no overview plugin outlives the hand-off.
- **FR-006**: In any non-interactive run (no terminal on stdin or stdout,
  `AGENT_MODE`, `--plain`, MCP) the hint MUST NOT be printed and `w` MUST have
  no effect. Output and exit codes MUST equal the current release.
- **FR-007**: The feature MUST NOT add a prompt, picker, or any input read to
  any run that has none today.
- **FR-008**: Bare `finfocus` outside a Pulumi project MUST keep printing help.
- **FR-009**: `w` MUST NOT fire while a text input or modal view has focus.
- **FR-010**: The web session started by `w` MUST use the same project
  directory, stack, and configuration the overview was using.
- **FR-011**: The `w` key and the hint MUST be listed wherever the overview's
  keys are documented, and the overview's golden render test MUST cover the
  footer.

### Key Entities

- **Footer hint**: One line of text in the interactive overview that points to
  the dashboard. Shown only when the view is interactive.
- **Hand-off**: The transition from the closing overview to the starting web
  session, carrying project directory, stack, and configuration.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable - this feature is the terminal's pointer to
  the web dashboard, not a web surface. It adds no data or view to the
  browser.
- **Web data**: None. The web session it starts is the existing one.
- **Layout fit**: Not applicable.
- **Additions**: none
- **Render evidence**: Not applicable. The existing `finfocus --web` session
  is unchanged.
- **Follow-up issue scope**: None. No `web-ui` issue is filed for this
  feature.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In an interactive terminal, 100% of overview renders at widths
  of 80 columns and above show the dashboard hint without wrapping.
- **SC-002**: From pressing `w`, the web link is printed in the terminal
  within the time `finfocus --web` takes plus no more than 1 second.
- **SC-003**: After interrupting a web session started with `w`, zero
  finfocus plugin processes remain.
- **SC-004**: For non-interactive runs, output and exit codes are identical to
  the previous release across the existing plain-text and agent-mode tests,
  with zero new input reads.
- **SC-005**: A person unfamiliar with the web dashboard can find the command
  that starts it by reading only the overview screen.

## Assumptions

- The interactive overview already exists, is the only terminal view reached
  by bare `finfocus` in a project, and already decides between interactive and
  plain output using the shared output-mode detection. This feature reuses
  that decision and adds none of its own.
- Starting the web session from the overview reuses the existing
  `finfocus --web` entry point and its shutdown handling; nothing in the web
  server changes.
- `w` is not already bound in the main overview. If it is, the key choice is
  revisited in planning.
- The `--web` flag keeps its current combination rules; the hand-off passes
  only values the overview already resolved.
- Windows has no SIGTERM; the interrupt behavior there matches what
  `finfocus --web` does today.
- The hint is not configurable or dismissible in this version.

## Out of Scope

- A first-run picker, prompt, or remembered default for terminal versus web.
- Opening a dashboard or any TUI outside a Pulumi project.
- Changes to the web UI, its data, or its layout.
- A hint in the plain-text overview or in any other command.
