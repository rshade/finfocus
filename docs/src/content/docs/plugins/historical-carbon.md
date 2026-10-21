---
title: "Historical AWS Carbon Reporting"
description: "Query provider-reported monthly AWS emissions with full provenance"
layout: "docs"
---

## Enable the report

Install an `aws-ce` version advertising `historical_carbon`. Historical reporting
is disabled by default. Enable it in `~/.finfocus/config.hujson` (or the project
configuration), then restart the plugin:

```json
{
  "plugins": {
    "aws-ce": {
      "carbon_enabled": true
    }
  }
}
```

Alternatively, export `FINFOCUS_AWS_CARBON_ENABLED=true`. The plugin advertises
this setting so FinFocus delivers configuration through its existing settings
mechanism. Standard AWS credentials and Sustainability read permissions are
required. Enabling the feature performs no AWS request by itself; ordinary
monetary cost requests do not request Sustainability data.

## Query complete months

```sh
finfocus carbon actual --start 2026-01-01 --end 2026-03-01 \
  --method market-based --scope total --output json
```

`--start` is inclusive and `--end` is exclusive. Both are required first-of-month
UTC dates in `YYYY-MM-DD` format. `--method` accepts `market-based` (default) or
`location-based`. `--scope` accepts `total` (default), `1`, `2`, or `3`.
`--output` accepts `table` (default) or `json`.

Filter using `--account`, `--region`, and `--service`. Each accepts comma-separated
values or repeated flags, for example `--account 111111111111,222222222222`
and `--service AmazonEC2 --service AmazonS3`. Empty filter values are invalid.
These are AWS aggregate dimensions; no resource matching or allocation occurs.
Only the `aws-ce` report provider is queried.

## Interpret available records

Every record retains provider, source, account, region, service, original month
bounds, accounting method, scope, model version, retrieval time, and emissions
in `gCO2e`. JSON timestamps use RFC3339 and enums use protocol names. Explicit
zero emissions remain explicit numeric zero values.

The report includes its resolved request even when no records are available.
An empty JSON report has `records: []` and `totalGrams: null`. A returned zero
record has `value: 0` and contributes to a numeric total. The total sums only
returned, available records; it does not claim a complete physical inventory.
AWS publication lag or missing data must not be interpreted as zero emissions.
Model revisions are not additive identities.

FinFocus validates all pages before writing output. It rejects duplicate record
identities, changed query dimensions, mixed methods or scopes, repeated page
tokens, invalid or non-finite values, and reports exceeding 100 pages. Provider
failures return an error without a partial report on stdout.

## Agent tool access

The existing MCP command surface exposes the read-only `finfocus-carbon-actual`
tool with the same flags. Its schema marks `start` and `end` as required and
declares the method, scope, and output choices. Listing tools or reading their
schemas does not launch the report provider or request AWS credentials. A tool
invocation still requires the plugin opt-in and performs the same validated
report query as the CLI.

## Web UI Fit

The authenticated `POST /api/carbon/actual/query` hook invokes the same report
operation as the CLI. It accepts a JSON body with `start`, `end`, `method`,
`scope`, and optional `accounts`, `regions`, and `services` arrays:

```json
{
  "start": "2026-01-01",
  "end": "2026-03-01",
  "method": "market-based",
  "scope": "total",
  "accounts": ["111111111111"],
  "services": ["AmazonEC2"]
}
```

The endpoint requires the existing active web session and session authentication.
The response has the same JSON request, records, and optional total as CLI JSON.
No separate browser calculation is used. A browser query form and provenance
table are tracked as a local UX-only follow-up; this change adds the domain hook.
