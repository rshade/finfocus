# Upgrade to finfocus-spec v0.7.11: spot fields on recommendations

v0.7.11 removed and renamed nothing in the protocol. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for the plugin binary. The
release adds two optional fields; a plugin that does not recommend spot capacity
needs no change.

## What changed

- `Recommendation.pricing_category` is the pricing model of the recommended
  target state. `DYNAMIC` means the recommendation moves the resource to spot
  capacity. `UNSPECIFIED` (the default) says nothing.
- `Recommendation.spot_interruption_risk_score` is the interruption probability
  of the recommended spot target, from 0.0 to 1.0. It means what the same-named
  field means on `GetProjectedCostResponse`: 0.0 is unknown.
- `pluginsdk.ValidateRecommendation` checks both fields.
- Plugin log lines for handled declines are written at debug level instead of
  `ERROR`.

## Plugins that recommend spot capacity

Set `pricing_category` to `DYNAMIC` and, when known, a score above 0 and at most
1.0. A non-zero score is valid only with `DYNAMIC`, and it must be finite.
`action_type` stays `MODIFY` or `MIGRATE`; there is no spot action type, and
savings stay in `impact`. Current core does not display these fields on
recommendations.

## Plugins that do not recommend spot capacity

Leave both fields unset. A zero score is valid for every category, so existing
recommendations stay valid.
