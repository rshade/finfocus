# Feature Specification: Web UI Log Viewer

**Feature Branch**: `1925-web-logs-viewer`
**Created**: 2026-11-02
**Status**: Implemented
**Input**: User description: "Web UI log viewer for `finfocus --web`: an
opt-in Logs view that shows core and per-plugin zerolog output, colour coded
by level and separable by source, with filters, pause/follow-tail, clear and
download. Off by default; enabled by `FINFOCUS_WEB_LOGS=1` or the config key
`web.logs.enabled`. The viewer is a self-contained folder that can be
published separately later."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See what core and the plugins are doing (Priority: P1)

A person running `finfocus --web` with logs enabled opens a Logs view and sees
log lines from finfocus itself and from every running plugin as they happen.
Each line shows its time, a level badge (colour plus text), its source, and
its message. Earlier lines are already there when the view opens.

**Why this priority**: Today a failing or slow plugin leaves no evidence in
the browser, and plugin output is discarded entirely when the terminal is not
interactive. Seeing the lines is the whole value of the feature.

**Independent Test**: Start the dashboard with logs enabled against a stack
that uses one plugin, open the Logs view, and confirm lines from core and from
that plugin appear, each labelled with its source and level.

**Acceptance Scenarios**:

1. **Given** logs are enabled and the overview has loaded, **When** the person
   opens the Logs view, **Then** the lines produced during the load are
   listed, oldest first, each with time, level badge, source, and message.
2. **Given** the Logs view is open, **When** a plugin writes a new line,
   **Then** it appears without a reload and the view follows the newest line.
3. **Given** a plugin that does not use the SDK logger writes plain text,
   **When** the line arrives, **Then** it is shown as an informational line
   attributed to that plugin, not dropped.
4. **Given** two tabs are open, **When** either opens the Logs view,
   **Then** each shows the same earlier lines and receives new ones.

---

### User Story 2 - Narrow the noise (Priority: P1)

The person filters the lines by source (core, or one or more plugins), by a
minimum level, and by text, and can pause following, clear the screen, and
download what is shown.

**Why this priority**: A busy load produces hundreds of lines. Without
filtering by plugin and level, the one failing plugin cannot be found, so this
is part of the minimum useful view.

**Independent Test**: With lines from core and two plugins present, hide one
plugin, raise the minimum level to warning, and search for a word; confirm
only matching lines remain, then download and check the file holds exactly
those lines.

**Acceptance Scenarios**:

1. **Given** lines from core and two plugins, **When** the person turns off
   one plugin's source chip, **Then** that plugin's lines are hidden and
   the others stay.
2. **Given** mixed levels, **When** the minimum level is set to warn,
   **Then** only warn, error, and fatal lines remain visible.
3. **Given** a text search, **When** it matches a message or a field value,
   **Then** only those lines remain.
4. **Given** following is paused, **When** new lines arrive, **Then** the
   scroll position does not move and the view shows how many lines are new.
5. **Given** a filtered view, **When** the person chooses download, **Then**
   the file holds the visible lines as newline-delimited JSON.
6. **Given** lines on screen, **When** the person chooses clear, **Then** the
   screen empties and only lines that arrive afterwards appear.

---

### User Story 3 - Off unless asked, safe when on (Priority: P1)

A production user never sees the Logs view unless they turned it on. When it
is on, no secret value reaches the browser.

**Why this priority**: Logs can name resources, paths, and, if a plugin is
careless, credentials. The cost data on this dashboard is already protected;
logs must not be a side door.

**Independent Test**: Start without the setting and confirm there is no Logs
entry and the log stream is not found. Start with it on, make a plugin emit a
line with a credential-named field and a Pulumi secret value, and confirm
neither appears in the page or the stream.

**Acceptance Scenarios**:

1. **Given** neither the environment variable nor the config key is set,
   **When** the dashboard loads, **Then** there is no Logs entry in the
   navigation, `GET /api/logs/status` reports that logs are not enabled, and
   a request for the log stream returns not found.
2. **Given** `web.logs.enabled` is true in the config, **When** the dashboard
   loads, **Then** the Logs view is available.
3. **Given** the config key is true and `FINFOCUS_WEB_LOGS=0`, **When** the
   dashboard loads, **Then** the Logs view is not available (the environment
   wins).
4. **Given** the config key is false and `FINFOCUS_WEB_LOGS=1`, **When** the
   dashboard loads, **Then** the Logs view is available.
5. **Given** a log line with a credential-named field or a Pulumi secret
   value, **When** it is shown or downloaded, **Then** that value is replaced
   and never present in the page, the stream, or the file.
6. **Given** a log message that contains markup such as `<img onerror=...>`,
   **When** it is shown, **Then** it appears as literal text and runs nothing.

---

### User Story 4 - Turn it on for testing, reuse the viewer later (Priority: P2)

A developer or test harness enables logs once, through the environment, and
gets the view in every dashboard run. The viewer itself is a self-contained
unit that does not depend on finfocus, so it can be published for other
zerolog users later.

**Why this priority**: It makes the feature useful for the project's own
testing straight away, and the packaging choice is cheap now and expensive to
retrofit.

**Independent Test**: Set the environment variable in the dev shell and in the
browser test fixtures and confirm the view appears; run the viewer's own tests
without any finfocus code present and confirm they pass.

**Acceptance Scenarios**:

1. **Given** the development environment file sets `FINFOCUS_WEB_LOGS=1`,
   **When** a developer starts the dashboard, **Then** the Logs view appears
   with no further setup.
2. **Given** the viewer folder on its own, **When** its tests run, **Then**
   they pass with no finfocus source on the path.
3. **Given** any file in the viewer folder, **When** the boundary test scans
   its imports, **Then** none leaves the folder or names a finfocus file.
4. **Given** raw newline-delimited zerolog text fed to the viewer by someone
   else's application, **When** a line is split across two chunks,
   **Then** it is joined and shown once.

---

### User Story 5 - Settable from the Config page (Priority: P3)

The person can set `web.logs.enabled` from the Config page or `finfocus config
set`, like other known settings.

**Why this priority**: Convenience; the environment variable and the file
already cover the need.

**Independent Test**: Run `finfocus config set web.logs.enabled true` and
confirm `config get` returns it and the Config page lists it as a boolean.

**Acceptance Scenarios**:

1. **Given** the setting is unknown today, **When** the feature ships,
   **Then** it appears in the Config page as a boolean with help text that
   says logs may contain resource names and paths.
2. **Given** a non-boolean value, **When** it is set, **Then** it is rejected
   as a validation error.

---

### User Story 6 - Read the same logs from the terminal (Priority: P2)

A person runs `finfocus logs` and sees the lines from the configured log file
rendered the same way as in the Logs view: time, level badge, source, message,
with core and each plugin told apart. They can narrow by source, minimum level,
and text, and ask for JSON or newline-delimited JSON. It works without
`--web` and after a run has ended.

**Why this priority**: The log file already holds core and plugin lines by
default, but reading it means `grep` over mixed JSON. A command that uses the
dashboard's own parsing and redaction gives a terminal and CI path, and makes
the dashboard endpoint a thin transport over the same methods.

**Independent Test**: Run any finfocus command so the log file has core and
plugin lines, then run `finfocus logs --source <plugin> --level warn` and
confirm only that plugin's warn-and-above lines print, with badges, and that
`--output ndjson` prints the same entries the Logs view would show.

**Acceptance Scenarios**:

1. **Given** a log file with core and plugin lines, **When** the person runs
   `finfocus logs`, **Then** the last 200 entries print oldest first, each with
   time, level badge, source, and message.
2. **Given** the same file, **When** they pass `--source`, `--level`, and
   `--grep`, **Then** the filters combine exactly as in the Logs view.
3. **Given** `--output json` or `--output ndjson`, **When** the command runs,
   **Then** each entry has the same fields the dashboard stream sends, after the
   same redaction.
4. **Given** a credential-named field or Pulumi secret value in the file,
   **When** it is printed, **Then** it is replaced and the file is not changed.
5. **Given** no log file at the default path, **When** the command runs,
   **Then** it says where it looked and how logging is configured, and exits 1.
6. **Given** `--file` names a missing path, **When** the command runs,
   **Then** it is a validation error (exit 2).
7. **Given** a non-JSON line in the file, **When** it is printed, **Then** it
   appears as an informational line with source `unknown`.

---

### Edge Cases

- `finfocus logs` on a very large file: a positive `--tail` reads from the
  end until it has the requested entries and keeps only those. `--tail 0`
  reads the whole file one line at a time and never loads it all at once.
- A line in the file cut off at the end by a crash: shown as a raw line.
- A burst of lines faster than the browser can show: the view stays
  responsive, keeps the newest lines, and says that older ones were dropped.
- A tab that falls behind or reconnects: it receives the buffered earlier
  lines again and is told if lines were lost in between.
- More lines than the buffer holds: the oldest are discarded; the buffer never
  grows without limit.
- A plugin line that arrives in pieces across several writes, or without a
  trailing newline when the plugin exits: it is reassembled and shown once.
- A very long line: it is shown shortened with a way to see the rest, so one
  line cannot freeze the page.
- A line that is not JSON, or JSON without a level or message: shown as
  informational raw text, never dropped, never an error banner.
- A plugin that exits or restarts: its lines stay and its source chip stays
  until the dashboard process ends, even after the screen is cleared.
- Logs enabled but the dashboard served with no plugins: only the core source
  is listed.
- The log file or terminal output that already exists is unchanged by turning
  the view on: same destination, same content, same exit codes.
- Log text containing field names that look like credentials (`token`,
  `password`, `api_key`): the value is replaced even when the line is
  otherwise ordinary.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The dashboard MUST offer a Logs view only when logs are enabled.
  Otherwise it MUST show no navigation entry and MUST answer the log stream as
  not found.
- **FR-002**: Enablement MUST resolve as: `FINFOCUS_WEB_LOGS` when set (`1` or
  `true` on, `0` or `false` off), then the config key `web.logs.enabled`, then
  off. An unrecognised environment value MUST be reported as a validation
  error rather than guessed.
- **FR-003**: The Logs view MUST show lines from finfocus itself and from each
  plugin the dashboard started, each with time, level badge, source, and
  message.
- **FR-004**: Level MUST be shown as a text badge as well as colour, in light
  and dark themes, so it can be read without colour.
- **FR-005**: Each plugin MUST appear as its own source even when it does not
  use the SDK logger. A line's source MUST be the plugin's name when the plugin
  states one, otherwise the plugin that wrote it, otherwise the finfocus
  component, otherwise `core`.
- **FR-006**: The view MUST let the person filter by source (several at
  once), by a minimum level, and by text matched against the message and field
  values, and these MUST combine.
- **FR-007**: The view MUST let the person pause following, see how many lines
  arrived while paused, resume, clear the screen, and download the visible
  lines as newline-delimited JSON.
- **FR-008**: Opening the view MUST first show the lines already captured, in
  order, then new lines as they happen, without a reload, and the same for a
  second tab.
- **FR-009**: Captured lines MUST be held in a bounded buffer of 5000 lines,
  discarding the oldest. The view MUST show at most 2000 rows and say when
  older rows are not shown.
- **FR-010**: A line that is not valid JSON MUST be kept as an informational
  raw line attributed to its source. A line split across writes MUST be
  reassembled before it is read.
- **FR-011**: Field names that are credential-like and values that are Pulumi
  secrets MUST be replaced before a line leaves the process, using the same
  rules that already protect plugin environments, and MUST NOT be present in
  the stream, the page, or a download. Keys beginning `__` MUST be omitted.
  No second rule list may be added.
- **FR-012**: The page MUST show log content as text only. Markup in a message
  or field MUST NOT be interpreted.
- **FR-013**: Turning the view on MUST NOT change where core or plugin logs
  are written today, what they contain, stdout, or exit codes.
- **FR-014**: The log stream MUST be its own channel, separate from the
  overview stream, MUST apply the same access checks as the other dashboard
  endpoints, and a slow tab MUST NOT delay other tabs or the dashboard.
- **FR-015**: `web.logs.enabled` MUST be a known setting: settable with
  `config set`, listed on the Config page as a boolean, with help text that
  names the risk (resource names and paths).
- **FR-016**: The viewer MUST live in a self-contained folder that imports
  nothing outside itself, takes lines or entries from whatever transport the
  host supplies, reads zerolog field names by default with an overridable
  mapping, and styles itself through overridable colour variables. A test MUST
  fail if the folder imports anything outside itself.
- **FR-017**: The viewer folder MUST ship with its own README, licence, and
  tests, MUST NOT be published or listed as a release unit in this change, and
  MUST be served to the dashboard without a second copy of its source.
- **FR-018**: The development environment and the browser test fixtures MUST
  set `FINFOCUS_WEB_LOGS=1`. Release builds MUST NOT.
- **FR-019**: Entries MUST carry display strings (time, level label, source)
  produced in Go, so the page and `finfocus logs` show the same text. The
  viewer MUST format these itself only when they are absent.
- **FR-020**: `finfocus logs` MUST read the configured log file (or `--file`)
  and print the last N entries (`--tail`, default 200, 0 for all), oldest
  first, as a table with level badge and source, or as `json` / `ndjson` through
  `--output`. It MUST support `--source` (repeatable), `--level`, and `--grep`,
  combined as in FR-006.
- **FR-021**: `finfocus logs` and the dashboard stream MUST use the same
  parsing, source resolution, redaction, filtering, and display strings, so the
  same line yields the same entry in both. On a positive `--tail`, `seq` is the
  position in the decoded suffix. A line longer than 64 KiB inside that window
  is decoded from the start of the file, and `seq` then stays the whole-file
  index. `--tail 0` numbers `seq` from the start of the file, which is what the
  parity check compares to the dashboard stream. The dashboard handler MUST
  call the same Go methods and add no logic of its own.
- **FR-022**: `finfocus logs` MUST work whether or not logs are enabled for the
  dashboard, MUST NOT modify the file, and MUST stream the file rather than
  load it whole.
- **FR-023**: A missing default log file MUST produce a clear message and exit
  1; a missing `--file` or a bad flag value MUST be a validation error (exit
  2). The command MUST read the file only, never start plugins.
- **FR-024**: User documentation MUST describe the setting, the default, the
  redaction, what is shown, `finfocus logs`, and that logs may contain resource
  names and paths. The architecture page MUST record the design choice.

### Key Entities

- **Log entry**: One line captured from core or a plugin. Has a sequence
  number, time, level, source, whether the source is core or a plugin,
  message, and the remaining fields after redaction. A raw line has level
  informational and only a message.
- **Source**: Core, or one named plugin. The unit the person filters by.
- **Log buffer**: The bounded, ordered history of entries shared by every open
  tab. Drops its oldest entries first.
- **Viewer filters**: Selected sources, minimum level, search text, and
  following or paused. Kept per tab, like other dashboard view state.

## Web UI Fit *(mandatory)*

- **Disposition**: New view, with a CLI counterpart, `finfocus logs`. The
  dashboard endpoint calls the same Go methods the command calls.
- **Web data**: The same entries `finfocus logs` prints: lines core and plugins
  write, redacted, with display strings (time, level label, source) produced in
  Go. The page computes nothing about them beyond filtering what it holds.
- **Layout fit**: Fills the existing slots: a controls row for source chips,
  minimum level, search, follow toggle, clear, and download; a status line for
  "Showing N of M lines", "K new lines while paused", and dropped-line notices;
  and a scrolling list of lines, matching the Overview table's typography and
  spacing. A row with fields is a button that expands to show them. The list
  announces new lines while following. While paused, the status line announces
  the pending count. A buffer hole says "N log lines were dropped from the
  buffer".
- **Additions**: Level colour with text badge; source badges; expandable
  field detail; a per-tab follow toggle. No dismiss, edit, or write actions.
- **Render evidence**: 2026-11-08, `../finfocus-demo` stack `dev`,
  `FINFOCUS_WEB_LOGS=1 finfocus --web --no-browser`. Firefox 159 and Chromium
  158 (Playwright): light and dark schemes, level borders use the level
  colour, and the focus ring follows the dashboard ring. At 360 px the toolbar
  wraps, a long plugin chip stays inside the viewport, and the page and list
  do not scroll sideways. Enter and Space expand and collapse a row's fields.
  Tab reaches every toolbar control and that row, each with a 2 px solid
  outline. Opening the fields scrolls them into view and pauses follow.
  Download saved an NDJSON file. Paused status read "Showing 2 of 2
  lines, 1 new line while paused"; the list was not live and the status line
  was. WebKit did not launch on this host (missing libevent, libavif, and
  libmanette).
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility). No engine, API, redaction, or data-shape work.

## Assumptions

- Core and plugin logs are zerolog JSON, one event per line, using the field
  names the SDK and core already emit.
- "On for testing" means the dev shell and test fixtures set the environment
  variable; the shipped default is off, as decided with the maintainer. No
  build-type detection is used.
- Capturing starts when the dashboard starts, so lines from the initial load
  are available. When logs are off nothing is captured.
- Buffer size 5000 and row cap 2000 are defaults; making them configurable is
  out of scope.
- `finfocus logs` is a snapshot: no `--follow` in this change. The default
  config already writes `~/.finfocus/logs/finfocus.log`, so the command works
  with no setup. Live tailing is a possible follow-up.
- Lines in the file carry no per-plugin writer tag, so a non-JSON line has
  source `unknown`; JSON lines from SDK plugins carry `plugin_name`.
- Redaction applies to field names and values. Free text inside `message`
  cannot be inspected reliably and is shown as written; plugins are already
  told never to log credentials.
- Logs may contain resource names and paths. They are shown only on localhost
  behind the dashboard's existing access protection; remote or multi-user
  exposure is out of scope.
- The viewer's npm name and scope are placeholders; publishing is a later
  decision.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With logs enabled, a person can find the plugin that failed
  during a load, by source and level, in under 30 seconds without leaving the
  browser.
- **SC-002**: With logs off, 100% of dashboard runs show no Logs entry,
  `GET /api/logs/status` reports that logs are not enabled, and the log
  stream returns not found, with no logs captured.
- **SC-003**: In seeded tests with credential-named fields and Pulumi secret
  values, 0 secret values appear in the stream, the page, or a download.
- **SC-004**: Turning the view on changes nothing in the existing log file,
  terminal output, stdout, or exit codes in the comparison tests.
- **SC-005**: A burst of 10,000 lines leaves the page responsive (it still
  accepts a filter change within 1 second) and shows no more than 2000 rows.
- **SC-006**: Every level can be told apart in a screenshot rendered without
  colour, using the text badge alone.
- **SC-007**: The viewer's own tests pass with no finfocus source present, and
  the boundary test fails when an outside import is added.
- **SC-008**: A line split across any number of writes appears exactly once,
  in order.
- **SC-009**: For a shared fixture file, `finfocus logs --tail 0 --output ndjson`
  and the dashboard stream produce identical entries (same fields, same
  redaction). A positive `--tail` numbers `seq` inside its suffix, unless a
  line longer than 64 KiB in that window forces a decode from the start of
  the file, in which case `seq` stays the whole-file index.
- **SC-010**: `finfocus logs` prints the last 200 entries of a 500 MB log file
  in under 5 seconds, while holding no more than the requested entries and one
  line in memory. A positive `--tail` reads backward from the end of a
  seekable file until it has that many matching entries, then decodes only
  that suffix. `--tail 0` still reads the whole file, because it prints every
  matching entry. `BenchmarkDecodeTail` records forward-scan throughput.
  Decision (#2020): the time limit stays. A forward scan at the recorded rate
  takes well over 5 seconds on a 500 MB file, and that wait grows with the
  file.
