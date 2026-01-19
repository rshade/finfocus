# Data Model: Engine Budget Logic

**Source**: `github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1`

## Entities

### Budget

Represents a spending limit.

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| `Id` | `string` | Unique identifier | Required |
| `Name` | `string` | Human-readable name | Required |
| `Source` | `string` | Provider (e.g., "aws", "gcp") | Required |
| `Amount` | `*BudgetAmount` | Limit and currency | Required |
| `Status` | `*BudgetStatus` | Current spend and health | Optional |
| `Metadata` | `map[string]string` | Provider-specific data (Regions, Tags?) | Optional |

### BudgetAmount

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| `Limit` | `float64` | Max spending | > 0 |
| `Currency` | `string` | ISO 4217 code | 3 chars, uppercase |

### BudgetStatus

| Field | Type | Description |
|-------|------|-------------|
| `CurrentSpend` | `float64` | Actual spend |
| `ForecastedSpend` | `float64` | Predicted spend |
| `Health` | `BudgetHealthStatus` | Enum (OK, WARNING, CRITICAL, EXCEEDED) |

### BudgetFilter

User-provided criteria to filter the list of budgets.

| Field | Type | Logic |
|-------|------|-------|
| `Providers` | `[]string` | OR logic (match any) |
| `Regions` | `[]string` | OR logic (match any) - Checked against Metadata["region"]? |
| `Tags` | `map[string]string` | AND logic (match all) - Checked against Metadata? |

### BudgetSummary

Aggregate health view.

| Field | Type | Description |
|-------|------|-------------|
| `TotalBudgets` | `int32` | Total count (including unspecified health) |
| `BudgetsOk` | `int32` | Count of OK |
| `BudgetsWarning` | `int32` | Count of WARNING |
| `BudgetsCritical` | `int32` | Count of CRITICAL |
| `BudgetsExceeded` | `int32` | Count of EXCEEDED |
