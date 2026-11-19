# Feature Specification: Cost in the Editor for VS Code and Its Forks

**Feature Branch**: `2017-vscode-cost-extension`
**Created**: 2026-11-16
**Status**: Implemented; owner and CI gates open (validation.md, "Requirement reconciliation")
**Input**: User description: "VS Code-family editor extension for finfocus
(Pulumi first): inline cost annotations at each resource's declaration line
plus a small side-bar cost dashboard, published to VS Code Marketplace and
Open VSX so VS Code, Cursor, Windsurf, VSCodium and other forks all install
it."

## Context and Decisions

This spec records a brainstorm held on 2026-11-16. The decisions below are
the owner's unless marked as the author's call.

- **Why**: developers write Pulumi code in an editor, and today finfocus
  reaches them only in the terminal, the web dashboard, CI, and (for coding
  agents) the MCP server. The extension puts the cost of each resource where
  the resource is written, and a small cost summary beside the code.
- **Two surfaces only**: inline annotations and a small side-bar dashboard.
  Agent integration is out of scope; the MCP server already serves Cursor,
  Windsurf, and VS Code agents.
- **Freshness**: annotations and the dashboard show the deployed stack's
  numbers at once. Resources whose code was edited since the last result are
  marked stale. A preview, which prices the edits, runs on demand, and on
  save only when the user turns that on. Users accept that a better number
  means waiting for pricing.
- **Dashboard shape** (chosen from three mockups): a side-bar summary card on
  top, with a native resource tree below it as the complete list and as the
  fallback when a fork's webview misbehaves, plus a status-bar item.
- **Pulumi first**: Terraform annotations are a later feature.
- **Placement**: the extension lives in this repository as its own package,
  so a later change to the data it reads and to the code that reads it can
  land in one pull request. The first delivery is split into a core pull
  request and a stacked extension pull request only because the extension's
  minimum finfocus version must name a core release that already exists.
- **Data source** (author's call, accepted with the placement): the extension
  runs the installed finfocus CLI and reads its JSON overview. It computes and
  formats no money of its own and does not embed the web dashboard.
- **Binary** (author's call): the extension uses a finfocus already installed
  on the machine, with a setting for its path. It does not bundle one.
- **Core JSON gaps** (found by the design review, owner approved): the JSON
  overview carries numbers only and reports plugin budgets only. Core adds
  display text (FR-005) and the config budget fallback (FR-006).
- **Publishing**: the publisher name is `finfocus`, so the extension ID is
  `finfocus.vscode-finfocus` from the first package on. Publishing to the
  stores, creating the store accounts that claim that name, and store tokens
  are a follow-up feature. *(Owner's decision, 2026-11-16.)*

## Clarifications

### Session 2026-11-18

- Q: Does the extension need a live finfocus process? → A: No. This
  feature runs the CLI per load and adds a result cache (FR-012a). A live
  `finfocus --web --no-browser` sidecar is a later, optional mode, not part
  of this feature. The owner is doing the core-side work that mode needs
  (reload endpoint, passphrase handling, a row shape the extension can
  read) in finfocus itself, separately from this spec. *(Owner's decision,
  2026-11-18; ISS-001.)*
- Q: What does SC-002 promise? → A: Two criteria. A cache hit appears
  within 2 s of activation. A cache-miss load on an idle machine appears
  within 20 s, which is a ceiling; the owner wants it much faster, and the
  measured 3 to 5 s is the working target. A slower load shows progress and
  ends complete or visibly incomplete. The "plugins already started once"
  qualifier is dropped. *(Owner's decision, 2026-11-18.)*
- Q: How does a partial result reach the user? → A: The extension detects
  unpriced rows itself, marks the total incomplete, retries once, and never
  caches such a result (FR-012b). A core completeness field is a later
  feature; the extension then prefers it. *(Planning refined this: the JSON
  cannot name a cause, so detection is by stability across two runs.)* *(Owner's decision, 2026-11-18.)*

## User Scenarios & Testing *(mandatory)*

For annotation acceptance, "demo project" means the planned Node.js
source-location demo with known priced fixture rows, defined in
[validation.md](validation.md). The existing YAML finfocus-demo remains a
dashboard-only check. Validation of Python, Go, and .NET uses the runtime
fixtures in that checklist.

### User Story 1 - See a resource's cost where it is declared (Priority: P1)

A developer opens a Pulumi program whose stack is deployed. Each resource
declaration in their own source files shows that resource's projected
monthly cost at the end of its line. Hovering it shows the month-to-date
actual cost and the delta, where finfocus reports them.

**Why this priority**: it is the reason the extension exists. Cost appears
at the moment and place a resource is written.

**Independent Test**: open a deployed demo project's program file and check
that every priced resource declared in it shows the same monthly figure that
`finfocus overview` prints for it.

**Acceptance Scenarios**:

1. **Given** a deployed stack and its program open in the editor, **When**
   the workspace finishes loading, **Then** each priced resource declared in
   an open project file shows its projected monthly cost at the end of its
   declaration line, using the figure finfocus displays.
2. **Given** a resource declared inside a reusable component in the user's
   project, **When** annotations render, **Then** the annotation appears at
   the line in the user's project that created it, not inside a third-party
   library.
3. **Given** a resource finfocus cannot price, **When** annotations render,
   **Then** that line shows no figure, or an unpriced marker, never `$0.00`
   presented as a price.
4. **Given** a resource with no recorded declaration location, **When**
   annotations render, **Then** no annotation is placed for it, and it still
   appears in the dashboard.

---

### User Story 2 - Know when a number is out of date (Priority: P1)

The developer changes `t3.large` to `t3.xlarge` and saves. The annotation on
that resource is marked stale at once, without waiting for pricing. The
dashboard says how many resources were edited since the last result.

**Why this priority**: without it, the deployed figure next to edited code
reads as the price of the edit, which is wrong.

**Independent Test**: edit and save a priced resource's declaration; the
stale marker appears immediately and no pricing run starts unless on-save
preview is enabled.

**Acceptance Scenarios**:

1. **Given** annotations from a result, **When** the user edits and saves a
   file that declares priced resources, **Then** the annotations of resources
   declared anywhere in that file are marked stale within one second, and no
   pricing run starts.
2. **Given** stale resources, **When** the dashboard is visible, **Then** it
   shows the count of stale resources and a Run preview control.
3. **Given** on-save preview is enabled, **When** the user saves a program
   file, **Then** a preview run starts, and a newer save supersedes a run
   still in progress: the older request is invalidated first, then
   cancelled, as the CLI invocation contract defines.

---

### User Story 3 - Price the edits on demand (Priority: P1)

The developer clicks Run preview. The editor shows that pricing is in
progress. When it finishes, edited and new resources show their new monthly
cost and their delta against the deployed cost, and stale markers clear.

**Why this priority**: it turns a stale figure into the price of the change,
which is the shift-left moment.

**Independent Test**: edit an instance size in the demo project, run the
preview command, and compare the new annotation and delta with
`finfocus overview` run with a preview in the terminal.

**Acceptance Scenarios**:

1. **Given** stale resources, **When** the user runs Run preview, **Then** a
   progress indicator shows until the run ends, and the editor stays usable.
2. **Given** a finished preview, **When** results arrive, **Then** updated
   and created resources show their projected cost and delta, deleted
   resources leave the dashboard's totals as finfocus reports them, and
   stale markers clear only for resources the preview covered whose files
   and stack config have no newer edits and no unsaved changes.
3. **Given** a preview that fails (missing credentials, wrong passphrase,
   program error), **When** it ends, **Then** the editor keeps the previous
   figures with their stale markers and shows the failure reason finfocus
   gave, with no secret values.
4. **Given** a stack whose secrets need a passphrase, **When** a run needs
   one that is not available, **Then** the user is asked for it in the
   editor, and an empty passphrase is accepted as a valid answer.

---

### User Story 4 - A small cost dashboard beside the code (Priority: P2)

A FinFocus side-bar container holds a summary card and, below it, a resource
tree. The card shows the stack name, projected monthly total, month-to-date
actual, budget status, the stale count, the five most expensive resources,
and Run preview. The tree lists every resource grouped by type, with an
unpriced group. Clicking a resource in either opens its declaration. A
status-bar item shows the monthly total and budget status, and clicking it
reveals the card.

**Why this priority**: the annotations answer "what does this resource
cost"; the dashboard answers "what does this stack cost, and am I over
budget".

**Independent Test**: open the demo project; the card's total, budget status
and top five match `finfocus overview --output json` for the same stack, and
the tree's resource count matches its resource list.

**Acceptance Scenarios**:

1. **Given** a loaded result, **When** the side bar is opened, **Then** the
   card and tree show figures identical to the finfocus result.
2. **Given** a configured budget, **When** the card renders, **Then** it
   shows the budget amount, the percent used, and the health status, with the
   status conveyed in text, not color alone.
3. **Given** a resource row in the card or tree, **When** the user activates
   it, **Then** the editor opens the file and line where it is declared, or
   says it has no recorded location.
4. **Given** a fork where the summary card fails to render, **When** the
   side bar opens, **Then** the tree and status-bar item still show the
   stack's figures.

---

### User Story 5 - Install in any VS Code fork (Priority: P2)

A developer using VS Code, Cursor, Windsurf, or VSCodium installs the same
extension package and gets the same behavior. Store publishing is a
follow-up; in this feature the package is installed from a file.

**Why this priority**: the request was explicitly for every VS Code
derivative, and forks differ in the editor API level they support.

**Independent Test**: install the release package from a file in each
supported editor and run User Story 1.

**Acceptance Scenarios**:

1. **Given** a release, **When** its build finishes, **Then** one extension
   package file is attached to it.
2. **Given** a supported editor, **When** that package is installed,
   **Then** User Stories 1 to 4 work without fork-specific settings.

---

### User Story 6 - Clear guidance when finfocus is not ready (Priority: P3)

A developer installs the extension on a machine without finfocus, without
plugins, or in a folder that is not a Pulumi project. The extension says
what is missing and how to fix it, and stays quiet in non-Pulumi folders.

**Why this priority**: first-run failures are where an extension loses its
users, but it is not needed for the core flow.

**Independent Test**: run with no finfocus on the path, with no plugins
installed, and in a non-Pulumi folder; each shows the matching guidance or
none.

**Acceptance Scenarios**:

1. **Given** no finfocus executable found, **When** a Pulumi workspace
   opens, **Then** the extension shows install guidance and a way to set the
   executable path, and makes no further attempts until settings change.
2. **Given** a finfocus older than the version the extension requires,
   **When** it starts, **Then** it says which version is needed.
3. **Given** a folder with no Pulumi project, **When** it opens, **Then**
   the extension shows nothing and runs nothing.

### Edge Cases

- A slow or failing plugin that fails the same resources on both runs looks
  like a type with no cost data: the result is shown as confirmed with its
  unpriced count, not incomplete. Only a core completeness field can tell
  these apart (FR-012b).
- A cache file that is unreadable, partly written, or fails to parse is
  deleted and treated as a miss; the extension never shows it.
- A changed finfocus path or version, a changed stack or project
  configuration file, a new stack update, or an update in progress is a miss.
  A multi-root workspace keeps one entry per project and stack.
- A workspace with several Pulumi projects, or a project with several
  stacks: the extension uses the project of the active file and the selected
  stack, and offers a way to switch stack.
- A file edited but not saved: resources in it are marked stale; no run
  starts from unsaved content.
- A declaration that moved since the result (lines inserted above it): the
  annotation follows the edit within the open document, and is placed by the
  recorded location again after the next result.
- One line declaring several resources (a loop): the annotation shows the
  count ("3 resources") and the hover lists each resource's figure. *(Author's
  call: no sum, so the extension does no money arithmetic.)*
- A resource whose recorded location points outside the workspace, such as
  a dependency's source: no annotation; the dashboard still lists it.
- Remote and containerized workspaces (WSL, SSH, dev containers): finfocus
  runs where the workspace lives.
- A run that is still in progress when another is requested: one run at a
  time per project and stack; the newer request supersedes the older.
- Plugin cold start: the first run can take tens of seconds; progress is
  shown and the editor stays responsive.
- Mixed currencies: the finfocus overview JSON refuses a stack whose rows
  carry different currencies. The extension shows that finfocus error and no
  figures, and never sums across currencies itself.
- A `Pulumi.yaml` that exists only under `node_modules` or another excluded
  directory can activate the extension, because activation globs cannot
  exclude directories. Discovery ignores it, so the workspace stays silent as
  in User Story 6.
- Untrusted workspaces: running finfocus executes the user's Pulumi program
  during a preview, so runs are disabled until the workspace is trusted.

## Requirements *(mandatory)*

### Functional Requirements

#### Core (finfocus CLI)

- **FR-001**: The finfocus overview JSON MUST include, for each resource
  that has one, the declaration location: a file path relative to the
  Pulumi project root, a 1-based line, and a 1-based column when the runtime records one.
  An unknown column MUST be omitted; navigation then opens the line.
- **FR-002**: The location MUST come from the location Pulumi records for
  the resource in stack state and in preview output. Where Pulumi records a
  call stack, finfocus MUST choose the innermost frame inside the project
  root, so a resource created by a component in the user's project points to
  the user's line.
- **FR-003**: A location MUST be omitted, not guessed, when Pulumi records
  none or when every frame lies outside the project root. Absolute paths
  outside the project MUST NOT appear in output.
- **FR-004**: Adding the location MUST NOT change any existing field, total,
  cache key, or exit code.
- **FR-005**: The finfocus overview JSON MUST carry display text beside the
  numbers: per resource (projected monthly, month-to-date actual, delta,
  status), for the summary (projected monthly, month-to-date actual, delta),
  and a top-level `budgetDisplay` (amount, percent used, health status,
  including per-budget details), using the existing web budget display shape.
  No new display object is required inside `budgets[]`. The text MUST come
  from the same formatting the overview table uses, so every surface shows
  identical strings. The new fields are additive; existing numeric fields
  keep their names and values. *(Owner's decision, 2026-11-16.)*
- **FR-006**: The JSON overview MUST report budgets configured in finfocus
  config when no plugin returns one, the same fallback the TUI and web
  dashboard already apply. This intentionally changes existing JSON output
  for stacks whose only budget is in config. *(Owner's decision,
  2026-11-16.)*

#### Extension

- **FR-010**: The extension MUST activate only in workspaces that contain a
  Pulumi project file.
- **FR-011**: The extension MUST obtain all figures by running the installed
  finfocus and reading its JSON overview; it MUST display finfocus's display
  strings and MUST NOT compute or reformat money.
- **FR-012**: On activation and on an explicit refresh, the extension MUST
  load the deployed-state result without running a preview.
- **FR-012a**: The extension MUST reuse the last deployed-state result
  instead of running finfocus when nothing it depends on changed: the
  stack's last-update time, the contents of `Pulumi.<stack>.yaml`, the
  finfocus version, and the state-only mode. A cached result expires after
  a time limit (default 60 minutes, and 0 turns the cache off), and an
  explicit refresh MUST bypass the cache. A cache
  entry holds only what finfocus printed and is displayed through the same
  safe paths as a fresh result (FR-011). The extension MUST NOT require a
  running finfocus process between loads.
- **FR-012b**: When a loaded result has unpriced rows (gaps) that finfocus
  does not explain (the JSON separates neither a failed price from a type
  with no cost data), the extension MUST decide by stability: it re-runs
  finfocus once when it has no confirmed baseline for the stack or the gaps
  differ from it. Gaps that two runs agree on are confirmed and noted as "N
  resources unpriced". Gaps that differ mean a slow or failing plugin:
  the extension MUST show an incomplete marker beside the total naming how
  many of how many resources are unpriced, MUST NOT show the total as
  complete, and MUST NOT write that result to the cache (FR-012a). It needs
  no core behavior change; a core completeness field (a later feature) takes
  precedence when present.
- **FR-013**: The extension MUST provide a Run preview action that loads a
  result including a preview, shows progress, and can be cancelled.
- **FR-014**: The extension MUST provide an on-save preview setting, off by
  default.
- **FR-015**: The extension MUST mark a resource stale, without running
  finfocus, when any of these happened after the result it is shown from:
  the file that declares it was edited (saved or unsaved), or the stack's
  Pulumi config file changed. *(Author's call: file granularity, no language
  parsers. It over-marks rather than misses an edit to a variable, a config
  value, or a helper in the same file. An edit inside a component's own file
  marks only the resources declared there.)*
- **FR-015a**: When a deployed-state result loads, the extension MUST mark a
  resource stale if its declaring file was modified after Pulumi last
  recorded a change to that resource, so work in progress from before the
  editor opened is not shown as current. *(Author's call: compares the file's
  modification time with the resource's recorded modification time; no git
  dependency.)*
- **FR-015b**: Core MUST expose the deployed resource's modification time as
  optional `resources[].modifiedAt` (RFC3339), carried explicitly through the
  overview pipeline. A missing or invalid time MUST NOT imply freshness:
  the extension marks that resource's deployed result `freshness unknown`.
  A missing or unreadable declaring file has the same treatment. A preview
  may establish freshness for the files it actually covered. Unknown
  freshness MUST be visible in the resource tree and included in the
  dashboard's stale/unknown count.
- **FR-015c**: A result MUST NOT clear stale marks caused by edits made after
  its request began, unsaved changes, or stack-config changes during its run.
  Refreshing deployed state MUST NOT clear work-in-progress marks merely
  because the refresh succeeded. Preview reads saved files only: when a
  project has dirty documents, manual preview MUST offer Save all and run,
  Run saved files, or Cancel; it MUST NOT silently save. On-save preview
  never saves other documents and leaves dirty resources stale.
- **FR-016**: Annotations MUST appear at the end of the declaration line and
  show the projected monthly display string; the hover MUST show the
  month-to-date actual, the delta, and the status, when finfocus reports
  them.
- **FR-017**: The side bar MUST contain a summary card and a resource tree
  as described in User Story 4, and a status-bar item MUST show the monthly
  total and budget status.
- **FR-018**: Activating a resource in the card or tree MUST open its
  declaration.
- **FR-019**: The extension MUST let the user choose the stack when the
  project has several, defaulting to the selected Pulumi stack. Discovery
  MUST support nested and multi-root projects, choose the nearest project
  containing the active file, and offer a project picker when ambiguous.
  The selection is session-local and MUST NOT change Pulumi's selected stack.
  Results, credentials, and requests MUST be isolated by project URI and
  stack reference. Switching either cancels the old run and rejects its late
  output; no result from another selection may be displayed.
- **FR-020**: The extension MUST ask for a stack passphrase when finfocus
  needs one, keep it only for the editor session, never write it to disk or
  logs, and accept an empty value. Dismissing the prompt MUST NOT retry, and
  the passphrase MUST be cleared when its project is removed or the extension
  deactivates.
- **FR-021**: Error messages MUST pass through what finfocus reports and
  MUST NOT add secret values, environment values, or full command lines.
- **FR-022**: The extension MUST NOT run finfocus in an untrusted workspace,
  and MUST NOT read or show a cached result there (FR-012a).
- **FR-023**: The extension MUST declare the minimum finfocus version it
  supports and report a clear message when the installed one is older.
- **FR-024**: Each release MUST build one installable extension package that
  works unchanged in VS Code, Cursor, Windsurf, and VSCodium, and that both
  the Visual Studio Marketplace and Open VSX would accept, under the
  publisher name `finfocus`. Publishing to the stores, the store accounts,
  and store tokens are a follow-up feature. *(Owner's decision, 2026-11-16.)*
- **FR-024a**: The extension MUST be versioned as its own release-please
  component, as the nested plugins are, with its own tag prefix and its path
  excluded from the root package. FR-023's minimum finfocus version is a
  separate declared value. *(Author's call.)*
- **FR-025**: The extension MUST run on the editor API level supported by
  current Cursor, Windsurf, and VSCodium releases, not only the newest VS
  Code. "Current" means the releases installed for the acceptance matrix:
  each editor's reported API level is recorded with its date in research R7,
  and the declared minimum (`engines.vscode`) MUST be at or below every one.
- **FR-026**: A repository test MUST fail when a JSON field the extension
  reads is renamed or removed from the finfocus overview output.
- **FR-027**: All text the extension shows MUST be reachable by keyboard and
  screen reader; status MUST NOT be conveyed by color alone. The resource
  tree MUST expose annotation and hover information through accessible labels
  and a keyboard command for resource details, including the stale reason.
- **FR-028**: Cancellation, project/stack switching, workspace removal, and
  extension deactivation MUST stop owned processes and invalidate pending
  results. Shutdown MUST have a bounded grace period and escalation on each
  supported OS, including descendant processes; a failed cleanup MUST report
  an error and block replacement runs for that context. Passphrase
  dismissal and clearing follow FR-020.

### Key Entities

- **Overview result**: one finfocus overview of one stack: resources,
  summary totals, budgets, and errors, with the time it was produced and
  whether it included a preview.
- **Cached result**: finfocus's output for one stack, stored with the
  stack's last-update time, the contents of its configuration, the finfocus
  version, and the time stored (FR-012a). It never holds a passphrase.
- **Completeness**: whether a result's unpriced rows are confirmed (two runs
  agree, or a baseline matches) or incomplete (they differ). Users see
  "unpriced" for a resource with no price, and "incomplete" only for an
  unconfirmed result (FR-012b).
- **Declaration location**: project-relative file, line, and column where a
  resource was declared.
- **Stale mark**: a resource whose declaration was edited after the result
  it is shown from, or whose freshness is unknown (FR-015b). The tree and the
  dashboard count both under "stale/unknown" and give the reason.

## Web UI Fit *(mandatory)*

- **Disposition**: Extends the Overview resource detail. The extension
  itself is an editor surface, not a CLI feature. The CLI-visible changes
  are the declaration location (FR-001), display text (FR-005), and the
  config budget fallback (FR-006) in the JSON overview. The web dashboard
  already has the last two, so only the location is new to it.
- **Web data**: the declaration location of the selected resource, as the
  CLI reports it.
- **Layout fit**: one "Declared at" row in the existing Overview detail
  panel, showing `path:line`. No new view, control, or table column.
- **Additions**: none.
- **Render evidence**: baseline Playwright renders of the Overview detail
  panel against a populated stack (61 resource rows, a priced EC2 instance
  opened), captured 2026-11-16:
  [desktop](validation/t001-detail-desktop-1440.png) and
  [narrow](validation/t001-detail-narrow-390.png). "Declared at" fits as a
  term/definition pair after Type in the existing definition list; long paths
  wrap (`break-all`), the row is omitted when there is no location, and it
  adds no tab stop. Notes: [validation.md](validation.md#baseline-render-evidence-t001-2026-11-16).
  Post-implementation renders (T024, 2026-11-17) of the built
  `v0.5.8-10-ga3d8d714-dirty` against the deployed `testdata/sourcepos/nodejs`
  stack: with a location
  ([desktop](validation/t024-location-desktop-1440.png),
  [narrow](validation/t024-location-narrow-390.png), a long path
  [narrow](validation/t024-location-long-path-narrow-390.png)) and without
  ([desktop](validation/t024-no-location-desktop-1440.png),
  [narrow](validation/t024-no-location-narrow-390.png)). The row reads
  `Declared at lib/box.js:14` and sits after Drift, not after Type as the
  baseline proposed. Notes:
  [validation.md](validation.md#post-implementation-render-evidence-t024-2026-11-17).
- **Follow-up issue scope**: one UX-only `web-ui` issue for the detail row's
  copy and placement. No engine, API, redaction, or data-shape work.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In the Node.js source-location demo fixture, every priced resource declared in the
  user's program files shows an annotation whose figure equals the CLI's
  figure for that resource (100% match).
- **SC-002**: After opening a deployed project, annotations and the
  dashboard appear within 2 seconds of activation when the result cache
  holds a current result (FR-012a). On a cache miss, with finfocus and its
  plugins installed and an idle machine, they appear within 20 seconds. The
  20 seconds is a ceiling, not the goal: measured idle loads take 3 to 5
  seconds (validation.md). A load that takes longer MUST show progress and
  end in a complete result or a visible incomplete state (FR-012b).
- **SC-003**: A stale marker appears within 1 second of saving an edit to a
  resource declaration, with no pricing run started.
- **SC-004**: The same release package installs in VS Code, Cursor,
  Windsurf, and VSCodium, and User Stories 1 to 4 pass in each.
- **SC-005**: Renaming any overview JSON field the extension reads fails the
  repository's test suite (measures FR-026).
- **SC-006**: Typing in the editor stays responsive while a preview runs: no
  pause longer than 100 ms attributable to the extension.

## Assumptions

- Pulumi records declaration locations for programs written with its
  Node.js, Python, Go, and .NET SDKs. Verified 2026-11-16 for Node.js and Go
  (Go records module paths and no column) and for preview output of created
  resources. Pulumi YAML records only frames inside its language host, so
  YAML programs, including the finfocus-demo project, get no annotations in
  this feature; the dashboard still works for them (see research R1).
  Python and .NET location support remains an unverified assumption, not a
  completed compatibility claim; recorded state and preview fixtures for
  both are required before the extension's first release (see
  validation.md). The core release may ship with them pending; its
  documentation then lists only the verified runtimes.
- finfocus and its plugins are installed by the user; the extension does not
  install plugins.
- One stack is shown at a time; cross-stack comparison is out of scope.
- Terraform, Kubernetes cluster allocation, recommendations, and the agent
  side are out of scope for this feature.
- Telemetry is out of scope; the extension sends nothing anywhere.
- A long-running finfocus process (`finfocus --web --no-browser`) as the
  data source is out of scope for this feature; it is a later mode that
  needs its own spec and contract (ISS-001). So is a core completeness
  field on `finfocus overview`.
