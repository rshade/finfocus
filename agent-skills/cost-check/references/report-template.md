<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# COST_CHECK.md template

````markdown
# Cost Check Report

Generated: YYYY-MM-DD
Input: [Pulumi plan / Pulumi auto-detect / Terraform state]
Tool: finfocus vX.Y.Z (required vX.Y.Z)
Plugins: [name vX.Y.Z, ...]

## Summary

| Metric | Value |
| --- | --- |
| Total monthly cost | X,XXX.XX CUR |
| Resources priced | N |
| Resources unpriced | N |
| Flagged items | N |
| Plan change | +/- XX.XX CUR (X%) |

Result: PASS | FLAGGED | ERROR (kind)

## By Resource Type

| Resource type | Count | Monthly cost |
| --- | --- | --- |
| aws:ec2/instance:Instance | N | XXX.XX |

## Flagged Items

### Expensive Resources

| Resource | Type | Monthly cost | Threshold |
| --- | --- | --- | --- |

### Cost Increase

| Metric | Before | After | Change |
| --- | --- | --- | --- |

### Total Cost

[Only when the total is above its threshold.]

## Unpriced Resources

| Resource | Type | Code | Note |
| --- | --- | --- | --- |

Every total above is a lower bound while this table is not empty.

## Thresholds Used

| Key | Value |
| --- | --- |

Source: .cost-check.yml | defaults

## Caveats

- [Terraform state only: pending changes are not priced.]
- [Version unverified.]
- [Other limits the CLI reported.]
````

## Quality checklist

- Every flag names the resource, its type, and the threshold it broke.
- Amounts carry two decimals and the currency code.
- The thresholds section shows the values actually applied.
- The tool version comes from `finfocus --version`.
- The unpriced table is present whenever any resource has an `error`.
