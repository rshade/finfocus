# Implementation Plan: Spot Badge

**Branch**: `1836-spot-badge` | **Date**: 2026-11-02 | **Spec**: `spec.md`

## Summary

Add `PricingCategory` and `SpotInterruptionRisk` to `CostResult` and to the
overview row's projected cost, fill them from the spec fields on the projected
and estimate paths, build one label with `engine.SpotDisplay`, and render that
label on every surface. See `research.md` for the choices and
`contracts/fields.md` for the names the next piece (#1837) builds on.

## Technical Context

- **Language**: Go 1.27.3; finfocus-spec v0.7.10 already has both proto fields
  on `GetProjectedCostResponse` (fields 8 and 9) and `EstimateCostResponse`
  (fields 3 and 4). No spec bump.
- **Storage**: projected cache stores `[]CostResult` JSON; new fields
  round-trip. Key suffix `/pricing-v1`.
- **Testing**: Testify, table-driven; TUI golden files; node tests for the
  web views.
- **Constraints**: non-spot output unchanged; core holds no spot knowledge.

## Constitution Check

- Plugin domain knowledge stays in the plugin: core reads two generic spec
  fields and attaches meaning only to DYNAMIC. PASS.
- No TODOs or stubs. PASS.
- Principle VII: the web hook uses the same Go label function; see `Web UI
  Fit` in `spec.md`. PASS.
- Tests first, Testify only, `t.Parallel()` throughout. PASS.

## Project Structure

### Documentation

```text
specs/1836-spot-badge/
  spec.md
  plan.md
  research.md
  tasks.md
  contracts/fields.md
```

### Source Code

```text
internal/engine/spot.go             PricingCategory, SpotDisplay
internal/engine/types.go            CostResult fields
internal/engine/engine_batch.go     mapProtoCostResultToEngine
internal/engine/estimate.go         estimate path; notes removed
internal/engine/engine.go           /pricing-v1 cache suffix
internal/engine/project.go          cost projected Notes column
internal/engine/overview_*.go       ProjectedCostData, OverviewRowResult
internal/proto/adapter.go           adapter fields, PricingCategoryLabel
internal/viewmodel/estimate.go      EstimateDisplay spot labels
internal/cli/cost_estimate.go       estimate table
internal/tui/                       styles, overview list/detail, cost view,
                                    estimate comparison
internal/webui/static/              overview.js, estimate.js, styles.css
```

## Complexity Tracking

None. One label function, two data fields, no new package.
