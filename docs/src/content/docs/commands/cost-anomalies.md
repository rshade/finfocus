---
title: cost anomalies
description: List resources whose month-to-date run-rate has diverged from the projected month
---

`finfocus cost anomalies` lists the overview's drift warnings as rows. A row
appears when a resource's extrapolated monthly run-rate differs from its
projected monthly cost by more than 10%. An agent or a person gets the short
list instead of reading every overview row.

The command runs the same pricing pipeline as `finfocus overview`, so it is as
slow as `overview --output json`. It does not change the overview or the drift
calculation, and it adds no threshold of its own. The web dashboard has an **Anomalies**
page that shows the same list from the rows it already holds, served at
`GET /api/overview/anomalies`; see the [web UI reference](../reference/web-ui.md).

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
| `--stack`, `-s` | Pulumi stack for auto-detection (inherited from `cost`) | Current stack |
| `--pulumi-state` | Path to Pulumi state JSON | Auto-detected |
| `--pulumi-json` | Path to Pulumi preview JSON; also runs with the plan | None |
| `--from`, `--to` | Window start and end, `2006-01-02` or RFC3339 | Start of the month to now |
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

| Field | Meaning |
| --- | --- |
| `resource` | Resource URN |
| `baseline` | Projected monthly cost, normalized to the calendar month |
| `observed` | Run-rate extrapolated to a full month. It is not the month-to-date spend |
| `deviation` | `observed` minus `baseline` |
| `percentDrift` | `deviation` as a percentage of `baseline` |
| `severity` | Always `warning`: a row exists only past the 10% threshold |
| `method` | Always `drift` |
| `currency` | Currency of the actual cost |

Rows are ordered by absolute deviation, largest first, then by URN. NDJSON
prints one anomaly per line and nothing when the list is empty. The table
prints a `N of M resources` line and never a total, because rows can be in
different currencies.

## What is not an anomaly

The overview already leaves drift unset for these, and so does this command:

- Drift at or under 10%.
- Less than two days of data in the window.
- A new resource with no actual spend yet.
- A deleted resource with no projection.

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
