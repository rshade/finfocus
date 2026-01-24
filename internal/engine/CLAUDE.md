# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Engine Package Overview

The `internal/engine` package is the core orchestration layer for FinFocus, responsible for coordinating cost calculations between plugins and local pricing specifications, then rendering results in multiple output formats.

**Key Capabilities:**
- **Multi-Provider Cost Calculation**: Orchestrates cost queries across AWS, Azure, GCP, and other cloud providers
- **Budget Health Monitoring**: Real-time status tracking, forecasting, and threshold alerting
- **Cross-Provider Aggregation**: Advanced time-based cost aggregation with currency validation and error handling
- **Plugin Architecture**: gRPC-based plugin system with graceful fallback to local YAML specifications
- **Flexible Grouping**: Resource-based and time-based grouping strategies for comprehensive cost analysis
- **Type Safety**: Comprehensive input validation and error handling for reliable cost calculations

## Architecture

### Core Components

1. **Engine** (`engine.go`)
   - Central orchestrator managing plugin clients and spec loaders
   - Handles both projected and actual cost calculations
   - Implements graceful fallback from plugins to local specs

2. **Types System** (`types.go`)
   - `ResourceDescriptor`: Represents cloud resources with type, provider, and properties
   - `CostResult`: Standardized cost output with breakdown, currency, and metadata
   - `CostResultWithErrors`: Results with aggregated error tracking
   - `ErrorDetail`: Detailed error information per resource/plugin failure
   - `PricingSpec`: YAML-based local pricing fallback structure
   - `CrossProviderAggregation`: Time-based multi-provider cost aggregation structure
   - `GroupBy`: Type-safe grouping strategies with validation methods
   - `ActualCostRequest`: Enhanced request structure for complex actual cost queries

3. **Output Rendering** (`project.go`)
   - Multi-format output: table, JSON, NDJSON
   - Enhanced table format with cost summaries and breakdowns
   - Aggregated JSON with totals by provider, service, and adapter
   - Streaming NDJSON for large result sets

4. **Budget Health Components** (`budget*.go`)
   - **Budget Engine** (`budget.go`): Orchestrates retrieval, filtering, and processing
   - **Health** (`budget_health.go`): Calculates status (OK/WARNING/CRITICAL) based on utilization
   - **Forecasting** (`budget_forecast.go`): Predicts end-of-period spend via linear extrapolation
   - **Thresholds** (`budget_threshold.go`): Evaluates alerts for actual and forecasted spend
   - **Summary** (`budget_summary.go`): Aggregates health statistics by provider and currency

### Data Flow Architecture

```text
Resources → Engine.GetProjectedCost/GetActualCost → Plugin Clients (gRPC) → Results
                                                 ↓ (fallback)
                                              Spec Loader (YAML) → Results
                                                 ↓
                                              RenderResults → Output
```

## Testing Commands

```bash
# Run all engine tests (currently no test files exist)
go test ./internal/engine/...

# Test engine integration via CLI tests
go test ./internal/cli/... -run TestCLIIntegration

# Run engine functionality through example commands
./bin/finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json
./bin/finfocus cost actual --pulumi-json examples/plans/aws-simple-plan.json --from 2025-01-01

# Test cross-provider aggregation features
./bin/finfocus cost actual --group-by daily --from 2024-01-01 --to 2024-01-31
./bin/finfocus cost actual --group-by monthly --from 2024-01-01
```

## Cost Calculation Strategy

### Projected Cost Flow

1. **Plugin First**: Query each plugin client via `GetProjectedCost` gRPC call
2. **Smart Spec Fallback**: Multi-level fallback pattern:
   - Try `provider-service-sku` pattern (e.g., `aws-ec2-t3.micro`)
   - Fallback to `provider-service-default` pattern (e.g., `aws-ec2-default`)
   - Try common SKUs: `standard`, `basic`, `default`
3. **Intelligent Cost Calculation**: Extract costs from spec pricing data:
   - Direct monthly estimates (`monthlyEstimate`)
   - Hourly rates (`onDemandHourly`, `hourlyRate`)
   - Storage calculations with size multipliers (`pricePerGBMonth`)
   - Fallback estimates based on resource type
4. **Multi-Plugin Support**: Collect results from all responding plugins

### Actual Cost Flow

1. **Plugin Only**: Query plugins via `GetActualCost` gRPC call with time range
2. **First Success**: Use first plugin that successfully returns data
3. **Time Calculations**: Convert historical data to monthly/hourly rates
4. **Advanced Features**: Support for filtering, tagging, and grouping
5. **Cross-Provider Aggregation**: Time-based cost aggregation across multiple providers
6. **No Fallback**: Actual costs require live plugin data

### Cross-Provider Aggregation Pipeline

**New in v0.2.0**: Comprehensive cross-provider aggregation system supporting:

1. **Input Validation**:
   - Empty results detection (`ErrEmptyResults`)
   - Time-based grouping validation (`ErrInvalidGroupBy`)
   - Currency consistency checks (`ErrMixedCurrencies`)
   - Date range validation (`ErrInvalidDateRange`)

2. **Cost Processing**:
   - Intelligent cost calculation (TotalCost vs Monthly with daily conversion)
   - Provider extraction from resource types ("aws:ec2:Instance" → "aws")
   - Period formatting (daily: "2006-01-02", monthly: "2006-01")

3. **Aggregation Output**:
   - Sorted time-period aggregations
   - Per-provider cost breakdowns
   - Total costs with consistent currency
   - Chronological ordering for trend analysis

### Cost Calculation Constants

```go
hoursPerDay = 24
hoursPerMonth = 730          // Standard business month
defaultEstimate = 100.0      // USD fallback estimate
monthlyConversion = 30.44    // Average days per month
```

## Output Format Handling

### Table Format

- Enhanced layout with cost summary sections
- Summary: Total monthly/hourly costs and resource count
- Breakdowns: By provider, service, and adapter
- Resource details: Individual resource costs and notes
- Truncates long resource names (>40 chars) with "..."

### JSON Format

- Aggregated results with summary and resource details
- Includes totals by provider, service, and adapter
- Full cost breakdown and analysis
- Pretty-printed JSON with 2-space indentation

### NDJSON Format

- Newline-delimited JSON for streaming
- One CostResult per line
- No array wrapper - suitable for large datasets

## Error Handling Patterns

### Plugin Communication

- **Continue on Error**: Plugin failures don't stop processing
- **Best Effort**: Collect all available results
- **Graceful Degradation**: Fall back to specs, then to "none" adapter

### Resource Processing

- **Per-Resource Isolation**: One resource failure doesn't affect others
- **Multi-Plugin Tolerance**: Continue if some plugins fail
- **Default Results**: Always return some result, even if placeholder

### Error Aggregation System

**New in v0.3.0**: Comprehensive error tracking with `CostResultWithErrors`:

```go
type CostResultWithErrors struct {
    Results []CostResult   // Successfully calculated costs
    Errors  []ErrorDetail  // Aggregated errors from all failures
}

type ErrorDetail struct {
    ResourceType string    // Resource type that failed
    ResourceID   string    // Resource ID that failed
    PluginName   string    // Plugin that returned the error
    Error        error     // The actual error
    Timestamp    time.Time // When the error occurred
}
```

**Error-Tracking Methods**:

```go
// Projected costs with error aggregation
func (e *Engine) GetProjectedCostWithErrors(ctx context.Context, resources []ResourceDescriptor) (*CostResultWithErrors, error)

// Actual costs with error aggregation
func (e *Engine) GetActualCostWithOptionsAndErrors(ctx context.Context, request ActualCostRequest) (*CostResultWithErrors, error)
```

**Key Features**:
- Continues processing on plugin failures (no early termination)
- Aggregates all errors for post-processing analysis
- Inline error display in Notes field with "ERROR:" prefix
- Summary methods: `HasErrors()`, `ErrorSummary()`
- Structured logging via zerolog for debugging

**Logging Integration**:

All plugin failures are logged with structured fields:

```go
config.Logger.Warn().
    Str("resource_type", resource.Type).
    Str("resource_id", resource.ID).
    Str("plugin", client.Name).
    Err(err).
    Msg("plugin call failed for projected cost")
```

### Cross-Provider Aggregation Errors

**New Error Types** (introduced in v0.2.0):

```go
var (
    ErrNoCostData       = errors.New("no cost data available")
    ErrMixedCurrencies  = errors.New("mixed currencies not supported in cross-provider aggregation")
    ErrInvalidGroupBy   = errors.New("invalid groupBy type for cross-provider aggregation")
    ErrEmptyResults     = errors.New("empty results provided for aggregation")
    ErrInvalidDateRange = errors.New("invalid date range: end date must be after start date")
)
```

**Error Handling Strategy**:
- **Early Validation**: Input validation before expensive processing
- **Specific Errors**: Detailed error messages for debugging and user feedback
- **Fail-Fast**: Return immediately on validation errors to prevent inconsistent state
- **Error Context**: Include relevant details (currency pairs, date ranges) in error messages

## Integration Points

### Plugin System

- Consumes `pluginhost.Client` instances with gRPC API
- Converts resource properties to proto format (`map[string]string`)
- Handles plugin failures transparently

### Spec System  

- `SpecLoader` interface for YAML pricing specs
- Type assertion for `*PricingSpec` objects
- Provider/service/SKU based lookups

### Proto Integration

- Uses real protobuf definitions from finfocus-spec
- `GetProjectedCostRequest`/`GetActualCostRequest` messages
- Automatic property conversion from `interface{}` to `string`

## Key Implementation Details

### Property Conversion

```go
// Converts arbitrary properties to string map for gRPC
func convertToProto(properties map[string]interface{}) map[string]string
```

### Time Range Processing

```go
// Converts actual costs to monthly rate using average month length
Monthly: result.TotalCost * 30.44 / float64(to.Sub(from).Hours()/hoursPerDay)
```

### Service Extraction

```go
// Extracts service from resource type: "aws:ec2:Instance" → "ec2"
func extractService(resourceType string) string
```

### Cross-Provider Processing

**Currency Validation**:

```go
// Ensures all results use consistent currency (defaults empty to USD)
func validateCurrencyConsistency(results []CostResult) error
```

**Time-Based Grouping**:

```go
// Groups results by time periods with provider-level aggregation
func groupResultsByPeriod(results []CostResult, groupBy GroupBy) (map[string]map[string]float64, string)
```

**Cost Calculation Logic**:

```go
// Intelligently selects TotalCost (actual) vs Monthly (projected) with period conversion
func calculateCostForPeriod(result CostResult, groupBy GroupBy) float64
```

## Cost Result Structure

```go
type CostResult struct {
    ResourceType string             // Cloud resource type (e.g., "aws:ec2:Instance")
    ResourceID   string             // Unique resource identifier/URN
    Adapter      string             // Plugin name or "local-spec"/"none"
    Currency     string             // ISO currency code (typically "USD")
    Monthly      float64            // Projected/actual monthly cost
    Hourly       float64            // Projected/actual hourly cost
    Notes        string             // Human-readable cost details
    Breakdown    map[string]float64 // Detailed cost breakdown by component
    // Enhanced fields for actual cost support
    TotalCost    float64            // Actual historical cost for the queried period
    DailyCosts   []float64          // Daily cost breakdown (for trend analysis)
    CostPeriod   string             // Human-readable period ("1 day", "2 weeks", "1 month")
    StartDate    time.Time          // Period start date
    EndDate      time.Time          // Period end date
}
```

## Cross-Provider Aggregation Structure

```go
type CrossProviderAggregation struct {
    Period    string             // Time period ("2006-01-02" or "2006-01")
    Providers map[string]float64 // Provider name → aggregated cost
    Total     float64            // Sum of all provider costs
    Currency  string             // Consistent currency (validated)
}
```

## GroupBy Type System

```go
type GroupBy string

// Resource-based grouping
const (
    GroupByResource GroupBy = "resource"  // Individual resources
    GroupByType     GroupBy = "type"      // Resource types
    GroupByProvider GroupBy = "provider"  // Cloud providers
)

// Time-based grouping (for cross-provider aggregation)
const (
    GroupByDaily    GroupBy = "daily"     // Daily aggregation
    GroupByMonthly  GroupBy = "monthly"   // Monthly aggregation
)

// Validation methods
func (g GroupBy) IsValid() bool                // Validates GroupBy value
func (g GroupBy) IsTimeBasedGrouping() bool   // Checks if time-based
func (g GroupBy) String() string              // String representation
```

## Common Usage Patterns

### Engine Initialization

```go
clients := []*pluginhost.Client{...}  // From registry
loader := spec.NewLoader(specDir)     // From config
engine := engine.New(clients, loader)
```

### Multi-Format Output

```go
results, err := engine.GetProjectedCost(ctx, resources)
engine.RenderResults(engine.OutputJSON, results)    // JSON
engine.RenderResults(engine.OutputTable, results)   // Table  
engine.RenderResults(engine.OutputNDJSON, results)  // NDJSON
```

## New Features (Issue #5 Implementation)

### Enhanced Spec Fallback System

The engine now implements intelligent spec loading with multiple fallback patterns:

```go
// 1. Try exact SKU match: aws-ec2-t3.micro.yaml
// 2. Try service default: aws-ec2-default.yaml  
// 3. Try common patterns: aws-ec2-standard.yaml, aws-ec2-basic.yaml
func (e *Engine) getProjectedCostFromSpec(resource ResourceDescriptor) *CostResult
```

### SKU and Service Extraction

Smart extraction of resource characteristics for spec lookup:

```go
func extractService(resourceType string) string    // "aws:ec2:Instance" → "ec2"
func extractSKU(resource ResourceDescriptor) string // Properties or type → "t3.micro"
```

### Cost Calculation from Specs

Flexible cost calculation supporting multiple pricing models:

```go
func calculateCostsFromSpec(spec *PricingSpec, resource ResourceDescriptor) (monthly, hourly float64)
```

Supports:
- Direct monthly estimates (`monthlyEstimate: 7.59`)
- Hourly rates (`onDemandHourly: 0.0104`)
- Storage pricing with size multipliers (`pricePerGBMonth: 0.10`)
- Intelligent fallbacks by resource type (compute: $20, database: $50, storage: $5)

### Cost Aggregation and Analysis

New aggregation system for comprehensive cost analysis:

```go
type CostSummary struct {
    TotalMonthly float64                   // Total monthly cost across all resources
    TotalHourly  float64                   // Total hourly cost across all resources
    Currency     string                    // Currency for all costs
    ByProvider   map[string]float64        // Costs grouped by cloud provider
    ByService    map[string]float64        // Costs grouped by service (ec2, rds, etc.)
    ByAdapter    map[string]float64        // Costs grouped by data source
}

func AggregateResults(results []CostResult) *AggregatedResults
```

### Resource Filtering

Flexible filtering system supporting multiple criteria:

```go
func FilterResources(resources []ResourceDescriptor, filter string) []ResourceDescriptor
```

Filter patterns:
- `provider=aws` - Filter by cloud provider
- `type=ec2` - Filter by service type
- `service=rds` - Filter by extracted service
- `instanceType=t3.micro` - Filter by any resource property
- `id=i-123` - Filter by resource ID

### Enhanced Output Formatting

Improved table output with comprehensive summaries:

```text
COST SUMMARY
============
Total Monthly Cost:    57.59 USD
Total Hourly Cost:     0.0788 USD
Total Resources:       3

BY PROVIDER
-----------
aws:        57.59 USD

BY SERVICE  
----------
ec2:        7.59 USD
rds:        50.00 USD

BY ADAPTER
----------
local-spec: 57.59 USD

RESOURCE DETAILS
================
Resource                     Adapter     Monthly  Hourly   Currency  Notes
--------                     -------     -------  ------   --------  -----
aws:ec2:Instance/i-123       local-spec  7.59     0.0104   USD       Calculated from local spec
aws:rds:Instance/db-456      local-spec  50.00    0.0685   USD       Calculated from local spec
```

## New Features (Cross-Provider Aggregation)

### Enhanced Actual Cost Pipeline

**Advanced Querying** (`GetActualCostWithOptions`):

```go
type ActualCostRequest struct {
    Resources []ResourceDescriptor  // Resources to query
    From      time.Time            // Start date
    To        time.Time            // End date
    Adapter   string               // Specific plugin to use
    GroupBy   string               // Grouping strategy
    Tags      map[string]string    // Tag-based filtering
}
```

**Features**:

- **Tag Filtering**: `tag:env=prod`, `tag:team=backend`
- **Adapter Selection**: Use specific plugins for cost queries
- **Flexible Grouping**: Resource, type, provider, or time-based
- **Date Range Validation**: Comprehensive date validation with helpful errors

### Cross-Provider Cost Analysis

**Core Function**:

```go
func CreateCrossProviderAggregation(results []CostResult, groupBy GroupBy) ([]CrossProviderAggregation, error)
```

**Usage Examples**:

```go
// Daily cross-provider analysis
results := []CostResult{
    {ResourceType: "aws:ec2:Instance", TotalCost: 100.0, Currency: "USD", StartDate: jan1},
    {ResourceType: "azure:compute:VM", TotalCost: 150.0, Currency: "USD", StartDate: jan1},
}
aggregations, err := CreateCrossProviderAggregation(results, GroupByDaily)
// Result: [{Period: "2024-01-01", Providers: {"aws": 100.0, "azure": 150.0}, Total: 250.0}]

// Monthly trend analysis
aggregations, err := CreateCrossProviderAggregation(results, GroupByMonthly)
// Result: [{Period: "2024-01", Providers: {"aws": 3100.0, "azure": 4650.0}, Total: 7750.0}]
```

### CLI Integration Examples

```bash
# Cross-provider daily aggregation
./bin/finfocus cost actual --group-by daily --from 2024-01-01 --to 2024-01-31

# Monthly cost trends across providers
./bin/finfocus cost actual --group-by monthly --from 2024-01-01 --to 2024-12-31

# Filter by tag (no time-based aggregation)
./bin/finfocus cost actual --group-by "tag:env=prod" --from 2024-01-01
```

### Performance and Best Practices

**Scalability**:
- Efficient for datasets up to 10,000 cost results
- Memory usage scales linearly with unique time periods
- Consider streaming for larger datasets

**Currency Handling**:
- All results must use the same currency
- Empty currencies default to USD
- Mixed currencies return `ErrMixedCurrencies`

**Error Recovery**:
- Validate inputs early with specific error messages
- Use time-based grouping only for cross-provider aggregation
- Check date ranges before expensive processing

This package acts as the central coordinating layer, making it critical for understanding the overall cost calculation flow in FinFocus.

## Budget Health System (New in v0.5.4)

### Budget Health Status Logic

**Health Status Calculation**:

Status is determined by `Spend / Limit` percentage:

- **OK**: < 80% utilization
- **WARNING**: 80% - 89% utilization
- **CRITICAL**: 90% - 100% utilization
- **EXCEEDED**: > 100% utilization

```go
// Calculates health status from utilization percentage
func CalculateBudgetHealthFromPercentage(percentage float64) pbc.BudgetHealthStatus
```

**Aggregation Strategy**:

When aggregating multiple budgets, the "worst" status wins:
`EXCEEDED > CRITICAL > WARNING > OK`

```go
// Returns the most severe health status from a list
func AggregateHealth(statuses []pbc.BudgetHealthStatus) pbc.BudgetHealthStatus
```

### Budget Filtering & Orchestration

**Provider Filtering**:

Case-insensitive filtering by cloud provider:

```go
// Filter budgets by provider list (case-insensitive)
func FilterBudgetsByProvider(ctx context.Context, budgets []*pbc.Budget, providers []string) []*pbc.Budget
```

**Orchestration Flow** (`Engine.GetBudgets`):

1. **Plugin Query**: Fetch all budgets from active plugins
2. **Filtering**: Apply provider filters if specified
3. **Forecasting**: Calculate end-of-period forecast via linear extrapolation
4. **Health Calculation**: Determine status based on actual AND forecasted spend
5. **Threshold Evaluation**: Check for triggered alert thresholds
6. **Summary Generation**: Aggregate statistics by status, provider, and currency

### Forecasting Logic

**Linear Extrapolation**:

Predicts end-of-period spend based on current run rate:

```go
// Forecast = CurrentSpend + (DailyRate * RemainingDays)
func CalculateForecastedSpend(current float64, start, end, now time.Time) float64
```

- Handles mid-period calculations
- Returns 0 if period hasn't started
- Returns current spend if period ended

### Threshold Evaluation

**Alert Logic**:

Evaluates both `ACTUAL` and `FORECASTED` thresholds:

```go
// Checks if spend exceeds threshold percentage
func EvaluateThresholds(ctx context.Context, budget *pbc.Budget, currentSpend, forecastedSpend float64) []ThresholdEvaluationResult
```

The function returns `[]ThresholdEvaluationResult` containing evaluation results for each threshold:

```go
type ThresholdEvaluationResult struct {
    Threshold   *pbc.BudgetThreshold // Original threshold
    Triggered   bool                 // Whether threshold was crossed
    TriggeredAt time.Time            // When triggered (zero if not)
    SpendType   string               // "actual" or "forecasted"
}
```

- **Default Thresholds**: 50%, 80%, 100% (if none specified)
- **State Tracking**: Records `TriggeredAt` timestamp for alerts

### Summary Statistics

**Aggregation Dimensions**:

Generates comprehensive summaries for executive reporting:

```go
type ExtendedBudgetSummary struct {
    *pbc.BudgetSummary              // Embedded proto summary
    ByProvider      map[string]*pbc.BudgetSummary // Per-provider breakdown
    ByCurrency      map[string]*pbc.BudgetSummary // Per-currency breakdown
    OverallHealth   pbc.BudgetHealthStatus        // Worst-case health
    CriticalBudgets []string                      // IDs of critical/exceeded budgets
}
```

- **Health Counts**: Number of budgets in each state (OK, WARNING, etc.)
- **Critical List**: Direct access to budgets needing attention
- **Multi-Currency**: Separate aggregations per currency code


