# Implementation Plan: Cost in the Editor for VS Code and Its Forks

**Branch**: `2017-vscode-cost-extension` | **Date**: 2026-11-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2017-vscode-cost-extension/spec.md`

## Summary

Add an editor extension, `packages/vscode-finfocus/`, that shows each Pulumi
resource's cost at its declaration line and a small side-bar dashboard (summary
card, resource tree, status-bar item) in VS Code, Cursor, Windsurf, and
VSCodium. The extension is a display layer: it runs the installed
`finfocus overview --output json` and shows the text finfocus formats.

Core gains additive changes to the JSON overview: each resource's
declaration location resolved from Pulumi's recorded `sourcePosition` and
`stackTrace` (new leaf package `internal/sourcepos`), display text for rows,
summary, and budgets, an optional deployed `modifiedAt` timestamp, and the
config budget fallback the TUI and web already use. The non-interactive overview also cancels cleanly on SIGINT/SIGTERM so the
extension can supersede a run without orphaning plugins.

## Technical Context

**Language/Version**: Go 1.27.4 (core); JavaScript (CommonJS, ES2022) with
JSDoc types checked by TypeScript `--checkJs` for the extension (R10)
**Primary Dependencies**: core: none new. Extension: no runtime dependencies;
dev only: `typescript`, `@types/vscode ~1.90.0`, `@types/node`,
`@vscode/vsce`, `@vscode/test-cli`, `@vscode/test-electron`
**Storage**: none. The passphrase lives in memory for the editor session only
(FR-020); nothing is written to disk.
**Testing**: Go testify; extension pure logic under `node --test` (no install
needed, like `packages/zerolog-viewer`); editor adapter under
`@vscode/test-cli` + `xvfb-run` in CI
**Target Platform**: minimum VS Code API `^1.90.0`, at or below every fork's
API level in research R7's dated table, which the release validation matrix
replaces with observed versions; Linux, macOS, Windows; local, WSL, SSH, dev containers
(`extensionKind: ["workspace"]`)
**Project Type**: Go CLI plus one new Node package in `packages/`
**Performance Goals**: SC-002 (results within 20 s after a warm plugin start),
SC-003 (stale mark within 1 s, no run), SC-006 (no editor pause over 100 ms)
**Constraints**: the extension computes and formats no money; it never runs
finfocus in an untrusted workspace; no absolute path outside the project in
output; no secret in logs or messages
**Scale/Scope**: one stack at a time; projects up to a few hundred resources

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: no plugin or spec change. Location,
  display, modification time, and budget fallback are generic core output; the extension is a
  consumer of the CLI like the GitHub Action.
- [x] **Test-Driven Development**: tests first for `internal/sourcepos`
  (recorded strings from research R1), the JSON renderer, and every
  extension logic module; 80% coverage on new Go packages, 95% on
  `sourcepos` (path-safety critical).
- [x] **Cross-Platform Compatibility**: path handling uses `filepath` with
  forward-slash output; cancellation differs on Windows and is documented
  (R4). Extension runs on all three OSes and remote hosts.
- [x] **Documentation Integrity**: new docs page
  `docs/src/content/docs/guides/editor-extension.md`, overview command page
  updated for `location`, `display`, `budgetDisplay`, and the budget
  fallback; package README; CLAUDE.md notes.
- [x] **Protocol Stability**: JSON changes are additive except FR-006, which
  the owner approved and the changelog states.
- [x] **Implementation Completeness**: no stubs; publishing is a separate
  follow-up feature, not a stub inside this one.
- [x] **Persistence Model**: no persistent state added.
- [x] **Web Parity**: planning gate passed 2026-11-16 with the populated
  Playwright render recorded in [validation.md](validation.md) (T001). Web UI
  Fit extends the Overview detail panel with "Declared at";
  [contracts/http-api.md](contracts/http-api.md) names the hook
  (`GET /api/overview/resource` already returns `engine.OverviewRowResult`,
  which gains `Location`, and `viewmodel.BuildOverviewDetailDisplay`, which
  gains `declaredAt`). The post-implementation render (T024) and the UX-only
  issue (T026) remain.
- [x] **Quality Gates**: `make lint` gains the extension type check and
  markdown; `make test` gains `node --test` for the extension.
- [x] **Multi-Repo Coordination**: none; no `finfocus-spec` change.

The Web Parity planning gate passed; its completion gates (T024, T026) remain open.

## Project Structure

### Documentation (this feature)

```text
specs/2017-vscode-cost-extension/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── validation.md            # scenarios, release matrix, pending render evidence
├── contracts/
│   ├── overview-json.md     # JSON additions the extension reads
│   ├── cli-invocation.md    # how the extension runs finfocus
│   └── http-api.md          # web hook for "Declared at"
└── tasks.md                 # /speckit.tasks
```

### Source Code (repository root)

```text
internal/sourcepos/                 # NEW leaf: parse + resolve Pulumi positions
├── sourcepos.go
└── sourcepos_test.go
internal/ingest/
├── state.go                        # StackExportResource: +SourcePosition, +StackTrace
└── pulumi_plan.go                  # PulumiState: +SourcePosition, +StackTrace
internal/stackload/convert.go       # resolve location; propagate deployed ModifiedAt
internal/pulumi/pulumi.go           # preview --show-sames; graceful stop of the Pulumi child
internal/pulumi/process_*.go        # OS-specific Pulumi child ownership (Windows job)
internal/engine/
├── overview_types.go               # StateResource, PlanStep, OverviewRow: +Location, +ModifiedAt
├── overview_merge.go               # copy Location into rows (state, plan, creates)
├── overview_result.go              # OverviewRowResult carries Location
└── overview_render.go              # JSON row wrapper: display + location; summary display
internal/cli/
├── overview.go                     # budgetFallback on JSON; signal cancel on plain path;
│                                   # compose budgetDisplay (viewmodel.BuildBudgetDisplay)
├── overview_json_contract_test.go  # FR-026: pins fields from the extension's list
└── overview_process_test.go        # descendant cleanup on cancel and forced kill
internal/viewmodel/overview_detail.go  # display.declaredAt for the web detail panel
internal/webui/static/views/overview.js  # "Declared at" row in the detail panel
internal/pluginhost/process_group_*.go   # descendant ownership (Windows Job Object)
testdata/sourcepos/                 # recorded state + preview per runtime (Node.js, Go, Python, .NET, YAML)
packages/vscode-finfocus/           # NEW extension package
├── package.json                    # manifest: publisher finfocus, views, commands, settings
├── jsconfig.json                   # checkJs settings
├── .vscodeignore
├── .vscode-test.js                 # @vscode/test-cli config (API 1.90 and current)
├── pnpm-lock.yaml                  # dev-only toolchain
├── LICENSE
├── README.md
├── contract/overview-fields.json   # fields the extension reads (FR-026 input)
├── src/
│   ├── extension.js                # activate: wiring only
│   ├── runner.js                   # spawn finfocus, args, env, session shutdown (no vscode import)
│   ├── annotations.js              # decoration projection                (no vscode import)
│   ├── dashboard.js                # card/tree projection, top five       (no vscode import)
│   ├── result.js                   # parse JSON / error envelope          (no vscode import)
│   ├── stale.js                    # stale tracking per file + mtime      (no vscode import)
│   ├── projects.js                 # discovery, context keys, selection (no vscode import)
│   ├── scheduler.js                # one run per stack, supersede, on-save, cache lookup (no vscode import)
│   ├── cache.js                    # cache key, TTL, entry validation       (no vscode import)
│   ├── completeness.js             # unpriced rows (gaps), stability rule    (no vscode import)
│   ├── version.js                  # --version parse + minimum check      (no vscode import)
│   └── editor/                     # vscode adapters
│       ├── annotations.js          # decorations + hover
│       ├── staleTracking.js        # document/file/config watchers
│       ├── preview.js              # Run preview, dirty-document choices
│       ├── readiness.js            # first-run prerequisite state
│       ├── summaryView.js          # WebviewViewProvider (card)
│       ├── treeView.js             # TreeDataProvider (all resources)
│       ├── cacheStore.js           # files under context.globalStorageUri, atomic write, 0600
│       ├── statusBar.js
│       └── passphrase.js           # input box, session memory only
├── media/summary.{html,css,js}     # card; CSP, no remote content
├── test/unit/*.test.js             # node --test
├── test/editor/*.test.js           # @vscode/test-cli
├── test/integration/shutdown.test.js  # real process-tree cleanup
└── test/fixtures/                  # projects, process-tree, performance
.github/workflows/test-vscode-extension.yml      # unit, type check, editor jobs per OS
.github/workflows/release-vscode-extension.yml   # build .vsix, attach to release
docs/src/content/docs/guides/editor-extension.md
```

**Structure Decision**: core changes stay in the existing overview pipeline,
with position resolution isolated in `internal/sourcepos` (imports only the
standard library). The extension follows `packages/zerolog-viewer`: plain
JavaScript, dependency-free at runtime, pure modules tested with `node --test`.
Everything that imports `vscode` sits under `src/editor/` so the logic can be
tested without an editor.

## Design Notes

### Location resolution (FR-001 to FR-003)

Resolved once per snapshot in `stackload` with the project root and runtime
from `Pulumi.yaml` or `Pulumi.yml` (and the module path from `go.mod` for Go), following the
rules in [research.md](research.md) R1. The engine never sees raw position
strings. `Location` is part of `OverviewRow` but not of any cache key: it is
not sent to plugins (FR-004).

### Display text (FR-005)

`engine.BuildOverviewJSONOutput` (split out of `RenderOverviewAsJSON`) builds
the document; rows embed `OverviewRow` and add `display` and `location` from
`OverviewRowResult`; the summary adds `display` from the same formatters. The
CLI adds top-level `budgetDisplay` from `viewmodel.BuildBudgetDisplay`, the
object the web dashboard already serves, so engine does not import viewmodel.

### Release (FR-024, FR-024a)

release-please package `packages/vscode-finfocus` (`release-type: node`,
component `vscode-finfocus`, `initial-version: 0.1.0`, `changelog-path:
CHANGELOG.md` written by release-please only, path added to the root
`exclude-paths`; not added to the manifest by hand). The new workflow builds
the `.vsix` on a `vscode-finfocus-v*` release and attaches it.
`release-monorepo-plugin.yml` must skip that prefix, because it builds every
non-`v` tag as a Go plugin.

### Extension behavior

- Activation: `workspaceContains:**/Pulumi.yaml` and
  `workspaceContains:**/Pulumi.yml`, with discovery excluding `node_modules`;
  runs disabled in untrusted workspaces
  (`capabilities.untrustedWorkspaces.supported: "limited"`).
- Load on activation and refresh: state-only run. Run preview: full run.
  On-save preview: setting `finfocus.previewOnSave`, default false.
- Discovery, selection, process isolation, and shutdown follow
  [contracts/cli-invocation.md](contracts/cli-invocation.md). Selection is
  session-local; no `pulumi stack select` call. All subprocesses, including
  discovery and version checks, are blocked in untrusted workspaces.
- Stale: file granularity plus stack settings (`.yaml` and `.yml`, short and
  qualified stack names as resolved by core). At load compare file mtime with
  the new `modifiedAt`; missing/invalid values mean freshness unknown.
  Per-file edit generations, dirty state, and request IDs prevent a completed
  run from clearing later edits; see [data-model.md](data-model.md).
- Manual preview offers Save all and run, Run saved files, or Cancel if
  documents in the project are dirty. Capture generations after any requested
  saves finish. On-save requests never save other documents.
- Annotations: one decoration type; `after` text is the row's
  `display.projected`; stale rows get a `⟳ stale` suffix and a dimmer theme
  color; several rows on one line show `N resources`.
- Result reuse and completeness: see the next section.
- Card: webview view in the primary side bar only (R7: Cursor renders blank in
  the panel and secondary side bar); strict CSP; text plus icon for status.

### Result reuse and completeness (FR-012a, FR-012b)

Decisions and alternatives are in research.md R12; entities in
[data-model.md](data-model.md); signals in the CLI contract.

- **Scope**: only deployed-state loads (activation, refresh) are cached. A
  preview depends on local, possibly unsaved files, so it is never read from
  or written to the cache.
- **Owner**: `Scheduler` asks a `cache` object (`get(key)`, `put(entry)`) before
  starting a state-only run, so supersede, cancel, and one-run-per-context
  rules stay in one place. `cache.js` is pure; `editor/cacheStore.js` does the
  file work under `context.globalStorageUri` (the extension host's side, so
  remote workspaces cache where finfocus runs). Workspace storage is not used:
  VS Code locks it per live extension host, so a second window or a lingering
  host gets a `-N` sibling folder and an empty cache (found in the web workbench
  run, validation.md).
- **Key**: ContextKey plus the stack's `lastUpdate` and `resourceCount` from
  discovery, the SHA-256 of `Pulumi.<stack>.yaml` (or `.yml`), the SHA-256 of
  the project's `.finfocus/config.hujson` when present, the finfocus version,
  and `stateOnly`. A stack with no `lastUpdate` or with `updateInProgress`
  true is never cached. Discovery runs on every refresh anyway
  (0.5 to 0.8 s measured), so the key costs nothing extra.
- **Expiry**: `finfocus.cache.ttlMinutes`, default 60, `0` turns the cache off.
  Month-to-date actuals move without a stack update; one hour keeps them
  honest without paying 4 s per reload. Explicit Refresh and a changed
  finfocus path skip the cache.
- **Content**: finfocus's stdout verbatim plus the key fields, `storedAt`, the
  original run's start time, and the confirmed gap set. The passphrase and
  environment are never stored. The file is created `0600`, written to a
  temporary name and renamed, and read back through `parseOverview`; an entry
  that fails to parse or validate is deleted and counts as a miss. A hit shows
  the result with its original `producedAt`, so stale comparison and edit
  marks behave as for a state-only refresh. Entries untouched for 30 days are
  removed at activation. On Windows the extension sets no ACL; the file sits
  in the per-user VS Code storage directory, which is the protection, and the
  guide says so. In an untrusted workspace the cache is neither read nor
  written: the key needs discovery, which is blocked, and the store is not
  opened.
- **A row is unpriced (a gap)** when its raw `status` is not `deleting` and its
  `display.projected` is the contract's `-` placeholder. The placeholder is
  pinned in `contract/overview-fields.json` and in a parser test, so a core
  change fails a test instead of silently breaking detection. No other display
  string is read.
- **Stability rule** (`completeness.js`): a cache-miss state-only result with
  unpriced rows is run once more when the context has no confirmed baseline,
  or when it has unpriced rows the baseline lacked. Equal sets across the
  two runs are confirmed gaps (types with no cost data): the result is
  complete, cacheable, and the card notes "N resources unpriced". Unequal
  sets mean a slow or failing plugin: the run with fewer gaps is kept, the card
  shows "incomplete: N of M resources unpriced" beside the total, the total is
  not presented as complete, and nothing is cached. Both runs are ordinary
  scheduler requests, so cancel and supersede apply. A result with no
  unpriced rows is complete and needs no retry.
- **Baseline**: the gaps the last complete or confirmed load of the context
  agreed on. It is kept in memory for the session and read from the last
  stored entry, even an expired one, on the first load; an `incomplete` result
  never replaces it.
- **Explained failures are separate**: a row whose price failed prints `ERR`
  and has an `errors[]` entry (pinned in `overview_json_contract_test.go`), so
  it is outside the placeholder rule; the tree already shows its error.
- **Cost of the rule**: nothing extra for a stack with no gaps or a confirmed
  baseline. A first load of a stack with gaps costs one extra run (about 4 s on
  an idle machine); later loads compare against the stored baseline.
- **Known limit**: a plugin that is slow on both runs, with the same rows
  failing each time, looks like a confirmed gap. The marker cannot catch that
  case until core reports completeness. The spec lists it under Edge Cases and
  the completeness tests pin it, so it is documented behavior, not a silent one.

### Modification times and preview coverage (FR-015a to FR-015c)

`StackExportResource.Modified` is parsed today, but
`stackload.convertStateResources` does not inject `pulumi:modified` into the
properties used by overview. Add `ModifiedAt *time.Time` to `StateResource`
and `OverviewRow`, copy it through merge and expose it as optional
`modifiedAt` in JSON. Preview merges preserve the deployed timestamp for
existing resources; creates omit it. It is display metadata and must not
change pricing inputs, cache keys, or plugin requests. Contract tests cover
present, absent, and invalid timestamp handling by the consumer.

Add `--show-sames` to the core Pulumi preview invocation so unchanged
resources receive locations from current source. Test unchanged resources
with lines inserted above them, updates, creates, deletes, and missing
locations. A preview step with no resolvable current location must not fall
back to a potentially wrong deployed annotation; keep the resource in the
tree without an annotation. Deletes retain their deployed location.

### Verification and implementation order

Use [validation.md](validation.md) as the acceptance checklist. First capture
and inspect the planning Playwright render. Then implement core timestamp,
location, preview coverage, and JSON contract tests before the consumer.
Implement context isolation, edit generations, and process cleanup before
wiring editor views. Finish with the packaged editor/OS matrix, accessibility
and performance evidence, docs, and the UX-only web follow-up issue.
[tasks.md](tasks.md) covers every row of that checklist, including the handler
contract, render check, docs updates, and issue filing. Pending evidence is
not a waived constitution gate.

### Delivery

Two stacked pull requests. Part A carries every core change (location,
display text, `modifiedAt`, budget fallback, `--show-sames`, cancellation and
descendant ownership, Web Parity) and is released first. Part B adds the
extension. release-please versions the two separately, so the extension's
`minFinfocusVersion` can name only a core release that already exists.
Python and .NET location fixtures may be pending at the core release (the
docs then list only verified runtimes) but block the extension's first
release.

### Descendant ownership today

On Unix each plugin runs in its own process group
(`internal/pluginhost/process_group_unix.go`), so a group kill aimed at
finfocus does not reach it; Linux adds `Pdeathsig`, which covers direct
children only, and macOS has neither. On Windows `configureProcessGroup` is a
no-op and `killProcessGroup` kills only the main process: there is no Job
Object yet. The Pulumi child is a bare `exec.CommandContext`.

Decision (research R4): on Unix the extension starts finfocus as a session
leader and sweeps the whole session on escalation and after every exit, which
reaches plugins in their own groups without a core change. Core's Unix share
is graceful cancellation: close plugin groups, stop the Pulumi child with a
signal and `WaitDelay`, and never `setsid` a child. On Windows core adds a
kill-on-close Job Object that every plugin and the Pulumi child join, because
Node cannot create one.

## Discovered Constraints

- **ISS-001 (2026-11-17, resolved by clarification 2026-11-18)**: the first
  reading (37 s, 1 GB for 65 resources) was a contention outlier. An idle
  `overview --state-only` takes 3.2 to 3.9 s, but the extension re-runs it on
  every activation, trust grant, configuration change, and refresh with no
  reuse, and a slow plugin start under load returned a silent partial total
  ($32.58 against $173.61). The design adds result reuse (FR-012a) and an
  incomplete-result rule (FR-012b); a live finfocus process is a later mode.
  See "Result reuse and completeness" under Design Notes and research.md R12.
- **Contract gap found while planning FR-012b**: the JSON gives no way to tell
  a failed price from a type with no cost data. In the partial run `errors[]`
  was empty, and both cases print `-` in `display.projected`. The extension
  therefore decides by stability across runs, not by reading a cause. A core
  completeness field would make that exact; it is the later feature named in
  the spec.

## Complexity Tracking

No architectural exception requested.
