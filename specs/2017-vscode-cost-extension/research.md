# Research: Cost in the Editor for VS Code and Its Forks

All findings below were checked on 2026-11-16 unless marked otherwise.

## R1. What Pulumi records as a resource's declaration location

**Method**: throwaway local-backend stacks (`PULUMI_BACKEND_URL=file://...`,
Pulumi v3.272.0) using `command:local:Command` and component resources, so no
cloud credentials were involved. Each was deployed, then `pulumi stack export`
and `pulumi preview --json` were read.

**Findings**:

| Runtime | `sourcePosition` example | Usable? |
| --- | --- | --- |
| Node.js | `project:///index.js#4,1`; a resource made inside a helper: `project:///lib/box.js#8,5` with a stack trace `lib/box.js#8,5`, `index.js#4,1` | Yes, project-relative with line and column |
| Go | `project:///../../../../../../../srcposgo/main.go#14` | Only after stripping the leading `../` run and the `go.mod` module path (`srcposgo/`): Go programs are built with `-trimpath`, so frames are module paths, and there is no column |
| YAML | `project:///../../../../../../../home/runner/work/pulumi-yaml/pulumi-yaml/pkg/server/server.go#316` | No: every frame is inside the YAML language host, never `Pulumi.yaml` |
| Python, .NET | not tested (SDK not installed locally) | Assumed like Node.js (absolute file paths made project-relative by the engine); verify in the integration fixtures |

- SDK frames outside the project appear with absolute paths under the home
  directory after the `../` run (for example
  `/home/<user>/.npm-global/lib/node_modules/@pulumi/runtime/stack.ts`). They
  must never reach output (FR-003).
- Node.js stack-trace entries for SDK frames can have no `sourcePosition`
  (`null`); skip them.
- `pulumi preview --json` carries `sourcePosition` and `stackTrace` in
  `newState` for `create` steps and, with `--show-sames`, for unchanged ones.
  Add `--show-sames` to the core preview runner (currently `preview --json`)
  and test moved-but-unchanged declarations. This is required for current
  locations, not merely a fixture-generation option.
- State resources carry `modified` (RFC3339), already parsed as
  `StackExportResource.Modified` (`internal/ingest/state.go`). FR-015a needs
  it, but overview's `stackload.convertStateResources` does
  not currently preserve it. Add an explicit `ModifiedAt` field through
  state conversion, merge, and JSON; do not assume `pulumi:modified` exists
  in the overview properties. Missing/invalid values mean freshness unknown.

**Decision**: resolve locations in a new leaf package `internal/sourcepos`:

1. Parse `project:///<path>#<line>[,<col>]`. Reject other schemes.
2. Clean the path. If it stays inside the project root, keep it, unless a segment
   is `node_modules` or `site-packages`: those are dependencies and the SDK
   (recorded as in-root paths), never the user's declaration.
3. Otherwise, for a `go` runtime only, drop the leading `..` segments; if the
   rest starts with the module path from the project's `go.mod`, keep the
   remainder.
4. Walk `stackTrace` from the innermost frame and take the first frame kept
   by rules 2 and 3; fall back to `sourcePosition` itself.
5. Keep only paths that name an existing regular file under the project
   root, without following a symlink out of the root.
6. Otherwise, no location.

**Known limit**: a resource class the user defines (`class Box extends
Resource`) records its own constructor line as the innermost frame, so all
instances share it. The rule does not guess past it (FR-003); a helper function
or a direct `new` resolves to the line that made the resource.

**Rationale**: the Node.js trace for `viaHelper-inner` shows that the innermost
in-project frame is the helper line (`lib/box.js#8`), which is where the user
wrote the resource. The Go rule is runtime knowledge of Pulumi, not plugin
domain knowledge, so it belongs in core ingest.

**Alternatives considered**:

- Searching source text for the logical name: breaks on loops, helper
  functions, and computed names. Rejected.
- Treating YAML specially by reading the `resources:` keys of `Pulumi.yaml`:
  exact for YAML, but the spec scoped YAML out, and it is a parser for one
  runtime. Recorded as a follow-up (see R8), not in this feature.

## R2. Display text in the JSON overview (FR-005)

**Finding**: `RenderOverviewAsJSON` (`internal/engine/overview_render.go`)
marshals each row's `Source OverviewRow`, which holds numbers only. The display
strings (`ActualDisplay`, `ProjectedDisplay`, `DeltaDisplay`, `StatusDisplay`)
already exist on `OverviewRowResult` (`overview_result.go`), computed once by
`ComputeOverviewResult` for every renderer. The summary totals are
`OverviewTotals`. A stack with mixed currencies returns `ErrMixedCurrencies`
and prints no JSON.

**Decision**: the JSON renderer wraps each row in a JSON-only type that embeds
`OverviewRow` and adds `display` (and `location`, R1) taken from the matching
`OverviewRowResult`. The summary gains a `display` object and the CLI adds top-level
`budgetDisplay` with per-budget details, built
from the same formatters (`FormatOverviewCurrency`, `FormatOverviewDelta`, and
the budget percent and health text the TUI already shows). No numeric field
changes name or value. Mixed currencies stay an error; the extension shows it.

**Rationale**: one formatter set, used by the table, TUI, web, and now JSON,
keeps every surface identical (the web's "browser formats nothing" rule).

**Alternatives considered**: adding `display` to `OverviewRow` itself; rejected
because `OverviewRow` also feeds NDJSON, the web session, and the cache, and
the display depends on `dayOfMonth`, which only `ComputeOverviewResult` knows.

## R3. Config budget fallback in JSON (FR-006)

**Finding**: the plain path sets `wantBudget: params.output == outputFormatJSON`
without `budgetFallback` (`internal/cli/overview.go`, comment "reports plugin
budgets only"). The TUI and web set `budgetFallback: true`
(`overview_pipeline.go`).

**Decision**: set `budgetFallback` to the same condition as `wantBudget` on
the plain path. No other path changes.

## R4. Cancelling a run (FR-013)

**Finding**: plain `overview` installs no signal handler, so SIGTERM kills it
without closing plugins. The aws-public router's per-region child can then
outlive it (CLAUDE.md, E2E notes). `mcp-server` already uses
`cancelOnShutdownSignal`.

**Decision**: the non-interactive `overview` path cancels its context on
SIGINT and SIGTERM through the existing `cancelOnShutdownSignal`, installed
for that path only (the TUI and `--web` keep their own handlers). The
extension uses bounded process-tree cleanup on every supported OS, as defined
in [contracts/cli-invocation.md](contracts/cli-invocation.md). Windows has no
Job Object today (`internal/pluginhost/process_group_windows.go` is a no-op),
and Unix plugins run in their own process groups, out of reach of a group
kill aimed at finfocus. Implementation adds descendant ownership on all three
OSes and proves cleanup in OS integration tests.

**Decision (forced termination)**:

- **Unix**: the extension starts finfocus as a session leader (Node `spawn`
  with `detached: true` calls `setsid`). Plugins call `setpgid`, which keeps
  them in that session, so the session, not the process group, is the unit the
  extension owns. Escalation sends SIGKILL to every process in the session,
  and confirmation waits until the session is empty. Membership comes from
  `/proc/<pid>/stat` (field 6, `session`, counted after the last `)` because
  the command name can contain spaces) on Linux, which needs no external
  tool, so minimal dev containers without procps work. macOS has no
  readable session id: `pgrep -s <sid>` returned no process, not even the
  session leader, on a GitHub macOS runner (ISS-003). The runner therefore
  sets a random `FINFOCUS_EXTENSION_RUN` token in finfocus's environment, which
  survives `setpgid` and re-parenting, and the members are the processes whose
  environment carries it (`/bin/ps -axEww`, same-user only). A `ps` that fails
  leaves the stop unconfirmed rather than empty. Confirmation runs after every exit, graceful or not, so a
  descendant that a clean exit left behind is swept too. Core needs no new
  forced-termination mechanism on Unix. Its share is graceful cancellation
  (close plugin groups, stop the Pulumi child with a signal and `WaitDelay`
  rather than an immediate kill) and never calling `setsid` for a child.
- **Windows**: Node cannot create a Job Object without a native addon, so core
  owns one: every plugin and the Pulumi child join a kill-on-close job that
  finfocus holds, and the job's last handle closing (finfocus exiting for any
  reason) kills them. The extension then terminates the finfocus tree with
  `taskkill /PID <pid> /T /F`.
- **Limit**: a descendant that calls `setsid` (Unix) or breaks away from the
  job (Windows) escapes. CLAUDE.md already assigns that case to the plugin.

**Alternatives considered**: a pipe that each plugin watches for EOF, or
kqueue `NOTE_EXIT` on macOS; both need a plugin SDK change in another
repository. A watchdog process in core; one more process to own and to kill. Graceful shutdown has a 5-second deadline and forced cleanup a further
2-second confirmation deadline; a failure blocks replacement runs.

**Alternatives considered**: leaving cancellation to process kill; rejected
because of orphaned plugin children.

## R5. Passphrase and error reporting (FR-020, FR-021)

**Finding**: with `--format json`, failures print an ax-go envelope on **stderr** and nothing on stdout
(verified 2026-11-16 against the built binary; exit code 1):
`{"error_code":"internal_error","message":"... (read_failed)","version":...}`.
`stackload` appends its category in parentheses, so a missing passphrase
reads `(passphrase_required)`. The envelope carries the finfocus `version`.
`--yes` skips the plain-path confirmation prompt.

**Decision**: the extension runs
`finfocus overview --output json --format json --yes [--state-only] --stack S`
with `PULUMI_CONFIG_PASSPHRASE` in the child environment only when the user
gave one. It recognizes a passphrase failure by the `passphrase_required`
category token, and a Go contract test pins that token and the envelope
fields it reads. Messages are shown as finfocus wrote them.

**Alternatives considered**: mapping `passphrase_required` to ax `ExitAuth`
(exit 4); cleaner, but it changes exit codes for every overview caller, which
the spec rules out (FR-004). Left for a later change.

## R6. Finfocus version check (FR-023)

**Finding**: there is no `version` subcommand; `finfocus --version` prints the
version, and every ax-go envelope includes `version`.

**Decision**: the extension runs `finfocus --version` once per session, parses
the semver, and compares it with `minFinfocusVersion` in its `package.json`
(set to the first release that contains FR-001, FR-005, and FR-006). A `dev`
or unparsable version is accepted with a warning in the output channel.

## R7. Editor and fork compatibility (FR-024, FR-025)

**Findings** (web research; sources in the PR description):

- Reported VS Code API levels, as found by research (not yet observed on
  installed editors):

  | Editor | API level | Source date | Observed in matrix |
  | --- | --- | --- | --- |
  | Cursor | 1.105.1 | mid-2026 | Pending (T073) |
  | Windsurf | 1.98 | 2025 | Pending (T073) |
  | VSCodium | 1.143 | October 2026 | Pending (T073) |
  | VS Code (minimum) | 1.90 | declared | Pending (T072) |

  The Windsurf figure is the oldest and least current; T073 replaces every
  row with the version each installed editor reports (Help > About), and the
  `engines.vscode` floor must stay at or below all of them (FR-025).
- Every API the extension needs is far older than 1.90: end-of-line
  decorations (`after`), `HoverProvider`, `StatusBarItem`, `TreeDataProvider`
  in a view container (1.23), `WebviewViewProvider` (1.50), `SecretStorage`
  (1.53), workspace trust (1.57), `extensionKind`.
- Cursor renders webviews blank in the bottom panel and the secondary side
  bar; the primary side bar works. This confirms the tree fallback.
- `@vscode/vsce package` builds a `.vsix` with no account; it needs a valid
  `publisher` string, `license`, and `repository`. Open VSX accepts the same
  file.

**Decision**: `engines.vscode: ^1.90.0`, `@types/vscode` pinned to `~1.90.0` so
the compiler rejects newer APIs. Views go in an activity-bar container in the
primary side bar only. `extensionKind: ["workspace"]` so finfocus runs where
the workspace lives (WSL, SSH, dev containers). The manifest's `publisher` is
`finfocus` (owner's decision, 2026-11-16), so the extension ID
`finfocus.vscode-finfocus` is stable from the first package; the follow-up
publishing feature creates store accounts that claim that name.

## R7a. Release tags

**Finding**: the tag `vscode-finfocus-v0.1.0` starts with `v`.
`release-monorepo-plugin.yml` builds only tags that do not start with `v`, so
it already skips this one. The workflows that treat every `v`-prefixed tag as
a core release do not: `goreleaser.yml`, `nightly.yml` (release-triggered
jobs) and `docker.yml` (`v*` tag pattern) would run for it.

**Decision**: those three exclude the `vscode-finfocus-` prefix (docker's
pattern becomes `v[0-9]*`); a new `release-vscode-extension.yml` handles the
extension tag. `release-monorepo-plugin.yml` needs no change.

## R10. Extension language

**Decision**: plain JavaScript (CommonJS) with JSDoc types, type-checked by
`tsc --noEmit --checkJs` in `make lint`.

**Rationale**: no build step, so `make test` runs the unit tests with
`node --test` and no `pnpm install`, as it does for `packages/zerolog-viewer`.
CommonJS because extension hosts at API 1.90 do not load ESM extensions.

**Alternatives considered**: TypeScript compiled with `tsc` or bundled with
esbuild; same type safety, but `make test` would need an install and a
compile step before every run.

## R8. Follow-ups identified

- YAML runtime locations from `Pulumi.yaml` `resources:` keys (R1).
- Store publishing, publisher identity, tokens (spec).
- `passphrase_required` as a distinct exit code (R5).
- Terraform annotations (spec).

## R9. Test approach

- Go: table tests for `internal/sourcepos` using the recorded Node.js, Go,
  and YAML strings above; renderer tests for `display`, `location`, and the
  budget fallback; a contract test that runs the real JSON renderer and
  pins every field the extension reads (FR-026), reading the list from
  `packages/vscode-finfocus/contract/overview-fields.json`.
- Extension: pure logic (result parsing, stale tracking, run scheduling,
  argument building) lives in modules that never import `vscode` and runs
  under `node --test`. A thin adapter layer is covered by `@vscode/test-cli`
  with `@vscode/test-electron` under `xvfb-run` in CI.
- Integration: a recorded fixture (state plus preview JSON from a Node.js
  program with a helper component) under `testdata/` drives the CLI test of
  FR-001 to FR-003 without Pulumi installed.

## Review decisions (2026-11-16)

- The source-location demo for annotations is the planned Node.js fixture at
  `testdata/sourcepos/nodejs`, including source files and recorded exports.
  The existing YAML finfocus-demo is dashboard-only evidence.
- Node.js and Go observations above are research evidence; Python and .NET
  remain unverified. Capture reproducible state/preview fixtures for each
  before claiming their location support. Missing SDKs are a pending gate,
  not evidence of compatibility.
- Stale tracking uses per-file generations and dirty state captured at request
  start, not completion time alone. State-only refresh does not prove local
  changes were priced. See data-model.md for exact clearing rules.
- Discovery invokes Pulumi only for stack identities. Cost and display text
  still come exclusively from finfocus. Selection stays in session memory
  and is isolated by project URI plus discovered stack reference.
- The planning Playwright render has not been captured. Web Parity stays
  unchecked until the populated render, inspection notes, and artifact paths
  are recorded in validation.md. Release acceptance is a separate gate.

## R11. Open Questions: load cost and a live process (ISS-001)

### Measurements (2026-11-17, deployed `initial-30` `dev`, 61 rows)

A first run of 36.8 s wall and 1 GB was taken under heavy machine load and is
discarded. On an idle machine:

| Path | Cost |
| --- | --- |
| `pulumi stack export --stack dev` | 1.1 s |
| `finfocus overview --state-only` (CLI, 4 runs) | 3.2-3.9 s wall, ~1.1 s CPU, ~100 MB; 5.1 s with the extension host's environment |
| `finfocus --web --no-browser`: listening | 0.22 s |
| same: ready after the passphrase answer (empty) | 2.1 s |
| same: a warm `POST /api/overview/query` | 0.01 s |
| same: resident, finfocus plus plugins | about 2.2 GB (two `aws-public` region children at about 1 GB each, plus civo and scorer) |

Observations:

- The web server stops at `passphraseRequired` where the CLI run did not, so a
  sidecar needs the extension's passphrase path (R5) wired to
  `POST /api/passphrase`.
- Its rows carry `location` (9 of 61, the same as the CLI) and
  `projectedDisplay`, `actualDisplay`, `deltaDisplay`, and `statusDisplay`,
  not the CLI JSON's `display` object, so the extension would need a second
  parser and a contract for that shape. No `modifiedAt` was seen.
- A session loads once. The page documents restarting the session to load
  fresh actual costs, and no endpoint was found that reloads the stack, so a
  sidecar answers quickly from data that goes stale, and picking up a change
  means a restart (which costs the same plugin start again) or new core work.
- Under load a slow plugin start returned a smaller total with 60 rows unpriced
  and no error (see validation.md, Finding 1b).

1. **Where does the time go?** Split one run into `pulumi stack export`, plugin
   launch (aws-public and its per-region children, civo, scorer), and pricing
   of 61 rows. The second consecutive run was not faster, so a cold disk cache
   is not the cause, and 11.8 s of system time points at process start and
   binary loading. A debug log (`FINFOCUS_LOG_LEVEL=debug`) timestamps each
   phase.
2. **Result cache in the extension.** Store finfocus's JSON verbatim, keyed by
   the stack's `lastUpdate` (already returned by the stack listing the extension
   runs), a hash of `Pulumi.<stack>.yaml`, the finfocus version, and the
   state-only flag. Month-to-date actuals and prices move without a stack
   update, so it needs a time limit, and the explicit Refresh command must
   bypass it. Which signals are enough, and what limit keeps the figures honest?
3. **A live finfocus.** Existing long-lived entry points, none yet checked for
   fit:
   - `finfocus --web --no-browser`: holds the plugin set and a session, with an
     overview event stream and a loopback token. Its payload is shaped for the
     page, a declaration location is exposed only in the detail view, and the
     extension would then depend on the web API rather than the CLI JSON
     contract (Part A). It would also have to own a long-lived server under the
     R4 process-ownership rules.
   - `finfocus mcp-server` (stdio): persistent, but one mutex serializes calls
     and the root command is re-executed per call, so whether plugins are kept
     open across calls is unverified.
   - A dedicated serve mode in core: a new command surface, which belongs to
     core's spec and constitution, not to this extension.
   Which one, if any, keeps the plugin set open between requests, and what does
   a request cost once it is warm?
4. **Core-side reduction.** If plugin start dominates, reusing launched plugins
   across overview runs, or skipping plugins for rows with no price, helps the
   CLI and the web UI too. Is that a separate core issue?
5. **Stale noise.** A deployed-state load marks every row "freshness unknown"
   (no `modifiedAt`), and the summary says so. Should unknown freshness show at
   the row level at all?

Constraints for any answer: the extension formats no money (FR-011); resource
text stays untrusted; no secret reaches the extension host beyond the
passphrase path in R5; plugin and process ownership follow R4.

## R12. Decisions for ISS-001 (2026-11-18)

- **Decision**: reuse deployed-state results through an extension-side cache
  (FR-012a); no live process in this feature; decide completeness by stability
  across runs (FR-012b).
- **Rationale**: an idle run costs 3.2 to 3.9 s, discovery 0.5 to 0.8 s, so a
  hit fits the 2 s criterion and a miss fits 20 s. The cache keeps FR-011 (it
  stores finfocus's output verbatim) and the one parser and contract.
- **Alternatives considered**: the `--web` sidecar (about 2.2 GB resident,
  different row shape, passphrase endpoint, no reload; owner is doing the core
  work, a later mode); `mcp-server` (one mutex, root re-executed per call,
  plugin reuse unverified); the cost-history DB (no location or display
  strings); a threshold on gap count (a magic number that fails small stacks);
  parsing money to compare totals (forbidden by FR-011).
- **Answers to R11 questions**: (2) signals are `lastUpdate`, `resourceCount`,
  the two config hashes, version, and mode, with a 60-minute default TTL and
  an explicit bypass. (3) a sidecar is deferred. (4) plugin reuse in core is
  separate core work. (5) "freshness unknown" noise is not part of this change
  and stays an open polish item.
- **Contract gap**: the JSON cannot separate failed prices from types with no
  cost data. FR-012b was reworded to match what the extension can observe.
