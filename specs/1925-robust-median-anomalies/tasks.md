# Tasks: Robust-Median Method for Cost Anomalies

**Input**: Design documents in `specs/1925-robust-median-anomalies/`
**Prerequisites**: plan.md, spec.md
**Tests**: Required (Constitution Principle II). Each engine table task
precedes the scoring code it covers and must fail first. Use testify only.

**Format**: `[ID] [P?] [Story] Description`

## Phase 1: Foundational scoring (User Story 1)

- [x] T001 Add the three constants to `internal/engine/anomalies.go`:
  `robustMedianMinBaseline` (14), `robustMedianZCutoff` (3.5), and
  `robustZNormalScore` (0.6745), beside the existing method constants and in
  the spirit of `driftWarningThreshold`. No config key.
- [x] T002 Table tests first in
  `internal/engine/anomalies_method_test.go` (one hand-checkable
  arithmetic-progression baseline so every z-score is derived by hand): the
  qualifying row's method, baseline, observed, deviation, percent drift,
  severity, currency, period, and baseline window; the 3.5 boundary (at,
  just under, just over); the 14-point minimum against 13; a two-day gap
  that must neither count nor zero-fill; a one-gap span that falls under the
  minimum; a flat series (MAD 0) with a drift row that survives and one
  without; a zero median that omits `percentDrift` from the JSON while
  reporting `deviation`; two currencies in one resource; a negative miss
  under the cutoff; statistical replacement of a drift row with no second
  row; the drift fallback on a short series; equal inputs giving equal
  output; and an empty series matching the drift output.
- [x] T003 Extend the shared function `engine.SelectAnomalies` with the
  daily point input, currency grouping, the observation/median/MAD/z
  scoring, the baseline window, the zero-median percentDrift omission, the
  per-URN replacement rule, and deletion/cost-error skips. Rendering gains
  the `baselineWindow` field and NDJSON carries it per line.

## Phase 2: CLI reader

- [x] T004 Add `loadDailyAnomalySeries`, `trimDailySeries`, and stack
  targeting in `internal/cli/daily_series.go`: window and current-partial-day
  filtering, missing file is no series, missing day stays missing, no
  interpolation. Table tests in `daily_series_test.go`.
- [x] T005 Wire the series into `runCostAnomalies` through `OnReady`; a
  series read failure fails the command. CLI test with a fixture store shows
  `method: robust-median` in the JSON, and a short-history test shows the
  drift fallback in `cost_anomalies_daily_test.go`.
- [x] T005a Bound the daily series by the dates the caller asked for: an
  explicit `--from` or `--to` clips the stored history, and the implicit
  default cost window (the current month) does not, so the method does not go
  silent in the first half of every month. Two CLI tests cover both sides.
- [x] T005b Name the series' stack: `--stack` wins, and the stack the
  overview resolved is the fallback, so the method also works for a bare
  `finfocus cost anomalies` and for an MCP client that passes no stack. The
  CLI test asserts the resolved stack reaches the reader.

## Phase 3: Web parity

- [x] T006 The anomalies handler calls `engine.SelectAnomalies` with the
  session's stored daily points; the handler test shows the same rows as the
  CLI for the same session rows and the same series, and the parity test
  compares rendered JSON. No page JavaScript computes statistics;
  `internal/webui` still does not import `internal/cli`.

## Phase 4: Docs

- [x] T007 Document `method: drift` and `method: robust-median`, the
  baseline window, the omitted `percentDrift`, the 14-day minimum, the 3.5
  cutoff (not config), the gap rule, the currency split, the MAD-of-zero
  hole, and the MCP field list in `docs/src/content/docs/commands/cost-anomalies.md`,
  `docs/src/content/docs/guides/mcp.md`, `docs/src/content/docs/guides/user-guide.md`,
  and `docs/src/content/docs/reference/web-ui.md`.

## Phase 5: Verification

- [x] T008 `go test ./...` green, `make validate` and `make lint` pass, and
  the MCP tool list (`tools.golden`) untouched.
