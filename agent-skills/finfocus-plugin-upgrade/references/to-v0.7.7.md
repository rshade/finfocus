# Upgrade to finfocus-spec v0.7.7: spot on-demand equivalent, cost parsing, recommendation history

v0.7.7 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for most plugins. The
items below are optional additions, except the one rule for plugins that start
advertising `COST_PARSE`.

## What changed

- `GetProjectedCostResponse` and `EstimateCostResponse` gain
  `optional double on_demand_equivalent_monthly_cost` (fields 18 and 8). It is
  the STANDARD on-demand monthly cost of the same resource when the response
  selects `DYNAMIC` (spot) pricing, in the same currency and monthly basis as
  `cost_per_month` / `cost_monthly`.
- A new optional `CostParseService` with one `Parse` RPC and the
  `PLUGIN_CAPABILITY_COST_PARSE` capability, so a plugin can turn inputs into
  priceable resource descriptors.
- A new optional `GetRecommendationHistory` RPC that returns a plugin's
  lifecycle events (`LifecycleEvent`) for a recommendation id.

## Spot pricing plugins: report the on-demand equivalent

A plugin that returns `DYNAMIC` pricing can set the field so hosts show spot
savings. Absent means unknown, and a present `0.0` is a real price, so hosts do
not treat absence as zero. A plugin that leaves it unset stays valid; hosts then
show spot savings as unavailable.

```go
pluginsdk.WithProjectedCostOnDemandEquivalent(onDemandMonthly)
// EstimateCost responses: pluginsdk.WithOnDemandEquivalent(onDemandMonthly)
```

The validation rules:

- A present value on any category other than `DYNAMIC` fails validation,
  including a present `0.0`.
- A present value must be finite and non-negative. There is no upper bound and
  no required ordering against the spot cost.
- The value is advisory and never summed into the selected cost. If a
  `STANDARD` `price_options` entry describes the same equivalent, the two
  should agree; the SDK does not cross-check them.
- `pluginsdk.CheckOnDemandEquivalentConsistency` returns advisory warnings, for
  example `DYNAMIC` with the field unset.

Run your conformance tests after setting it: `ValidateGetProjectedCostResponse`
and `ValidateEstimateCostResponse` now reject a value on a non-`DYNAMIC`
category.

## Plugins that parse inputs: advertise the parse limits

Only plugins that implement `CostParseService` are affected. A plugin that lists
`PLUGIN_CAPABILITY_COST_PARSE` must also advertise its limits, or
`GetPluginInfo` is rejected with `ErrInvalidParseLimits`:

```go
pluginsdk.WithParseLimits(formats, inputModes, maxInputBytes)
```

A malformed advertisement is rejected, not repaired. Plugins that do not parse
need no change.

## Recommendation plugins: history is optional

Implement `pluginsdk.RecommendationHistoryProvider` only if your plugin keeps
server-side lifecycle history. A plugin that does not implement it answers
`Unimplemented`, which hosts treat as "no server-side history". An empty
`recommendation_id` is rejected with `InvalidArgument`, and event order is not
guaranteed.

## Field numbers

The `EstimateCostResponse` comment that held fields for a future
`cost_breakdown` now says it must use field 9 or later, because field 8 is
`on_demand_equivalent_monthly_cost`.
