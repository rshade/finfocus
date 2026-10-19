<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# COST_DRIFT.md template

````markdown
# Cost Drift Report

Generated: YYYY-MM-DD
IaC: Pulumi
Tool: finfocus vX.Y.Z (required vX.Y.Z)
Stack: [metadata.stackName]
Period: [metadata.timeWindow.start] to [metadata.timeWindow.end]

## Summary

| Metric | Value |
| --- | --- |
| Total actual (window) | X,XXX.XX CUR |
| Total projected (month) | X,XXX.XX CUR |
| Overall drift (derived) | +/- X.X% |
| Drifting resources | N |
| Resources with actuals | N |
| Trend | better / worse / stable / none |

Result: PASS | DRIFTING | ERROR (kind)

## Drifting Resources

### [Resource name] ([resource type])

| Metric | Value |
| --- | --- |
| Actual (extrapolated month) | XXX.XX CUR |
| Projected | XXX.XX CUR |
| Delta | +/- XX.XX CUR |
| Drift | +/- X.X% |

**Likely cause:** [from drift-explanations.md]

**Investigate:** [next step]

[Order by absolute drift percentage.]

## Other Resources

| Resource | Type | Actual (window) | Projected | Drift |
| --- | --- | --- | --- | --- |

Drift reads `within 10%`, `new or deleted`, or `insufficient data`.

## Errors

[Every `errors[]` entry, or "none".]

## Thresholds Used

| Key | Value |
| --- | --- |

Source: .cost-check.yml drift section | defaults

## Trend

| Metric | Previous | Current | Direction |
| --- | --- | --- | --- |

[Omit when there is no previous COST_DRIFT.md.]

## Caveats

- Actuals can lag the billing source by 24 to 48 hours.
- Overall drift is derived and approximate.
- [Version unverified, a drift_pct below 10, partial data.]
````

## Quality checklist

- Every flagged resource shows actual, projected, delta, and drift.
- Explanations match the resource type and the direction of the drift.
- Amounts carry two decimals and the currency code.
- The thresholds section shows the values actually applied.
- The tool version comes from `finfocus --version`.
