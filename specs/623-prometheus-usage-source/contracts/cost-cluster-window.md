# Contract: `finfocus cost cluster` window

Extends [the run-rate command contract](../../613-cost-cluster/contracts/cost-cluster.md).
Flags and fields not named here stay as they are.

## CLI

```text
finfocus cost cluster
  [--from <time>] [--to <time>]
  ...existing flags...
```

- `--from` and `--to` accept `2006-01-02` and RFC3339.
- `--to` defaults to now when `--from` is set. `--from` is required when
  `--to` is set.
- Date-only values are midnight UTC. A same-day date-only pair is zero-length
  and fails. End must be after start. Future, too-old, and over-long ranges
  fail with the same rules as `finfocus cost actual`.
- Those failures happen before any plugin is loaded and exit 1.
- No window: the request omits `start` and `end`, and the report matches the
  run-rate contract (`mode` `run-rate`, `period` `monthly`, footer cites
  730 hours).
- Plugin selection is unchanged. Two usage sources still require
  `--usage-source`.

## Stats request

With a window, `GetStatsRequest.start` and `GetStatsRequest.end` are that
window. The usage source must return `STATS_MODE_HISTORICAL`. A run-rate
response on a windowed request fails. A historical response with no window
stays `ErrHistoricalUnsupported`.

## Pricing request

Historical pricing is `GetActualCost` over the same instants, with
state-based estimation off. The allocated amount is `TotalCost`. The
projected monthly price is not requested on this path.

## JSON

`mode` is `historical`. `period` is the formatted window (the same string
`engine.FormatPeriod` uses for actual cost), not `monthly`.

`priced[].monthly` keeps its name. For `mode=historical` the number is the
window `TotalCost`, not a monthly rate. `priced=false` and a note still mean
unpriced, including a `$0` actual.

`incomplete` is true when any priceable is unpriced or any stats warning
starts with `incomplete:`.

Table footer for a windowed run:

```text
Mode:    historical (<period>)
```

It does not print `monthly` or `730 h`. Idle, policy, and the incomplete
line are unchanged.

## Errors

| Condition | Result |
| --- | --- |
| Bad window | Exit 1 before plugins. Validation text from `ParseTimeRange` |
| Prometheus URL missing outside the cluster | Exit 1 naming `FINFOCUS_PROMETHEUS_URL` |
| Store unreachable | Exit 1. Not an empty success |
| Usage source rejects the window (kubernetes plugin) | Exit 1 with that plugin's run-rate-only error |
| Prometheus source called with no window | Exit 1, historical only |
| Mode mismatch | Exit 1 naming the requested mode and the returned mode |
| Every node unpriced | Exit 1, same as run-rate |
| Bearer token | Never present in stdout, stderr, or the error string |
