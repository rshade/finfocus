# Tasks: Cost Anomalies Command

**Input**: Design documents in `specs/1923-cost-anomalies/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/
**Tests**: Required (Constitution Principle II). Each test task precedes the
code it covers and must fail first. Use testify only.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Record the baseline: run `go vet ./...` and `go test ./internal/engine/... ./internal/cli/... ./internal/webui/...` in the worktree and note pre-existing failures

## Phase 2: Foundational (blocks every story)

- [X] T002 [P] Write table-driven tests for `SelectAnomalies` in `internal/engine/anomalies_test.go`: over threshold, just over, at threshold (nil drift), under threshold, elapsed under 2 days, new resource, deleted resource, error row with drift, two currencies listed separately, empty input (`considered` 0, non-nil empty slice), ordering by absolute deviation then URN, field mapping from `CostDrift` and `ActualCost`
- [X] T003 Implement `Anomaly`, `AnomalyReport`, and `SelectAnomalies(rows []OverviewRow) AnomalyReport` in `internal/engine/anomalies.go` per data-model.md (no threshold or normalization logic; `observed` is `CostDrift.ExtrapolatedMonthly`)

**Checkpoint**: `go test ./internal/engine/ -run Anomal` passes.

## Phase 3: User Story 1 - List what looks off right now (P1)

**Goal**: `finfocus cost anomalies` prints the anomaly list in table, JSON, NDJSON.
**Independent test**: fixture state with drifting and steady rows yields exactly the drifting ones.

- [X] T004 [P] [US1] Write renderer tests in `internal/engine/anomalies_render_test.go`: JSON has `considered` and a `[]` array when empty, every FR-004 key present, NDJSON one object per line and no lines when empty, table columns and `N of M resources` line, no cross-currency total
- [X] T005 [US1] Implement `RenderAnomaliesAsTable`, `RenderAnomaliesAsJSON`, `RenderAnomaliesAsNDJSON` in `internal/engine/anomalies_render.go`
- [X] T006 [P] [US1] Write CLI tests in `internal/cli/cost_anomalies_test.go`: takes no positional arguments, JSON from the command equals `SelectAnomalies` output for a stubbed pipeline result, empty stack exits 0 with `considered`, error row not listed, command is mounted under `cost`
- [X] T007 [US1] Implement `NewCostAnomaliesCmd` in `internal/cli/cost_anomalies.go`: build `overviewParams` (state-only unless `--pulumi-json`), run `newOverviewPipeline` with `plainOverviewLoader`, nil `ConfirmEnrichment`, `wantBudget` false, read `OnReady` rows, call `SelectAnomalies`, render; wire a deps seam so tests can supply rows without plugins
- [X] T008 [US1] Mount the command in the `cost` group in `internal/cli/root.go` (after `NewCostHistoryCmd()`)

**Checkpoint**: `go test -run CostAnomalies ./internal/cli/` passes.

## Phase 4: User Story 2 - MCP tool (P1)

**Goal**: `finfocus-cost-anomalies` is a listed, callable MCP tool.

- [X] T009 [US2] Add `finfocus-cost-anomalies` to `internal/cli/testdata/mcp/tools.golden` via `UPDATE_GOLDEN=1 go test -run <golden test> ./internal/cli/` (that file only) and confirm the diff is one line
- [X] T010 [US2] Confirm `--format json` selects JSON for the command (test in `internal/cli/cost_anomalies_test.go` through `resolveOutputFormatWithConfig`)

## Phase 5: User Story 3 - Inputs and validation (P2)

**Goal**: Same inputs as overview; bad input fails early with exit 2.

- [X] T011 [P] [US3] Add tests in `internal/cli/cost_anomalies_test.go`: unknown `--output` returns a validation error before the loader runs (loader stub records no call), malformed `--from` is a validation error, `--filter type=...` and `status=`/`provider=` narrow rows, an unknown filter key errors
- [X] T012 [US3] Implement the early checks in `internal/cli/cost_anomalies.go` with `toValidationError`: output check first, then dates via `resolveOverviewDateRange`, then filter keys

## Phase 6: User Story 4 and Web Parity (P2, Principle VII)

**Goal**: The dashboard serves the same list from the session rows.

- [X] T013 [P] [US4] Write handler tests in `internal/webui/handlers_anomalies_test.go`: response equals `engine.SelectAnomalies` over the same rows (parity with the CLI fixture), empty session rows give `considered` 0 and `[]`, unauthenticated request rejected, no second pipeline started
- [X] T014 [US4] Implement `GET /api/overview/anomalies` in `internal/webui/handlers_anomalies.go` calling `(*Session).Rows` and `engine.SelectAnomalies` only; register it beside the other overview routes in `internal/webui/handlers_overview.go`; no import of `internal/cli`
- [X] T015 [US4] Run `go test ./internal/webui/...` and any `parity_*_test.go`; add a parity test if the package has a CLI-versus-web fixture helper
- [X] T016 [US4] Render check: start `finfocus --web --no-browser --port <port>` from a populated project (`../finfocus-demo` or `initial-30`), render the Overview with Playwright, call the endpoint, and record stack, date, and result under "Render evidence" in `spec.md`; stop the server with an anchored `pkill`
- [X] T017 [US4] Update `docs/src/content/docs/reference/web-ui.md` (the page that lists endpoints; the architecture page lists none)

## Phase 6a: User Story 5 - Anomalies page (P2)

**Goal**: The dashboard has an Anomalies page that renders the endpoint's
payload, with no formatting or arithmetic in the browser.
**Independent test**: the Node renderer test and the Chromium test show the
table, count line, empty state, and error alert.

- [X] T024 [P] [US5] Write display tests in `internal/viewmodel/anomalies_display_test.go`: currency text for baseline, observed, and signed deviation, signed one-decimal drift, period text, zero period gives an empty string, negative deviation, a non-USD currency
- [X] T025 [US5] Implement `AnomalyDisplay` and `BuildAnomalyDisplay` in `internal/viewmodel/anomalies_display.go` using the existing currency helper
- [X] T026 [US5] Change `handleAnomalies` in `internal/webui/handlers_anomalies.go` to return `AnomalyPage` (`considered`, `summary`, anomalies with inline fields and `display`); update `handlers_anomalies_test.go` and `parity_anomalies_test.go` so the CLI-parity assertion compares the anomaly fields after removing `display` and `summary`
- [X] T027 [P] [US5] Write the renderer test `internal/webui/frontendtest/anomalies.test.mjs` in the style of `carbon.test.mjs`: table rows and cells come from the payload text, count line, empty state with the considered count, loading state, error alert hides stale rows, refresh on `ready` and `snapshot`, a stale response is ignored, unmount unsubscribes, no arithmetic (cells equal payload strings)
- [X] T028 [US5] Implement `internal/webui/static/views/anomalies.js` per the page contract in `contracts/http-api.md`; build nodes with `el` and `text` only (no `innerHTML`)
- [X] T029 [US5] Add the `anomalies` route to `internal/webui/static/app.js`, the `Anomalies` link to `internal/webui/static/index.html`, and any missing numeric-column or empty-state class to `internal/webui/static/styles.css`
- [X] T030 [P] [US5] Write `test/e2e/web_anomalies_test.go` in the style of `web_forecast_test.go`: real Chromium against a fixture session with drifting rows, header link reaches the page, table and count line, empty state, keyboard focus reaches the scroll region
- [X] T031 [US5] Run `make test-frontend` and `make test-e2e-web`; fix findings
- [X] T032 [US5] Render the Anomalies page against a stack, screenshot it next to the Overview, compare with the spec's Web UI Fit section, and update its Render evidence; stop the server by PID
- [X] T033 [US5] Add the Anomalies view to the Views section of `docs/src/content/docs/reference/web-ui.md` and update the `GET /api/overview/anomalies` row for the display fields; mention the page in `docs/src/content/docs/commands/cost-anomalies.md`

## Phase 7: Polish

- [X] T018 [P] Write `docs/src/content/docs/commands/cost-anomalies.md` following `forecast.md` (frontmatter, flags, JSON example, exit codes, filter keys, state-only note)
- [X] T019 [P] Update the `detect_anomalies` row in `docs/src/content/docs/guides/mcp.md` to name `finfocus-cost-anomalies`
- [X] T020 [P] Add a short section next to the other cost commands in `docs/src/content/docs/guides/user-guide.md`
- [X] T021 Run `make validate`, `make test`, `make test-frontend`, `make test-e2e-web`, `make test-race`, `make lint`, `make docs-lint`, and markdownlint on changed Markdown outside `docs/`; fix findings
- [X] T022 Run `/code-review` on the diff against the base commit and fix substantiated findings
- [X] T023 Dropped: the Anomalies page ships in this feature (Phase 6a), so no `web-ui` follow-up issue is filed. The pull request body states this.

## Dependencies

- T002 -> T003 -> everything else.
- US1: T004 -> T005; T006 -> T007 -> T008. T005 precedes T007.
- Phase 6a depends on T014: T024 -> T025 -> T026; T027 -> T028 -> T029; T030 and T031 follow T029; T032 and T033 follow T031. T021 and T022 run after Phase 6a.
- US2 depends on T008. US3 depends on T007. US4 depends on T003 only and may run beside US1.
- T016 needs T014. T021 to T023 come last.

## Parallel examples

- T002 with T004, T006, T013 (distinct test files, written first).
- T018, T019, T020 (distinct docs).

## Implementation strategy

MVP is Phases 1 to 3. Add MCP (Phase 4), validation (Phase 5), then the web
hook (Phase 6), then docs and gates.
