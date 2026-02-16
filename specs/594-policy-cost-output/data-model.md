# Data Model: Policy-Compatible Cost Output

**Feature Branch**: `594-policy-cost-output`
**Date**: 2026-02-16

## Entities

### CostSummary

Produced after each `AnalyzeStack()` call and written to disk as JSON.

| Field | Type | Required | Description |
| ----- | ---- | -------- | ----------- |
| schema_version | string | yes | Schema version for forward compatibility, starting at "1" |
| timestamp | string (RFC 3339) | yes | Time the summary was generated |
| stack | string | yes | Pulumi stack name (from ConfigureStack RPC) |
| project | string | yes | Pulumi project name (from ConfigureStack RPC) |
| total_monthly_cost | float64 | yes | Sum of all per-resource monthly costs |
| currency | string | yes | ISO 4217 currency code (e.g., "USD") |
| resource_count | int | yes | Number of resources with cost data |
| mixed_currencies | bool | no | True if multiple currencies detected across resources |
| resources | []ResourceCost | yes | Per-resource cost breakdown (may be empty) |

**Identity**: One file per project, overwritten on each run. File path: `$PROJECT/.finfocus/last-cost-summary.json` or `~/.finfocus/last-cost-summary.json`.

**Lifecycle**: Created on first AnalyzeStack, overwritten on subsequent runs, never rotated or deleted by the system.

### ResourceCost

Per-resource cost entry within the CostSummary.

| Field | Type | Required | Description |
| ----- | ---- | -------- | ----------- |
| type | string | yes | Pulumi resource type (e.g., "aws:ec2/instance:Instance") |
| name | string | yes | Resource name/ID extracted from URN |
| monthly_cost | float64 | yes | Estimated monthly cost for this resource |
| currency | string | yes | Currency for this resource's cost |
| adapter | string | yes | Source of pricing data (plugin name or "local-spec") |

### AnalyzerThresholdConfig

Extension to existing `AnalyzerConfig` in `internal/config/config.go`.

| Field | Type | Default | Source | Description |
| ----- | ---- | ------- | ------ | ----------- |
| max_monthly_cost | float64 | 0 (disabled) | config.yaml / env var | Maximum allowed monthly cost |
| enforcement | string | "advisory" | config.yaml / env var | Enforcement mode: "advisory" or "mandatory" |

**Config file path**: `analyzer.max_monthly_cost` and `analyzer.enforcement` in YAML.

**Environment variables**:

- `FINFOCUS_MAX_MONTHLY_COST` — overrides `max_monthly_cost`
- `FINFOCUS_ENFORCEMENT` — overrides `enforcement`

**Precedence**: env var > project-local config > global config > default.

**Validation rules**:

- `max_monthly_cost <= 0` → treated as "no threshold configured", warning logged
- `enforcement` not in {"advisory", "mandatory"} → defaults to "advisory", warning logged

### DiagnosticCostMetadata

Machine-parseable metadata embedded in diagnostic Message field as HTML comment.

| Field | Type | Required | Description |
| ----- | ---- | -------- | ----------- |
| monthly | float64 | yes | Monthly cost estimate |
| currency | string | yes | ISO 4217 currency code |
| adapter | string | yes | Source of pricing data |

**Format**: `<!-- finfocus:cost:{"monthly":150.00,"currency":"USD","adapter":"aws-public"} -->`

**Parsing contract**: Extract content between `<!-- finfocus:cost:` and `-->`, parse as JSON.

### ThresholdDiagnostic

Additional diagnostic emitted by AnalyzeStack when a threshold is configured.

| Field | Value | Description |
| ----- | ----- | ----------- |
| PolicyName | "cost-threshold" | Distinct from "cost-estimate" and "stack-cost-summary" |
| PolicyPackName | "finfocus" | Same as existing policies |
| EnforcementLevel | ADVISORY or MANDATORY | Based on configured enforcement mode |
| Severity | MEDIUM (within budget) or HIGH (exceeded) | Conveys urgency independent of enforcement |
| URN | "" (empty) | Stack-level diagnostic, not resource-specific |
| Message | See below | Includes actual cost and threshold |

**Message formats**:

- Within budget: `"Stack cost $3,000.00/mo is within threshold $5,000.00/mo"`
- Exceeded (advisory): `"Stack cost $7,500.00/mo exceeds threshold $5,000.00/mo"`
- Exceeded (mandatory): `"Stack cost $7,500.00/mo exceeds threshold $5,000.00/mo — deployment blocked"`

## Relationships

```text
AnalyzerConfig ──has──▶ AnalyzerThresholdConfig (max_monthly_cost, enforcement)
     │
     ▼
  Server ──uses──▶ CostCalculator (existing interface)
     │
     ├── Analyze() ──produces──▶ CostToDiagnostic (with DiagnosticCostMetadata)
     │                    └──caches──▶ costCache
     │
     └── AnalyzeStack() ──reads──▶ costCache
              ├──produces──▶ StackSummaryDiagnostic (existing)
              ├──produces──▶ ThresholdDiagnostic (NEW, if threshold configured)
              └──writes──▶ CostSummary (to disk)
```

## State Transitions

### CostSummary File

```text
[not exists] ──first run──▶ [created with schema_version "1"]
[exists]     ──next run───▶ [overwritten atomically]
[write fail] ──graceful──▶ [unchanged, warning logged]
```

### Threshold Evaluation

```text
[no threshold] ──▶ skip (no threshold diagnostic emitted)
[threshold > 0, cost <= threshold] ──▶ ADVISORY "within budget"
[threshold > 0, cost > threshold, advisory mode] ──▶ WARNING "exceeds threshold"
[threshold > 0, cost > threshold, mandatory mode] ──▶ MANDATORY "deployment blocked"
[mixed currencies] ──▶ skip enforcement, warning logged
[all resources failed] ──▶ skip (no threshold diagnostic emitted)
```
