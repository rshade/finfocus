# Tasks: Cost in the Editor for VS Code and Its Forks

**Feature**: `2017-vscode-cost-extension`
**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/overview-json.md](contracts/overview-json.md),
[contracts/cli-invocation.md](contracts/cli-invocation.md),
[contracts/http-api.md](contracts/http-api.md), [quickstart.md](quickstart.md),
and [validation.md](validation.md).
**Status**: Implemented except the open gates: T029, T058, T067, T072, T073, T074 and T086 (CI runners, other machines, assistive-technology checks). See validation.md, "Requirement reconciliation".

## Execution rules

Tests are mandatory under Constitution II, the plan's test-first decisions,
and FR-026. Write the listed failing tests before corresponding implementation;
use testify require/assert for Go and colocate Go unit tests with their package.
Maintain 80% overall coverage, 95% critical-path coverage, and 95% for sourcepos.
No stubs, TODO implementations, fabricated screenshots or unexecuted acceptance
claims. Keep docs concurrent with implementation. Do not change .golangci.yml.
Never edit a `CHANGELOG.md` by hand; release-please writes them.

Task IDs are sequential. `[P]` permits parallel work only within its stated
phase after prior phase prerequisites, in disjoint files; it never overrides
test-first order or a listed dependency. All paths are relative to repository
root. New paths are planned artifacts, not claims that files already exist.
A task naming several files owns those files for its duration.

`[owner]` marks a task that needs hardware, editors, or assistive technology
an agent does not have (macOS or Windows desktops, Cursor, Windsurf, NVDA,
VoiceOver, remote hosts). An implementing agent stops at that task, prepares
the commands and evidence template in `validation.md`, and hands off; it never
marks the task complete or records results it did not observe.

## Delivery: two stacked pull requests

The work lands as two pull requests, the second stacked on the first:

- **Part A — core** (T001–T029): JSON location, display text, `modifiedAt`,
  config budget fallback, preview `--show-sames`, cancellation and descendant
  ownership, and Web Parity. Independently useful to every JSON consumer.
- **Part B — extension** (T030–T101): `packages/vscode-finfocus`, its release
  component, and its acceptance evidence.

Part A must be merged and released before Part B is finalized, because the
extension's `minFinfocusVersion` (FR-023) names the first core release that
carries the new fields; release-please versions the two separately, so that
number does not exist until Part A ships. Retarget Part B to `main` before
Part A's branch is deleted.

## Part A — Core pull request

### Phase 1: Planning gate

**Goal and checkpoint**: Close the planning Web Parity gate. No product
implementation starts before T001.

- [X] T001 Confirm a populated stack first: a deployed stack whose Overview shows priced rows, read from live state with no deploy or spend, opened alone (one `--stack`). If only `../finfocus-demo` is available and it is not deployed, stop and ask the owner before any `pulumi up`, and `pulumi destroy` afterwards. Then capture and inspect populated Overview resource details with Playwright at desktop and narrow sizes; save baseline screenshots in `specs/2017-vscode-cost-extension/validation/`, record long-path/absent-location/focus fit in `specs/2017-vscode-cost-extension/validation.md`, and link evidence from `spec.md` before checking the planning Web Parity gate in `plan.md`; stop product work if evidence cannot be obtained.

### Phase 2: JSON contract and display text

**Goal and checkpoint**: The JSON overview carries display strings and the
config budget fallback, pinned by a field inventory.

- [X] T002 [P] Create the field inventory `packages/vscode-finfocus/contract/overview-fields.json` (required/optional JSON paths and types); add failing real-renderer/envelope tests in `internal/cli/overview_json_contract_test.go` that read it, for display strings, metadata names, no/config/plugin budgets, the passphrase category, and the mixed-currency error envelope (FR-005/006/026). The inventory is the only file Part A adds under `packages/vscode-finfocus/`.
- [X] T003 [P] Add failing table/JSON formatter parity tests in `internal/engine/overview_render_test.go` for projected/actual/delta/status and summary strings, unpriced markers, zero values, empty resources, and mixed-currency errors (FR-005).
- [X] T004 Extract `engine.BuildOverviewJSONOutput` in `internal/engine/overview_render.go`, keep `RenderOverviewAsJSON` delegating to it, and add row/summary display objects from the existing `OverviewRowResult` display strings without renaming numeric fields or altering mixed-currency/exit behavior (FR-005).
- [X] T005 Apply config-budget fallback on the JSON plain path in `internal/cli/overview.go` and compose top-level budgetDisplay via `viewmodel.BuildBudgetDisplay`; satisfy T002 including empty details for no budget and plugin precedence (FR-006).

### Phase 3: Declaration locations (core of US1)

**Goal and checkpoint**: Every runtime fixture resolves to the expected
project-relative location or to none; pricing, cache keys and exit codes are
unchanged.

- [X] T006 [P] Capture sanitized Node.js source/state/preview fixtures and expected positions in `testdata/sourcepos/nodejs/`, including helper components, several resources on one line, outside frames, and a deterministic pricing test-plugin setup; document SDK/Pulumi versions and reproducible capture commands in `testdata/sourcepos/README.md` (SC-001).
- [X] T007 [P] Capture Go, Python, .NET and YAML state/preview/source fixtures under `testdata/sourcepos/` with SDK/Pulumi versions and expected locations in `testdata/sourcepos/README.md`, including Go module paths with no column and YAML no-location. If an SDK is unavailable, record that runtime as pending in `validation.md` rather than claiming support. Pending Python/.NET does not block the core release (docs list only verified runtimes) but blocks the extension's first release (T072).
- [X] T008 Write failing resolver tests in `internal/sourcepos/sourcepos_test.go` from the T006/T007 recordings for innermost project frame, fallback position, Go trimpath/module paths, absent column, invalid URI/line, traversal, symlink escape, missing files, and cross-platform separators (FR-001/002/003).
- [X] T009 Implement `internal/sourcepos/sourcepos.go` as a standard-library-only resolver returning project-relative regular-file locations; reject escaping paths and omit unknown columns, following research R1 and the sourcepos tests.
- [X] T010 Write failing ingest/conversion/merge tests in `internal/ingest/state_test.go`, `internal/stackload/convert_test.go`, and `internal/engine/overview_merge_test.go` for sourcePosition/stackTrace preservation, location precedence on every operation, and unchanged pricing properties/cache identity; extend `internal/cli/overview_json_contract_test.go` and the inventory to pin the optional `location` schema.
- [X] T011 Parse raw source positions/frames in `internal/ingest/state.go` and `internal/ingest/pulumi_plan.go`; resolve them with runtime/project root/module path in `internal/stackload/convert.go` and its loader callers, keeping raw paths out of engine/plugin output.
- [X] T012 Thread Location through `internal/engine/overview_types.go`, `internal/engine/overview_merge.go`, `internal/engine/overview_result.go`, and `internal/engine/overview_render.go`; preview create/update/replace/same takes current location, unresolved preview location stays absent, delete retains state location; satisfy T010.

### Phase 4: Modification time (core of US2)

**Goal and checkpoint**: Deployed rows carry `modifiedAt`; creates omit it;
pricing inputs are unchanged.

- [X] T013 Add failing deployed timestamp propagation tests in `internal/stackload/convert_test.go`, `internal/engine/overview_merge_test.go`, and `internal/cli/overview_json_contract_test.go` for present/absent Modified, creates, updates/replacements, and no changes to pricing/cache/plugin inputs (FR-015a/015b).
- [X] T014 Propagate ModifiedAt from deployed state through `internal/stackload/convert.go`, `internal/engine/overview_types.go`, `internal/engine/overview_merge.go`, and `internal/engine/overview_render.go` as optional RFC3339 modifiedAt, preserving deployed values for existing rows and omitting creates; update the field inventory in `packages/vscode-finfocus/contract/overview-fields.json`.

### Phase 5: Preview coverage and cancellation (core of US3)

**Goal and checkpoint**: Unchanged resources get current locations; a
cancelled or killed non-interactive overview leaves no descendant running on
Linux, macOS, or Windows.

Today plugins start in their own process group on Unix
(`internal/pluginhost/process_group_unix.go`, `Setpgid`), so a group kill sent
to finfocus does not reach them; Linux adds `Pdeathsig`, which reaches direct
children only, and macOS has neither. On Windows `configureProcessGroup` is a
no-op and `killProcessGroup` kills only the main process. The Pulumi child in
`internal/pulumi/pulumi.go` is a bare `exec.CommandContext`.
Research R4 decides the split: on Unix the extension owns finfocus's session
and sweeps it, so core handles graceful cancellation only; on Windows core
adds the Job Object.

- [X] T015 [P] Add failing preview tests in `internal/pulumi/pulumi_test.go` and `internal/stackload/convert_test.go` for --show-sames and current locations on unchanged declarations moved by inserted lines; include create/update/replace/delete and unresolved current locations.
- [X] T016 [P] Write real descendant-process tests in `test/integration/overview_process_test.go` (Unix; reuses the orphan fixture, Windows case runs under T029) with child/grandchild fixtures, asserting by PID, never by a mocked `kill()`: graceful SIGINT/SIGTERM cancellation leaves no plugin, plugin grandchild, or Pulumi child running; a child that ignores termination; on Unix, after SIGKILL of finfocus every descendant is still in finfocus's session (no child calls `setsid`), the property the extension's session sweep relies on (research R4); on Windows, killing finfocus kills every plugin and the Pulumi child through the job; unrelated processes survive.
- [X] T017 Add --show-sames to `internal/pulumi/pulumi.go` and preserve current locations through loader/merge for unchanged resources; satisfy T015 without adding guessed or old deployed annotation locations.
- [X] T018 Install cancelOnShutdownSignal for non-interactive overview in `internal/cli/overview.go`; propagate cancellation through `internal/pulumi/pulumi.go` and verify plugin closure before exit while preserving TUI/web signal ownership.
- [X] T019 Unix graceful ownership: stop the Pulumi child with SIGINT/SIGTERM through `cmd.Cancel` and a bounded `WaitDelay` in `internal/pulumi/pulumi.go` instead of an immediate kill, so Pulumi can stop its own children; keep plugins in their own groups (`killProcessGroup` depends on it) and in finfocus's session; satisfy T016's Unix cases. Forced termination is the extension's session sweep (research R4), so core adds no watchdog.
- [X] T020 Windows descendant ownership: replace the no-op `configureProcessGroup` and main-process-only `killProcessGroup` in `internal/pluginhost/process_group_windows.go` with a kill-on-close Job Object (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, `golang.org/x/sys/windows`, already in `go.mod`), and add the matching Windows helper for the Pulumi child under `internal/pulumi/`; satisfy T016 on Windows, including forced termination of finfocus.

### Phase 6: Web Parity

**Goal and checkpoint**: The shared Go method serves the location, the
dashboard renders a server-formatted "Declared at" row, and populated render
checks pass before the CLI additions count as complete.

- [X] T021 [P] Write failing tests: `internal/viewmodel/overview_detail_test.go` for a `declaredAt` display field (`file:line`, empty when no location, outside locations never present); `internal/webui/handlers_overview_test.go` for `GET /api/overview/resource?urn=...` returning `row.location` and `display.declaredAt` through `RedactJSON`; and a full rendered-detail test in `internal/webui/frontendtest/overview.test.mjs` using the existing harness.
- [X] T022 Add `DeclaredAt` to `viewmodel.OverviewDetailDisplay` in `internal/viewmodel/overview_detail.go`, built by `BuildOverviewDetailDisplay` from `OverviewRowResult.Location`. `Server.handleOverviewResource` already serves `ComputeOverviewResult` rows and that display object, so it needs no change; T021's handler test verifies it.
- [X] T023 Render an escaped "Declared at" row from `display.declaredAt` only when it is non-empty in `internal/webui/static/views/overview.js`; the browser formats nothing. Keep `specs/2017-vscode-cost-extension/contracts/http-api.md` in step with the implementation.
- [X] T024 Capture and inspect post-implementation desktop/narrow Playwright renders against populated details with/without locations and long paths; store screenshots in `specs/2017-vscode-cost-extension/validation/` and compare with Web UI Fit, recording evidence in `spec.md` and `validation.md`.
- [X] T025 Document the JSON additions (`location`, `display`, `summary.display`, `budgetDisplay`, `modifiedAt`), the config budget fallback, and the "Declared at" detail row in `docs/src/content/docs/commands/overview.md` and `docs/src/content/docs/reference/web-ui.md`; update `CLAUDE.md` where it says `cancelOnShutdownSignal` is installed only on the MCP entry points, and record the new descendant-ownership rules under Plugin Host.
- [X] T026 After CLI/API/redaction/data-shape work and its verification are complete, file one UX-only web-ui follow-up issue for row layout/copy/interaction/accessibility; record its URL and labels in `specs/2017-vscode-cost-extension/validation.md`, with no engine/API/redaction/data-shape work deferred.

### Phase 7: Part A verification

**Goal and checkpoint**: Part A is ready to merge on its own.

- [X] T027 Exercise secret/error/path safety and JSON field-removal/rename regressions through `internal/cli/overview_json_contract_test.go`; verify each required inventory field mutation fails, optional cases behave correctly, and no absolute path outside the project appears.
- [X] T028 Run quickstart §1 against `testdata/sourcepos/nodejs`, then make lint, make test, make test-race, make validate, make docs-lint, govulncheck, and builds and vets for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, and windows/amd64; verify >=80% overall/new code and >=95% critical-path test coverage and >=80% Go docstring coverage (godoc on every new exported symbol, including `internal/sourcepos`); record results in `specs/2017-vscode-cost-extension/validation.md`.
- [ ] T029 Run T016's process tests on macOS and Windows (CI runners where available, otherwise `[owner]` machines) and record PIDs, deadlines, and results in `specs/2017-vscode-cost-extension/validation.md`. *(2026-11-18: T016's Go tests did not run on macOS or Windows in the first CI run; `ci.yml` runs `./test/integration/...` on Linux only.)*

**Release checkpoint**: Part A merged and released by release-please. Record
that core version in `validation.md`; it becomes `minFinfocusVersion`.

## Part B — Extension pull request (stacked on Part A)

### Phase 8: Setup

**Goal and checkpoint**: The extension package builds, type-checks, and tests
in `make` and CI.

- [X] T030 Create `packages/vscode-finfocus/package.json`, `jsconfig.json`, `.vscodeignore`, and `LICENSE` for CommonJS ES2022, checkJs, pinned `@types/vscode ~1.90.0`, Node test/editor/package scripts, no runtime dependencies, workspace extension kind, limited untrusted-workspace support, and `minFinfocusVersion` set to the core version recorded at the Part A release checkpoint; set `publisher` to `finfocus` (extension ID `finfocus.vscode-finfocus`, owner's decision) and include license/repository/icon (PNG) metadata without store publishing.
- [X] T031 Install and lock the plan's dev-only toolchain with pnpm (the `packageManager` the root `package.json` declares) in `packages/vscode-finfocus/pnpm-lock.yaml`; configure `packages/vscode-finfocus/.vscode-test.js` for minimum-API editor tests and fixture workspaces.
- [X] T032 Wire `Makefile`: `test-vscode-extension` runs `node --test packages/vscode-finfocus/test/unit/` with no install and joins `make test`; `lint-vscode-extension` runs `pnpm --dir packages/vscode-finfocus install --frozen-lockfile` and then the checkJs type check, and joins `make lint`. Add OS/editor jobs in `.github/workflows/test-vscode-extension.yml` (xvfb on Linux; Linux, macOS, Windows runners) that run on every pull request touching the package, including stacked ones; preserve existing Go and docs gates.

### Phase 9: Extension foundation

**Goal and checkpoint**: Shared parsing, project identity, and trusted
process execution block every story.

- [X] T033 [P] Write failing inventory-driven parser tests in `packages/vscode-finfocus/test/unit/result.test.js` for metadata.stackName/generatedAt/isStateOnly, optional fields, malformed JSON, row errors, failure envelopes, and the mixed-currency error; read the same `contract/overview-fields.json` as the Go contract test so an unused inventory cannot hide schema drift.
- [X] T034 [P] Write failing project/context tests in `packages/vscode-finfocus/test/unit/projects.test.js` with fixtures in `packages/vscode-finfocus/test/fixtures/projects/` for nested/multi-root projects, same-name dev stacks, remote authorities, stack ls JSON/current flags, missing Pulumi/no stacks, and backend/config invalidation.
- [X] T035 [P] Write failing spawn/trust tests in `packages/vscode-finfocus/test/unit/runner.test.js` for argument arrays, cwd, workspace host, version/discovery/overview trust guards, explicit --stack, inherited environment, empty context-specific passphrases, and redacted diagnostics.
- [X] T036 Implement `packages/vscode-finfocus/src/result.js` as the inventory-checked overview/error parser; retain CLI display strings unchanged, decode omitted isStateOnly as false, and distinguish unpriced/missing data from numeric zero (FR-011/021/026).
- [X] T037 Implement `packages/vscode-finfocus/src/projects.js` for Pulumi.yaml/yml discovery, nearest ancestor/picker rules, safe stack-setting candidates, session-local choices, context keys, selection epochs, and current-flag defaults; use pulumi stack ls --json only for identity and never stack select (FR-010/019).
- [X] T038 Implement `packages/vscode-finfocus/src/runner.js` for trusted workspace-host version/discovery/state-only/preview spawns using shell-free argument arrays and per-context environments; provide real owned-process handles and error outcomes to later lifecycle scheduling (FR-011/012/021/022).

### Phase 10: User Story 1 — See cost at the declaration (P1)

**Goal and checkpoint**: Independent test: open the deployed Node.js fixture with deterministic pricing and verify every priced declaration string equals CLI JSON; unpriced/missing/outside locations never get a fabricated price or position.

- [X] T039 [P] [US1] Write failing annotation mapping tests in `packages/vscode-finfocus/test/unit/annotations.test.js` for 1-based positions, end-of-line strings, resource collisions/count hover lists without summation, missing columns, unpriced rows, and navigation bounds.
- [X] T040 [P] [US1] Write failing activation/decoration adapter tests in `packages/vscode-finfocus/test/editor/annotations.test.js` for deployed state-only load, opening/closing documents, safe source navigation, moving decorations after line insertions, refresh, and no subprocesses in non-Pulumi/untrusted workspaces.
- [X] T041 [US1] Implement pure decoration projection in `packages/vscode-finfocus/src/annotations.js` and the VS Code adapter in `packages/vscode-finfocus/src/editor/annotations.js`; show CLI projected/hover strings, count shared-line resources, follow document edits, and skip unresolved/outside locations (FR-016/018).
- [X] T042 [US1] Wire trusted activation, project/stack pickers, explicit refresh, state-only loading, and disposal of the annotation provider in `packages/vscode-finfocus/src/extension.js`; declare activation events and working commands/settings in `packages/vscode-finfocus/package.json`, with adapters under `src/editor/`.
- [X] T043 [US1] Run the T006 source-location fixture through the released CLI and the editor adapter using `packages/vscode-finfocus/test/editor/annotations.test.js`; compare every display string, resource count, and missing-location behavior; record results in `specs/2017-vscode-cost-extension/validation.md`.
- [X] T044 [US1] Document annotation setup, supported recorded positions, and missing/unpriced behavior in `packages/vscode-finfocus/README.md`.

### Phase 11: User Story 2 — Know when costs are stale (P1)

**Goal and checkpoint**: Independent test: saved and unsaved file edits mark all declaring resources within 1 s without pricing; initial timestamp comparisons and edits during refresh never imply false freshness.

- [X] T045 [US2] Write failing freshness tests in `packages/vscode-finfocus/test/unit/stale.test.js` for older/newer/equal mtimes, invalid/missing modifiedAt, unreadable/absent locations, pre-existing dirty documents, per-file generations, config generations, and edits after request start (FR-015/015a/015b/015c).
- [X] T046 [US2] Implement reasoned per-context stale sets and RequestSnapshot generation comparison in `packages/vscode-finfocus/src/stale.js`; state-only refresh preserves edit/config marks, unknown timestamps never imply freshness, and only covering previews without later edits may clear marks.
- [X] T047 [US2] Write failing stale adapter tests in `packages/vscode-finfocus/test/editor/stale.test.js` for saved/unsaved/external edits, rename/deletion, .yaml/.yml stack settings, qualified names, and no subprocess when on-save preview is disabled.
- [X] T048 [US2] Connect document/file/config watchers and generation snapshots in `packages/vscode-finfocus/src/editor/staleTracking.js` and `packages/vscode-finfocus/src/extension.js`; update stale/unknown annotation styling and accessible reasons in `packages/vscode-finfocus/src/editor/annotations.js` without launching pricing.
- [X] T049 [US2] Run stale tests and measure saved/unsaved notification latency at <=1 s with zero overview spawns using `packages/vscode-finfocus/test/editor/stale.test.js`; record initial-load, refresh-race, and config-change evidence in `specs/2017-vscode-cost-extension/validation.md` (SC-003).
- [X] T050 [US2] Document file-granularity limits, unknown freshness, mtime behavior, config invalidation, and state-only-refresh semantics in `docs/src/content/docs/guides/editor-extension.md` and `packages/vscode-finfocus/README.md`.

### Phase 12: User Story 3 — Price edits on demand (P1)

**Goal and checkpoint**: Independent test: preview reprices edited/new resources and clears only covered unchanged generations; failed/cancelled previews retain figures and reasons, and all owned descendants terminate within the shutdown deadline.

- [X] T051 [P] [US3] Write failing scheduler tests in `packages/vscode-finfocus/test/unit/scheduler.test.js` for latest-request/selection epochs, one overview per context, superseding saves, cleanupFailed, dirty snapshot semantics, late results, and prompt invalidation.
- [X] T052 [P] [US3] Write failing preview/passphrase adapter tests in `packages/vscode-finfocus/test/editor/preview.test.js` for progress/cancel, Save all and run/Run saved files/Cancel, failed saves, on-save off/default, other dirty documents, wrong/empty/dismissed passphrases, and retaining prior stale figures on failure.
- [X] T053 [P] [US3] Write real extension-side shutdown tests in `packages/vscode-finfocus/test/integration/shutdown.test.js` with owned child/grandchild fixtures under `packages/vscode-finfocus/test/fixtures/process-tree/`, including a grandchild in its own process group (as plugins are) that the session sweep must still reach and a descendant left behind after a clean exit; include ignored termination, escalation, deactivation, selection change, removal, cleanup failure, and unrelated-process survival.
- [X] T054 [US3] Extend `packages/vscode-finfocus/src/runner.js` with owned shutdown per research R4: on Unix, spawn finfocus as a session leader (`detached: true`) and sweep the session on escalation and after every exit (`/proc/<pid>/stat` on Linux, `/usr/bin/pkill -s`/`pgrep -s` on macOS); on Windows, `taskkill /PID <pid> /T /F`. 5 s grace, escalation, 2 s confirmation, and cleanup-failure blocking; never terminate by image name or allow an unconfirmed replacement.
- [X] T055 [US3] Implement `packages/vscode-finfocus/src/scheduler.js` for per-context runs, request IDs/selection epochs, supersession, dirty/config generations, stale-preserving failures, and session-only credential handoff; connect teardown promises and invalidate late output/prompts.
- [X] T056 [US3] Implement `packages/vscode-finfocus/src/editor/passphrase.js` and `packages/vscode-finfocus/src/editor/preview.js` for progress/cancel, context-scoped in-memory passphrases, empty values, explicit retry/dismissal, and dirty-document choices; capture generations only after requested saves succeed.
- [X] T057 [US3] Wire Run preview, cancel, on-save settings/watchers, context switching, workspace removal, and asynchronous deactivate in `packages/vscode-finfocus/src/extension.js` and `packages/vscode-finfocus/package.json`; dispose timers/watchers/prompts and clear removed-context credentials (FR-013/014/020/028).
- [ ] T058 [US3] Run the shutdown and preview tests in the Linux/macOS/Windows jobs of `.github/workflows/test-vscode-extension.yml`, including real finfocus plus plugin descendants and saved-edit/in-flight-edit preview scenarios; record PID cleanup, deadlines, preserved marks, and CLI delta parity in `specs/2017-vscode-cost-extension/validation.md`. *(2026-11-18: the fixture-based Node shutdown tests pass on Linux, macOS and Windows in CI after ISS-003. The real-CLI shutdown tests with plugin descendants skipped on every OS because the job sets no `FINFOCUS_BINARY`, and that file skips on Windows regardless; this task stays open for them.)*
- [X] T059 [US3] Document preview/dirty-file choices, on-save default, passphrase lifecycle, cancellation deadlines, and cleanup errors in `docs/src/content/docs/guides/editor-extension.md` and `packages/vscode-finfocus/README.md`.

### Phase 13: User Story 4 — Dashboard beside the code (P2)

**Goal and checkpoint**: Independent test: summary, budget, top five, complete grouped tree and status match CLI output; keyboard details and navigation work with the card disabled.

- [X] T060 [P] [US4] Write failing dashboard projection tests in `packages/vscode-finfocus/test/unit/dashboard.test.js` for summary/budget strings, top-five numeric ordering without money arithmetic, all-resource/type/unpriced groups, row errors, and stale/unknown counts.
- [X] T061 [P] [US4] Write failing view adapter tests in `packages/vscode-finfocus/test/editor/dashboard.test.js` for card/tree/status updates, source navigation, absent location guidance, keyboard details exposing every hover field/reason, and a deliberately broken webview fallback.
- [X] T062 [US4] Implement pure dashboard projections in `packages/vscode-finfocus/src/dashboard.js` using CLI display strings only; select the top five by numeric cost without summing/reformatting and retain the complete resource/error list.
- [X] T063 [US4] Implement `packages/vscode-finfocus/src/editor/treeView.js` with grouped/unpriced resources, full accessible labels, stale/unknown reasons, keyboard resource details and safe declaration navigation; implement `packages/vscode-finfocus/src/editor/statusBar.js` with total/budget text and reveal action.
- [X] T064 [US4] Implement primary-sidebar `packages/vscode-finfocus/src/editor/summaryView.js` and `packages/vscode-finfocus/media/summary.html`, `summary.css`, and `summary.js` with strict CSP, no remote content, escaped resource text, validated resource actions, summary/budget/top-five/stale text and preview control.
- [X] T065 [US4] Register the activity-bar container, card/tree views and resource-detail commands in `packages/vscode-finfocus/package.json` and wire providers in `packages/vscode-finfocus/src/extension.js`; keep tree/status functional independently of webview loading.
- [X] T066 [US4] Run the automated accessibility and safety checks: keyboard-only discovery, stack selection, preview/cancel, tree navigation and resource details through editor tests; malicious resource text rendering; missing locations; webview failure; record results in `specs/2017-vscode-cost-extension/validation.md` (FR-017/018/027).
- [ ] T067 [owner] [US4] Verify NVDA on Windows and VoiceOver on macOS, high contrast, zoom, and visible focus; record exact assistive-tool and editor versions in `specs/2017-vscode-cost-extension/validation.md` (FR-027).
- [X] T068 [US4] Update dashboard, budget/no-budget, accessible-details, and fork fallback guidance in `docs/src/content/docs/guides/editor-extension.md` and `packages/vscode-finfocus/README.md`.

### Phase 14: User Story 5 — Install in VS Code forks (P2)

**Goal and checkpoint**: Independent test: build one VSIX and run Stories 1–4 with the identical SHA-256 package in each of the twelve editor/OS cells, plus minimum-API and remote-host checks.

- [X] T069 [P] [US5] Add packaging/release assertions in `packages/vscode-finfocus/test/unit/manifest.test.js` for engines ^1.90.0, publisher `finfocus`, workspace extension kind, trust limits, package metadata, no runtime dependencies, no `enabledApiProposals`, minFinfocusVersion, a PNG icon, and excluded test/secret artifacts.
- [X] T070 [US5] Configure release-please component vscode-finfocus in `release-please-config.json` with node release type, `include-component-in-tag`, `tag-separator` `-`, `changelog-path` `CHANGELOG.md`, and `initial-version`, matching the plugin packages; add `packages/vscode-finfocus` to the root package's `exclude-paths`; do not add it to `.release-please-manifest.json` (FR-024a).
- [X] T071 [US5] Create `.github/workflows/release-vscode-extension.yml` to reproducibly package and attach one VSIX for vscode-finfocus-v* releases and record its SHA-256, without store credentials. Validate store acceptance concretely: `vsce package` succeeds with no `--allow-*` override, and the manifest meets the Open VSX publish rules (license file, publisher usable as a namespace). Skip the vscode-finfocus- prefix in `.github/workflows/release-monorepo-plugin.yml`, whose non-`v` tag filter would otherwise build it as a Go plugin (FR-024).
- [ ] T072 [US5] Build the VSIX using `packages/vscode-finfocus/package.json`, inspect packaged contents, and run the adapter suite on VS Code API 1.90 plus current VS Code in CI; confirm `minFinfocusVersion` against the Part A release; block the release if Python or .NET location fixtures from T007 are still pending; record in `specs/2017-vscode-cost-extension/validation.md` (FR-025).
- [ ] T073 [owner] [US5] Install identical VSIX bytes on VS Code/Cursor/Windsurf/VSCodium across Linux/macOS/Windows and execute Stories 1–4 including tree fallback and cancel; record all twelve cell results, actual editor/API versions, OS/architecture and package hash in `specs/2017-vscode-cost-extension/validation.md`, and replace research R7's reported API levels with the observed ones; every observed level must be at or above `engines.vscode` (SC-004, FR-025).
- [ ] T074 [owner] [US5] Verify WSL, SSH and dev-container hosts with discovery, CLI execution and location resolution on the workspace host; record one passing run per host type in `specs/2017-vscode-cost-extension/validation.md`. A failure returns to an agent task that repairs the host-specific adapter.
- [X] T075 [US5] Document VSIX installation in all four editors, minimum versions, remote prerequisites and package release policy in `packages/vscode-finfocus/README.md` and `docs/src/content/docs/guides/editor-extension.md`; keep publishing/identity/token follow-up out of this feature.

### Phase 15: User Story 6 — Guidance when prerequisites are missing (P3)

**Goal and checkpoint**: Independent test: no binary, old binary, missing plugins/Pulumi, and non-Pulumi folders produce actionable guidance or silence as specified, without repeated automatic attempts.

- [X] T076 [P] [US6] Write failing version/readiness tests in `packages/vscode-finfocus/test/unit/version.test.js` for semver minimum, old versions, dev/unparsable warning, missing binary, settings changes, and no repeated attempts after failure.
- [X] T077 [P] [US6] Write failing first-run adapter tests in `packages/vscode-finfocus/test/editor/readiness.test.js` for install/path guidance, unavailable plugins, missing Pulumi, no stacks, malformed discovery, untrusted workspaces, silent non-Pulumi activation, and a workspace whose only `Pulumi.yaml` is under `node_modules` (activates, discovery ignores it, nothing runs or shows).
- [X] T078 [US6] Implement `packages/vscode-finfocus/src/version.js` with --version checking against declared minFinfocusVersion and dev-version warnings; add readiness state in `packages/vscode-finfocus/src/editor/readiness.js` that pauses missing-binary retries until settings change.
- [X] T079 [US6] Connect prerequisite diagnostics, install documentation links and finfocus.path configuration in `packages/vscode-finfocus/src/extension.js` and `packages/vscode-finfocus/package.json`; preserve sanitized CLI failure reasons, never log secrets/full commands, and keep non-Pulumi workspaces silent.
- [X] T080 [US6] Run first-run/readiness scenarios and add troubleshooting for CLI/plugins/Pulumi/version/credentials to `docs/src/content/docs/guides/editor-extension.md`; record outcomes in `specs/2017-vscode-cost-extension/validation.md`.

### Phase 16: Polish and cross-cutting verification

**Goal and checkpoint**: All required stories, documentation and evidence must pass before feature completion; no acceptance gate is waived by a missing local tool, and `[owner]` gates stay visibly open until the owner records them.

- [X] T081 Create a deterministic 300-resource fixture and repeatable 60-second edit workload in `packages/vscode-finfocus/test/fixtures/performance/` and `packages/vscode-finfocus/test/editor/performance.test.js`; implement real timing/profile capture rather than assertion-free smoke tests.
- [X] T082 Take the SC-002 evidence from T099 (do not repeat it), and execute saved/unsaved stale latency (<=1 s, no spawn when previewOnSave is off), and preview typing/profile comparisons (no attributable pause >100 ms); record raw timings, hardware, versions, traces and cold-start results in `specs/2017-vscode-cost-extension/validation.md` (SC-002/003/006).
- [X] T083 Exercise secret/error/path safety through the extension tests; verify resource text cannot inject markup/commands, messages carry no passphrase, environment value, or full command line, and each required inventory field mutation fails the parser tests (FR-021, SC-005).
- [X] T084 Finalize `docs/src/content/docs/guides/editor-extension.md` (with guide navigation following repository docs conventions), `packages/vscode-finfocus/README.md`, root `README.md`, and `CLAUDE.md` with implemented examples; leave `packages/vscode-finfocus/CHANGELOG.md` to release-please.
- [X] T085 Run quickstart §2–§5 against the created fixtures and package and correct any drift; reconcile all acceptance evidence in `validation.md` and all interface descriptions in `contracts/` with actual implementation.
- [ ] T086 Run make lint, make test, make test-race, make validate, make docs-lint, govulncheck, and the extension minimum-API/current editor jobs through `Makefile` and `.github/workflows/test-vscode-extension.yml`; verify >=80% overall and >=95% critical-path test coverage and >=80% Go docstring coverage, and record results in `specs/2017-vscode-cost-extension/validation.md`.
- [X] T087 Reconcile every FR/SC and validation gate with actual test/artifact evidence in `specs/2017-vscode-cost-extension/validation.md`, update `spec.md` status and completed `tasks.md` checkboxes only for verified work, check the T026 issue URL, and leave any unavailable or `[owner]` gate visibly incomplete.

### Phase 17: Result reuse and completeness (ISS-001, FR-012a, FR-012b)

**Goal and checkpoint**: Reloads and reopens read a valid cached deployed-state
result without starting finfocus (SC-002 cache-hit criterion), and a partial
result under load is marked incomplete and never cached. Independent test:
with a populated cache, activation paints annotations and the card with zero
finfocus spawns; a changed `lastUpdate`, config hash, finfocus version, an
expired TTL, or an explicit Refresh runs finfocus; a result whose gap set
differs between two runs shows the incomplete marker and writes no cache entry.
Order: T091, T092, and T095 finish before T096; T096 before T097 and T098.
Web Parity: Not applicable (extension-only; no core or `internal/webui` change,
see spec Web UI Fit).

- [X] T088 [P] [US1] Write failing cache tests in `packages/vscode-finfocus/test/unit/cache.test.js` for key composition (context key, `lastUpdate`, `resourceCount`, stack-config hash, project-config hash, finfocus version, state-only), a miss on each field changing, TTL expiry, `ttlMinutes` 0 disabling, no entry for a stack without `lastUpdate` or with `updateInProgress`, an entry whose stdout fails `parseOverview` counting as a miss, preview results never stored, no passphrase or environment value in a stored entry, and no read or write while the workspace is untrusted (FR-012a, FR-022).
- [X] T089 [P] [US1] Write failing completeness tests in `packages/vscode-finfocus/test/unit/completeness.test.js` with fixtures in `packages/vscode-finfocus/test/fixtures/completeness/` (a complete result, a 60-of-61 partial result modeled on validation.md Finding 1b, a result with stable no-cost-data gaps, and a `deleting` row) for the unpriced-row (gap) rule (`status` not `deleting` and `display.projected` equal to `-`), the three outcomes `complete`, `confirmed`, `incomplete`, equal gap sets across two runs, differing sets keeping the run with fewer gaps, a baseline superset triggering a retry, no retry when there are no gaps, and the documented limit: the same gaps on both runs are `confirmed`, never `incomplete` (FR-012b).
- [X] T090 [P] [US1] Extend `packages/vscode-finfocus/test/unit/scheduler.test.js` with failing cases: a cache hit starts no process; explicit Refresh bypasses the cache; a preview request never reads or writes it; a state-only result with gaps and no baseline runs once more through the same supersede and cancel rules; an `incomplete` result is published but not stored; a `confirmed` result is stored with its gap set as the baseline; a hit keeps edit and config stale marks as a state-only refresh does.
- [X] T091 [US1] Implement `packages/vscode-finfocus/src/cache.js` (pure: key, validity, TTL, entry shape per data-model.md CacheEntry) so T088 passes.
- [X] T092 [US1] Implement `packages/vscode-finfocus/src/completeness.js` (gap rule and stability rule per plan.md) so T089 passes; add the `-` placeholder and the non-`deleting` status rule to `packages/vscode-finfocus/contract/overview-fields.json` under a new `placeholders` key and read them in `packages/vscode-finfocus/src/result.js`, with a mutation case in `packages/vscode-finfocus/test/unit/result.test.js` proving a changed placeholder fails.
- [X] T093 [P] [US1] Make `internal/cli/overview_json_contract_test.go` assert against the real overview JSON that an unpriced row prints the pinned `placeholders` value in `display.projected` (the struct there ignores unknown keys, so extend its inventory type); keep the extension and core tests reading the one file (FR-026). This is a test-only core change; it lands in the Part A branch while Part A is open, otherwise in the Part B pull request.
- [X] T094 [P] [US1] Write failing store tests in `packages/vscode-finfocus/test/unit/cacheStore.test.js` using a temp directory for atomic write, `0600` mode on Unix, on Windows only that the file sits under the storage directory (no ACL is set), corrupt-file deletion, a half-written temp file ignored, and pruning of entries untouched for 30 days.
- [X] T095 [US1] Implement `packages/vscode-finfocus/src/editor/cacheStore.js` over `context.globalStorageUri` (changed from `storageUri` after the web run: workspace storage is per-host locked) so T094 passes.
- [X] T096 [US1] Read `lastUpdate`, `resourceCount`, and `updateInProgress` in `packages/vscode-finfocus/src/projects.js` (extend `packages/vscode-finfocus/test/unit/projects.test.js` and the recorded `pulumi stack ls --json` fixture in `packages/vscode-finfocus/test/fixtures/projects/`); hash the stack settings and project config files; wire the cache and completeness into `packages/vscode-finfocus/src/scheduler.js`; wire the store, the Refresh bypass, and the `finfocus.cache.ttlMinutes` setting (default 60, minimum 0) in `packages/vscode-finfocus/src/extension.js` and `packages/vscode-finfocus/package.json`.
- [X] T097 [US4] Write failing projection and adapter tests, then implement the incomplete marker ("incomplete: N of M resources unpriced" beside the total, never shown as complete) and the confirmed-gap note ("N resources unpriced") in `packages/vscode-finfocus/src/dashboard.js`, `packages/vscode-finfocus/src/editor/summaryView.js`, `packages/vscode-finfocus/src/editor/statusBar.js`, and `packages/vscode-finfocus/media/summary.js` through `textContent` only, with text plus icon and a keyboard-reachable explanation (FR-027); tests in `packages/vscode-finfocus/test/unit/dashboard.test.js` and `packages/vscode-finfocus/test/editor/dashboard.test.js`.
- [X] T098 [US1] Write editor tests in `packages/vscode-finfocus/test/editor/cache.test.js` for a reopened window painting annotations and the card from a valid entry with no finfocus spawn (fake finfocus counting launches), a changed `lastUpdate` or `Pulumi.<stack>.yaml` invalidating, an expired TTL, explicit Refresh bypassing, an untrusted workspace never reading or showing an entry, a corrupt entry deleted and treated as a miss, and an incomplete result leaving no entry on disk.
- [X] T099 Measure SC-002 on the deployed `initial-30` `dev` stack and record raw timings, machine load, and versions in `specs/2017-vscode-cost-extension/validation.md`: five cache hits (each <=2 s from activation), five idle cache-miss loads (each <=20 s), one cache-miss load that includes the stability retry on a stack or fixture with confirmed gaps (a real stack holding a type with no cost data, or a fake finfocus with fixed delay; total <=20 s), a cache hit on the 300-resource fixture from T081, and one bounded induced-load run (short, `nice`d CPU load, stopped afterwards) showing the incomplete marker, the retry, and that no cache file was written. Do not repeat the unbounded load that produced the 36.8 s outlier. *(Measured 2026-11-18, see validation.md. The marker, retry, and no-write were then shown in a real workbench with a wrapper that makes two runs disagree; a real slow plugin did not reproduce under bounded load, so the owner accepted the modeled-data and simulated-wrapper coverage on 2026-11-18.)*
- [X] T100 Document the cache, `finfocus.cache.ttlMinutes`, the incomplete marker and its known limit in `docs/src/content/docs/guides/editor-extension.md`, `packages/vscode-finfocus/README.md`, and `specs/2017-vscode-cost-extension/quickstart.md`; add the cache and placeholder rules to the extension bullet in `CLAUDE.md`; run markdownlint on every changed file.
- [X] T101 After T099 passes, set ISS-001 to Resolved with the evidence in `specs/2017-vscode-cost-extension/issues.md`, and remove the `[BLOCKED: ISS-001]` markers from T082 and T087.

## Dependencies and execution order

```text
Part A:
T001 planning render -> JSON contract (T002–T005)
JSON contract -> Locations (T006–T012) -> Modification time (T013–T014)
Locations -> Preview coverage and cancellation (T015–T020)
Locations -> Web Parity (T021–T026)
All of Part A -> Part A verification (T027–T029) -> merge -> core release

Part B (stacked on Part A; finalize after the core release):
Setup (T030–T032) -> Extension foundation (T033–T038)
Foundation -> US1 -> US2 -> US3 -> US4
Foundation -> US6 (separate tests/version module may start early)
US1 + US2 + US3 + US4 -> US5 packaged acceptance matrix
US1 + US4 -> Result reuse and completeness (T088–T101)
All stories + Phase 17 -> Polish (T082, T087) -> completion evidence
```

The phase sequence is the default safe execution order. Part B may be
developed against a local build of Part A, but T030's `minFinfocusVersion`,
T043, and T072 need the released core version. US6 is listed after the P2
stories to preserve specification priority, although its isolated version
work may run earlier once the extension foundation is ready. Wiring in
extension.js, package.json, runner.js, shared Go files, and validation.md must
serialize; finish the earlier owner before another task edits those files.
US2 needs US1's locations; US3 needs US2's snapshots and stale reasons; US4
needs their results/progress/reasons. US5's matrix requires Stories 1–4.
Each story has its own executable checkpoint; independent testing does not
mean these real data dependencies can be bypassed.

Within each phase: failing tests, then implementation, then adapter/integration
verification and concurrent documentation. Test tasks sharing a Go file run
sequentially. Complete the no-location and no-budget branches as real behavior.
The Web Parity issue task is the last task of that phase and only files UX
debt after core/API correctness is proven.

## Parallel execution examples

These are scheduling examples, not instructions to start agents now.
All examples wait for the prior phase and any named prerequisite.

- **Part A JSON contract**: T002, T003. Contract tests and formatter parity tests own different files.
- **Part A locations**: T006, T007. Fixture captures own different `testdata/sourcepos/` subdirectories; T008 waits for both.
- **Part A cancellation**: T015, T016. Preview tests and process tests own different files; T019 and T020 touch disjoint OS-specific files once T018 lands.
- **Extension foundation**: T033, T034, T035. Parser, project, and runner tests own different files.
- **Phase 17**: T088, T089, T090 (cache, completeness, scheduler tests own different files); T093 and T094 are independent of T091/T092; T096 and T097 wait for T091, T092, and T095.
- **US1**: T039, T040. Pure annotation tests and editor annotation tests; implementation waits for both.
- **US3**: T051, T052, T053. Scheduler, adapter, and shutdown tests own different files; runner/extension integration remains sequential.
- **US4**: T060, T061. Projection and view adapter tests; projection/provider wiring follows both.
- **US5**: T069. Manifest tests are isolated. After a VSIX is built, the twelve editor/OS checks and three remote-host checks can execute concurrently with separate evidence artifacts and one serialized validation.md update.
- **US6**: T076, T077. Version/readiness pure tests and first-run adapter tests; shared extension.js wiring waits for earlier story integration.

## Implementation strategy

1. Close T001's planning gate, then land Part A: the JSON contract, locations
   for every recorded runtime, `modifiedAt`, preview coverage, descendant
   ownership on all three OSes, and Web Parity. Release it.
2. Build Part B on top. Implement US1 as the first internal demo:
   deployed-state annotations from deterministic pricing, with
   untrusted/non-Pulumi behavior already enforced.
3. Treat US1–US3 together as the usable P1 MVP: annotations, truthful freshness,
   explicit preview, passphrases and bounded cleanup. US1 alone is an internal
   checkpoint because edited code without stale/preview support is misleading.
4. Add dashboard/accessibility, package/release support, and first-run
   guidance. Run each story's checkpoint rather than waiting for the end.
5. Hand `[owner]` gates off with prepared commands, and complete all
   runtime/editor/OS/remote/performance evidence before claiming the full
   feature complete. Store publishing, Terraform, YAML source parsing, and
   agent integration remain out of scope.

## Task summary

| Scope | Task IDs | Count |
| --- | --- | --- |
| A: Planning gate | T001 | 1 |
| A: JSON contract and display text | T002–T005 | 4 |
| A: Declaration locations | T006–T012 | 7 |
| A: Modification time | T013–T014 | 2 |
| A: Preview coverage and cancellation | T015–T020 | 6 |
| A: Web Parity | T021–T026 | 6 |
| A: Part A verification | T027–T029 | 3 |
| B: Setup | T030–T032 | 3 |
| B: Extension foundation | T033–T038 | 6 |
| B: US1 | T039–T044 | 6 |
| B: US2 | T045–T050 | 6 |
| B: US3 | T051–T059 | 9 |
| B: US4 | T060–T068 | 9 |
| B: US5 | T069–T075 | 7 |
| B: US6 | T076–T080 | 5 |
| B: Polish and cross-cutting verification | T081–T087 | 7 |
| B: Result reuse and completeness (ISS-001) | T088–T101 | 14 |
| **Total** | T001–T101 | **101** |
