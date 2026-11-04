# Implementation Plan: Robust-Median Method for Cost Anomalies

**Branch**: `agent/TASK-1` | **Date**: 2026-11-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/1925-robust-median-anomalies/spec.md`
(Upstream: rshade/finfocus#1863)

## Summary

`finfocus cost anomalies` grows a second method. `engine.SelectAnomalies`
(#1593's shared function) takes the stack's stored daily actuals as an
additional input: when one resource's latest complete UTC day has 14 or more
earlier points in one currency, it is scored against those points with a
robust z-score (`0.6745 * (observed - median) / MAD`), and a score of at
least 3.5 replaces the resource's drift row with one `method: robust-median`
row. Series are split by currency, gaps are never zeros, and a MAD of 0 or a
short history leaves the drift row standing. The stored series is a rolling
history of the stack: an explicit `--from` or `--to` bounds it and the
implicit default cost window does not. The series belongs to one stack:
`--stack` names it, and the stack the overview resolved does otherwise. The
CLI reads the series from the #1862 store through its history-directory
reader; the web handler calls the same function with the session's points. No
new command, flag, MCP tool, config key, web page, or scorer.

## Technical Context

**Language/Version**: Go 1.27.3
**Primary Dependencies**: Cobra, testify, the existing `internal/history`
daily store (#1862). No finfocus-spec bump.
**Storage**: Read-only over `<project>@<stack>.daily.db`; this feature
writes nothing.
**Testing**: Table-driven testify tests in `internal/engine`,
`internal/cli`, and `internal/webui`; the CLI table covers the JSON `method`
through a fixture store, and a web parity test keeps the handler on the same
rows as the command.
**Target Platform**: Linux, macOS, Windows
**Project Type**: Single Go CLI module
**Constraints**: `internal/webui` must keep not importing `internal/cli`;
scoring is in Go, never in page JavaScript. `CalculateCostDrift` and the
drift threshold are unchanged. The three constants live in
`internal/engine/anomalies.go` next to the scoring function, not in
`config.hujson`.
**Scale/Scope**: One extra pass over the stored points per command run
(which is as slow as `overview --output json` already is); one function with
one new parameter.

## Constitution Check

*GATE: Must pass before implementation.*

- [x] **Plugin-First Architecture**: No new plugin RPC; the store already
  holds plugin-returned amounts.
- [x] **Test-Driven Development**: The table tests below exercise the
  boundary, gap, MAD-zero, median-zero, threshold-edge, currency, fallback,
  replacement, and single-row behavior first.
- [x] **Cross-Platform Compatibility**: Only existing storage and time
  handling; no new path code.
- [x] **Documentation Integrity**: The command page, MCP guide, user guide,
  and web UI reference document `method` values, the 14-day minimum, the 3.5
  cutoff, and the MAD-of-zero hole.
- [x] **Protocol Stability**: No proto or finfocus-spec change. MCP tool
  list unchanged (`finfocus-cost-anomalies` already exists).
- [x] **Implementation Completeness**: No stubs; a read failure fails the
  command instead of silently dropping the method.
- [x] **Persistence Model**: Read-only over the optional store; an absent
  file is no series, not an error.
- [x] **Web Parity**: The existing anomalies hook gains the new fields; the
  same function serves both surfaces. No new page and no new follow-up issue:
  the #1590 UX follow-up already covers the page.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint`, and
  `make test-race` before the pull request.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/1925-robust-median-anomalies/
├── spec.md
├── plan.md
└── tasks.md
```

### Source Code (relative to repository root)

```text
internal/engine/
├── anomalies.go              # constants, scoring, selection wiring
├── anomalies_method_test.go  # the table tests for this feature

internal/cli/
├── cost_anomalies.go         # series parameter, read inside OnReady
├── daily_series.go           # window trim, gap behavior, stack targeting
├── cost_anomalies_daily_test.go
├── daily_series_test.go

internal/webui/
├── handlers_anomalies.go     # calls the shared function with DailyPoints
├── handlers_anomalies_test.go
├── parity_anomalies_test.go

docs/src/content/docs/commands/cost-anomalies.md
docs/src/content/docs/guides/mcp.md
docs/src/content/docs/guides/user-guide.md
docs/src/content/docs/reference/web-ui.md
```

## Post-Design Constitution Check

The design adds one parameter and one scoring branch to the shared function;
every gate above still holds once the wiring blocks are in place.
