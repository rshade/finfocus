---
description: "Task list for Web UI (finfocus --web) implementation"
---

# Tasks: Web UI (Browser-Based SPA)

**Input**: Design documents from `/specs/627-web-ui/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http-api.md, quickstart.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths). TUI changes
MUST include golden file snapshot tests and visual render verification —
for this feature, `go test ./internal/tui/...` golden files MUST pass
unchanged after every extraction task (no `UPDATE_GOLDEN=1`).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Data Integrity (FR-011a/b)**: No new cost, filter, sort, aggregation, or
estimation logic may be written for the web. All such behavior is extracted
into `internal/viewmodel` or unified in `internal/engine` and consumed by
TUI, plain CLI, and web alike.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Unit tests colocated: `internal/[package]/[name]_test.go` (run: `go test ./internal/...`)
- E2E: `test/e2e/` is its own module; browser e2e uses build tag `e2e_web` (run: `make test-e2e-web`)
- `test/unit/` is RETIRED — do not place tests there

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Package skeletons so extraction and web work have homes

- [X] T001 Create `internal/viewmodel/` package with `doc.go` (package godoc: presentation-neutral view logic shared by TUI and web, operating on `internal/engine` types)
- [X] T002 [P] Create `internal/webui/` package structure: `server.go`, `static.go`, `static/` (`index.html`, `app.js`, `styles.css`, `views/`) with minimal compile-ready placeholders that are completed in story phases (no stubs committed at story completion; placeholders allowed only mid-phase)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extract shared logic (FR-011a/b) and build the server core. **⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational (MANDATORY - TDD Required) ⚠️

> **Write these tests FIRST, ensure they FAIL before implementation**

- [X] T003 [P] Table-driven tests for extracted sort comparators (cost, name, type, delta fields; descending/ascending rules copied from TUI behavior) in `internal/viewmodel/sort_test.go`
- [X] T004 [P] Table-driven tests for substring filters (case-insensitive match on URN+type for overview, ResourceID+ResourceType for cost, ResourceID+Type+Description for recommendations) in `internal/viewmodel/filter_test.go`
- [X] T005 [P] Tests for cluster tree flattening + pagination units (expanded/collapsed sets, filter-active behavior, 250 rows/page) in `internal/viewmodel/cluster_display_test.go`
- [X] T006 [P] Test that unified carbon aggregation produces identical totals to current `internal/tui/cost_view.go:517` output on fixture `[]engine.CostResult` in `internal/engine/sustainability_test.go`
- [X] T007 [P] `httptest` tests for server core: valid token bootstrap sets `HttpOnly; SameSite=Strict; Path=/` cookie named per port and answers 303 to `/`; the token works a second time; missing/invalid cookie → 401; wrong `Host` header → 403; `POST` with foreign or missing `Origin` → 403 and non-JSON content type → 415; security headers present; token and request bodies absent from log output; server listens on loopback only; port 0 auto-assign prints actual port; pinned busy `--port` fails with clear error; interrupt shuts down cleanly in `internal/webui/server_test.go`
- [ ] T008 [P] Tests for event-driven overview pipeline: phase events 1..6 emitted in order, per-row events match `engine.EnrichOverviewRows` output, an expansion event carries the rows `expandOverviewClusters` returns, error events carry phase number in `internal/cli/overview_pipeline_test.go`
- [ ] T008b [P] Root-flag tests in `internal/cli/web_test.go` (`NewRootCmd*` builders are process-wide, so these are not parallel; `//nolint:paralleltest` with the reason): `--web`, `--port`, `--no-browser`, `--pulumi-json`, `--pulumi-state`, `--stack`, `--from`, `--to`, `--adapter`, `--filter` are root-local (present in `root.Flags()`, absent from `root.PersistentFlags()`) and no subcommand has `--web` (`cost actual --web`, `overview --web` → unknown flag); each companion flag without `--web` → error naming `--web`; `--web` with `--mcp` → error; `--terraform-state` with `--web` → unknown flag; `--web` while MCP is serving returns `errRootNotATool`; `cost --stack`, `cost history view --plain`, and `overview --stack` still parse; the existing MCP golden tests (`internal/cli/testdata/mcp/tools.golden`, `test/integration/mcp_server_test.go`) pass UNCHANGED (the root is already excluded from MCP, so `--web` adds no tool); a launch from a Pulumi project directory with no source flags resolves the same project, stack, and state as `finfocus overview` (through the same `detectPulumiProject`/`loadOverviewFromAutoDetect` seam), and `--project-dir` does not change where detection looks
- [X] T008a [P] Redaction tests: a fixture with a Pulumi secret input, a credential-like property name (`password`, `token`), and a budget with notification destinations produces none of those values in the overview, detail, cost, recommendations, estimate, and budget payloads; the payload is no wider than `finfocus overview --output json` in `internal/webui/redaction_test.go`

### Implementation for Foundational

- [X] T009 [P] Extract sort fields/comparators from `internal/tui/cost_model.go:67-78`, `internal/tui/overview_model.go:713,945-975`, `internal/tui/recommendations_model.go:18-27,482` into pure functions `SortOverviewRows`, `SortCostResults`, `SortRecommendations` + `SortField`/`RecommendationSortField` types in `internal/viewmodel/sort.go`
- [X] T010 [P] Extract substring filters from `internal/tui/overview_model.go:920`, `internal/tui/cost_model.go:326`, `internal/tui/recommendations_model.go:453` into pure functions `FilterOverviewRows`, `FilterCostResults`, `FilterRecommendations` in `internal/viewmodel/filter.go`
- [X] T011 Extract cluster display flattening/pagination from `internal/tui/overview_model.go` (`displayEntries`:772, `paginationUnits`:987, `pageUnits`:1007) into `FlattenClusterRows(rows, expanded, filterActive, page)` in `internal/viewmodel/cluster_display.go` (depends on T010 for filter interaction)
- [X] T012 [P] Unify carbon aggregation: create exported `AggregateSustainability(results []CostResult)` in `internal/engine/sustainability.go` absorbing `internal/tui/cost_view.go:517` logic (uses `greenops.NormalizeToKg`/`CalculateFromMap` as at `internal/engine/project.go:328-406`); refactor both call sites
- [X] T013 Refactor TUI models to delegate to viewmodel/engine: `internal/tui/overview_model.go` (filter/sort/flatten calls), `internal/tui/cost_model.go:326,351`, `internal/tui/recommendations_model.go:453,482`, `internal/tui/cost_view.go:517` → `engine.AggregateSustainability`; verify `go test ./internal/tui/...` golden files pass UNCHANGED (depends on T009-T012)
- [ ] T014 Extract event-driven `OverviewPipeline` in `internal/cli/overview_pipeline.go` from the TUI goroutine orchestration at `internal/cli/overview.go:1257-1500` (phases `phaseLoadStackState`..`phaseEnrichResources` at `overview.go:1214-1224`, enrichment bridge `overview.go:1741`, cluster expansion `overview_cluster.go:286`, budget fetch `overview.go:1407-1433`): expose callback API `OnPhase/OnRow/OnProgress/OnBudget/OnExpansion/OnError/OnReady/OnPassphraseRequired` plus a preview-ready callback (the TUI's `OverviewChangesReadyMsg`) (depends on T013)
- [ ] T014a Settle the overview payload shape: `engine.OverviewRowResult`/`OverviewTotals` have no JSON tags and `RenderOverviewAsJSON` serializes the full `OverviewRow` schema, so add one shared serializable form in `internal/engine` (consumed by `RenderOverviewAsJSON` and the web handlers, not a web-only copy) and record it in `contracts/http-api.md` (depends on T013)
- [X] T015 Implement web server core in `internal/webui/server.go` + `internal/webui/static.go`: stdlib `net/http` on `127.0.0.1`; 128-bit `crypto/rand` hex token at startup; `GET /?token=` validates once and sets session cookie; cookie-auth middleware on all other routes; `Host` header must match `127.0.0.1:<port>` or `localhost:<port>`; port default 0 (kernel-assigned, actual URL printed), `--port` pins; graceful shutdown on `os.Interrupt`; `//go:embed static` assets
- [X] T015a Redaction and request hardening in `internal/webui`: reuse `engine.BuildAttributes`/`redactedProperties`, `history.IsPulumiSecret` and the `isCredentialKey` rule for every payload (no web-specific rule list); Origin and Content-Type checks, `Cache-Control`/`Referrer-Policy`/CSP headers; request log omits query string and bodies (FR-016, FR-018) (depends on T015)
- [ ] T016 Wire `--web` as a root-local flag (`cmd.Flags()`, never `PersistentFlags()`, following `mcpFlag` in `internal/cli/root.go` and `internal/cli/mcp.go`) with the root-local companions `--port`, `--no-browser`, `--pulumi-json`, `--pulumi-state`, `--stack`, `--from`, `--to`, `--adapter`, `--filter` (no short forms) in `internal/cli/root.go` and `internal/cli/web.go`: root `RunE` branches to `--web` after the MCP branch (`lifecycle.hostsMCP` first, so a dispatched call still gets `errRootNotATool`); any companion flag without `--web`, and `--web` with `--mcp`, fail with a clear error; `--project-dir` is reused unchanged; no `--terraform-state`; with no source flags reuse the overview auto-detect path (`detectPulumiProject`, `exportStateFromProject`, `loadOverviewFromAutoDetect`) via the `OverviewPipeline`; build engine exactly as the equivalent CLI command does (`openPlugins` `common_execution.go:173`, `newEngineWithCache` `common_execution.go:442`), start server, open browser via `runtime.GOOS` switch (`xdg-open`/`open`/`rundll32`), print URL always (FR-001/FR-002/FR-012/FR-013) (depends on T008b, T014, T015)
- [ ] T016a Update the MCP-facing docs and checks for the new root flag: add `--web` and its companions to `docs/src/content/docs/reference/cli-commands.md` next to `--mcp`, and note in `docs/src/content/docs/guides/mcp.md` that `--web` is not an MCP entry point; confirm `finfocus __schema --as=mcp` output and `tools.golden` need no regeneration (no `UPDATE_GOLDEN=1`) (depends on T016)

**Checkpoint**: Foundation ready — server starts, authenticates, serves embedded shell; TUI renders identically on shared logic; user story implementation can now begin

---

## Phase 3: User Story 1 - Launch a Browser-Based Cost Dashboard (Priority: P1) 🎯 MVP

**Goal**: `finfocus --web`, run from a Pulumi project directory, serves an SPA whose overview dashboard progressively loads the same rows/totals/budget footer as the TUI overview, with filter/sort/drill-down/cluster-expand/passphrase support

**Independent Test**: Run `bin/finfocus --web --pulumi-json testdata/simple-plan.json --no-browser --port 8484`; verify SSE phase/row/ready sequence via curl and that `/api/overview/query` JSON equals `bin/finfocus overview --pulumi-json testdata/simple-plan.json --output json` (quickstart Scenarios 1-2)

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

- [ ] T017 [P] [US1] Contract tests for overview endpoints (query/cluster toggle/preview/budget/resource detail/passphrase) against `contracts/http-api.md` shapes in `internal/webui/handlers_overview_test.go`
- [ ] T018 [P] [US1] SSE stream test: connect to `/api/overview/stream` with fixture plan, assert `snapshot` first, `phase` 1..6 ordered, ≥1 `row` events, terminal `ready` with totals; a reconnect receives a `snapshot` of the state so far; `error` events carry phase in `internal/webui/sse_test.go`
- [ ] T019 [P] [US1] Parity test (after T014a): `/api/overview/query` response rows+totals equal `engine.RenderOverviewAsJSON` output for the same fixture in `internal/webui/parity_overview_test.go`
- [ ] T020 [P] [US1] Encrypted-stack test: pipeline emits `passphrase_required`, POST `/api/passphrase` unlocks, wrong passphrase surfaces `error` event; passphrase never logged in `internal/webui/handlers_overview_test.go`

### Implementation for User Story 1

- [ ] T021 [US1] Implement overview handlers in `internal/webui/handlers_overview.go`: `POST /api/overview/query` (viewmodel filter/sort/flatten via `OverviewQuery{Filter, Sort, Page, Expanded}` — unknown sort field → 400), `POST /api/overview/cluster/toggle` (stateless: takes and returns the client's expanded set), `GET /api/overview/budget` (`eng.GetBudgets` + `engine.BuildConfigBudgetResult` fallback), `GET /api/overview/resource?urn=` detail payload (property diffs, breakdowns, per-budget status), `POST /api/overview/preview` (TUI `p`-key equivalent with elapsed-ms; single-flight, a second call attaches to the running preview), `POST /api/passphrase` (held in memory only for the unlock call; never stored/logged/echoed) (depends on T014, T015)
- [ ] T022 [US1] Implement SSE overview stream in `internal/webui/sse.go`: map `OverviewPipeline` callbacks to `snapshot` (first on every connect)/`phase`/`row`/`progress`/`budget`/`expansion`/`error`/`ready`/`passphrase_required`/`preview` events per `contracts/http-api.md` (stdlib Flusher, no deps) (depends on T014, T015)
- [ ] T023 [P] [US1] Build SPA shell in `internal/webui/static/index.html` + `app.js`: hash-based view router (overview/cost/recommendations/estimate), EventSource client with reconnect, fetch wrapper, semantic landmarks (`<main>`, `<nav>`, `<table>`), focus-visible styling
- [ ] T024 [US1] Build overview view in `internal/webui/static/views/overview.js`: 9-column table (Resource, Type, Status, Actual MTD, Projected, Delta, Drift%, Recs, Warn) populated from SSE `row` events, loading checklist from `phase` events, budget-health footer, filter input, sort cycling, pagination, cluster ▸/▾ expand/collapse (POST cluster/toggle), detail panel from `/api/overview/resource`, preview button with elapsed timer, masked passphrase modal on `passphrase_required`; all numbers rendered from server-computed fields — client-side math forbidden (FR-011b) (depends on T023)
- [ ] T025 [US1] Styles in `internal/webui/static/styles.css`: light/dark palettes meeting WCAG AA contrast (FR-017), visible focus indicators, responsive table layout, loading/spinner states

**Checkpoint**: US1 fully functional — quickstart Scenarios 1-2 pass; TUI golden tests still green

---

## Phase 4: User Story 2 - Explore Actual Costs and Recommendations (Priority: P2)

**Goal**: SPA gains actual-cost view (trends, carbon, plus the web-only group-by selector over resource/type/provider/daily/monthly and a tag filter) and recommendations view (summary, scorer-signal detail, plus the web-only include-dismissed toggle) with the same filter/sort/drill-down behavior as the TUI. The selector and the toggle go beyond the TUI and re-fetch through the shared code path (spec FR-008a, FR-009a)

**Independent Test**: `/api/cost/actual/query` with each offered `groupBy` equals `finfocus cost actual --group-by <value>` plain output; `/api/recommendations/query` with `{"includeDismissed":true}` equals `finfocus cost recommendations --include-dismissed` (quickstart Scenario 3)

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [ ] T026 [P] [US2] Contract + parity tests for `/api/cost/actual/query` (every `groupBy` the web UI offers — none, resource, type, provider, daily, monthly — equals `finfocus cost actual --group-by <value>`; `tag` `key=value` equals `--group-by tag:key=value`; `date` is accepted by the shared validator but not offered; unsupported group-by or malformed tag → 400) and `/api/cost/actual/resource` detail (breakdown, sustainability, linked recommendations) in `internal/webui/handlers_cost_test.go`
- [ ] T027 [P] [US2] Carbon parity test: web `summary.carbon` equals `engine.AggregateSustainability` output (which TUI now also uses) in `internal/webui/handlers_cost_test.go`
- [ ] T028 [P] [US2] Contract + parity tests for `/api/recommendations/query` (filter/sort/includeDismissed via `eng.GetRecommendationsForResourcesWithDismissed`) and `/api/recommendations/item` scorer-signal detail in `internal/webui/handlers_recommendations_test.go`

### Implementation for User Story 2

- [ ] T029 [US2] Implement cost handlers in `internal/webui/handlers_cost.go`: actual-cost load reusing `executeCostActual` pipeline pieces (`loadActualResources` `cost_actual.go:545`, `newEngineWithCacheAndHistory` `common_execution.go:479`, `eng.GetActualCostWithOptionsAndErrors` `engine.go:1456`), grouping (`engine.CreateCrossProviderAggregation` for daily/monthly, `engine.GroupResults` for resource/type/provider, the CLI's shared tag-filter parser for `tag`; no web-specific grouping code), trends from `history.TableTrends`, carbon via `engine.AggregateSustainability` (depends on T012, T015)
- [ ] T030 [US2] Implement recommendations handlers in `internal/webui/handlers_recommendations.go`: load via `fetchRecommendationsWithProgress` (`cost_recommendations.go:316`) + scoring step, viewmodel filter/sort, `includeDismissed` flag; NO dismiss/undismiss mutations (read-only parity per spec Assumptions) (depends on T015)
- [ ] T031 [P] [US2] Build cost actual view in `internal/webui/static/views/cost.js`: table with the web-only group-by selector (none, resource, type, provider, daily, monthly; `date` not offered as a deprecated alias of `daily`) and a tag filter input (`key=value`), each change re-fetching, trend sparklines rendered from server-provided series (inline SVG, no client computation), cost summary header, detail panel
- [ ] T032 [P] [US2] Build recommendations view in `internal/webui/static/views/recommendations.js`: summary header (count, savings, by-action-type), virtualized/paginated table, web-only include-dismissed toggle (re-fetches; no dismiss or undismiss controls), detail panel with full scorer signals

**Checkpoint**: US1 AND US2 both work independently; quickstart Scenario 3 passes

---

## Phase 5: User Story 3 - What-If Cost Estimation in the Browser (Priority: P3)

**Goal**: SPA estimate view: pick resource, edit properties inline, live recalculation via `eng.EstimateCost`, baseline-vs-modified comparison, pricing-mode switching

**Independent Test**: POST `/api/estimate/recalculate` with a property override returns deltas equal to `finfocus cost estimate` non-interactive output for the same override (quickstart Scenario 3, SC-002a)

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [ ] T033 [P] [US3] Contract + parity tests for `/api/estimate/baseline` (resource descriptor, initial `EstimateResult`, pricing modes via `eng.DiscoverPricingSpec`) and `/api/estimate/recalculate` (overrides map passed unchanged to `engine.EstimateRequest.PropertyOverrides`; invalid pricing mode → 400) in `internal/webui/handlers_estimate_test.go`

### Implementation for User Story 3

- [ ] T034 [US3] Implement estimate handlers in `internal/webui/handlers_estimate.go`: baseline from session plan resources; recalculate as a direct `eng.EstimateCost(ctx, &engine.EstimateRequest{Resource, PropertyOverrides})` call — the exact shape of the TUI callback at `internal/cli/cost_estimate.go:773` (depends on T015)
- [ ] T035 [US3] Build estimate view in `internal/webui/static/views/estimate.js`: editable property table (key/original/current/delta), commit-on-Enter triggers POST recalculate, baseline-vs-modified comparison with ↑/↓/→ indicators, pricing-mode selector; optimistic loading state during recalculation (depends on T023)

**Checkpoint**: All three user stories independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Validation, accessibility, docs, and quality gates across all stories

- [ ] T036 [P] Browser e2e in `test/e2e/web_browser_test.go` (separate `test/e2e` module, build tag `e2e_web`, runs the real `bin/finfocus --web` binary through Playwright via `github.com/playwright-community/playwright-go` added to `test/e2e/go.mod` only; the root `go.mod` does not change): overview progressive load, keyboard-only journey (tab to filter/sort/detail/cluster expand, visible focus), group-by change and include-dismissed toggle, estimate edit-recalculate, masked passphrase flow with encrypted fixture (quickstart Scenario 4). The test skips with a clear message when `FINFOCUS_BINARY` or the browser is missing
- [ ] T036a Browser install and CI wiring: add a Makefile `test-e2e-web` target (build the binary, install chromium with the playwright-go install step from `test/e2e`, run `go test -tags e2e_web`) and list it in `make help`; add a dedicated CI job modeled on `e2e-kind` in `.github/workflows/ci.yml` that runs the target; confirm `go mod tidy -diff` is clean at the root and that the CI go.mod sync check still passes for `test/e2e/go.mod` (depends on T036)
- [ ] T037 [P] Accessibility pass against FR-017: axe-core or manual keyboard/contrast verification on all four views; fix violations in `internal/webui/static/`
- [ ] T038 [P] Performance validation: 1,000-resource fixture — filter/sort responses <1s (SC-004), launch→ready <30s for ≤250-resource stack (SC-001)
- [ ] T039 [P] Documentation: add `--web` section to `README.md` (run from the Pulumi project directory; root-local flags; Pulumi only, no Terraform; group-by selector and include-dismissed toggle noted as web-only), new web UI page under `docs/src/` with frontmatter (`title`, `description`, `layout`), godoc for all new exported symbols (80%+ coverage); run `make docs-lint`
- [ ] T040 Run full gates: `make lint`, `make test`, `make test-race`, `make validate`, govulncheck clean, `go mod tidy -diff` clean (no new dependencies in the root module), then execute every `quickstart.md` scenario end-to-end
- [ ] T041 When the feature is complete, append it to `ROADMAP.md` under `## Completed Milestones`, `### 2026-Q4` (or the quarter then current), as a checked entry in that file's format (a `- [x]` line with the scope `web` and a short description, plus a `Closed` date and size continuation line, as the neighboring entries have). This is a shadow feature with no tracking issue, so the entry carries no issue number; do not invent one. Do not edit `ROADMAP.md` before completion

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories. Internal order: T003-T008b tests first → T009/T010/T012 parallel → T011 (needs T010) → T013 (needs T009-T012) → T014 and T014a (need T013) → T015 → T015a → T016 → T016a
- **User Stories (Phase 3-5)**: All depend on Phase 2; US2/US3 handlers also depend on T015 and (US2) T012
- **Polish (Phase 6)**: Depends on all targeted stories

### User Story Dependencies

- **US1 (P1)**: After Phase 2 — no story dependencies
- **US2 (P2)**: After Phase 2 — independent of US1 (different handlers/views), though it reuses the SPA shell (T023); if parallel, coordinate on `app.js`
- **US3 (P3)**: After Phase 2 — independent of US1/US2; reuses SPA shell (T023)

### Within Each User Story

- Tests MUST be written and FAIL before implementation (constitution)
- Handlers before views that consume them
- TUI golden tests re-run after any extraction-adjacent change

### Parallel Opportunities

- Phase 2: T003/T004/T005/T006/T007/T008 tests; T009/T010/T012 implementations
- US1: T017/T018/T019/T020 tests; T023 shell parallel with T021/T022 handlers
- US2: T026/T027/T028 tests; T031/T032 views parallel with T029/T030 handlers
- US3: T033 tests; T035 view parallel with T034 handler
- US2 and US3 can run in parallel after US1 lands the SPA shell

---

## Parallel Example: User Story 1

```bash
# Launch all US1 tests together (they fail until implementation lands):
Task: "Contract tests for overview endpoints in internal/webui/handlers_overview_test.go"
Task: "SSE stream test in internal/webui/sse_test.go"
Task: "Parity test in internal/webui/parity_overview_test.go"

# Then handlers and SPA shell in parallel (different files):
Task: "Overview handlers in internal/webui/handlers_overview.go"
Task: "SPA shell in internal/webui/static/index.html + app.js"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — the extraction phase; blocks everything)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: quickstart Scenarios 1-2 + TUI golden tests
5. Demo: `finfocus --web` overview dashboard

### Incremental Delivery

1. Setup + Foundational → shared logic extracted, server skeleton live, TUI provably unchanged
2. +US1 → browser overview dashboard (MVP)
3. +US2 → cost actual + recommendations views
4. +US3 → interactive estimate
5. Polish → e2e, a11y, docs, gates

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- FR-011a/b audit at every review: grep webui/static JS for arithmetic on cost fields — there must be none
- Extraction tasks (T009-T014) are behavior-preserving refactors: proof is unchanged golden files + existing test suites
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
