# Implementation Plan: Cost Anomalies Command

**Branch**: `1923-cost-anomalies` | **Date**: 2026-10-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1923-cost-anomalies/spec.md`

## Summary

`finfocus cost anomalies` runs the existing overview pipeline in-process and
prints only the rows whose `CostDrift` is set. One function,
`engine.SelectAnomalies`, turns `[]engine.OverviewRow` into an
`AnomalyReport`; the command, the MCP tool (same command, via ax-go), and a new
`GET /api/overview/anomalies` handler over `Session.Rows()` all call it. The
10% threshold, calendar-month normalization, and early/new/deleted skips stay
in `CalculateCostDrift`; nothing is reimplemented.

The dashboard gets an Anomalies page (`static/views/anomalies.js`) fed by that
endpoint. Amounts, percentages, and periods arrive as display text built by
one Go function in `internal/viewmodel`, because the browser formats nothing.
The page ships with the command, so no UX follow-up issue is filed.

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`)
**Primary Dependencies**: Cobra, ax-go (`ax.Execute`, MCP), testify; no new modules
**Storage**: None. Reads nothing beyond what the overview already reads
**Testing**: testify table tests for the selector and renderers; CLI tests with
the overview pipeline test seams; webui handler test; MCP golden
**Target Platform**: Linux, macOS, Windows (CLI and web)
**Project Type**: Single Go module (CLI plus embedded web dashboard)
**Performance Goals**: Same cost as `finfocus overview --output json`
**Constraints**: No second pipeline in the web handler; `internal/webui` must
not import `cli`; output rows carry no properties, tags, or secrets
**Scale/Scope**: One selector, three renderers, one command, one handler, one
display builder, one web view with a Node renderer test and a Chromium test,
docs

## Constitution Check

- [x] **Plugin-First Architecture**: Orchestration only. Pricing and drift come
  from existing plugin-fed overview enrichment; no provider logic.
- [x] **Test-Driven Development**: Selector and renderer tests are written
  first (tasks.md orders them before code). No TUI change, so no golden TUI.
- [x] **Cross-Platform Compatibility**: No OS-specific code.
- [x] **Documentation Integrity**: Command page, user guide, MCP guide updated
  in the same change; `make docs-lint`.
- [x] **Protocol Stability**: No protocol change.
- [x] **Implementation Completeness**: No stubs.
- [x] **Persistence Model**: No new state.
- [x] **Web Parity**: Web UI Fit present; `contracts/http-api.md` names
  `engine.SelectAnomalies`, `viewmodel.BuildAnomalyDisplay`, and the
  `Session.Rows` call. The page ships in this feature, so no UX follow-up
  issue is filed (the spec's Web UI Fit records why).
- [x] **Quality Gates**: `make lint`, `make test`, `make validate`.
- [x] **Multi-Repo Coordination**: None; no spec or plugin change.

**Violations Requiring Justification**: None.

Re-check after design: unchanged.

## Project Structure

### Documentation (this feature)

```text
specs/1923-cost-anomalies/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli.md
│   └── http-api.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/engine/anomalies.go            # Anomaly, AnomalyReport, SelectAnomalies
internal/engine/anomalies_render.go     # table, JSON, NDJSON renderers
internal/engine/anomalies_test.go
internal/engine/anomalies_render_test.go
internal/cli/cost_anomalies.go          # command, mounted in cost group (root.go)
internal/cli/cost_anomalies_test.go
internal/cli/testdata/mcp/tools.golden  # + finfocus-cost-anomalies
internal/viewmodel/anomalies_display.go # BuildAnomalyDisplay (amounts, drift, period text)
internal/viewmodel/anomalies_display_test.go
internal/webui/handlers_anomalies.go    # GET /api/overview/anomalies (report + display)
internal/webui/handlers_anomalies_test.go
internal/webui/static/views/anomalies.js   # the Anomalies view (no build step)
internal/webui/static/app.js               # route table entry
internal/webui/static/index.html           # header link
internal/webui/static/styles.css           # only if the Overview classes do not cover it
internal/webui/frontendtest/anomalies.test.mjs   # Node renderer test (make test-frontend)
test/e2e/web_anomalies_test.go          # real Chromium (make test-e2e-web)
docs/src/content/docs/commands/cost-anomalies.md
docs/src/content/docs/guides/mcp.md
docs/src/content/docs/guides/user-guide.md
docs/src/content/docs/reference/web-ui.md  # endpoint row and the Anomalies view
```

**Structure Decision**: Selection and rendering live in `internal/engine`
beside `overview_drift.go` and the overview renderers, so `internal/webui`
(which already imports `engine`) reuses them without touching `cli`. Display
text lives in `internal/viewmodel` with the other presentation builders, and
the page module follows the existing `mount(container, ctx)` view contract.

## Complexity Tracking

No violations.
