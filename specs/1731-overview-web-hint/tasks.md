# Tasks: Overview Web Hint

**Input**: Design documents from `specs/1731-overview-web-hint/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/tui-contract.md, quickstart.md

**Tests**: Mandatory and written first (Constitution Principle II). TUI changes
carry golden snapshots and a rendered inspection (quickstart). Testify only:
`require` to stop, `assert` to continue.

**Completeness**: No stubs, no TODO comments (Principle VI).

**Web Parity**: The spec's Web UI Fit is `Not applicable`, so there is no
`internal/webui` hook, render check, or `web-ui` follow-up issue.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an
  incomplete task)
- **[Story]**: User story from spec.md

## Phase 1: Setup

- [X] T001 Run `go test ./internal/tui/... ./internal/cli/...` on the branch and record which tests already fail, so later failures are attributable (the failure set, not the count; see CLAUDE.md "Integration Tests"). No file changes
- [X] T002 Build `bin/finfocus` with `make build` and capture baseline plain-text overview output from `../finfocus-demo` (`PULUMI_CONFIG_PASSPHRASE='' bin/finfocus </dev/null`) into the scratchpad, for the byte-equality check in T024

---

## Phase 2: Foundational

No shared blocking work: each story touches its own function. Proceed to the
stories.

---

## Phase 3: User Story 1 - Discover the dashboard from the terminal (Priority: P1)

**Goal**: The interactive overview list view shows a one-line hint that names
`w` and `finfocus --web`, shortened or dropped to fit, never wrapping.

**Independent Test**: Render the list view at 120, 70, 12, and 0 columns and
read the output; the hint is long form, short form, short form or absent as
the width allows, and no line exceeds the width.

### Tests for User Story 1 (write first, confirm they fail)

- [X] T003 [P] [US1] In `internal/tui/overview_view_test.go`, add a table-driven `TestOverviewWebHint_Width` covering widths 120 (long form `[w] web dashboard · finfocus --web`), 20 (short form `[w] web`), 5 (omitted), and 0 (omitted); assert the hint line's `lipgloss.Width` never exceeds the width. Each `t.Run` is parallel
- [X] T004 [P] [US1] In `internal/tui/overview_view_test.go`, add `TestOverviewWebHint_NotInOtherViews` asserting the hint text is absent from the initializing view, the passphrase prompt, the error view, and the detail view
- [X] T005 [P] [US1] In `internal/tui/overview_view_test.go`, add `TestOverviewWebHint_ErrorRowStillShowsHint` asserting the hint renders in the list view when a row carries an error and when the stack is empty, without removing the error text
- [X] T006 [US1] In the file that holds the overview list-view goldens (the one reading `internal/tui/testdata/overview_collapsed_cluster.golden`), confirm which tests own `overview_collapsed_cluster.golden` and `overview_expanded_cluster.golden`; they will need regenerating after T007. No file changes

### Implementation for User Story 1

- [X] T007 [US1] In `internal/tui/overview_view.go`, add `renderWebHint(width int) string` returning the long form, short form, or empty string by `lipgloss.Width`, styled with `SubtleStyle`, with a Godoc comment. Call it from `renderListView` directly after the status bar and before the filter input, appending the line only when non-empty
- [X] T008 [US1] Run `UPDATE_GOLDEN=1 go test -run TestGolden ./internal/tui/...`, then read the diff of every changed `.golden` under `internal/tui/testdata/`; the only difference must be the added hint line. Run `go test ./internal/tui/...` and confirm T003 to T005 pass

**Checkpoint**: The hint shows in the list view and is covered by goldens.

---

## Phase 4: User Story 3 - Agents and scripts see no change (Priority: P1)

**Goal**: Every non-interactive run produces the output and exit code it
produces today, with no new input read.

**Independent Test**: Run the new regression tests plus the existing
plain-output tests; all pass and the T002 baseline equals current output.

### Tests for User Story 3

These pin behavior this feature must not change. They pass before any
implementation and stay green throughout.

- [X] T009 [P] [US3] In `internal/cli/overview_web_handoff_test.go`, add `TestOverviewPlain_NoWebHint` that runs the plain overview path (`overviewInteractive` false: non-TTY writer) against the existing plain-mode fixture used in `internal/cli/overview_test.go` and asserts the output contains neither `[w]` nor `finfocus --web`
- [X] T010 [P] [US3] In `internal/cli/overview_web_handoff_test.go`, add a table-driven `TestOverviewInteractive_NonInteractiveInputs` asserting `overviewInteractive` returns false for `--plain`, `--no-color`, `NO_COLOR`, `TERM=dumb`, a non-terminal writer, and a non-table output format. Env-setting subtests are not parallel and carry `//nolint:paralleltest` with a reason
- [X] T011 [P] [US3] In `internal/cli/overview_web_handoff_test.go`, add `TestOverviewPlain_ReadsNoInput` running the plain path with a stdin reader that fails the test on any `Read`, proving FR-007

### Implementation for User Story 3

- [X] T012 [US3] Run T009 to T011 with `go test -run 'TestOverviewPlain|TestOverviewInteractive_NonInteractive' ./internal/cli/` and confirm they pass against the unmodified plain path. If any fails, fix the test, not the product code; this story adds no production code

**Checkpoint**: Plain-path behavior is pinned.

---

## Phase 5: User Story 2 - Switch to the dashboard with one key (Priority: P2)

**Goal**: `w` in the list view closes the overview, tears down its plugins,
and starts the same web session as `finfocus --web`.

**Independent Test**: Model test shows `w` quits and records the request;
CLI test shows the hand-off calls the web start with the overview's source
values after teardown; the quickstart interactive run shows the web link and
no orphan plugins after Ctrl+C.

### Tests for User Story 2 (write first, confirm they fail)

- [X] T013 [P] [US2] In `internal/tui/overview_model_test.go`, add `TestOverviewModel_WebKey` driving `Update` with a `tea.KeyPressMsg` for `w` in the list view: assert `WebRequested()` is true, the state is `ViewStateQuitting`, and the returned command yields `tea.QuitMsg`
- [X] T014 [P] [US2] In `internal/tui/overview_model_test.go`, add `TestOverviewModel_WebKeyIgnoredWhenTyping` as a table: with the filter input focused, the text box receives `w` and `WebRequested()` stays false; with the passphrase prompt visible, the prompt receives `w` and `WebRequested()` stays false; in the detail view, `WebRequested()` stays false
- [X] T015 [P] [US2] In `internal/cli/overview_web_handoff_test.go`, add `TestWebFlagsFromOverviewParams` asserting the new helper copies `pulumiJSON`, `pulumiState`, `stack`, `fromStr`, `toStr`, `adapter`, and `filter` into a `webFlagValues` with `web` true, `port` 0, and `noBrowser` false
- [X] T016 [US2] In `internal/cli/overview_web_handoff_test.go`, add `TestInteractiveOverview_HandsOffToWebAfterCleanup`: swap the web start through the new package-level function variable (same pattern as `webPipelineHook` in `internal/cli/web.go`), run the hand-off decision with a final model whose `WebRequested()` is true, and assert the web start ran once, received the expected `webFlagValues`, and ran after the supplied plugin cleanup function. Add a second case with `WebRequested()` false asserting the web start is never called. Not parallel (package-level variable); mark `//nolint:paralleltest // swaps the web start seam`
- [X] T017 [US2] In `internal/cli/overview_web_handoff_test.go`, add `TestInteractiveOverview_WebStartErrorPropagates` asserting an error from the swapped web start is returned unchanged, so the exit code matches `finfocus --web` (FR-003)

### Implementation for User Story 2

- [X] T018 [US2] In `internal/tui/overview_model.go`, add the `webRequested bool` field to `OverviewModel`, the exported `WebRequested() bool` accessor with Godoc beside `Err()`, and a `keyW = "w"` constant next to the other key constants in `internal/tui/cost_model.go`
- [X] T019 [US2] In `internal/tui/overview_model.go`, add a `case keyW:` to `handleListKeypress` that sets `m.webRequested = true`, sets `m.state = ViewStateQuitting`, and returns `tea.Quit`
- [X] T020 [US2] In `internal/cli/web.go`, add `webFlagsFromOverview(params overviewParams) *webFlagValues` copying the seven source fields, plus the package-level `startWebSession = runWebSession` function variable with a comment stating it is a test seam
- [X] T021 [US2] In `internal/cli/overview.go`, change `runInteractiveOverviewWithInit` to extract the final-model check into a small function that, after `finishInteractiveOverview` and after the existing `m.Err()` check, calls `startWebSession(cmd, webFlagsFromOverview(params))` when `m.WebRequested()` is true and returns its error wrapped as `running web session: %w`. Keep the audit success and failure calls correct for both paths
- [X] T022 [US2] Run `go test -race -shuffle=on -count=3 ./internal/tui/ ./internal/cli/` and confirm T013 to T017 pass with no data race

**Checkpoint**: `w` works end to end in unit tests.

---

## Phase 6: User Story 4 - Bare finfocus outside a project is unchanged (Priority: P3)

**Goal**: Bare `finfocus` outside a Pulumi project keeps printing help.

**Independent Test**: Run bare `finfocus` in an empty directory; help prints,
exit 0, no wait.

### Tests for User Story 4

- [X] T023 [P] [US4] In `internal/cli/root_test.go`, add or extend a test that calls `runRootDefault` with a temp directory that has no `Pulumi.yaml` and asserts the output contains root help (`Usage:`) and neither `[w]` nor an overview table. If such a test already exists, record its name in the PR and add no duplicate

**Checkpoint**: The boundary is pinned.

---

## Phase 7: Polish & Cross-Cutting

- [X] T024 Verify non-interactive equality: rebuild with `make build`, rerun the T002 commands, and diff against the baseline; the output must be byte-identical (SC-004)
- [X] T025 Verify interactively with `timeout` per `quickstart.md`: hint at wide and narrow widths, `/` then `w` types into the filter, `w` starts the web session and prints the URL, Ctrl+C stops it, and `ps -eo pid,args | grep '[f]infocus-plugin'` is empty (SC-001 to SC-003). Record what was observed in the PR description
- [X] T026 [P] Update `docs/src/content/docs/commands/overview.md`: add `w` to the key table and show the hint line, in the page's existing format
- [X] T027 [P] Add one sentence to the `internal/cli` section of `CLAUDE.md` stating that the overview hands off to `runWebSession` only after plugin cleanup, through the `startWebSession` seam, and that the hint and `w` live inside the interactive model so non-interactive runs never see them. Do not edit `CHANGELOG.md` (release-please)
- [X] T028 Run `make lint`, `make test`, `make validate`, and markdownlint on changed Markdown; fix every finding and do not touch `.golangci.yml`. Lint can exceed five minutes, so use an extended timeout

---

## Dependencies & Execution Order

- Phase 1 first. Phase 2 is empty.
- US1 (Phase 3) and US3 (Phase 4) are independent of each other and of US2.
  US3 adds no production code, so it can run alongside US1.
- US2 (Phase 5) needs no US1 code, but T013 to T017 touch the same two test
  files as US1 and US3, so run those phases' test edits before US2's to avoid
  merge noise in one working tree.
- T021 depends on T018 and T020. T019 depends on T018. T008 must run before
  any other golden run.
- US4 (Phase 6) is independent.
- Polish runs last. T024 and T025 need T007, T019, T021 complete.

### Parallel opportunities

- T003, T004, T005 (same file; write sequentially, run in parallel)
- T009, T010, T011
- T013, T014, T015
- T026 and T027

## Implementation Strategy

**MVP**: Phase 1 plus US1. The hint alone delivers discovery (the spec's P1
value) and can ship before the key.

**Incremental**: US1, then US3 as the regression net, then US2, then US4 and
polish. Each phase leaves the tree green.

## Notes

- Do not run `git commit` unless asked. Branch is `1731-overview-web-hint`.
- Test in a populated project (`../finfocus-demo`, empty passphrase) before
  claiming the TUI works; unit tests passing is not enough.
- Never share a mutable fixture across parallel subtests; use `t.Cleanup`.
