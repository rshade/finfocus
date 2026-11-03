# Research: Spot Savings vs On-Demand

## What the spec gives us

finfocus-spec v0.7.10 (already pinned by core and the nested modules) has
`on_demand_equivalent_monthly_cost` as a proto3 `optional double`, so the Go
field is `*float64` and presence is observable. `pluginsdk` validates it
(`ValidateOnDemandEquivalent`: finite, non-negative, DYNAMIC only) for plugin
authors. No spec change and no SDK bump are needed.

## Decisions

- **Pointer on the result.** `CostResult.OnDemandEquivalentMonthly` is a
  `*float64`, so 0 (a present, odd value) and unset stay different through JSON
  and the cache.
- **Percent is stored, not recomputed.** `SpotSavingsPercent` is computed once
  at the mapping boundary and carried with the result, so the label, the JSON,
  and the overview row cannot disagree, and the TUI estimate model's snapshot
  needs no arithmetic.
- **Zero on-demand has no percentage.** `(0 - spot) / 0` is undefined. It reads
  as unavailable, not as 0% and not as a signed infinity.
- **Saving folds into the one label.** Putting it in `SpotDisplay` reaches every
  surface #1836 wired, including the web badges, with no new renderer code.
  The cost is that `SpotDisplay` gains a parameter and every DYNAMIC label
  without the field changes to carry the unavailable marker, which the issue
  requires.
- **Sign.** `-62% vs on-demand` follows the issue and reads as a price change
  against on demand. `spotSavingsPercent` in JSON is the opposite sign (62),
  named for what it is.
- **Defensive drop in core.** `proto.OnDemandEquivalent` repeats the SDK rule so
  a plugin that skips validation cannot put NaN into JSON or a wrong percent on
  screen.

## Cache

The projected cache stores `[]CostResult` as JSON, so the fields round-trip.
The key hashes what is sent to a plugin; nothing new is sent. A cached DYNAMIC
entry written before this change has no on-demand value and reads as
unavailable until it expires. `/pricing-v1` has not shipped, so no released
cache holds such an entry; a released one would need `/pricing-v2`.
