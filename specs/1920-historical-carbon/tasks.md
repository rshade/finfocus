---

description: "Task list for Historical Carbon Emissions Reporting"
---

# Tasks: Historical Carbon Emissions Reporting

**Input**: `specs/1920-historical-carbon/` (spec.md, plan.md,
contracts/carbon-actual.md)

**Completeness**: Per Principle VI, no stubs, no TODOs. Every task is a full
implementation. All tasks below were implemented on `feat/historical-carbon`
and verified as recorded in the pull request.

## Phase 1: Dependency

- [x] T001 Pin finfocus-spec v0.7.10 in `go.mod`, `plugins/*/go.mod`, and
  `test/e2e/kind/fixedcost/go.mod`
- [x] T002 Add the v0.7.10 hop in `internal/pluginupgrade/hops.go`,
  `agent-skills/finfocus-plugin-upgrade/references/to-v0.7.10.md`, the
  `SKILL.md` row, and the rebuilt `.skill` archive

## Phase 2: Report package (User Stories 1 and 2)

- [x] T003 Selectors and request validation in
  `internal/carbonreport/selectors.go` with `selectors_test.go`
- [x] T004 Page loop, record validation, and report in
  `internal/carbonreport/report.go` with `report_test.go`
- [x] T005 `Client.GetCarbonEmissions` gated on `historical_carbon` in
  `internal/pluginhost/carbon.go` with `carbon_test.go`

## Phase 3: CLI and MCP (User Stories 1, 2, 3)

- [x] T006 `carbon actual` command, table and JSON output in
  `internal/cli/carbon_actual.go` with unit and process tests
- [x] T007 Route invalid input through `toValidationError` (exit 2)
- [x] T008 Register the command in `internal/cli/root.go` and update
  `internal/cli/testdata/mcp/tools.golden`

## Phase 4: Web hook (User Story 3)

- [x] T009 `POST /api/carbon/actual/query` and error mapping in
  `internal/webui/handlers_carbon.go` with tests
- [x] T010 Inject `queryHistoricalCarbon` through `SessionOptions.HistoricalCarbon`

## Phase 5: Documentation

- [x] T011 `docs/src/content/docs/plugins/historical-carbon.md` and the
  `cli-commands.md` entry
- [x] T012 Record the Web UI Fit and file the UX-only follow-up #1921

## Follow-up (separate issue)

- [x] T013 Browser query form and provenance table, delivered by #1921
