# Contract: Spot fields

Downstream work (#1837 exposure totals, #1838 savings) reads these names. Do
not rename them without updating those specs.

## `engine.CostResult`

| Field | Go type | JSON key | Meaning |
| --- | --- | --- | --- |
| `PricingCategory` | `engine.PricingCategory` (string) | `pricingCategory`, omitted when empty | `STANDARD`, `COMMITTED`, `DYNAMIC`, `OTHER`, or empty (unspecified) |
| `SpotInterruptionRisk` | `float64` | `spotInterruptionRisk`, omitted when 0 | Plugin score in [0, 1]. Meaningful only with `DYNAMIC`; 0 there means unknown |

Methods and helpers (package `engine`):

- `PricingCategory.IsSpot() bool` is true only for `DYNAMIC`.
- `SpotDisplay(category PricingCategory, risk float64) string` is the single
  label builder.
- `(*CostResult).SpotDisplay() string` is nil-safe and calls the function
  above.

## `engine.ProjectedCostData` (overview row)

Same two fields with the same JSON keys: `PricingCategory`,
`SpotInterruptionRisk`. Copied from the `CostResult` in
`enrichProjectedCost`. The baseline projection used for delta math does not
carry them.

## `engine.OverviewRowResult`

`SpotDisplay string` (`spotDisplay`, omitted when empty). Computed once in
`computeDisplayStrings`; empty for error rows.

## `viewmodel.EstimateDisplay`

`BaselineSpot string` (`baselineSpot`) and `ModifiedSpot string`
(`modifiedSpot`), each omitted when empty.

## Plugin-adapter type

`proto.CostResult` gained `PricingCategory string` and
`SpotInterruptionRisk float64`. `proto.PricingCategoryLabel` maps the spec
enum to the strings above; unknown enum values map to empty.

## Label

| Category | Risk | Label |
| --- | --- | --- |
| DYNAMIC | > 0 | `SPOT · risk 0.12` (two decimals) |
| DYNAMIC | <= 0 | `SPOT · risk unknown` |
| anything else | any | empty |

## Cache

Projected cache keys end `/growth-v1/pricing-v1`. Results are stored as JSON
of `[]CostResult`, so the new fields round-trip with no extra code.
