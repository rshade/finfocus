# Tasks: Spot Recommendations from Plugins

**Input**: [spec.md](spec.md), [plan.md](plan.md)

## Phase 1: Mapping and label (US1)

- [x] T001 Tests: proto mapping of `pricing_category` and
  `spot_interruption_risk_score` (DYNAMIC, non-DYNAMIC with risk, out of
  range) in `internal/proto`; engine conversion in `internal/engine`.
- [x] T002 Tests: `SpotRiskDisplay` and `Recommendation.SpotDisplay` (risk,
  unknown, floor, non-spot) and that `SpotDisplay` output is unchanged.
- [x] T003 Implement the proto fields, mapping, engine fields, and labels.

## Phase 2: Renderers (US1)

- [x] T004 CLI: `PRICING` column only with a spot row; JSON and NDJSON fields.
  Tests for both, for unchanged output without spot rows, and that the label
  is plain text (FR-009).
- [x] T005 TUI: list suffix and detail `Pricing:` line; overview detail line.
  Golden or rendered-view tests.
- [x] T005a Linked recommendations in the cost detail (TUI cost view, web Cost
  detail) through `viewmodel.LinkedCostRecommendations` (found in review).
- [x] T006 Viewmodel: `RecommendationPage.ItemSpot` and
  `OverviewDetailRecommendation.SpotDisplay`; item handler `spotDisplay`.
- [x] T007 Web: `recommendations.js` and `overview.js` render the badge via
  `spotBadge`; node harness tests.
- [x] T007a Web contract: `contracts/http-api.md` names each endpoint and Go
  method; a parity test asserts the web strings equal the CLI method output.
- [x] T007b Docs: confirm `docs/src/content/docs/architecture/web-ui.md`
  covers the Recommendations badge; update the page if it lists view contents.
  (It describes the layout, not view contents: no change.)

## Phase 3: Dismissal and cache (US2)

- [x] T008 Snapshot fields in `config.LastKnownRecommendation`; restore them in
  the CLI and overview conversions. Round-trip tests, including an old file.
  The CLI `dismiss`/`snooze` never write a snapshot (pre-existing; spec US2).
- [x] T009 Cache key `/recs-v1` suffix and test.

## Phase 4: Scorer (US3)

- [x] T010 `toProto` sends both fields; allowlist names accepted by config and
  honored by `applyAllowlist`. Tests.

## Phase 5: Polish

- [x] T011 Docs: user guide recommendations section; CLAUDE.md note.
- [x] T012 Gates: `make validate`, `make test`, `make lint`, `make test-race`
  on changed packages, `make docs-lint`; real-binary check.
- [x] T013 Playwright render of the Recommendations page and Overview detail
  against a populated stack; compare with `Web UI Fit`. Run
  `make test-e2e-web` because the views changed. Rendered 2026-11-08 with a
  throwaway spot plugin: Recommendations table badge in the Action cell,
  detail dialog `Pricing:` badge, Overview detail badge after the savings.
- [x] T014 After the PR opens, file one UX-only `web-ui` follow-up issue
  (badge layout and copy on Recommendations; no engine, API, or data work).
  Filed as #2054.
