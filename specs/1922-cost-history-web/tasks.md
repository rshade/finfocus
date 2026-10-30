# Tasks: Cost History on the Web

**Input**: Design documents from `/specs/1922-cost-history-web/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http-api.md

**Tests**: Principle II requires tests before implementation. Each phase
writes its tests first, then the code that makes them pass.

## Phase 1: Foundational - shared history types and cmd-free reads

**Purpose**: One implementation for the CLI and the web. Blocks every story.

- [X] T001 Add `ErrNoCostHistory`, `ErrNoSnapshots`, and `QueryError` (message-preserving wrapper with `Unwrap`) in `internal/history/errors.go`, with tests in `internal/history/errors_test.go`
- [X] T002 Return `ErrNoSnapshots` from `ResolveCostSnapshot` for an empty set (message `no cost history snapshots` unchanged) in `internal/history/diff.go`, with a test in `internal/history/diff_test.go`
- [X] T003 Move `costHistoryView` to `history.ViewDocument` and `costDiffJSON`/`newCostDiffJSON` to `history.DiffDocument`/`history.NewDiffDocument` in `internal/history/document.go`; update `internal/cli/cost_history_view.go` and `internal/cli/cost_history_diff.go` to use them; add `datakey:"true"` to `CostSnapshot.ByProvider`, `CostSnapshot.ByType`, and `CostAnnotation.ResourceChanges` in `internal/history/cost_types.go`
- [X] T004 Extract `historyViewBounds`, `selectViewCurrency`, `checkDiffInputs`, `readHistoryView(db, stack, query)`, `readHistoryDiff(db, stack, query)`, and `openHistoryTargetReadOnly(target)` into `internal/cli/cost_history_query.go`; caller mistakes are wrapped in `history.QueryError`, a missing file is `history.ErrNoCostHistory`
- [X] T005 Route `viewBounds`, `applyViewCurrency`, `readDiffFlags`, `runView`, and `runDiff` through T004's functions, and wrap the missing-file message in `openHistoryTargetReadExisting` with `history.ErrNoCostHistory`, in `internal/cli/cost_history_view.go`, `internal/cli/cost_history_diff.go`, and `internal/cli/cost_history_export.go`; existing view/diff tests pass unchanged
- [X] T006 Unit tests for the extracted functions (bounds, reversed range, currency selection and strict, missing `from`, negative threshold, empty database, read-only open never writing) in `internal/cli/cost_history_query_test.go`

**Checkpoint**: `go test ./internal/history/ ./internal/cli/ -run 'History|View|Diff'` passes with CLI output unchanged.

## Phase 2: User Story 1 - Stack history in the browser (Priority: P1)

**Goal**: `GET /api/history/view` returns `readHistoryView` for the session stack plus the budget and display strings.

**Independent Test**: For one fixture stack and bounds, the web document equals `cost history view --output json`.

- [X] T007 [P] [US1] Handler tests for 200, query parsing (`strict`), 400 `invalid_history_query`, 503, 502, and breakdown keys that survive `RedactJSON` in `internal/webui/handlers_history_test.go`
- [X] T008 [US1] Add `HistoryViewQuery`, `HistoryViewResponse`, `SessionOptions.HistoryView`, `registerHistory`, and `GET /api/history/view` in `internal/webui/session.go`, `internal/webui/handlers_history.go`, and `internal/webui/server.go`
- [X] T009 [US1] Implement `(*webData).historyView` (`ctx.Err()` check, stack from `forecastStack` under the gate, `resolveHistoryTarget`, `openHistoryTargetReadOnly`, `readHistoryView`, `globalBudget`) in `internal/cli/web_history.go`, and wire it in `internal/cli/web.go`
- [X] T010 [US1] Parity test: CLI `cost history view --output json` vs the web callback for bounds, currency, and mixed-currency warning, plus the configured global budget, in `internal/cli/web_history_test.go`

## Phase 3: User Story 2 - Compare two snapshots (Priority: P1)

**Goal**: `GET /api/history/diff` returns `readHistoryDiff` for the session stack.

**Independent Test**: For one fixture stack, selector pair, and threshold, the web document equals `cost history diff --output json`.

- [X] T011 [P] [US2] Handler tests for diff 200, threshold parsing, 400 for selector errors, display strings, and redaction in `internal/webui/handlers_history_test.go`
- [X] T012 [US2] Add `HistoryDiffQuery`, `HistoryDiffResponse`, `SessionOptions.HistoryDiff`, and `GET /api/history/diff` in `internal/webui/session.go` and `internal/webui/handlers_history.go`
- [X] T013 [US2] Implement `(*webData).historyDiff` (`ctx.Err()` check before the open) in `internal/cli/web_history.go` and wire it in `internal/cli/web.go`
- [X] T014 [US2] Parity test: CLI `cost history diff --output json` vs the web callback, including default `to` and threshold, in `internal/cli/web_history_test.go`

## Phase 4: User Story 3 - Resource series (Priority: P2)

**Goal**: `GET /api/history/resource` returns `loadDetailHistory` points.

**Independent Test**: The web points equal `loadDetailHistory` for the same id and session stack.

- [X] T015 [P] [US3] Handler tests for 200, missing `id` (400 `invalid_query`), empty series, and display strings in `internal/webui/handlers_history_test.go`
- [X] T016 [US3] Add `HistoryPoint`, `HistoryResourceResponse`, `SessionOptions.HistoryResource`, and `GET /api/history/resource` in `internal/webui/session.go` and `internal/webui/handlers_history.go`
- [X] T017 [US3] Implement `(*webData).historyResource` (calls `loadDetailHistory`; maps `detail.ErrHistorySkipped` to `history.ErrNoCostHistory`) in `internal/cli/web_history.go`, wire it in `internal/cli/web.go`, and test parity with the TUI loader in `internal/cli/web_history_test.go`

## Phase 5: User Story 4 - "No history" versus failure (Priority: P2)

**Goal**: Missing data is a `no_history` status; a damaged database is a pathless 502.

**Independent Test**: With no database, all three endpoints answer `no_history`. With a damaged file, all three answer 502 without a path.

- [X] T018 [US4] Map `history.ErrNoCostHistory`/`history.ErrNoSnapshots` to `{status: "no_history"}` and other errors to 502 `history_unavailable` (logged, no path) in `internal/webui/handlers_history.go`
- [X] T019 [US4] Tests for no stack, missing file, empty database diff, a canceled context, and a corrupt database file across the three callbacks in `internal/cli/web_history_test.go` and the handler status mapping in `internal/webui/handlers_history_test.go`

## Phase 6: Web Parity

- [X] T020 Confirm `specs/1922-cost-history-web/contracts/http-api.md` names the Go methods each handler calls (`readHistoryView`, `readHistoryDiff`, `loadDetailHistory`, and the history functions under them)
- [X] T021 Document the three endpoints and their statuses in `docs/src/content/docs/reference/web-ui.md`
- [X] T022 Render the Overview and Cost detail panels with Playwright against a populated stack with collected history, query the three endpoints from that session, and record the result in the spec's Web UI Fit `Render evidence`
- [ ] T023 File one UX-only `web-ui` issue for the detail-panel history section and the Cost-view history chart (layout, rendering, interaction, copy, accessibility; no engine, API, redaction, or data-shape work)

## Phase 7: Polish and Verification

- [X] T024 Add a Recent Changes entry for 1922 by hand in `CLAUDE.md` (do not run `update-agent-context.sh`)
- [X] T025 `make validate`, `make test`, `make test-race` (history, cli, webui), `make lint`, `make docs-lint`
- [X] T026 `make test-e2e-web` because the web route surface changed

## Dependencies

- Phase 1 blocks every story.
- US1 and US2 share `handlers_history.go` and `web_history.go`; do them in order.
- US3 depends only on Phase 1 and the session wiring from US1.
- US4 maps errors for all three endpoints and follows them.
- Web Parity follows the stories; Polish is last.

## Parallel Opportunities

- T007, T011, and T015 are test-first tasks that can be drafted together.
- T001 and T002 touch different files.

## Implementation Strategy

MVP is Phase 1 plus US1 (the stack history). US2 adds the diff, US3 the
detail-panel series, and US4 hardens the statuses. Each phase leaves the CLI
output unchanged and all tests green.
