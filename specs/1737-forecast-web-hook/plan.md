# Implementation Plan: Cost Forecast Web Hook

**Branch**: `1737-forecast-web-hook` | **Date**: 2026-10-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1737-forecast-web-hook/spec.md`

## Summary

Expose `finfocus cost forecast` to `finfocus --web` through
`POST /api/forecast/query`. The CLI's flag parsing, budget, history, and
document assembly move into cmd-free functions that the CLI and a new
`webData.forecast` callback both call. The web callback prices the
session's loaded plan-origin resources with the session query engine, so
it opens no plugins. The forecast document type moves to
`internal/forecast` so the CLI JSON and the web payload share one shape;
the handler adds server-formatted display strings.

## Technical Context

**Language/Version**: Go 1.27.3
**Primary Dependencies**: Cobra, `internal/forecast`, `internal/engine`,
`internal/viewmodel` (money formatting), `internal/webui`
**Storage**: Reads the existing per-stack cost-history BoltDB read-only; no
new store
**Testing**: testify; CLI/web parity test in `internal/cli`, handler tests in
`internal/webui`, existing `cost forecast` tests unchanged
**Target Platform**: Linux, macOS, Windows (localhost web server)
**Project Type**: Single Go module (CLI plus embedded web server)
**Performance Goals**: One projected-cost pass per query, the same work as
one CLI run without plugin startup
**Constraints**: `internal/webui` must not import `cli`, `registry`, or
`pluginhost`; no arithmetic in the browser; CLI output byte-identical
**Scale/Scope**: About 8 Go files changed or added, 1 docs page, spec
artifacts

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Orchestration only; pricing still comes
  from plugins through the existing engine.
- [x] **Test-Driven Development**: Parity, handler, and builder tests are
  written before the refactor and endpoint.
- [x] **Cross-Platform Compatibility**: Pure Go, no OS-specific code.
- [x] **Documentation Integrity**: `docs/.../reference/web-ui.md` gains the
  endpoint; CLI docs are unchanged because CLI behavior is unchanged.
- [x] **Protocol Stability**: No protocol change.
- [x] **Implementation Completeness**: The endpoint is complete; the view is
  a filed UX follow-up, as Principle VII prescribes.
- [x] **Persistence Model**: Read-only use of the optional history store; a
  missing database is not an error.
- [x] **Web Parity**: The spec has a Web UI Fit section;
  [contracts/http-api.md](contracts/http-api.md) names the Go methods.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint`,
  `make test-e2e-web` planned.
- [x] **Multi-Repo Coordination**: None; core only.

**Violations Requiring Justification**: None.

Post-design re-check: unchanged, all gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/1737-forecast-web-hook/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── http-api.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/forecast/
├── document.go          # Document (moved forecastJSON), InputError,
│                        # ParseGrowth, ValidateMonths, SeriesList,
│                        # NewDocument
└── document_test.go
internal/cli/
├── cost_forecast.go     # runForecast uses the shared builder
├── forecast_build.go    # forecastSettings, buildForecast,
│                        # forecastBudgetFor, readForecastHistory
├── web_data.go          # webData.forecast, priceForecast seam
├── web.go               # SessionOptions.Forecast wiring
└── web_forecast_test.go # CLI/web parity and callback tests
internal/webui/
├── session.go           # SessionOptions.Forecast
├── server.go            # registerForecast
├── handlers_forecast.go # POST /api/forecast/query, display strings
└── handlers_forecast_test.go
docs/src/content/docs/reference/web-ui.md
```

**Structure Decision**: Single project. The shared document lives in
`internal/forecast`, which `internal/webui` may import; pricing and flag
glue stay in `internal/cli`.

## Complexity Tracking

No violations.
