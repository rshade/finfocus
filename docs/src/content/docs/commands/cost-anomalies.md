---
title: cost anomalies
description: List resources whose month-to-date run-rate has diverged from the projected month
---

`finfocus cost anomalies` lists rows by two methods. When a resource has
enough daily actuals stored, its latest complete day is scored against its own
recent days (`method: robust-median`). When it does not, the overview's drift
warnings stand (`method: drift`): a row appears when a resource's extrapolated
monthly run-rate differs from its projected monthly cost by more than 10%. An
agent or a person gets the short list instead of reading every overview row.

The command runs the same pricing pipeline as `finfocus overview`, so it is as
slow as `overview --output json`. It does not change the overview or the drift
calculation, and the drift path adds no threshold of its own. The web dashboard
has an **Anomalies** page that shows the same list from the rows it already
holds, served at `GET /api/overview/anomalies`; see the
[web UI reference](../reference/web-ui.md).

## Usage

```bash
finfocus cost anomalies
finfocus cost anomalies --stack prod --output json
finfocus cost anomalies --pulumi-state state.json --filter provider=aws
finfocus cost anomalies --from 2026-10-01 --to 2026-10-15 --output ndjson
```

## Options

| Flag | Description | Default |
| --- | --- | --- |
| `--stack` | Pulumi stack for auto-detection (inherited from `cost`) | Current stack |
| `--pulumi-state` | Path to Pulumi state JSON | Auto-detected |
| `--pulumi-json` | Path to Pulumi preview JSON; also runs with the plan | None |
| `--from`, `--to` | Window start and end, `2006-01-02` or RFC3339. They also bound the days the daily series is read from | Start of the month to now |
| `--adapter`, `-a` | Restrict to one adapter plugin | All |
| `--filter`, `-f` | `key=value`, repeatable. Keys: `type`, `status`, `provider` | None |
| `--output` | `table`, `json`, or `ndjson` | `table` |

The command takes no positional arguments. Without `--pulumi-json` it reads the
deployed state and does not run `pulumi preview`, because drift compares actual
spend with deployed resources. `--filter` uses the overview's keys, so `tag:`
filters are not accepted.

## Output

JSON carries the number of rows the overview produced next to the list, so an
empty list is distinguishable from a failed load:

```json
{
  "considered": 49,
  "anomalies": [
    {
      "resource": "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-1",
      "type": "aws:ec2/instance:Instance",
      "period": { "start": "2026-10-01T00:00:00Z", "end": "2026-10-11T00:00:00Z" },
      "baseline": 720,
      "observed": 1080,
      "deviation": 360,
      "percentDrift": 50,
      "severity": "warning",
      "method": "drift",
      "currency": "USD"
    }
  ]
}
```

A robust-median row carries the baseline window instead of a projected month:

```json
{
  "considered": 49,
  "anomalies": [
    {
      "resource": "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-1",
      "type": "aws:ec2/instance:Instance",
      "period": { "start": "2026-10-15T00:00:00Z", "end": "2026-10-15T00:00:00Z" },
      "baseline": 83.5,
      "observed": 412,
      "deviation": 328.5,
      "percentDrift": 393.4,
      "severity": "warning",
      "method": "robust-median",
      "currency": "USD",
      "baselineWindow": { "start": "2026-10-01T00:00:00Z", "end": "2026-10-14T00:00:00Z" }
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `resource` | Resource URN |
| `baseline` | `drift`: projected monthly cost, normalized to the calendar month. `robust-median`: the median daily cost of the days before the observation |
| `observed` | `drift`: run-rate extrapolated to a full month, not the month-to-date spend. `robust-median`: the latest complete UTC day's amount |
| `deviation` | `observed` minus `baseline` |
| `percentDrift` | `deviation` as a percentage of `baseline`. Omitted when a robust-median row's median is 0, because the percentage is undefined then |
| `severity` | Always `warning`: a row exists only past a documented cutoff |
| `method` | `drift` or `robust-median` |
| `currency` | Currency of the actual cost, or of the daily series |
| `baselineWindow` | `robust-median` only: the first and last day inside the median |

Rows are ordered by absolute deviation, largest first, then by URN. NDJSON
prints one anomaly per line and nothing when the list is empty. The table
prints a `N of M resources` line and never a total, because rows can be in
different currencies. Its `DRIFT %` column shows `n/a` for a robust-median row
whose median is 0, because that percentage is undefined; the web page marks the
same rows `n/a`.

## The robust-median method

A resource whose stored daily actuals cover at least 14 earlier days in one
currency gets its drift row replaced. The latest complete UTC day of the
series is the observation; the current partial day is not an observation.
The baseline is the median of those earlier points, and the observation day is
not part of the median, so the day under test cannot pull the median toward
itself.

The score is a robust z-score: `0.6745 * (observed - median) / MAD`, where the
MAD is the median of `|point - median|` over the same baseline points. A day
is reported when the absolute score is at least 3.5, with severity `warning`.
Both the 14-day minimum and the 3.5 cutoff are fixed in the code, not config
keys. The 0.6745 factor exists to put the score on the standard z-score's
scale for a normal sample; cost days are not normal, and 3.5 is a cutoff, not
a probability.

A missing day is a gap. It is never stored as zero, never interpolated, and it
does not count toward the 14. A resource's currencies are never pooled into
one median: a currency change splits the series, and each side needs its own
14 points. A resource appears at most once, and when no daily series qualifies
for it, its drift row stands unchanged.

The daily series follows the dates you ask for. `--from` and `--to` bound the
days it reads, and without them the whole stored history of the stack is
used: the daily store is already a rolling history, and the default cost
window (the current month, which the drift comparison needs) never truncates
it. The method therefore works as soon as the store holds fifteen days of one
currency, instead of only in the second half of a month.

The series belongs to one stack, and the command reads the stack it just
priced: `--stack` names it when given, and the stack the overview resolved
does otherwise. A stack that has never been collected has no series, so every
resource keeps its drift row.

Two things stay silent by design:

- **A flat series.** When the MAD is 0 the score is undefined, so no
  robust-median row is emitted and the drift row remains. A flat series hides
  a spike from the MAD, and no second threshold catches that today.
- **A stack that has never been collected.** With no stored daily file the
  output is exactly the drift list.

## What is not an anomaly

The overview already leaves drift unset for these, and so does this command:

- Drift at or under 10%.
- Less than two days of data in the window.
- A new resource with no actual spend yet.
- A deleted resource with no projection.
- A resource a preview marks for deletion, even if it has drift data.
- A resource with fewer than 14 earlier daily points in its currency: its
  drift row stands, and if drift did not fire either, it gets no row.

A resource whose cost lookup failed is also left out, even if drift data is
present. An empty stack exits 0 with an empty list.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success, including an empty list |
| 1 | Pipeline or plugin failure |
| 2 | Invalid input: an unknown `--output`, a bad date, or a bad filter. These fail before any state is loaded |

## MCP

The command is exposed as the MCP tool `finfocus-cost-anomalies`. See the
[MCP guide](../guides/mcp.md).
