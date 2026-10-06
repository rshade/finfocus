# Implementation Plan: Cost Forecast

**Branch**: `626-cost-forecast` | **Date**: 2026-10-05 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/626-cost-forecast/spec.md`

## Summary

Add `finfocus cost forecast`. It prices the plan the same way `cost projected`
does, keeps each plugin `growth_type`, and projects with
`pricing.ApplyGrowth` from finfocus-spec. Plain output is an asciigraph chart.
JSON and NDJSON emit timestamped series that issue #550 can plot later.
History snapshots are an optional `history` series. No new module and no new
database.

## Technical Context

**Language/Version**: Go 1.27.1
**Primary Dependencies**: finfocus-spec v0.7.5 `sdk/go/pricing` (already in
go.mod), existing `github.com/guptarohit/asciigraph`, Cobra, testify
**Storage**: N/A. Reads the existing cost-history database when `--stack`
has one. Does not create a store.
**Testing**: `go test` with testify. Chart golden file under
`internal/forecast/testdata/`.
**Target Platform**: Linux, macOS, and Windows, same as the CLI
**Project Type**: single Go CLI
**Performance Goals**: Projection is in-memory arithmetic after the existing
projected-cost call. No extra plugin RPC.
**Constraints**: Provider-agnostic. No protocol change. No ntcharts. Months
1–36. Commands succeed with no history file.
**Scale/Scope**: One command, one pure package, growth type threaded through
the existing projected-cost mapping.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Core orchestrates. Plugins still return
  the price and the growth model. Core does not price cloud services.
- [x] **Test-Driven Development**: Unit tests cover the formulas, the chart
  golden, the CLI flags, and the mapping. The chart is not a Bubble Tea view,
  so `internal/tui` goldens do not apply. The full chart text is pinned.
- [x] **Cross-Platform Compatibility**: Stdlib time, asciigraph, and the
  existing CLI. No OS-specific code.
- [x] **Documentation Integrity**: Command page, user guide, README, ROADMAP,
  and CLAUDE.md update with the command.
- [x] **Protocol Stability**: No proto change. Core reads `growth_type` that
  finfocus-spec v0.7.5 already defines.
- [x] **Implementation Completeness**: The command, the math, the chart, and
  the history series are implemented. #550 and #539 are named follow-ups, not
  stubs in this code.
- [x] **Persistence Model**: No new store. Missing history does not fail the
  command.
- [x] **Quality Gates**: `make validate`, `make test`, `make test-race`, and
  `make lint` run from this worktree.
- [x] **Multi-Repo Coordination**: No spec or plugin release required.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/626-cost-forecast/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/series.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/forecast/
├── forecast.go
├── forecast_test.go
├── chart.go
├── chart_test.go
└── testdata/chart.golden

internal/cli/cost_forecast.go
internal/cli/cost_forecast_test.go
internal/proto/adapter.go
internal/engine/types.go
internal/engine/engine.go
internal/engine/engine_batch.go
docs/src/content/docs/commands/forecast.md
```

The command is registered on `cost` and exposed to MCP. The tool golden gains
`finfocus-cost-forecast`.

## Phase 0 and Phase 1

Research is in [research.md](research.md). The series contract is in
[data-model.md](data-model.md) and [contracts/series.md](contracts/series.md).
Usage is in [quickstart.md](quickstart.md).

## Post-design Constitution Check

The design still passes. The chart library stays out of the series type.
`internal/forecast` does not import the CLI, the engine, or the history
database. The CLI composes them.
