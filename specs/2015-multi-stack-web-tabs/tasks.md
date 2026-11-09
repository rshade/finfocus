# Tasks: Multi-Stack Web Dashboard with a Tab Bar

**Input**: Design documents in `specs/2015-multi-stack-web-tabs/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http-api.md

Tests come first in each phase (Constitution Principle II). Use testify in Go
tests. Parallel Go tests follow the rules in `CLAUDE.md`. Every task is a full
implementation (Principle VI).

## Phase 1: Setup

- [X] T001 Read `internal/webui/static/app.js` `OverviewStream` (lines 180-397) and `internal/webui/frontendtest/stream.test.mjs` and `base.test.mjs` to confirm how the harness fakes `document`, `fetch`, and `EventSource`; reuse that fake in the new `internal/webui/frontendtest/stacks.test.mjs` (no code change).

## Phase 2: Foundational (blocks all stories)

- [X] T002 [P] Write failing tests in `internal/webui/stacks_test.go` for `ValidStackName` (exported wrapper of `validStackName`), `StackOptions.Activate` (called once across concurrent activations), and the entry state derivation (`idle` before activation; afterward `locked` when a passphrase prompt is pending, else `failed` when a failure is recorded and the pipeline has ended, `ready` from `Session.IsReady`, else `loading`). Include a recorded failure with a pending prompt (must read `locked`) and a recorded failure while the pipeline still runs after a passphrase submit (must read `loading`).
- [X] T003 Export `ValidStackName` and add `Activate func()` to `StackOptions`, with a per-stack `sync.Once`, an `activated` flag, and registration-ordered `stackOrder` in `internal/webui/stacks.go`; add `(*Session).SetPipelineEnded()`, `(*Session).Failed() bool` (true when a phase failure is recorded and the pipeline has ended), and `(*Session).PassphraseRequired() bool` in `internal/webui/session.go`; `stackEntries` checks locked, then ready, then failed (research R3, data-model), and `SetPipelineEnded` publishes a snapshot when a failure is recorded.
- [X] T004 [P] Write failing tests in `internal/webui/handlers_stacks_test.go`: `GET /api/stacks` returns `{"stacks":[]}` with none registered and ordered entries with states (`idle`, `loading`, `locked`, `ready`, `failed`) otherwise; `POST /api/stacks/{name}/activate` returns 202, is idempotent, runs `Activate` once, 404 for an unknown or invalid name; both need authentication and Origin hardening (401/403 as other routes); neither exists under `/s/<name>/`.
- [X] T005 Implement `internal/webui/handlers_stacks.go` (`registerStacks` called from `buildMux` only when `s.parent == nil`; `stackEntries`; `activateStack`) per `contracts/http-api.md`, and the root shell's `finfocus-stacks` meta tag (`stacksMeta`, written by `handleRoot` for two or more stacks).
- [X] T006 [P] Write a failing test in `internal/cli/web_stack_test.go` that two calls to `webStack.start` start the pipeline once and that `stop` on a never-started stack still returns. Keep `newWebStack` free of an error return and of plugin starts (spec FR-009); a test asserts building a stack calls no pipeline `Run`, and that `start` calls `Session.SetPipelineEnded` after `Run` returns, including when `Run` returns an error.
- [X] T007 Make `webStack.start` idempotent with a `sync.Once` in `internal/cli/web_stack.go`, and call `s.session.SetPipelineEnded()` in its goroutine after `pipe.Run` returns.

**Checkpoint**: server can list, activate, and lazily start stacks.

## Phase 3: User Story 6 - One stack behaves as today (P1)

**Goal**: the flag becomes a list with no change for zero or one name.
**Independent Test**: the existing `web_test.go` and `web_single_stack_test.go` pass after only the flag-type pins change.

- [X] T008 [P] [US6] Write failing tests in `internal/cli/web_multistack_test.go` for `normalizeStackList`: `a,b`, repeats, whitespace trimming, duplicates collapse in first-seen order, `a,,b`, `a,`, `--stack a --stack ""`, and an invalid name return an error, a lone empty value returns nil (no stack, as today), and nil for none.
- [X] T009 [US6] Change `stack` to `[]string` in `webFlagValues`, register it with `StringArrayVar` (like `--allowed-host`, so an empty value reaches `normalizeStackList`; research R1), and update `webFlagsFromOverview` in `internal/cli/web.go`; add `normalizeStackList` in new `internal/cli/web_multistack.go`; single or empty list sets `params.stack` to that name or `""` and runs the existing path.
- [X] T010 [US6] Update the pins in `internal/cli/web_test.go` (companion flags, `TestWebCompanionsRequireWeb`, `TestWebPrintsURLOnPinnedPort`, `TestWebAutoDetectParity`, `TestWebEncryptedStackRetriesWithoutEcho`, `--stack dev`) and `internal/cli/web_stack_parity_test.go` for the slice type; regenerate `internal/cli/testdata/schema-root.golden.json` with `UPDATE_GOLDEN=1`.
- [X] T011 [P] [US6] Write failing frontend test in `internal/webui/frontendtest/stacks.test.mjs`: with `{stacks: []}` or one entry, no tablist is built, no header stack label is shown, `#/<view>` addresses are kept, and `base()` is unchanged.

## Phase 4: User Story 1 - Several stacks in one dashboard (P1) and Story 2 - Lazy load (P1)

**Goal**: `--stack=a,b` serves two tabs; only the selected stack starts.
**Independent Test**: two stacks, switch tabs, check data, header, address, and that the second stack starts only on selection.

- [X] T012 [P] [US1] Write failing tests in `internal/cli/web_multistack_test.go` for `validateStackList`: rejects with exit 2 when combined with `--pulumi-json` or `--pulumi-state`, for a Terraform project, and for a name absent from a stubbed `pulumi.ListStacks` (message lists the available names, via `pulumi.ValidateStacks`).
- [X] T013 [US1] Implement `validateStackList` in `internal/cli/web_multistack.go`, returned through `toValidationError`, and call it from `runWebSession` before any server starts; `checkWebProjectSource` runs per name.
- [X] T014 [US1] Implement `runWebStacks` in `internal/cli/web_multistack.go`: per name copy `params` with `stack` set, `newWebStack`, an empty root `webui.Session`, `listenAndPrintWeb` on that root, `srv.AddStack(name, StackOptions{Session: ws.session, Activate: ws.start})`, `ServeUntilInterrupt`, then `stop()` on every stack; audit `RowsEnriched` is the sum over started stacks. Route `runWebSession` to it when two or more names remain, and delete the interim validation error `runWebSession` returns for two or more names (added with T009 so a list is never silently reduced to one stack).
- [X] T015 [P] [US2] Write a failing integration-style test in `internal/cli/web_multistack_test.go` (fake pipeline seam `webPipelineHook`) proving only the activated stack's pipeline runs, shutdown with a never-started stack completes, reselecting an activated stack starts no second pipeline run and serves the retained snapshot (US2 AC3), and the audit `RowsEnriched` total is the sum over started stacks only.
- [X] T016 [P] [US1] Add failing frontend tests in `internal/webui/frontendtest/stacks.test.mjs`: with three entries the tablist renders in order; parsing `#/s/b/cost` selects `b`; a hash without a stack or with an unknown stack redirects to the first stack's same view; `base()` returns `/s/<active>`; `/api/stacks` and activate calls bypass the stack base; the header label shows the active stack's name (FR-010); every tab link is fragment-only (`#/s/...`), with no absolute URL (Tailscale edge case).
- [X] T017 [US1] Implement in `internal/webui/static/app.js`: `{ root: true }` option on `request`; stack names read synchronously from the shell's `finfocus-stacks` meta tag (root page only, no request at boot; research R8); `activeStack` and multi mode; `routeFromHash` returning `{stack, view}`; `ctx.navigate` producing `#/s/<active>/<name>`; `base()` returning `/s/<active>` in multi mode; keep legacy behavior for fewer than two stacks.
- [X] T018 [US1] Add `stop`-and-reset to `OverviewStream` in `internal/webui/static/app.js` (close the source, clear remembered events and timers) and, on a stack change, run stop, reset, `POST /api/stacks/<name>/activate` (root), start, then mount the view; exactly one stream at a time (SC-002). Add a frontend test for this order in `stacks.test.mjs`.
- [X] T019 [US1] Add the tablist container and the header stack label to `internal/webui/static/index.html` (hidden until multi mode) and styles to `internal/webui/static/styles.css` using existing header tokens; show the active stack name in the label.

**Checkpoint**: Stories 1, 2, and 6 work end to end.

## Phase 5: User Story 3 - A broken stack keeps its tab (P1)

**Independent Test**: one failing and one healthy stack; the failing tab shows the error, the other loads.

- [X] T020 [P] [US3] Write failing Go test in `internal/cli/web_multistack_test.go` where one stack's loader fails: `GET /api/stacks` reports `failed` for it and `ready` for the other, and the other stack's stream is unaffected. Add a case where the failing stack's `pipe.Run` returns an error at start: it reaches that stack's `Session.Error` with the safe label, the server keeps serving, and the other stack loads (spec FR-009, research R3a).
- [X] T021 [P] [US3] Write failing frontend tests in `stacks.test.mjs`: after the stream emits `error`, the entry refetches from `/api/stacks` and the tab shows a failed marker (and a locked marker for `locked`); the refresh also runs on `passphrase_required` and `snapshot`, and a stream that emits `error` and then `passphrase_required` ends with the locked marker, not failed; the view region shows the stack's own error through the existing view (no skipped tab).
- [X] T022 [US3] Implement the refresh of stack states in `internal/webui/static/app.js` on stream `error`, `passphrase_required`, `snapshot`, `ready`, and tab change, and the failed and locked markers (text and `aria` label, not color alone) in `app.js` and `styles.css`.

## Phase 6: User Story 4 - Per-stack passphrase (P2)

- [X] T023 [P] [US4] Write a failing Go test in `internal/cli/web_multistack_test.go` with two encrypted-stack fakes: a passphrase submitted through one stack's session reaches only that stack's pipeline, and the other keeps waiting (extends spec 2013's channel isolation to the registered server). Add the rejected-passphrase case: after a wrong passphrase `GET /api/stacks` reports `locked`, not `failed`; right after the next submit, while the pipeline still runs, it reports `loading`, not `failed`; after the correct one it reports `ready` (`Session.Ready` clears the recorded failure).
- [X] T024 [P] [US4] Write a failing frontend test in `stacks.test.mjs`: a `passphrase_required` snapshot after switching back to a stack re-opens the prompt for that stack, and the prompt submit goes to `/s/<stack>/api/...`.
- [X] T025 [US4] Fix whatever the tests T023 and T024 expose in `internal/webui/static/app.js` and `internal/webui/static/views/overview.js` (the prompt must read the active stack's snapshot after reset); no new server logic expected.

## Phase 7: User Story 5 - Keyboard and ARIA (P2)

- [X] T026 [P] [US5] Write failing frontend tests in `stacks.test.mjs`: `role=tablist` with `aria-label`, each tab `role=tab`, `aria-selected`, roving `tabindex`, `aria-controls` pointing at `#view`; `#view` gets `role=tabpanel` and `aria-labelledby` in multi mode only; Left/Right wrap, Home/End jump, and each moves selection.
- [X] T027 [US5] Implement the tablist semantics and key handling in `internal/webui/static/app.js`.
- [X] T028 [P] [US5] Extend `internal/webui/frontendtest/route_focus.test.mjs` to prove a tab switch obeys the "focus the user already moved is not stolen" rule and the new `#view-announcer` text on failure.

## Phase 8: Web Parity

- [X] T029 Confirm the contract in `specs/2015-multi-stack-web-tabs/contracts/http-api.md` names `(*webui.Server).stackEntries`, `activateStack`, `(*webui.Session).Failed`, `(*webui.Session).PassphraseRequired`, `(*cli.webStack).start`, `pulumi.ListStacks`, and `pulumi.ValidateStacks` as implemented; fix names that drifted.
- [X] T030 Run `finfocus --web --no-browser --port <port> --stack=<two stacks>` from `../finfocus-demo` (empty `PULUMI_CONFIG_PASSPHRASE`) and render it with Playwright: tabs, header label, a switch, a failing stack name, keyboard use; compare with the spec's Web UI Fit; store screenshots under `.playwright-mcp/` and record the result in the PR. Stop the server with an anchored `pkill -f`.
- [X] T031 Add a two-stack Playwright e2e in `test/e2e/` (fixture stacks via the existing web e2e helpers) and run `make test-e2e-web`.
- [X] T032 Update `docs/src/content/docs/architecture/web-ui.md` (tab bar slot, stack list endpoints, one stream), the web guide, and the CLI reference for the `--stack` list form, the five tab states, and that a failed stack stays failed until the dashboard restarts; run `make docs-lint`.
- [X] T033 Not applicable, no issue filed: this feature is itself the `web-ui` work (#2039), so Principle VII's follow-up, which exists to keep web work out of CLI features, has nothing to carry. Any real polish found later gets its own issue then. Original text: File one `web-ui` follow-up issue (UX only, per the spec's Web UI Fit scope: tab bar overflow and wrapping with many or long stack names, tab marker copy, keyboard and screen-reader polish from the T030 render), with no engine, API, redaction, or data-shape work, and link it in the PR body.

## Phase 9: Polish

- [X] T034 [P] Add the multi-stack notes to `CLAUDE.md` (Key Patterns or the web paragraph): lazy start through `POST /api/stacks/{name}/activate`, root-only routes, tabs only for two or more stacks, one stream, the five tab states (`locked` before `failed`), and `--stack` as `StringArray` split on commas. Also record that the root `--stack` is a list while subcommand `--stack` flags are strings, so a read of the root flag must use `stackFlagName` (a `GetString` on the root returns an error and yields an empty name, which made `--import-cloud-history --stack dev` ignore the stack), and that `--import-cloud-history` rejects a list.
- [X] T035 [P] Run `go test -race -shuffle=on -count=3 ./internal/cli/... ./internal/webui/...` and the frontend harness (`node --test internal/webui/frontendtest/`).
- [X] T036 Run `make validate`, `make test`, `make lint`; run the quickstart steps 1-5 by hand.
- [X] T037 Review the diff against the base commit with the `code-review` skill and fix substantiated findings; update the spec Status to Implemented.

## Dependencies

- Phase 2 blocks everything. T003 before T005; T007 before T014.
- US6 (T008-T011) before US1 (the flag type is used by T013, T014).
- US1/US2 (T012-T019) before US3, US4, and US5, which extend the same `app.js` and tests.
- Web Parity (T029-T033) after all stories; Polish last.

## Parallel Opportunities

- T002, T004, T006 (different test files) run together.
- T008 and T011; T012, T015, T016 are separate test files.
- T020/T021, T023/T024, and T026/T028 are test-writing pairs on different files.

## Implementation Strategy

MVP is Phases 2 to 4 (list, lazy start, tab bar). Then US3, US4, US5, then
parity and polish. Commit only when the user asks.
