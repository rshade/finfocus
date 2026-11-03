# Contract: Spot savings fields

Builds on `specs/1836-spot-badge/contracts/fields.md`.

## Plugin input (finfocus-spec v0.7.10)

`optional double on_demand_equivalent_monthly_cost` on
`GetProjectedCostResponse` (field 18) and `EstimateCostResponse` (field 8).
The Go field is `*float64`; `nil` means unset. `pluginsdk` rejects a negative,
non-finite, or non-DYNAMIC value on the plugin side; core drops the same
values on the way in instead of trusting the plugin.

## `engine.CostResult`

| Field | Go type | JSON key | Meaning |
| --- | --- | --- | --- |
| `OnDemandEquivalentMonthly` | `*float64` | `onDemandEquivalentMonthlyCost`, omitted when nil | The plugin's on-demand price. Present only on DYNAMIC. An explicit 0 is kept |
| `SpotSavingsPercent` | `*float64` | `spotSavingsPercent`, omitted when nil | `(onDemand - Monthly) / onDemand * 100`. Nil when the on-demand price is nil or 0 |

## `engine.ProjectedCostData` (overview row)

The same two fields and keys, copied from the `CostResult` in
`enrichProjectedCost`.

## Functions (package `engine`)

- `SpotSavingsPercent(category, spot, onDemand *float64) *float64`: nil unless
  the category is DYNAMIC, `onDemand` is present, finite, and above 0, and
  `spot` is finite and not negative. It is also nil when the result is not
  finite or its magnitude exceeds 999 (a bad plugin figure such as an
  on-demand price near zero), so the label reads unavailable and JSON stays
  encodable.
- `SpotDisplay(category, risk, savingsPercent *float64) string`: the one label.
- `(*CostResult).applyOnDemandEquivalent(*float64)`: sets both fields; called
  by `mapProtoCostResultToEngine` and `estimateResponseToCostResult`.
- `proto.OnDemandEquivalent(category, *float64) *float64`: keeps the value only
  for DYNAMIC, finite, and non-negative; returns a copy.

## Label

| Category | Risk | On-demand | Label |
| --- | --- | --- | --- |
| DYNAMIC | > 0 | 100, spot 38 | `SPOT · risk 0.12 · -62% vs on-demand` |
| DYNAMIC | <= 0 | 100, spot 38 | `SPOT · risk unknown · -62% vs on-demand` |
| DYNAMIC | any | 100, spot 115 | `SPOT · risk 0.12 · +15% vs on-demand` |
| DYNAMIC | any | 100, spot 100 (or rounds to it) | `SPOT · risk 0.12 · 0% vs on-demand` |
| DYNAMIC | any | 100, spot 0.2 | `SPOT · risk 0.12 · -99% vs on-demand` (`-100%` only when spot is exactly 0) |
| DYNAMIC | any | 0.01, spot 300 (beyond the cap) | `SPOT · risk 0.12 · spot savings unavailable` |
| DYNAMIC | any | unset | `SPOT · risk 0.12 · spot savings unavailable` |
| DYNAMIC | any | 0 | `SPOT · risk 0.12 · spot savings unavailable` |
| anything else | any | any | empty |

## Surfaces

The label already reaches every #1836 surface, so those carry the saving with
no further code: `cost projected` notes and diff rows, the `cost estimate`
table and TUI, the overview table, the TUI detail, and the web Overview and
Estimate badges. The overview list in the interactive TUI keeps the short
`SPOT` tag.

## Cache

`[]CostResult` JSON round-trips both fields. No suffix change (see spec FR-008).
