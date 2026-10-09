---
title: Vantage Plugin
description: Read imported actual cloud cost from Vantage Cost Reports.
parent: Plugins
nav_order: 14
---

## Overview

The `vantage` plugin reads imported actual cost from a Vantage Cost Report and
serves it to FinFocus. It is an actual-cost source only: projected cost,
pricing specs, and estimates return `Unimplemented`.

It needs a Vantage service token and the token of the cost report to read.

## Features

- **Actual Costs**: Daily cost rows from Vantage for a resource and date range,
  summed by FinFocus into the resource total.
- **Providers**: `Supports` recognizes `aws`, `azure`, `gcp`, `kubernetes`, and
  `custom`.
- **Per-Request Credentials**: The `x-finfocus-credential-vantage-token` gRPC
  metadata key overrides the environment token for one request.
- **FOCUS Records**: Validated FOCUS records are included when the source
  fields allow it.

## Installation

```bash
finfocus plugin install vantage

# Or from the repository path
finfocus plugin install github.com/rshade/finfocus-plugin-vantage
```

`finfocus setup` does not install this plugin by default.

## Configuration

```bash
export FINFOCUS_VANTAGE_TOKEN="read-only-service-token"
export FINFOCUS_VANTAGE_COST_REPORT_TOKEN="cost-report-token"
```

| Variable                            | Purpose                                              |
| ----------------------------------- | ---------------------------------------------------- |
| `FINFOCUS_VANTAGE_TOKEN`            | Read-only Vantage service token.                     |
| `FINFOCUS_VANTAGE_COST_REPORT_TOKEN`| Cost report to query. Required even with per-request credentials. |
| `FINFOCUS_VANTAGE_BASE_URL`         | Defaults to `https://api.vantage.sh/v2`. HTTPS only. |

Tokens are never logged.

## Usage

```bash
finfocus cost actual --pulumi-state state.json \
  --from 2026-09-01 --to 2026-09-30 --adapter vantage --output json
```

Use exported state that carries real cloud resource ids. A preview may not have
the id needed to find billed cost.

The plugin must know the Vantage service for each resource. It infers EC2, S3,
Lambda, RDS, and DynamoDB from common AWS resource types and ARNs. For any other
type, set a `service` tag with the exact Vantage service name.

## Limitations

- **Actual Cost Only**: No projected cost, pricing spec, or estimate.
- **Billing Lag**: Vantage imports billing data days after usage, so an empty
  result does not prove zero spend.
- **Dates**: Rows are matched on UTC calendar dates in the half-open interval
  `[start, end)`.
- **Currency**: Mixed billing currencies fail with `FailedPrecondition`. There
  is no currency conversion.
- **Rate**: Requests are paced to one per second per plugin instance.
- **Service Tag**: Non-AWS resources need a `service` tag.

See the
[plugin README](https://github.com/rshade/finfocus-plugin-vantage#readme) and
its usage guide for resource selection and mapping rules.
