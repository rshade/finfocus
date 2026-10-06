# Data Model: Cost Forecast

## Forecast resource

| Field | Type | Rules |
| --- | --- | --- |
| ID | string | Copied from the projected cost. Used in warnings. |
| Provider | string | Normalized with `resourcetype.NormalizeProvider`. Empty becomes `<none>`. |
| Monthly | number | After-change projected monthly cost. Zero is a real price. |
| Currency | string | Blank is USD. A blank resource next to EUR is mixed USD and EUR and fails. |
| GrowthType | string | `none`, `linear`, `exponential`, or empty. Empty is `none`. |
| GrowthRate | number, optional | Decimal per month. Required for linear and exponential when the user set the type. Nil keeps a plugin linear or exponential resource flat. |

Resources with a structured error, or with notes prefixed `ERROR:` or
`VALIDATION:`, are not resources. They become warnings.

## Point

| Field | Type | Rules |
| --- | --- | --- |
| Time | RFC3339 UTC | First day of the month, 00:00:00Z. |
| Value | number | Sum of projected resource costs for that month. Never negative, never non-finite. |

Month 0 is the current month. Month N is N months later. `--months 12` yields
13 points.

## Series

| Name | Points |
| --- | --- |
| `forecast` | One point per month, always present on success. |
| provider (`aws`, `gcp`, …) | Same months as `forecast`, that provider only. Always computed. Drawn when `--split-providers` is set. |
| `history` | One point per stored snapshot. Omitted when there is no matching database. Not resampled onto the forecast months. |

Budget is a single number on the document, omitted when there is no global
budget or `--no-budget` is set. The chart repeats it as a flat line. It is not
a series, so a later interactive chart can toggle it without guessing which
series is the threshold.

## Projection

Currency, months ahead, forecast series, provider series, warnings. Warnings
are sorted. They include skipped resources, a missing rate, a clamped value,
and the SDK high-growth warning.

## Validation

| Input | Result |
| --- | --- |
| months < 1 or months > 36 | error, no plugin call |
| rate < -1 | error, no plugin call |
| `--growth-type linear` or `exponential` and no rate | error, no plugin call |
| unknown `--output` | error, no plugin call |
| no priced resource | `no cost data available` |
| two currencies | mixed-currency error |
| history file missing | success, no history series |
| history currency differs | success, warning, no history series |
