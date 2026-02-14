# Engine API Reference

## Table of Contents

- [Core Types](#core-types)
- [Cost Calculation Methods](#cost-calculation-methods)
- [Aggregation](#aggregation)
- [Budget System](#budget-system)
- [Error Types](#error-types)
- [Constants](#constants)
- [Data Flow](#data-flow)

## Core Types

### ResourceDescriptor

```go
type ResourceDescriptor struct {
    Type       string                 // "aws:ec2:Instance"
    ID         string                 // Resource URN or ID
    Provider   string                 // "aws", "azure", "gcp"
    Properties map[string]interface{} // Resource properties
}
```

### CostResult

```go
type CostResult struct {
    ResourceType string             // "aws:ec2:Instance"
    ResourceID   string             // Resource URN
    Adapter      string             // Plugin name or "local-spec"/"none"
    Currency     string             // "USD"
    Monthly      float64            // Monthly cost
    Hourly       float64            // Hourly cost
    TotalCost    float64            // Actual historical cost for period
    Notes        string             // Details or "ERROR:"/"VALIDATION:" prefix
    Breakdown    map[string]float64 // Cost breakdown by component
    DailyCosts   []float64          // Daily cost trend
    CostPeriod   string             // "1 day", "2 weeks", "1 month"
    StartDate    time.Time
    EndDate      time.Time
}
```

### GroupBy

```go
type GroupBy string

// Resource-based
GroupByResource GroupBy = "resource"
GroupByType     GroupBy = "type"
GroupByProvider GroupBy = "provider"

// Time-based (for cross-provider aggregation)
GroupByDaily    GroupBy = "daily"
GroupByMonthly  GroupBy = "monthly"
```

Methods: `IsValid()`, `IsTimeBasedGrouping()`, `String()`.

## Cost Calculation Methods

### Projected Costs

```go
// Basic - returns results only
func (e *Engine) GetProjectedCost(ctx, resources) ([]CostResult, error)

// With error tracking
func (e *Engine) GetProjectedCostWithErrors(ctx, resources) (*CostResultWithErrors, error)
```

Pipeline: plugins first -> spec fallback -> placeholder result.

### Actual Costs

```go
// Basic with time range
func (e *Engine) GetActualCost(ctx, resources, from, to) ([]CostResult, error)

// Advanced with options
func (e *Engine) GetActualCostWithOptions(ctx, ActualCostRequest) ([]CostResult, error)

// With error tracking
func (e *Engine) GetActualCostWithOptionsAndErrors(ctx, request) (*CostResultWithErrors, error)
```

```go
type ActualCostRequest struct {
    Resources []ResourceDescriptor
    From      time.Time
    To        time.Time
    Adapter   string
    GroupBy   string
    Tags      map[string]string  // Tag-based filtering
}
```

## Aggregation

### Result Aggregation

```go
func AggregateResults(results []CostResult) *AggregatedResults

type CostSummary struct {
    TotalMonthly float64
    TotalHourly  float64
    Currency     string
    ByProvider   map[string]float64
    ByService    map[string]float64
    ByAdapter    map[string]float64
}
```

### Cross-Provider Aggregation

```go
func CreateCrossProviderAggregation(results []CostResult, groupBy GroupBy) ([]CrossProviderAggregation, error)

type CrossProviderAggregation struct {
    Period    string             // "2024-01-15" or "2024-01"
    Providers map[string]float64 // "aws" -> 100.0
    Total     float64
    Currency  string
}
```

Validates: non-empty results, time-based grouping, consistent currency, valid date ranges.

## Budget System

```go
func (e *Engine) GetBudgets(ctx, BudgetFilterOptions) ([]*pbc.Budget, error)

type BudgetFilterOptions struct {
    Providers []string          // OR logic, case-insensitive
    Tags      map[string]string // AND logic, case-sensitive, glob patterns
}
```

Health statuses: OK (<80%), WARNING (80-89%), CRITICAL (90-100%), EXCEEDED (>100%).

Forecasting: linear extrapolation from current daily rate.

## Error Types

| Error | Cause |
|-------|-------|
| `ErrNoCostData` | No data available for resource |
| `ErrMixedCurrencies` | Different currencies in aggregation |
| `ErrInvalidGroupBy` | Non-time grouping for cross-provider |
| `ErrEmptyResults` | Empty results for aggregation |
| `ErrInvalidDateRange` | End date before start date |

## Constants

```go
hoursPerDay       = 24
hoursPerMonth     = 730    // Standard business month
monthlyConversion = 30.44  // Average days per month
defaultEstimate   = 100.0  // USD fallback

OutputTable  = "table"
OutputJSON   = "json"
OutputNDJSON = "ndjson"
```

## Data Flow

```text
Resources -> Engine
  -> Plugin Clients (gRPC)  -> CostResult[]
  -> Spec Loader (YAML)     -> CostResult[] (fallback)
  -> RenderResults(format)  -> Output
```

Plugin failures: logged at WARN, processing continues.
Notes prefix: `"ERROR:"` for plugin failures, `"VALIDATION:"` for pre-flight failures.
