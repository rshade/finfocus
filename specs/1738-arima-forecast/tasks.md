# Tasks: Statistical Cost Forecast (ARIMA and Drivers)

**Input**: Design documents from `specs/1738-arima-forecast/`
**Tests**: Required (constitution Principle II). Tests precede code.

## Phase 1: Setup

- [X] T001 Add `github.com/rshade/goarima` (commit `6420b78`, untagged) to go.mod and go.sum (`go get`), and confirm `make validate` passes

## Phase 2: Foundational

- [X] T002 [P] Write spend CSV tests (header, both date formats, daily and monthly cadence, gaps, duplicates, mixed cadence, bad value, negative value, missing column, empty file; line numbers in every message) in internal/forecast/spend_test.go
- [X] T003 Implement `SpendHistory`, `Cadence`, and `ParseSpendCSV` returning `InputError` in internal/forecast/spend.go
- [X] T004 Add `Model`, `ModelOrder`, `Confidence`, `Lower`, `Upper`, `Baseline`, `Spend` to `Projection`, the document fields, and the series order to `SeriesList`/`NewDocument` in internal/forecast/forecast.go and internal/forecast/document.go, with tests that growth documents are unchanged in internal/forecast/document_test.go

## Phase 3: User Story 1 - Forecast from past spend (P1) MVP

**Goal**: `--model arima --spend-history` projects monthly totals with bounds.

**Independent Test**: Fixture CSVs produce monthly totals within the
success-criteria tolerance and bounds that contain them.

- [X] T005 [P] [US1] Write `ProjectStatistical` tests (180-day trend plus weekly pattern within 5% for three months and weekly seasonal order; monthly input; partial current month; constant history; too short; bounds contain values; deterministic) in internal/forecast/statistical_test.go
- [X] T006 [US1] Implement `ProjectStatistical(history, months, start, confidence, currency)` with goarima, daily-to-monthly sums, and `ValidateConfidence` in internal/forecast/statistical.go
- [X] T007 [P] [US1] Write CLI tests for `--model`, `--spend-history`, `--confidence` validation before pricing, the arima JSON (model fields, series order, baseline equals the growth forecast), NDJSON meta, the plain chart, and the `--split-providers` warning in internal/cli/cost_forecast_arima_test.go
- [X] T008 [US1] Add the flags, parse the CSV in `forecastFlagSettings`, extend `forecastSettings`, select the model and attach the baseline in `buildForecast`, warn that `--split-providers` is ignored, and emit model fields in NDJSON in internal/cli/cost_forecast.go and internal/cli/forecast_build.go
- [X] T009 [US1] Draw lower, upper, and baseline lines and name the model in the caption in internal/forecast/chart.go, with tests in internal/forecast/chart_test.go

## Phase 4: User Story 2 - Business drivers (P2)

**Goal**: Exclusions, steps, cap, and floor give exact, reported results.

**Independent Test**: Driver tables on fixed series produce exact values.

- [X] T010 [P] [US2] Write driver tests (parse errors, interpolation, end drops, step before month 0, negative step, cap with bounds, floor, warnings, growth-model use, `--exclude` requiring arima) in internal/forecast/drivers_test.go
- [X] T011 [US2] Implement `ParseExclude`, `ParseStep`, `ValidateGrowthCap`, `Drivers.Exclude`, and `Drivers.Apply` in internal/forecast/drivers.go
- [X] T012 [US2] Add `--exclude`, `--step`, `--growth-cap` flags and apply drivers in `buildForecast` for both models in internal/cli/cost_forecast.go and internal/cli/forecast_build.go, with CLI tests in internal/cli/cost_forecast_arima_test.go

## Phase 5: User Story 3 - Dashboard (P3)

**Goal**: The web query accepts the same inputs and matches the CLI.

**Independent Test**: Parity test between the web document and CLI JSON.

- [X] T013 [P] [US3] Write web parity and input-error tests for the new fields in internal/cli/web_forecast_test.go and a handler round-trip test in internal/webui/handlers_forecast_test.go
- [X] T014 [US3] Add the `ForecastQuery` fields in internal/webui/handlers_forecast.go and map them in `webForecastSettings` in internal/cli/web_data.go

## Phase 6: Web Parity (Principle VII)

- [X] T015 Confirm specs/1738-arima-forecast/contracts/http-api.md names the implemented methods
- [X] T016 Render the Overview view and query the endpoint with Playwright against a populated stack; record in specs/1738-arima-forecast/quickstart.md
- [X] T017 [P] Document the flags, CSV format, drivers, and series in docs/src/content/docs/commands/forecast.md and the query fields in docs/src/content/docs/reference/web-ui.md
- [X] T018 No UX follow-up issue: the Forecast view ships in this feature (T022-T027)

## Phase 6b: Forecast view (FR-015)

- [X] T022 [P] Write renderer tests (query body for growth and arima, CSV file read, months-aligned table from display strings, totals, warnings, server error text, abort on unmount) in internal/webui/frontendtest/forecast.test.mjs
- [X] T023 Implement the view in internal/webui/static/views/forecast.js
- [X] T024 Register `#/forecast` in internal/webui/static/app.js and the header link in internal/webui/static/index.html; add layout rules to internal/webui/static/styles.css
- [X] T025 Add `TestWebUIForecast` (real binary, routed endpoint responses, 1280px and 390px, light and dark, no overflow, contrast) in test/e2e/web_forecast_test.go
- [X] T026 [P] Document the Forecast view in docs/src/content/docs/reference/web-ui.md and the guide docs/src/content/docs/guides/web-ui.md
- [X] T027 Mark specs/1737-forecast-web-hook/tasks.md T019 as satisfied by this view

## Phase 7: Polish

- [X] T019 Update the MCP tool golden if the flag schema changed (`UPDATE_GOLDEN=1 go test ./internal/cli/ -run MCP`), and check `agent-skills` pins in internal/cli/cost_skills_test.go
- [X] T020 Run `make validate`, `make test`, `make test-race`, `make lint`, `make docs-lint`, `make test-e2e-web`; fix every failure
- [X] T021 Mark the spec Implemented and add a Recent Changes line in CLAUDE.md

## Dependencies

Phase 2 blocks all stories. US1 before US2 (drivers apply to its output)
and US3. Web Parity after US1-US3; Polish last.

## Parallel Examples

T002 alongside T004 design; T005 and T007 together; T010 and T013 together;
T017 alongside T016.

## Implementation Strategy

MVP is US1. Drivers and the web fields are additive.
