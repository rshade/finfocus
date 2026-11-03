# Research: Spot Exposure Totals

## Where the totals live

- `cost projected` (no plan diff) and every JSON summary go through
  `engine.AggregateResults`, which builds `CostSummary`. It does not check
  currency, so the exposure runs its own currency check.
- `cost projected` with a plan goes through `DiffResult`; `renderDiffJSON`
  already reuses `AggregateResults(diff.AfterCosts())`, so `summary.spot` is
  the after-state exposure with no extra code.
- The overview totals are `OverviewTotals`, built once by
  `summarizeOverviewRows` and read by the table footer, JSON, and the web
  `totalsPayload`. The overview TUI has no totals line.

## Decisions

- **Absent, not zero.** `weightedRisk` and `spotShare` are pointers so the
  JSON can omit them; a zero would read as "no interruption risk".
- **One function.** `ComputeSpotExposure` takes plain items (cost, currency,
  category, risk) so the cost-result and overview-row paths share it.
- **Diff basis.** Entries are already one per resource, so summing `Before`
  and `After` once each cannot double count a replacement. The stack-wide
  total is left out: it would need state resources repriced against the full
  inventory, and the issue asks for the plan basis.
- **Mixed currencies.** The overview refuses to render at all; `cost
  projected` sums them. The exposure returns nil for either rather than add
  amounts of different currencies.
- **Risk validity.** Known means finite, above 0, and at most 1, even though
  the adapter already guarantees it; the aggregation does not trust its input.
