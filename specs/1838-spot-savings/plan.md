# Implementation Plan: Spot Savings vs On-Demand

**Branch**: `1838-spot-savings` | **Date**: 2026-11-02 | **Spec**: `spec.md`
**Stacked on**: `1837-spot-exposure-totals`

## Summary

Read the spec's presence-tracked `on_demand_equivalent_monthly_cost`, keep it
on `CostResult` and the overview row, compute `SpotSavingsPercent` once, and
fold the saving (or `spot savings unavailable`) into `engine.SpotDisplay`, the
one label every surface already renders. See `research.md` and
`contracts/fields.md`.

## Technical Context

- **Language**: Go 1.27.3; no new dependency, no spec bump (v0.7.10 has the
  field).
- **Testing**: Testify, table-driven, `t.Parallel()`; golden files for the TUI
  detail view; node tests for the web views.

## Constitution Check

- Plugin domain knowledge stays in the plugin: core reads two plugin numbers and
  the category DYNAMIC, and estimates nothing. PASS.
- Principle VII: see `Web UI Fit` in `spec.md`. PASS.
- No TODOs or stubs; tests first. PASS.

## Source Code

```text
internal/proto/adapter.go               OnDemandEquivalent, CostResult field
internal/engine/spot_savings.go         SpotSavingsPercent, display, apply
internal/engine/spot.go                 SpotDisplay takes the percentage
internal/engine/types.go                CostResult fields
internal/engine/overview_types.go       ProjectedCostData fields
internal/engine/overview_enrich.go      copy to the row
internal/engine/overview_result.go      row label
internal/engine/engine_batch.go         projected mapping
internal/engine/estimate.go             estimate mapping
internal/tui/delta_view.go              estimate pricing snapshot
docs/src/content/docs/guides/user-guide.md
docs/src/content/docs/commands/overview.md
```

The web views need no code change: they render `spotDisplay`, `baselineSpot`,
and `modifiedSpot` verbatim.

## Complexity Tracking

None.

## Verification

2026-11-02, `make build` binary, isolated `FINFOCUS_HOME`, and a throwaway
plugin (not committed) built against finfocus-spec v0.7.10 that answers
DYNAMIC with the field (spot 100, on-demand 263.16), DYNAMIC without it,
DYNAMIC with an on-demand of 0, DYNAMIC with spot above on-demand, and
STANDARD:

- `cost projected` diff: `SPOT · risk 0.12 · -62% vs on-demand`,
  `SPOT · risk 0.20 · spot savings unavailable`, the zero case also
  `spot savings unavailable`, `SPOT · risk unknown · +15% vs on-demand`, and no
  label on the STANDARD row.
- `cost projected --output json`: `onDemandEquivalentMonthlyCost: 263.16` and
  `spotSavingsPercent: 62` on the first row; the no-field row has neither key;
  the zero row has `onDemandEquivalentMonthlyCost: 0` and no
  `spotSavingsPercent`.
- `cost estimate` (single resource, table and JSON): `Modified:  $100.00/mo
  (USD)  SPOT · risk 0.12 · -62% vs on-demand`; JSON carries both fields.
- `overview --plain` prints the same labels beside the projected amount; the
  JSON rows carry `projectedCost.onDemandEquivalentMonthlyCost` and
  `projectedCost.spotSavingsPercent`.
- `--web` (Playwright): the Overview badges read `SPOT · risk 0.12 · -62% vs
  on-demand` and `SPOT · risk 0.20 · spot savings unavailable` on one line
  beside the amount, and the detail panel `Pricing` row shows the full label.

Not verified: the web Estimate page in a browser (covered by the node test and
the viewmodel test); a real aws-ce plugin and `finfocus-demo` (no spot-priced
plugin installed here); the TUI estimate screen interactively (covered by a
snapshot unit test).
