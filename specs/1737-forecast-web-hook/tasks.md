# Tasks: Cost Forecast Web Hook

**Input**: Design documents from `specs/1737-forecast-web-hook/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/http-api.md

**Tests**: Required (constitution Principle II, TDD). Tests are written
before the code they cover.

## Phase 1: Setup

- [X] T001 Record the baseline: run `go test ./internal/forecast/ ./internal/cli/ -run 'Forecast'` and confirm it passes before any change

## Phase 2: Foundational (shared document and builder)

- [X] T002 [P] Write tests for `forecast.Document` JSON shape, `forecast.SeriesList` order, `forecast.NewDocument` budget omission, `forecast.ValidateMonths`, `forecast.ParseGrowth` (every CLI message), and `forecast.InputError` unwrapping in internal/forecast/document_test.go
- [X] T003 Implement `Document`, `NewDocument`, `SeriesList`, `ValidateMonths`, `ParseGrowth`, and `InputError` in internal/forecast/document.go, moving `forecastJSON`, `forecastDocument`, `forecastSeriesList`, and the growth parsing out of internal/cli/cost_forecast.go
- [X] T004 Extract `forecastSettings`, `buildForecast(costs, settings, readHistory)`, `forecastBudgetFor(cfg, currency, noBudget)`, `readForecastHistory(stack)`, and `forecastHistoryFrom(snapshots, currency)` into internal/cli/forecast_build.go, and make `runForecast` in internal/cli/cost_forecast.go call them
- [X] T005 Run the existing `cost forecast` tests (internal/cli/cost_forecast_test.go, internal/cli/cost_forecast_total_test.go) unchanged and confirm identical plain, JSON, and NDJSON output

**Checkpoint**: The CLI behaves exactly as before and the builder is cmd-free.

## Phase 3: User Story 1 - Query the forecast for the session stack (P1) MVP

**Goal**: `POST /api/forecast/query` returns the CLI JSON document plus
display strings for the session stack.

**Independent Test**: Parity test comparing the callback document with
`cost forecast --output json` for the same costs and history.

- [X] T006 [P] [US1] Write handler tests (200 payload and display strings, 503 before load, 502 generic message on callback failure, months default 12, cross-origin and non-JSON POST rejected) in internal/webui/handlers_forecast_test.go
- [X] T007 [P] [US1] Write the CLI/web parity test (no growth, linear 0.10, exponential 0.05; same document) and a callback test that the session engine prices plan-origin resources only in internal/cli/web_forecast_test.go
- [X] T008 [US1] Add `ForecastQuery` and `SessionOptions.Forecast func(context.Context, ForecastQuery) (forecast.Document, error)` in internal/webui/session.go
- [X] T009 [US1] Implement `registerForecast` and `handleForecastQuery` (beginQuery, callback, display strings via `viewmodel.FormatMoney`) in internal/webui/handlers_forecast.go and register it in internal/webui/server.go
- [X] T010 [US1] Implement `(*webData).forecast` with a `priceForecast` seam (snapshot plan-origin descriptors or Terraform planned resources, `ApplyFilters`, `resolveResourceTypes`, `GetProjectedCostDiff(...).AfterCosts()` on the session engine under the query gate) in internal/cli/web_data.go
- [X] T011 [US1] Wire `Forecast: data.forecast` into the session options in internal/cli/web.go

**Checkpoint**: US1 tests pass.

## Phase 4: User Story 2 - Clear errors for invalid settings (P2)

**Goal**: Invalid input returns 400 with the CLI's text.

**Independent Test**: Each invalid case returns `invalid_forecast` with the
same message the CLI prints.

- [X] T012 [P] [US2] Add table tests for months 0 and 37, unknown growth type, unparseable rate, rate below -1, and linear or exponential without a rate (400, `invalid_forecast`, CLI text) in internal/webui/handlers_forecast_test.go and internal/cli/web_forecast_test.go
- [X] T013 [US2] Map `forecast.InputError` to 400 `invalid_forecast` in internal/webui/handlers_forecast.go, and return `InputError` from `(*webData).forecast` before any pricing in internal/cli/web_data.go

## Phase 5: User Story 3 - History beside the projection (P3)

**Goal**: The history series appears when the session stack has history and
can be turned off.

**Independent Test**: With a temp history database the series list matches
the CLI; with `noHistory` or no database there is no history series.

- [X] T014 [P] [US3] Add callback tests for history present, missing database, other currency, and `noHistory` in internal/cli/web_forecast_test.go
- [X] T015 [US3] Resolve the session stack (`snapshot.StackName`, then launch `--stack`) and pass `readForecastHistory` into `buildForecast` from `(*webData).forecast` in internal/cli/web_data.go

## Phase 6: Web Parity (Principle VII)

- [X] T016 Confirm contracts/http-api.md names `handleForecastQuery`, `webData.forecast`, `buildForecast`, `GetProjectedCostDiff`, and the forecast helpers as implemented in specs/1737-forecast-web-hook/contracts/http-api.md
- [X] T017 Render the Overview budget panel (nearest view) against a populated stack with Playwright, call the endpoint, compare with the Web UI Fit section, and record the result in specs/1737-forecast-web-hook/quickstart.md
- [X] T018 [P] Document `POST /api/forecast/query` in docs/src/content/docs/reference/web-ui.md
- [X] T019 No follow-up issue: the Forecast view ships in `specs/1738-arima-forecast/` (T022-T027)

## Phase 7: Polish

- [X] T020 Run `make validate`, `make test`, `make test-race`, `make lint`, `make docs-lint`, and `make test-e2e-web`; fix every failure
- [X] T021 Mark spec status Implemented in specs/1737-forecast-web-hook/spec.md and add a Recent Changes line in CLAUDE.md

## Dependencies

- Phase 2 blocks everything else (T003 before T004 before T005).
- US1 (T006-T011) depends on Phase 2. US2 and US3 depend on US1's handler
  and callback but not on each other.
- Web Parity depends on US1-US3. Polish is last.

## Parallel Examples

- T002 runs alongside the start of T003's design.
- T006 and T007 are different files and run together.
- T012 and T014 run together after US1.
- T018 runs alongside T017.

## Implementation Strategy

MVP is US1: a working endpoint with parity. US2 and US3 are small additions
to the same handler and callback. The view is the follow-up issue.
