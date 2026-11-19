# Validation: VS Code Cost Extension

**Status**: Planning Web Parity gate recorded (2026-11-16); no product
execution evidence has been recorded. Unchecked gates remain pending. Record exact versions, commands,
fixture revision, results, and artifact paths when each gate is executed.

## Planning Web Parity gate

- [x] Run the existing web Overview against a populated stack; open resource
  details with Playwright and capture desktop and narrow viewport renders.
- [x] Inspect the complete detail panel and record where the proposed
  "Declared at" row fits, including long paths, absent locations, and keyboard
  focus order. A baseline render checks layout fit; it does not prove the
  unimplemented row works.
- [x] Store screenshots under this feature's `validation/` directory and add
  their paths and inspection notes here and in spec.md. Only then mark the
  planning Web Parity gate in plan.md complete.

### Baseline render evidence (T001, 2026-11-16)

- **Build**: `make build` at `a3d8d714` plus uncommitted spec changes
  (`v0.5.8-10-ga3d8d714-dirty`); no product code changed.
- **Stack**: the owner's reference Pulumi Cloud stack `dev`, read from live
  state with no deploy or spend, opened alone:
  `PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port 8771 --stack dev`.
  The Overview listed 61 resource rows. The panel inspected is an
  `aws:ec2/instance:Instance` created inside a component, priced
  (Actual MTD `$4.39`, Projected `$8.39`), with recommendations, a four-row
  blast radius, and an empty cost history.
- **Screenshots**: [desktop, 1440×900](validation/t001-detail-desktop-1440.png)
  and [narrow, 390×844](validation/t001-detail-narrow-390.png), each the
  `section.detail-panel` element only.
- **Cleanup**: browser closed, server stopped with SIGINT (exit 0), and no
  `finfocus-plugin*` process remained.

Inspection notes:

- **Placement**: the panel opens with a definition list (`dl`, CSS grid):
  URN, Type, Status, Actual MTD, Projected, Delta, Drift. "Declared at" fits
  as one more term/definition pair after Type, keeping the identity rows
  together ahead of the cost rows. No new section, control, or column.
- **Long paths**: definitions use `word-break: break-all`. At 1440 px the
  value column is about 1224 px wide and the full URN fits on one line; at
  390 px the list stacks each term above its value and the URN wraps across
  three lines without horizontal overflow. A long `file:line` wraps the same
  way.
- **Absent location**: the list has no placeholder rows today (Delta and
  Drift print `-` because the overview prints `-`). With no location the row
  is omitted, matching contracts/http-api.md, rather than showing `-`, so a
  missing location is not read as a value.
- **Focus order**: opening a row moves focus to the panel heading (`h3`,
  active element). The panel's tab stops are Close detail, the blast-radius
  table region, then its four resource buttons. The definition list has no
  focusable element, and a plain-text "Declared at" row adds none, so the
  order is unchanged.

## Core and contract coverage

| Gate | Required cases and evidence |
| --- | --- |
| Location safety | In-project/third-party frames, no location, invalid schemes, traversal, symlink escape, Windows paths, missing files, absent column |
| Runtime fixtures | Node.js helper/component and loop; Go module paths; Python and .NET state plus preview; YAML no-location behavior |
| Preview coverage | Add `--show-sames`; inserting lines above an unchanged resource moves its location; creates/updates/replacements use preview location; deletes retain state location |
| Modification time | State `Modified` reaches `resources[].modifiedAt`; present/absent values, creates without timestamp, no change to pricing/cache/plugin inputs |
| JSON consumer | Real renderer plus consumer field inventory, names/types/optional fields, state-only/preview metadata, display parity, no/config/plugin budgets, row errors, passphrase category |
| Regression gates | `make lint`, `make test`, Go 80% overall and 95% critical paths; demonstrate existing totals, numeric fields, cache keys, and exit codes remain unchanged except approved budget fallback |

Runtime evidence must include source files, SDK/Pulumi versions, sanitized
state and preview JSON, and expected path/line/column. Use
`testdata/sourcepos/nodejs` as the annotation demo with known priced fixture
rows; local-backend `command:local:Command` alone proves locations, not cloud
pricing. Pricing acceptance needs a deterministic test plugin or configured
pricing plugin with documented expected figures. Keep YAML finfocus-demo for
dashboard-only checks. Python/.NET fixture capture is mandatory before the
extension's first release; the core release may ship with them pending, and
its docs then list only verified runtimes. If their SDKs are unavailable,
record pending rather than marking support passed.

### Runtime fixture status (recorded 2026-11-17)

| Runtime | State | Evidence |
| --- | --- | --- |
| Node.js | Captured | `testdata/sourcepos/nodejs`, Pulumi v3.272.0, `@pulumi/pulumi` 3.276.0 |
| Go | Captured | `testdata/sourcepos/go`, `-trimpath` binary, no column |
| Python | Captured | `testdata/sourcepos/python`, Python 3.14, no column |
| YAML | Captured | `testdata/sourcepos/yaml`, no location by design |
| .NET | Pending | No .NET SDK on the capture machine; blocks the extension's first release (T072), not the core release |

## Freshness and selection tests

- [ ] Cold open: older/newer/equal file mtime versus `modifiedAt`, missing or
  malformed timestamps, unreadable files, and dirty documents at activation.
  Unknown freshness is visible in the tree and summary stale/unknown count.
- [ ] Edits anywhere in a declaring file mark all its resources within 1 s;
  stack config changes invalidate all rows, including `.yml` and qualified
  stack references. External edits, rename, and deletion follow the same rules.
- [ ] Edit/save while preview is running, then complete the old run: newer
  marks survive. Newer saves supersede older runs; late results/prompts never
  publish. State-only refresh cannot clear unpriced local edits.
- [ ] Manual preview: Save all and run, Run saved files, Cancel, failed save,
  and edits during saves. On-save never saves other dirty documents.
- [ ] Two workspace roots with stacks both named `dev`, nested projects,
  ambiguous active file, no stacks, missing Pulumi, malformed discovery,
  removed project, backend change, external selected-stack change, and
  switching context while a run or passphrase prompt is pending.
- [ ] Selection remains session-local; no `pulumi stack select` invocation.
  Passphrases never cross contexts, empty values work, dismissal does not
  retry, and teardown clears credentials. No processes run before trust.

## Shutdown integration tests

On Linux, macOS, and Windows, start a real process fixture with a child and
grandchild, including a child that ignores graceful termination where the OS
supports it. Record PIDs and verify they all exit after cancellation,
supersession, selection change, workspace removal, and deactivation. Exercise
real finfocus plus Pulumi/plugin ownership as well as the process fixture.
Require the 5 s grace plus 2 s forced-cleanup confirmation deadline, no late
publication, and no replacement after failed cleanup. Test Windows Job Object
ownership and forced CLI termination explicitly; a documented best effort is
not a passing result. On Unix, verify that forced termination of finfocus
leaves every descendant in the session the extension created, and that the
session sweep (`/proc` on Linux, `pgrep`/`pkill -s` on macOS) removes them;
include a minimal container without procps. Scope termination to owned
processes only.

## Packaged editor and OS matrix (SC-004)

Build one VSIX and record its SHA-256. Install those identical bytes in every
cell below and run Stories 1–4, including preview cancellation, missing
locations, unpriced resources, card failure/tree fallback, and stack switching.
Record actual editor/API versions; do not infer compatibility from typings.
Minimum API 1.90 also requires a VS Code 1.90 adapter test in CI.

| Editor | Linux | macOS | Windows |
| --- | --- | --- | --- |
| VS Code | Pending | Pending | Pending |
| Cursor | Pending | Pending | Pending |
| Windsurf | Pending | Pending | Pending |
| VSCodium | Pending | Pending | Pending |

Separately verify WSL, SSH, and dev-container extension hosts: discovery,
finfocus, and file resolution must occur on the workspace host. Record at
least one passing remote run for each host type before release.

## Accessibility (FR-027)

- [ ] Complete discovery, stack selection, preview/cancel, tree navigation,
  resource details, and source navigation using keyboard only.
- [ ] Verify NVDA on Windows and VoiceOver on macOS expose resource name,
  projected/actual/delta/status, stale reason, progress, and errors. Record
  versions and results; inline decoration text alone is insufficient.
- [ ] Verify the keyboard resource-details command exposes all hover text;
  tree labels remain complete when the webview fails. Check high contrast,
  visible focus, zoom, and status conveyed without color.

## Performance (SC-002, SC-003, SC-006)

Use a fixed 300-resource fixture, recording editor/OS, hardware, plugin
versions, and fixture revision. Run five warm-start loads after one completed
plugin start: measure activation to both annotations and dashboard populated;
each must finish within 20 s. Report cold starts separately, without applying
the warm-start threshold. Compare rendered strings to the CLI output.

During a real preview, run a repeatable 60 s typing/edit script and capture
an extension-host CPU profile plus UI timing trace. Compare with the extension
disabled under the same workload; investigate any pause over 100 ms and fail
if attributable to the extension. Measure stale notification latency from
saved and unsaved edits (each at most 1 s) and assert no pricing process
starts when on-save preview is off. Store traces and raw timings, not just a
statement that the editor felt responsive.

## Part A verification results (T028, 2026-11-17)

| Gate | Result |
| --- | --- |
| `make lint` | pass (golangci-lint 2.15.0, markdownlint, actionlint) |
| `make test` | pass (root and nested plugin modules) |
| `go test -race` | pass: engine, viewmodel, webui, sourcepos, pulumi, stackload, procjob, cli |
| `make validate` | pass |
| `make docs-lint` | pass, 0 issues |
| `govulncheck ./...` | no vulnerabilities found |
| `go build` and `go vet` | pass for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 |
| Coverage | sourcepos 95.7%, viewmodel 96.5%, stackload 98.9%, pulumi 84.5%; procjob 0% on Linux (Windows-only code, see T029) |

Quickstart section 1 against `testdata/sourcepos/nodejs/program` (saved
preview, run from the project directory): `location.file` is `lib/box.js`,
no absolute or escaping path, `summary.display` and `budgetDisplay` present.

Finding: with an explicit `--pulumi-state` file, `stackload.Source` carries no
`ProjectDir`, so the resolver is nil and no `location` is printed. The
quickstart originally used that form and printed none, so it now runs the
saved preview from the project directory. Whether an explicit state file
should resolve against the nearest `Pulumi.yaml` is open for the owner.

## Annotation adapter results (T040-T043, 2026-11-17)

- **Editor tests**: `packages/vscode-finfocus/test/editor/annotations.test.js`,
  run with `xvfb-run vscode-test` on VS Code 1.90.0 (the minimum) and 1.146.0
  (stable): 9 passing on each. Cases: deployed state-only load, closing and
  reopening a document, annotation moving after inserted lines and after undo,
  refresh replacing annotations, unsafe/absent locations skipped, safe source
  navigation with a clamped position, and zero subprocesses in an untrusted
  workspace and in a workspace with no Pulumi project.
- **Real CLI (T043)**: the `testdata/sourcepos/nodejs/program` fixture was
  deployed to a throwaway local-backend stack (`pulumi up`, Pulumi v3.272.0,
  Node.js 26) and read with the local build `bin/finfocus`
  `v0.5.8-10-ga3d8d714-dirty`, **not a released binary**. Run with
  `FINFOCUS_BINARY`, `FINFOCUS_FIXTURE_PROJECT` (the deployed project) and
  `NODE_PATH`/`PULUMI_*` set. Nine resources: `lib/box.js:14` holds six
  (`web`, `worker-a/b/c`, `edge-left/right`), `index.js:9` and `index.js:10`
  one each, and the `default` provider has no location. The editor adapter
  produced `6 resources`, `-`, and `-` at those lines, equal to the CLI's
  `display.projected` strings, with the same counts and no annotation for the
  location-less resource.
- **Limits**: these resources are `pulumi-nodejs:dynamic:Resource`, which no
  plugin prices, so every figure is `-`; priced-string parity needs a
  deterministic pricing plugin and is still pending. The comparison must be
  rerun against a released CLI when Part A is released.

## Stale tracking results (T045-T049, 2026-11-17)

- **Unit tests**: `packages/vscode-finfocus/test/unit/stale.test.js`, 19
  passing under `node --test`: older/newer/equal file times, missing/invalid
  `modifiedAt`, unreadable or absent files and locations, pre-existing dirty
  documents, per-file generations, config generations, edits after request
  start, and state-only refresh preserving marks.
- **Editor tests**: `test/editor/stale.test.js`, 11 cases, run with
  `xvfb-run vscode-test` on VS Code 1.90.0 and 1.146.0; the full editor suite
  (20 tests) passed on both in four consecutive runs.
- **Notification latency (SC-003)**, ms from the edit or file write to the
  annotation showing the reason, last run (1.90.0 / 1.146.0 ranges over the
  four runs): unsaved edit 34-55, saved edit 25-55, external write 128-130,
  stack settings write 127-130. Each is far under the 1000 ms budget. Raw
  timings print as `STALE_LATENCY_MS` in the test output.
- **Zero overview spawns**: after the initial state-only load the fake runner
  counts one overview call; the unsaved, saved and external edit tests assert
  it stays at one. The fake counts runner calls; it does not observe the OS, so
  this proves the extension never asks the runner, not that no process exists.
- **Initial load**: a file newer than `modifiedAt` shows `edited`, a bad
  timestamp or missing file shows `freshness unknown`, and a document already
  dirty shows `unsaved`.
- **Refresh race**: an edit typed while a state-only refresh was running is
  still `unsaved` after the result is published.
- **Config change**: `Pulumi.dev.yaml`, `Pulumi.dev.yml`, and a qualified stack
  name (`acme/fixture/shop` with `Pulumi.shop.yaml`) mark every row, including
  one with no location; another stack's settings file does not.
- **Save echo**: the file watcher reports the editor's own save; it is ignored
  once, so a saved edit does not count twice.
- **Not covered here**: clearing marks after a covering preview is unit-tested
  only; the editor path arrives with the preview tasks (T056 onward).

## Shutdown and preview results (T051-T058, 2026-11-17)

- **Scheduler**: `test/unit/scheduler.test.js`, 13 passing: request ids,
  one overview per context, supersession waiting for confirmed cleanup,
  selection-epoch drops, late output after cancel, `cleanupFailed` blocking
  until a retry confirms, passphrase asked once per answer (empty value
  passed), dismissal, aborted prompt with its late answer discarded,
  per-context credentials, and `closeAll`.
- **Real process trees (Linux)**: `test/integration/shutdown.test.js`, 9
  passing, repeated three times. A node parent starts a child and a grandchild
  that job control moves into its own process group, inside the session the
  runner created (checked with `ps`). By PID, never a mocked kill: graceful
  stop, a parent that exits and leaves the grandchild, SIGTERM ignored by every
  process (escalates, `forced: true`), a clean exit that left a descendant
  (swept), SIGKILL of the parent (swept by `done`), an unrelated detached
  process (survives), and a stubborn session reported as unconfirmed with its
  PIDs. A stop took at most the 600 ms test grace plus the sweep.
- **Real finfocus plus plugin (Linux)**: `test/integration/real_cli_shutdown.test.js`
  against the scratch `testdata/sourcepos/nodejs` stack with the
  `finfocus-plugin-civo-public` plugin installed: the session held `pulumi` and
  the plugin; stopping mid-preview took 41 ms with `forced: false`, and after
  SIGKILL of finfocus the sweep took 91 ms with `forced: true`. In both runs no
  member of the session survived. The aws-public router child case was not run.
- **Editor preview tests**: `test/editor/preview.test.js`, 15 cases, run on VS
  Code 1.90.0 and 1.146.0: Save all and run, Run saved files, Cancel, failed
  save, clean project, on-save off by default and on (saves no other document),
  edit during a run keeping its mark, cancelling the progress notification,
  failed preview keeping figures and marks, and the passphrase box (masked,
  empty value, dismissal, retry text, abort). The full editor suite is 36
  tests per version, two runs.
- **Not run**: macOS and Windows. The workflow now runs
  `node --test "test/integration/*.test.js"` in all three matrix jobs, but no result is
  recorded for macOS or Windows (the Windows `taskkill` path and the macOS
  `pgrep -s` path are untested), and neither is the minimal container without
  procps. The Windows Job Object also needs T029. CLI delta parity for preview
  results is not checked: the fixture resources are unpriced.

## Dashboard results (T060-T066, 2026-11-17)

- **Projection**: `test/unit/dashboard.test.js`, 12 passing: CLI summary and
  budget strings untouched, `No budget configured`, top five by numeric cost
  (not display text, ties by URN, unpriced excluded, nothing summed), grouped
  list with an Unpriced group last, row errors attached and kept in the full
  list, stale vs unknown counts with reasons, detail lines, unsafe or missing
  locations not openable, empty stack.
- **Views**: `test/editor/dashboard.test.js`, 7 cases, run on VS Code 1.90.0 and
  1.146.0 (43 editor tests per version): tree grouping and labels, status-bar
  text and reveal command, accessible labels carrying figures/location/stale
  reason, tree and status updating on a stale change, declaration opening and
  the no-location message, the details command listing every hover field and
  the location, a deliberately broken webview leaving tree and status intact,
  and a malicious resource name that reaches the page only as posted data under
  `default-src 'none'` with no remote URL.
- **Not covered here**: screen readers, high contrast, zoom and focus
  visibility (T067, owner), and keyboard-only use of a real webview. The
  keyboard path is exercised as commands, not as key events.

## Post-implementation render evidence (T024, 2026-11-17)

- **Build and stack**: `bin/finfocus` `v0.5.8-10-ga3d8d714-dirty`, **not a
  released binary**. The `testdata/sourcepos/nodejs` program was copied to a
  scratch directory, packed (`npm pack` in `pack-lib`, which the repository
  does not hold), installed, and deployed to a throwaway local backend with
  Pulumi v3.272.0 and `NODE_PATH` set. The dashboard ran from that directory:
  `PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port 8774`.
  Nine rows; the CLI reported eight locations.
- **Finding**: `--pulumi-state <file>` shows no "Declared at" row. An explicit
  state file has no project directory (`locationResolver` returns nil), so
  locations need a live stack or a project-resolved export. This matches the
  design but was not documented for web users; see the open item below.
- **With a location** (`web`): the detail list ends `Drift`, `Declared at =
  lib/box.js:14` ([desktop](validation/t024-location-desktop-1440.png),
  [narrow](validation/t024-location-narrow-390.png)). The row follows Drift,
  not Type.
- **Without a location** (the `default` provider): the list is URN, Type,
  Status, Actual MTD, Projected, Delta, Drift, with no placeholder
  ([desktop](validation/t024-no-location-desktop-1440.png),
  [narrow](validation/t024-no-location-narrow-390.png)).
- **Long path**: a synthetic 150-character path put into the row's text at 390
  px wraps over several lines inside the panel with no panel overflow
  ([narrow](validation/t024-location-long-path-narrow-390.png)). The text was
  set in the page, not produced by the server. The page itself is 735 px wide
  at a 375 px viewport with or without the panel, from the resource table.
- **Escaping and focus**: the row is a `dd` set with text, and
  `viewmodel` and handler tests cover unsafe locations. Opening the row moved
  focus to the panel as before, and the row adds no tab stop. A hostile path
  was not rendered live.
- **Cleanup**: browser closed, server stopped with SIGINT, no
  `finfocus-plugin*` process remained.

## Packaging and release workflow results (T069-T072, 2026-11-17)

- **Manifest assertions**: `test/unit/manifest.test.js`, 8 passing under
  `node --test`: engine `^1.90.0`, publisher `finfocus`, workspace extension
  kind, limited trust with a reason, package metadata and LICENSE, no runtime
  dependencies or `enabledApiProposals`, semver `minFinfocusVersion`, a PNG
  icon (signature checked), and a `.vscodeignore` allow-list that names no
  test, secret, lockfile or editor-download path and covers `main` and `icon`.
- **Local VSIX**: `vsce package --no-dependencies` (vsce 4.1.1, no `--allow-*`
  flag) gave 31 files, 49.35 KB, with no warning. The listing holds
  `package.json`, `LICENSE.txt`, `readme.md`, `contract/`, `media/` and `src/`
  only. Two builds of the same tree differ in SHA-256, because vsce stamps
  generated entries with the build time. The release workflow therefore builds
  once, hashes those bytes, and attaches that file with `SHA256SUMS`.
- **Release wiring**: `release-please-config.json` has the `vscode-finfocus`
  node component (tag `vscode-finfocus-vX.Y.Z`, `initial-version` 0.1.0) and
  lists the package in the root `exclude-paths`; the manifest file is
  untouched. `release-vscode-extension.yml` checks the tag against
  `package.json`, the publisher as an Open VSX namespace, and LICENSE, runs
  the unit tests and type check, packages, lists contents, and uploads the
  VSIX with its hash. It has no store credential.
  `test-vscode-extension.yml` gained a `package` job that builds the VSIX,
  rejects any test, `node_modules`, `.vscode-test`, `.env`, `.npmrc`, lockfile
  or `jsconfig` entry, and uploads it. `actionlint` passes on all touched
  workflows. None of the workflows has run on GitHub yet.
- **Tag collision found**: the tag `vscode-finfocus-v0.1.0` starts with `v`,
  so `release-monorepo-plugin.yml` already skips it (research R7a assumed it
  would run), but `goreleaser.yml`, `nightly.yml` and `docker.yml` treat any
  `v`-prefixed tag as a core release. Their conditions now exclude the
  `vscode-finfocus-` prefix and docker's tag pattern is `v[0-9]*`.
- **Editor API levels**: the editor job already runs the suite on VS Code
  1.90.0 (`minimum-api`) and current stable.
- **Not done (T072 stays open)**: `minFinfocusVersion` is 0.5.8, the latest
  release, which predates Part A; it must be confirmed against the release
  that ships Part A. The .NET location fixture is still pending (see Runtime
  fixture status), and that blocks the first extension release.

## First-run readiness tests (T076-T077, 2026-11-18)

- **Written before the code (TDD)**: `test/unit/version.test.js` (a `Readiness`
  class in `src/editor/readiness.js` with `ready()` and `settingsChanged()`)
  and `test/editor/readiness.test.js`. The new unit cases fail until T078
  because the module does not exist; the three original `checkVersion` cases
  still pass.
- **Editor run** (stable 1.146.0 under `xvfb-run`): 8 passing, 3 failing. The
  failures are the new behavior: a missing binary is retried on every refresh
  (3 version runs, expected 1), `Controller.settingsChanged()` is absent, and
  a plugin failure carries no `finfocus plugin install` guidance. The eight
  passing cases pin behavior that already holds: old and development
  versions, missing Pulumi, no stacks, a malformed stack listing, an untrusted
  workspace, a non-Pulumi workspace, and a `Pulumi.yaml` only under
  `node_modules`.
- **Issue filed (T026)**: <https://github.com/rshade/finfocus/issues/2199>.

## First-run readiness results (T078-T080, 2026-11-18)

- **Implementation**: `src/editor/readiness.js` (`Readiness`, `withGuidance`),
  used by `Controller`; `extension.js` calls `settingsChanged()` and refreshes
  when `finfocus.path` or `finfocus.pulumiPath` changes. A failure is reported
  once and not retried until then. Plugin guidance is added when the CLI's
  own message mentions plugins, because core gives plugin failures no
  separate category.
- **Results**: unit suite 210 passing (`node --test "test/unit/*.test.js"`); `tsc
  --noEmit` clean; editor suite on stable 1.146.0 under `xvfb-run`, 54
  passing, including all 11 readiness scenarios. The minimum-API 1.90 job (superseded: the editor job runs both API levels, see "CI results") has
  not been run locally.
- **Troubleshooting** added to the guide and README.

## Performance harness (T081, 2026-11-18)

- **Fixture**: `test/fixtures/performance/fixture.js` generates the same 300
  rows and 301-line program on every run. `test/editor/performance.test.js`
  times five warm loads against the 20 s limit, compares each annotation
  string to the fixture, then runs a typing workload twice, once with no
  extension components and once with them. It samples event-loop delay and
  per-edit latency and fails if the extension adds more than 100 ms to the
  longest pause or starts a pricing run.
- **Smoke run only**: `FINFOCUS_PERF_SECONDS=8` on VS Code 1.146.0, Linux x64,
  8 CPUs (AMD Ryzen AI 7 PRO 350): loads 60-88 ms, both tests passing. This is
  a harness check, not SC-002/003/006 evidence: it uses a fake runner (no real
  pricing process), 8 s rather than 60 s, no CPU profile, and no cold-start
  run. T082 holds the full measurement.

## Secret, error, and path safety (T083, 2026-11-17)

- **Hover and inline text** (FR-021): `test/unit/hover_safety.test.js` drives
  `AnnotationProvider.paint` with a hostile string (a `command:` link, a
  `<script>` tag, shell syntax) in the hover lines, the inline text, and the
  stale reason. Resource text reaches the hover only through `appendText`
  (escaped); the only `appendMarkdown` calls are the fixed `\n\n` separators;
  the hover is neither `isTrusted` nor `supportHtml`.
- **Webview**: `test/editor/dashboard.test.js` already renders a resource named
  `<img src=x onerror=alert(1)>` and asserts the HTML has no such tag while the
  state message carries it as data. The tree view uses plain-string tooltips.
- **No secrets in messages**: `runner.test.js` ("a spawn error is reported
  without the command line") asserts a message has no subcommand, flag, or
  passphrase (`hunter2`); `version.test.js` asserts reports carry no stderr or
  command line.
- **Inventory mutations** (SC-005): `result.test.js` generates a missing-path
  and a wrong-type case for every required path in
  `contract/overview-fields.json`, and a wrong-type case for each optional one.
- **Results**: unit suite 213 passing; `tsc --noEmit` clean.

## Quickstart run (T085, 2026-11-17)

- **§2**: `node --test packages/vscode-finfocus/test/unit/` ran 213 passing.
- **§3 package**: the `pnpm` corepack shim is broken on this machine (missing
  `pnpm.cjs`), so `node_modules/.bin/vsce package --no-dependencies` built the
  VSIX: 32 files, 51.28 KB, SHA-256
  `5ac3db66c5d51e86b9d2073d43c9e3391ebed298eae0f480da8e823bfdfc9652`
  (a local build, not the release artifact). Steps 1-4 were not performed by
  hand in an installed editor; the editor suite covers the same behavior
  (annotations, stale marks within a second with no spawn, preview, untrusted
  workspace: 54 passing on stable 1.146.0 under `xvfb-run`). Installing into
  each editor is T073 and stays an `[owner]` gate.
- **Drift fixed**: quickstart §5 said the fixtures and package were planned and
  not yet created; §3 now names the `vsce` fallback.
- **§4** runs under T086.

## Repository gates (T086, partial, 2026-11-17)

- **Passing**: `make test`, `make test-race`, `make validate`, `make docs-lint`,
  and `govulncheck ./...` (no vulnerabilities) all exit 0. `make lint` exits 0
  when run as `make lint -o lint-vscode-extension`; the sub-target fails here
  only because the `pnpm` corepack shim is broken (missing `pnpm.cjs`), so
  `node_modules/.bin/tsc --noEmit -p jsconfig.json` ran by hand instead (clean).
- **Go coverage** (this Linux run): `internal/sourcepos` 95.7%,
  `internal/stackload` 98.9%, `internal/viewmodel` 96.5%, `internal/engine`
  92.7%, `internal/pulumi` 84.5%. `internal/procjob` shows 0% because its
  code is `//go:build windows`; it is exercised only by the Windows job (T029).
- **Extension coverage** (unit suite only, `node --test --experimental-test-coverage`):
  88.8% lines overall; `result.js` 97.0%, `runner.js` 97.2%, `scheduler.js`
  95.0%, `stale.js` 99.0%. `processTree.js` (29%) and `shutdown.js` (26%) are
  covered by `test/integration/`, and `editor/annotations.js` (77%) by the
  editor suite; neither is in this figure.
- **Not done, so T086 stays open**: the Go docstring coverage was not
  re-measured; the minimum-API (1.90) and current-editor CI jobs had not run then (superseded: see "CI results");
  a combined extension coverage figure (unit + integration + editor) was not
  produced.

## Performance run (T082, partial, 2026-11-17)

- **Host**: AMD Ryzen AI 7 PRO 350, 8 CPUs, Linux x64, VS Code 1.146.0 under
  `xvfb-run`. The harness uses a fake runner (no real pricing process) and the
  generated 300-row fixture.
- **Five warm loads** (SC-002, limit 20 s): 125, 71, 47, 162, 90 ms.
- **60 s typing workload** (SC-006, limit: no attributable pause over 100 ms):
  without the extension, 1414 edits, longest loop delay 83.2 ms, longest edit
  199.4 ms; with it, 1268 edits, longest loop delay 23.2 ms, longest edit
  23.5 ms, p99 edit 14.0 ms. The extension added no pause, and no pricing run
  started.
- **Stale latency** (SC-003, limit 1 s): unsaved 63 ms, saved 64 ms, external
  128 ms, config `.yaml`/`.yml`/qualified stack 129 ms. Neither case spawned
  a process.
- **Superseded by the real-CLI run below.** The SC-002 evidence is taken from
  T099 (accepted 2026-11-18, ISS-001 Resolved), and the three items this note
  listed (CPU profile, cold start, profile comparison against a real finfocus)
  are recorded in "Real-CLI performance run (T082, 2026-11-18)".

## Real-CLI performance run (T082, 2026-11-18)

- **Harness**: `test/editor/performance-real.test.js`, opt-in through
  `FINFOCUS_BINARY` and `FINFOCUS_FIXTURE_PROJECT` (skipped otherwise). It
  runs the real `finfocus` against the deployed `initial-30` `dev` stack (61
  rows), types into the generated 300-line fixture for 30 s, and attaches a
  `node:inspector` CPU profiler to the extension host.
- **Host**: AMD Ryzen AI 7 PRO 350, 8 CPUs, Linux x64, VS Code 1.147.0 (stable)
  under `xvfb-run`, finfocus `bin/finfocus` built from this branch. Run under
  `nice -n 15`, started only after the load average fell below 1.5; the load
  stayed under 6 (VS Code's own launch).
- **Cold start and repeats** (state-only, no cache, a fresh scheduler per
  load, so each load runs the stability retry): 7701 ms first, then 6967, 6197,
  7073, 6992. Every `finfocus` run starts its plugins anew, so the first load
  is not separated from the rest by a plugin warm-up; it is 0.7 to 1.5 s
  slower here. These are above the 3.6 to 3.8 s baseline-miss timing in T099
  because each fresh scheduler has no baseline and so runs twice.
- **Real preview** (`pulumi preview` through `finfocus overview`, passphrase
  empty): completed in 15011 ms with 61 rows, inside the 30 s typing window.
  An earlier attempt in the same harness ran in 20163 ms.
- **Typing during that preview** (SC-006, limit: no attributable pause over
  100 ms). Baseline with no extension components: 705 edits, longest loop delay
  87.7 ms, longest edit 139.1 ms, p99 edit 6.1 ms. With the extension and the
  real preview running: 714 edits, longest loop delay 15.8 ms, longest edit
  14.8 ms, p99 edit 4.6 ms. The extension added no pause.
- **CPU profile** (30178 ms sampled): 5.5 ms of self time in the extension's
  `src/` (`editor/staleTracking.js:keysFor` 2.2 ms, `processTree.js:sessionOfStat`
  2.2 ms, `result.js:parseOverview` 1.1 ms). The raw profile is kept outside
  the repository at `~/.cache/finfocus-perf/perf-real.cpuprofile` (400 KB; it
  embeds absolute local paths, so it is not committed) next to the raw timings
  in `perf-real.json`.
- **Limits of this evidence**: one run of the real-CLI suite on stable 1.147.0 only (it was not
  run on 1.90, and CI does not run it); the baseline typing window ran first, so its 87.7 ms pause may be
  workbench warm-up rather than the absence of the extension; the profile is a
  V8 sampling profile of the extension host, not a DevTools UI timeline; the
  typed document is the generated fixture, while the preview priced
  `initial-30`, so annotations were painted for the preview's rows without that
  project's files open. `PULUMI_CONFIG_PASSPHRASE=''` set in the launching shell
  does not reach the extension host, so the harness passes it to the `Runner`
  explicitly.

## Package inspection and coverage (T072 and T086 follow-up, 2026-11-17)

- **VSIX contents** (T072, local): `vsce ls --no-dependencies` lists only
  `LICENSE`, `README.md`, `package.json`, `contract/overview-fields.json`,
  `media/*` (four files), and `src/**` (no tests, fixtures, `node_modules`, or
  lockfile). The earlier local build was 32 files, 51.28 KB.
- **T072 is still open**: the .NET location fixture is still pending (no
  `dotnet` on this machine), which blocks the first extension release; the
  1.90 and current-editor CI jobs had not run then (superseded: see "CI results"), and `minFinfocusVersion`
  (0.5.8) has not been confirmed against a released Part A.
- **Go docstring coverage** (T086): every exported symbol in the 26 changed or
  new non-test Go files was checked with a go/ast script: 148 of 156 (94.9%)
  have a doc comment. The eight without are `projectRuntime.UnmarshalYAML`
  (now documented), the older `pulumi.Run`, two `orphanplugin` test-plugin
  methods, and `Pair`/`NewPair` in the two `testdata/sourcepos/go` fixtures,
  which are fixtures rather than shipped code. Excluding the fixtures and the
  documented method, coverage of shipped Go is above the 80% gate.
- **Extension coverage** (T086): unit plus integration suites, 95.75% lines
  overall (88.81% from the unit suite alone). `shutdown.js` 96.1%,
  `processTree.js` 87.6%, `runner.js` 99.2%. `editor/annotations.js` is 76.7%
  here because its painting code runs only inside the editor suite.

## Editor suite on both API levels (T086 follow-up, 2026-11-17)

- **Minimum API**: VS Code 1.90.0 under `xvfb-run`, 56 passing, 1 pending.
- **Current**: stable 1.146.0, 56 passing, 1 pending. The pending case is the
  real-CLI test, which skips without `FINFOCUS_BINARY` and
  `FINFOCUS_FIXTURE_PROJECT`.
- **Still open**: these ran locally, not through `test-vscode-extension.yml`
  (only `actionlint` has checked that file), so T086 stays open until the
  workflow run is recorded.

## Real-stack load cost (2026-11-17, found during the live web-workbench run)

- **Setup**: `initial-30` stack `dev` deployed (65 resources, bun runtime),
  extension installed in a VS Code web workbench (1.146.0), trusted folder.
  Untrusted state ran no process; trusting started `finfocus overview`
  immediately. Summary card, tree, and status-bar total ($32.58) rendered.
- **Finding 1, cost per refresh (corrected 2026-11-17)**: the first measurement
  (36.8 s wall, 1 GB) was taken while the machine was thrashing (load average
  about 58, swap in use, a deploy and a workbench still settling) and is an
  outlier. Repeated on an idle machine, `finfocus overview --state-only --stack
  dev` takes 3.2-3.9 s wall (about 1.1 s CPU, 100 MB for the CLI process; 5.1 s
  with the extension host's environment), of which `pulumi stack export` is
  1.1 s. SC-002 (20 s) holds in that state. The extension still has no result
  reuse, and every run starts the full plugin set; two `aws-public` region
  children are each about 1 GB resident while running.
- **Finding 1b, silent partial result under load**: the first load, run during
  that contention, showed a projected total of $32.58 with 60 of 61 rows
  unpriced; an idle run of the same command gives $173.61 with every row
  priced and no reported error. A slow plugin start therefore produced a
  smaller figure with nothing in the summary saying it was partial.
- **Finding 2, locations under bun**: 9 of 61 rows carry a location, all from
  a component module (`tailscaleDeviceNode/index.ts`). Top-level resources
  have `stackTrace: [{}]` and a null `sourcePosition`, so the tree reads "No
  recorded location" and no inline annotation is placed. bun is not one of the
  runtimes captured under T007, so it must be recorded as unverified, not
  supported.
- **Finding 3, noise**: every row reads "freshness unknown" on a deployed
  state load (no `modifiedAt`), and the summary says "52 freshness unknown".
- **Resolved by design, evidence below**: ISS-001 (issues.md, research.md R11,
  R12). The cache and completeness rules are measured in "Result reuse and
  completeness (T099)". T082 and T087 stay blocked until T099 is accepted.

## Result reuse and completeness (T099, 2026-11-18)

**Setup.** Deployed `initial-30` `dev` (65 resources, 61 overview rows),
Pulumi Cloud backend, `finfocus v0.5.8-10-ga3d8d714-dirty`, `pulumi v3.272.0`,
WSL2 with 8 logical CPUs and about 13.8 GB RAM. The harness is a Node script
outside the repository that drives the real `Runner`, `Scheduler`, `ResultCache`,
and `FileCacheStore` (the same modules the extension loads) with the real
`pulumi stack ls --json` discovery, so each figure includes the discovery call
and the settings-file hashing the controller does. An earlier attempt overlapped
another process's Go tests (load 10) and is discarded; the runs below started
once the one-minute load average fell under 1.5 (1.47). The harness itself
raised it to 2 to 4 during the runs.

| Case | Runs | Wall time |
| --- | --- | --- |
| Cache hit (new scheduler per run, as a reopened window) | 5 | 0.43 to 0.67 s, of which discovery is all but 1 to 2 ms |
| Cache miss with a stored baseline (one finfocus run) | 5 | 3.6 to 3.8 s |
| Cache miss with no baseline, including the stability retry | 5 | 6.5 to 8.6 s |
| Same, under bounded induced load (16 CPU-bound processes for 40 s, load 9 to 13) | 1 | 17.9 s |

- `finfocus --version`, which the readiness check runs once per session, takes
  0.01 s, so a hit's activation cost stays under one second at the module level.
  The editor tests (`test/editor/cache.test.js`) prove a hit starts no
  `finfocus overview` process; end-to-end time inside a real window was not
  timed.
- A hit on the 300-resource fixture from T081 (110 KB entry): lookup and parse
  1 to 6 ms over five runs.
- **42 of 61 rows print `-`** (providers, route tables, internet gateways,
  `command:local:Command`, `kubernetes:*`, Tailscale keys, and similar types
  with no cost data). Every run agreed on the same 42, so the result is
  `confirmed`, the total stays `$173.61`, and it is stored. The stability retry
  therefore fires on the first load of nearly every real stack, which is the
  "6.5 to 8.6 s" row; a stored baseline removes it afterward.
- **Not reproduced**: the silent partial result (Finding 1b) did not recur at
  bounded load. Under 16 burners the two runs still agreed, so the incomplete
  marker was not seen on the real stack. The first occurrence needed a load
  average near 58 with swap in use, which this run deliberately avoided. The
  marker is covered with modeled data in `completeness.test.js`,
  `scheduler.test.js`, and the editor suites (a 60-of-61 partial fixture and
  differing gap sets), not by a real slow plugin.
- All five idle cache-miss loads are inside the 20 s ceiling, and all five hits
  inside 2 s, so SC-002 holds on this stack. The 20 s ceiling held under load
  too (17.9 s), narrowly.

### Web workbench run (T099 follow-up, 2026-11-18)

Driven with playwright-mcp against a VS Code web server (`code-server` bundle,
commit matching the client) with this build unpacked into its extensions
directory, on `initial-30` `dev`. Screenshots are under `.playwright-mcp/`
(`cache-01-first-load.png`, `cache-02-reload-hit.png`, `cache-03-incomplete.png`).

- **First load** ran finfocus, showed `42 resources unpriced` (a confirmed
  note, not a warning), and stored an entry with mode `0600` and 42 confirmed
  gaps.
- **Developer: Reload Window with a stored entry** started no `finfocus
  overview` process in 32 s of 0.5 s samples; discovery ran once at the start
  and the stored entry was untouched. The card, tree, and status bar painted
  from it.
- **Incomplete marker** (simulated, see below): `⚠ Incomplete: 47 of 61
  resources unpriced` in the card in the warning color, a warning notice as the
  first tree line, and `⚠ incomplete` in the status bar. The wrapper ran twice
  (the load and its stability retry) and the stored entry's modification time
  did not change.
- **Recovery**: removing the setting ran a bypassing refresh with the real
  binary, the marker cleared, and a result was stored again.
- The incomplete case used a throwaway `finfocus` wrapper, set through
  `finfocus.path`, that calls the real binary but blanks 10 priced rows on its
  first overview run and a different 5 on its second. It simulates two runs
  that disagree; it does not show a real slow plugin, which still did not
  reproduce under bounded load.

**Findings from this run**

- **Workspace storage was the wrong home for the cache.** VS Code locks a
  workspace-storage folder per live extension host and gives later hosts a
  `-1`, `-2` sibling, so every window or reload in this server started with an
  empty cache (the first reload spawned finfocus and wrote `68276cc6-1`).
  The cache now lives in `context.globalStorageUri`; the context key already
  names the project and stack, and the retest above hit.
- **Lingering extension hosts are expensive.** Each reload left an extension
  host of about 850 MB resident for the server's grace period; five of them
  pushed the load average to 96 and one first load took 84 s. This is a
  property of the web server and my repeated reloads, not of the extension, but
  it is a plausible cause of the original silent partial result (Finding 1b).
  The old hosts were stopped by PID and the load returned to under 3.
- **The top-five list is empty on real data.** Core emits cost as
  `projectedCost.monthlyCost`, but the extension and the contract read a
  top-level `monthlyCost`, which no real row has (0 of 61), so the card's
  "Most expensive resources" list never fills. Fixtures used the top-level
  field, so no test caught it. Logged as ISS-002.
- **ISS-002 fixed and re-checked in the workbench (2026-11-18).** The parser
  now reads `projectedCost.monthlyCost`. One page load of `initial-30` `dev`
  with the rebuilt VSIX, no reloads and a load average under 3 throughout: the
  stored cache entry painted the status bar (`$173.61 · no budget`) and the
  card with no `finfocus` process running. "Most expensive resources" listed
  `windowsClientInstance $33.37`, `clientNatGateway $32.85`, `webNatGateway
  $32.85`, `civoCluster $32.58`, `clientInstance $8.39`, and the card read
  `42 resources unpriced` (the stable-gaps note). The render is in
  `.playwright-mcp/iss002-card.png` (git-ignored).

### Repository gates (T086, partial, 2026-11-18)

Run sequentially under `nice` on the local Linux/WSL2 machine, load average
under 4 throughout.

| Gate | Result |
| --- | --- |
| `make validate` | pass (`go mod tidy -diff`, `go vet`) |
| `make test` | pass, no `FAIL` |
| `make test-race` | pass, 43 packages `ok`, no data race |
| `make lint` | 0 issues in the root and 4 plugin modules; boundaries, license check, markdownlint, actionlint pass. The extension type-check step ran as `tsc --noEmit -p jsconfig.json` (the `pnpm` corepack shim is broken here) and is clean |
| `make docs-lint` | 0 issues |
| `govulncheck ./...` | no vulnerabilities |
| Extension unit tests | 286 tests, 285 pass, 0 fail, 1 skipped |
| Editor tests (`vscode-test`, minimum API and stable) | 70 passing, 1 pending (the real-CLI case, which needs `FINFOCUS_BINARY`) |

Coverage (Linux, `node --test --experimental-test-coverage` over `test/unit/`
and `test/integration/`; 294 pass, 0 fail, 3 skipped): extension 96.10% lines,
91.46% branches, 91.30% functions. Critical paths: `completeness.js` 100%,
`cache.js` 99.0%, `scheduler.js` 95.5%, `result.js` 97.0%, `stale.js` 99.0%,
`shutdown.js` 96.1%, but `processTree.js` 87.6% (its uncovered lines are the
Windows `taskkill` branch, which only the Windows job exercises).
`src/editor/annotations.js` shows 76.7% from unit tests alone; its remaining
lines need the editor host, which the editor suite above exercises, and no
merged figure exists. Go: `internal/sourcepos` 95.7%, `internal/stackload`
98.9%; `internal/procjob` reports 0% on Linux because its code builds only on
Windows.

Go docstring coverage was measured earlier (94.9% of exported symbols in the
changed files; see "Package inspection and coverage"). Still open for T086: the
95% critical-path target for `processTree.js` (87.6%; the gap is the Windows
branch) and a merged figure for the editor-host files, plus the CI matrix jobs
on macOS and Windows. T086 stays open until the CI jobs have run.

## Requirement reconciliation (T087, 2026-11-18)

Status values: **Verified** (an automated test or a recorded real-stack run
exists and was re-run in this session), **Partial** (verified on what could be
run here; the named part is open), **Open** (no evidence yet). Tests are cited
by file because they do not carry requirement IDs. Everything was run on Linux
(WSL2), VS Code 1.90.0 and stable.

| Requirement | Status | Evidence and what is open |
| --- | --- | --- |
| FR-001 to FR-004 (location in the JSON, omitted not guessed, no other field changes) | Partial | `internal/sourcepos/sourcepos_test.go` and `internal/cli/overview_json_contract_test.go` (location and additions tests). Node.js recorded; Go, Python and YAML fixtures per "Runtime fixture status"; the .NET fixture is open (no `dotnet` here, T072) |
| FR-005, FR-006 (display strings, budgets in the JSON) | Verified | the same contract test file; the `display` strings are compared with the CLI in `test/editor/annotations.test.js` (real CLI case) |
| FR-010 (activate only in a Pulumi project, trusted) | Verified | `test/unit/projects.test.js`, `test/editor/readiness.test.js` |
| FR-011 (figures come from the installed finfocus) | Verified | `test/unit/runner.test.js`, the real-CLI case in `test/editor/annotations.test.js`, "Real-CLI performance run" |
| FR-012, FR-012a, FR-012b, FR-022 (load, result cache, unpriced-row stability, no run when untrusted) | Verified, with a known limit | `test/unit/{cache,cacheStore,completeness,scheduler}.test.js`, `test/editor/cache.test.js`, "Result reuse and completeness (T099)". A real silent partial result never reproduced; accepted 2026-11-18 (ISS-001) |
| FR-013, FR-014 (Run preview, preview on save) | Verified | `test/editor/preview.test.js`; a real preview in "Real-CLI performance run" |
| FR-015, FR-015a, FR-015b, FR-015c (stale marking, `modifiedAt`, no early clear) | Verified | `test/unit/stale.test.js`, `test/editor/stale.test.js`, `TestOverviewJSONContract_ModifiedAtShape` |
| FR-016, FR-017, FR-018 (annotations, side bar, open the declaration) | Verified | `test/unit/{annotations,dashboard}.test.js`, `test/editor/{annotations,dashboard}.test.js`; ISS-002 re-checked in the web workbench |
| FR-019, FR-020 (stack choice, passphrase) | Verified | `test/unit/projects.test.js`, `test/unit/scheduler.test.js`, `test/editor/preview.test.js` |
| FR-021 (CLI errors passed through, no secrets) | Verified | "Secret, error, and path safety (T083)", `test/unit/hover_safety.test.js` |
| FR-023 (minimum finfocus version) | Partial | `test/unit/version.test.js`, `test/editor/readiness.test.js`. `minFinfocusVersion` is not yet confirmed against a released core (T072) |
| FR-024, FR-024a (one package, own release line) | Partial | VSIX contents inspected locally (T072); the release workflow has not run on CI |
| FR-025 (supported editor API level) | Partial | The editor job runs both `minimum-api` (1.90.0) and `stable` through `pnpm run test:editor`, and passes on Linux, macOS and Windows in CI (PR #2244). Open: the other editors and the .NET fixture (T072, T073), and `minFinfocusVersion` against a released core |
| FR-026 (renaming a JSON field fails a test) | Verified | `test/unit/result.test.js` mutates every path in `contract/overview-fields.json`; Go `RemovingAnyRequiredFieldIsDetected` and `RetypingAnyRequiredFieldIsDetected` |
| FR-027 (keyboard and screen reader) | Partial | Labels and keyboard paths are tested in `test/unit/dashboard.test.js` and `test/editor/dashboard.test.js`. NVDA and VoiceOver, high contrast and zoom are open (T067, owner) |
| FR-028 (stop owned processes, bounded shutdown) | Partial | `test/integration/shutdown.test.js` and `test/unit/{runner,scheduler,processTree}.test.js`; the fixture-based shutdown tests pass on Linux, macOS and Windows in CI (PR #2244, ISS-003). Open: the real-CLI tests with plugin descendants skipped on every OS, and T016's Go tests ran on Linux only (T029, T058) |
| SC-001 (every priced resource annotated, Node.js demo) | Partial | Node.js recorded and rendered; other runtimes open (see FR-001) |
| SC-002 (cache hit within 2 s, idle miss within 20 s) | Verified | "Result reuse and completeness (T099)": hit 0.43 to 0.67 s, miss 6.5 to 8.6 s; "Real-CLI performance run" |
| SC-003 (stale marker within 1 s) | Verified | "Performance run (T082)": 63 to 129 ms, no process spawned |
| SC-004 (same package in four editors on three OSes) | Open | T073 (owner) |
| SC-005 (field rename fails the build) | Verified | see FR-026 |
| SC-006 (typing stays responsive during a preview) | Verified, one run | "Real-CLI performance run": the extension added no pause over 100 ms; one run, stable only |

Open gates, none checked off here: T029 (T016's Go tests on macOS and Windows) and T058 (real-CLI shutdown tests), T067
(assistive technology), T072 (CI jobs, .NET fixture, `minFinfocusVersion`),
T073 (four editors on three OSes), T074 (WSL, SSH, dev container), and T086
(CI matrix jobs, `processTree.js` at 95%). The follow-up issue
<https://github.com/rshade/finfocus/issues/2199> was checked on 2026-11-18: it
is open, labeled `enhancement`, `web-ui`, `effort/small`, and holds layout
only.

## CI results (PR #2244, 2026-11-18)

Run on GitHub-hosted runners with Node 22; the PR checks are the evidence.

- **Linux, Windows, macOS unit job and web E2E**: pass after two fixes found by
  this run: `node --test <directory>` is a module path on Node 22, so the
  suites are now run through quoted globs; and the web parity test now drops
  the CLI-only `display` object before comparing the CLI rows with the web
  `source` rows.
- **Process shutdown tests (T058, extension Node tests)**: the fixture-based
  tests pass on all three OSes after the ISS-003 fix (macOS: 9 of 11 pass, 0
  fail, 2 skipped; Windows: 6 pass, 5 skipped by design, including the
  session-only cases; Linux: 9 pass, 2 skipped), run with
  `node --test "test/integration/*.test.js"`. On Windows `stop` always runs
  `taskkill /T /F` and reports `forced: true`, so the graceful case no longer
  expects an unforced stop there; the fixture has no job object, so this does
  not exercise core's kill-on-close job. The first macOS run failed (ISS-003):
  `pgrep -s` matches no process there. The runner now puts a random
  `FINFOCUS_EXTENSION_RUN` token in finfocus's environment on macOS and the
  sweep finds the members with `ps -axEww` (read with a 256 MiB buffer, because
  the listing carries every process's environment). The tests treat a zombie
  as gone and print `ps` for a survivor. The two real-CLI shutdown tests
  (plugin descendants) skipped on every OS: the job sets no `FINFOCUS_BINARY`,
  and that file skips on Windows even when it is set.
- **T029 (T016's Go process tests)**: not run on macOS or Windows. `ci.yml`
  runs `./test/integration/...` on Linux only, and
  `test/integration/overview_process_test.go` is `//go:build unix`.
- **Editor tests**: the job on each OS runs both the 1.90.0 and stable configurations (`pnpm run test:editor` with no `--label`), and passes on Linux, macOS and Windows. They pass on Linux and Windows after the on-save test waits for
  the file watcher's late save notice. On macOS the 60 s performance gate had
  one run in which a single 530 ms stall appeared with the extension while its
  other windows were 29 ms above baseline. The gate now compares the smaller
  of two half-length windows on each side, so a one-off runner stall is not
  attributed to the extension; this is a looser estimator than a single-window
  maximum and the real-CLI run keeps the CPU-profile attribution.
- **Packaging**: the job failed because `vsce` does not create the output
  directory; it now runs `mkdir -p ../../dist` first and the VSIX is built and
  inspected on CI.
- **Still open**: the real-CLI cases on every OS (set `FINFOCUS_BINARY` and a
  fixture project in the job); T016 on macOS and Windows.

## Completion gates

- [ ] All matrix cells, runtime fixtures, and checks above have evidence.
- [ ] Capture post-implementation Playwright renders of populated resource
  details with and without location; verify escaping and long paths.
- [ ] Update the web HTTP contract, overview reference, editor guide, package
  README, and quickstart to match implementation.
- [x] File the UX-only `web-ui` follow-up issue and record its URL here:
  <https://github.com/rshade/finfocus/issues/2199> (labels `enhancement`,
  `web-ui`, `effort/small`; 2026-11-18).
- [ ] Generate tasks.md with explicit coverage of these gates; no gate may
  be checked off from a documentation assertion alone.
