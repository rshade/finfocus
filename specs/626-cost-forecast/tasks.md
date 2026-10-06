# Tasks: Cost Forecast

**Input**: [plan.md](plan.md), [spec.md](spec.md)
**Prerequisites**: plan.md, data-model.md, contracts/series.md, research.md

## Phase 1: Setup

- [x] T001 Confirm finfocus-spec v0.7.5 `pricing.ApplyGrowth` is the math helper and no module is added

## Phase 2: Foundational

- [x] T002 Copy `growth_type` from the projected-cost response onto the proto and engine cost results in `internal/proto/adapter.go`, `internal/engine/engine.go`, and `internal/engine/engine_batch.go`
- [x] T003 Test that mapping in `internal/proto/adapter_test.go` and `internal/engine/engine_batch_test.go`

## Phase 3: User Story 1 - Project a stack forward (Priority: P1)

- [x] T004 [P] [US1] Write projection tests in `internal/forecast/forecast_test.go` for linear, exponential, mixed currency, clamp, and a missing rate
- [x] T005 [US1] Implement `Project` in `internal/forecast/forecast.go` with the spec helpers

## Phase 4: User Story 2 - Chart and data (Priority: P2)

- [x] T006 [P] [US2] Write chart and CLI output tests in `internal/forecast/chart_test.go` and `internal/cli/cost_forecast_test.go`
- [x] T007 [US2] Implement `Render` and `finfocus cost forecast` (`internal/forecast/chart.go`, `internal/cli/cost_forecast.go`, `internal/cli/root.go`)
- [x] T008 [US2] Add `finfocus-cost-forecast` to `internal/cli/testdata/mcp/tools.golden`

## Phase 5: User Story 3 - History series (Priority: P3)

- [x] T009 [US3] Include a currency-matched `history` series and warn on a mismatch or a missing file without failing

## Phase 6: Polish

- [x] T010 Document the command in `docs/src/content/docs/commands/forecast.md`, the user guide, README, ROADMAP, and CLAUDE.md
- [x] T011 Run `make validate`, `make test`, `make test-race` for the touched packages, and `make lint`

## Dependencies

US1 before US2. US3 uses the series type from US1 and the command from US2.
T002 is required before the command can see a plugin growth type. Docs after
the command exists.

## Parallel example

T004 and T006 can be written together. T003 is independent of the forecast
package once the mapping fields exist.
