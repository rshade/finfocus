# Implementation Plan: Statistical Cost Forecast (ARIMA and Drivers)

**Branch**: `1738-arima-forecast` | **Date**: 2026-10-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1738-arima-forecast/spec.md`

## Summary

Add `--model arima` to `cost forecast`. A `date,value` spend CSV is parsed
and validated in `internal/forecast`, excluded ranges are interpolated, and
`github.com/rshade/goarima` auto-selects a (seasonal) ARIMA model. Daily
results are summed into the command's calendar-month points with
conservative bounds. Deterministic drivers (steps, growth cap, zero floor)
then adjust the forecast and its bounds; steps and the cap also apply to the
growth model. The existing plan projection becomes the `baseline` series.
The shared `buildForecast` from 1737 carries the new settings, so the web
hook gains the feature by extending `ForecastQuery`.

## Technical Context

**Language/Version**: Go 1.27.3
**Primary Dependencies**: `github.com/rshade/goarima` at commit `6420b78`
(new, pure Go, no transitive dependencies, MIT); existing Cobra, asciigraph,
`internal/forecast`, `internal/webui`
**Storage**: None; the CSV is read once
**Testing**: testify; table tests for CSV parsing and drivers, accuracy
fixtures for the model, CLI JSON/NDJSON tests, web parity test
**Target Platform**: Linux, macOS, Windows
**Project Type**: Single Go module
**Performance Goals**: A 3-year daily history fits in well under a second
**Constraints**: Growth-model output byte-identical; no input error after a
plugin starts; `internal/webui` imports no CLI packages
**Scale/Scope**: 3 new `internal/forecast` files, CLI flags and builder
changes, web query fields, docs

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: The model forecasts user-supplied spend
  in core orchestration; no provider pricing logic enters core, and plugins
  still price the baseline.
- [x] **Test-Driven Development**: Parser, driver, and model tests precede
  the code.
- [x] **Cross-Platform Compatibility**: Pure Go dependency.
- [x] **Documentation Integrity**: `commands/forecast.md`, web reference,
  CLAUDE.md updated.
- [x] **Protocol Stability**: No protocol change.
- [x] **Implementation Completeness**: All flags and the web hook are
  complete; the view is the UX follow-up.
- [x] **Persistence Model**: No state.
- [x] **Web Parity**: Web UI Fit present; contract names the Go methods.
- [x] **Quality Gates**: validate, test, race, lint, docs-lint, e2e-web.
- [x] **Multi-Repo Coordination**: The fork `rshade/goarima` fix is on its
  `main` (commit `6420b78`); no finfocus-spec change.

**Violations Requiring Justification**: None.

Post-design re-check: unchanged.

## Project Structure

```text
specs/1738-arima-forecast/
├── spec.md  plan.md  research.md  data-model.md  quickstart.md  tasks.md
├── contracts/http-api.md
└── checklists/requirements.md

internal/forecast/
├── spend.go            # ParseSpendCSV, SpendHistory, cadence detection
├── drivers.go          # ParseExclude, ParseStep, ValidateGrowthCap,
│                       # ValidateConfidence, Drivers.Apply, excludes
├── statistical.go      # ProjectStatistical (goarima wrapper, monthly sums)
├── document.go         # Model, ModelOrder, Confidence, extra series
├── chart.go            # draw lower, upper, baseline
└── *_test.go
internal/cli/
├── cost_forecast.go    # new flags, NDJSON meta fields
├── forecast_build.go   # model selection, baseline, drivers
├── web_data.go         # webForecastSettings for new query fields
└── *_test.go
internal/webui/handlers_forecast.go  # ForecastQuery fields
docs/src/content/docs/commands/forecast.md
docs/src/content/docs/reference/web-ui.md
```

**Structure Decision**: The math and parsing live in `internal/forecast`
(pure, testable, importable by both CLI and web); flag glue stays in
`internal/cli`.

## Complexity Tracking

No violations.
