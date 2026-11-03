# Research: Spot Badge

## Where the data enters

`GetProjectedCostResponse` and `EstimateCostResponse` both carry
`pricing_category` and `spot_interruption_risk_score`. Before this feature,
only `estimateResponseToCostResult` read them, as note text. The projected
path goes through `internal/proto/adapter.go` (`mapProjectedProto`, used by
the single, batch, and chunked calls) and then `mapProtoCostResultToEngine`,
so one pair of edits covers all three projected entry points.

## Decisions

- **Field types**: `PricingCategory` is a string type, like `GrowthType`, so
  JSON reads `"DYNAMIC"` and a new spec value cannot break decoding. Unknown
  enum values map to empty.
- **One label function**: `engine.SpotDisplay`. The overview already has the
  pattern of display strings on `OverviewRowResult` that table, TUI, and web
  all read, so the label lives there (`SpotDisplay`) rather than in
  `internal/viewmodel`, which imports `engine` and could not be called from
  the engine's own table renderer. `viewmodel.EstimateDisplay` carries it for
  the Estimate page.
- **Zero risk**: proto3 cannot tell 0.0 from unset (see the field docs in
  `costsource.proto`), so DYNAMIC with a non-positive score renders
  `risk unknown`. The JSON field is `omitempty`, so unknown and unset look the
  same in data, which matches the spec.
- **`cost projected` table**: the label is prefixed to the Notes column
  (`SPOT · risk 0.12; <notes>`) so no column is added and other rows are
  unchanged. This is also the `cost projected` diff view, which reuses the
  same notes formatter.
- **Overview TUI list**: the Projected column is 12 wide, too narrow for
  `SPOT · risk 0.12`. The list shows `$21.90 SPOT` and widens the column by
  five only while a spot row is loaded, so lists without spot rows keep their
  layout and goldens. The detail view shows the full label with a cyan
  accent. Table cells carry plain text because the table widget measures
  cell width and mishandles embedded color codes.
- **Web**: the tag is a `span.spot-badge` after the amount. Both view
  modules define a three-line helper instead of importing one from `app.js`,
  because the frontend tests stub `app.js` with only `el`.
- **Cache**: results are cached as `[]CostResult` JSON, so the fields
  round-trip. An entry written earlier lacks them and would show a spot
  resource as on-demand until it expired, so the key gains `/pricing-v1`.
  Cost: every existing projected cache entry misses once. The same move was
  made for `/growth-v1`.
- **Overview cache seeding**: no separate seeding path exists; the overview
  calls `GetProjectedCostWithErrors`, which reads and writes the same cache.

## Not done, on purpose

- Actual costs: `GetActualCostResponse` has no category field.
- `OverviewRow.BaselineProjectedCost` does not carry the fields; it only feeds
  delta math.
- The multi-resource estimate table prints the Modified side's tag on the
  change line; the per-side detail is in the JSON.

## Discrepancies with the issue

- The issue lists `internal/viewmodel` as the home of "one shared field".
  The shared field is `OverviewRowResult.SpotDisplay` in `engine` (the
  `CalculateRowDelta` precedent); `viewmodel` only adds the estimate pair.
- The issue says to accent in cyan "in TUI/table/web". Plain CLI tables are
  uncolored by design, so they carry the text only.
