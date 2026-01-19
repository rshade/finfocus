# Data Model: Cost Commands TUI Upgrade

**Feature Branch**: `106-cost-tui-upgrade`
**Date**: 2025-12-25

## Overview

This document defines the data structures for the cost commands TUI upgrade.
The model extends existing engine types with a new Delta field and introduces
Bubble Tea model structures for interactive display.

## Entity Definitions

### 1. CostResult (Modified)

**Location**: `internal/engine/types.go`

Existing structure with new optional Delta field for cost change indication.

```go
type CostResult struct {
    // Existing fields (unchanged)
    ResourceType   string                          `json:"resourceType"`
    ResourceID     string                          `json:"resourceId"`
    Adapter        string                          `json:"adapter"`
    Currency       string                          `json:"currency"`
    Monthly        float64                         `json:"monthly"`
    Hourly         float64                         `json:"hourly"`
    Notes          string                          `json:"notes"`
    Breakdown      map[string]float64              `json:"breakdown"`
    Sustainability map[string]SustainabilityMetric `json:"sustainability,omitempty"`
    TotalCost      float64                         `json:"totalCost,omitempty"`
    DailyCosts     []float64                       `json:"dailyCosts,omitempty"`
    CostPeriod     string                          `json:"costPeriod,omitempty"`
    StartDate      time.Time                       `json:"startDate,omitempty"`
    EndDate        time.Time                       `json:"endDate,omitempty"`

    // NEW: Cost delta for trend indication
    Delta float64 `json:"delta,omitempty"`
}
```

**Validation Rules**:

- Delta can be positive (cost increase), negative (savings), or zero
- Delta is optional - zero value means no delta available
- Currency applies to both Monthly and Delta values

### 2. CostViewModel

**Location**: `internal/tui/cost_model.go`

Bubble Tea model for interactive cost display.

```go
type CostViewModel struct {
    // View state
    state      ViewState
    results    []engine.CostResult
    aggregated *engine.AggregatedResults

    // Interactive components
    table     table.Model
    spinner   spinner.Model
    selected  int

    // Display configuration
    width     int
    height    int
    sortBy    SortField
    sortAsc   bool
    filter    string
    showHelp  bool

    // Loading state
    loading   *LoadingState

    // Error handling
    err       error
}
```

**State Transitions**:

| From State | Event | To State |
|------------|-------|----------|
| Loading | AllPluginsDone | List |
| Loading | Timeout | List (with errors) |
| List | Enter | Detail |
| List | q | Quit |
| Detail | Escape | List |
| Detail | q | Quit |

### 3. ViewState

**Location**: `internal/tui/cost_model.go`

Enum for current view state in the TUI.

```go
type ViewState int

const (
    ViewStateLoading ViewState = iota
    ViewStateList
    ViewStateDetail
    ViewStateQuitting
)
```

### 4. LoadingState

**Location**: `internal/tui/cost_loading.go`

Tracks plugin query progress during loading phase.

```go
type LoadingState struct {
    spinner   spinner.Model
    plugins   map[string]*PluginStatus
    startTime time.Time
    message   string
}

type PluginStatus struct {
    Name      string
    Done      bool
    Count     int      // Resource count when done
    Error     error    // Error if failed
    StartTime time.Time
}
```

**Validation Rules**:

- Plugin names must be unique
- Count >= 0 when Done == true
- Error set only when Done == true and failed

### 5. ResourceRow

**Location**: `internal/tui/cost_view.go`

Row representation for the interactive table.

```go
type ResourceRow struct {
    ResourceName string  // Truncated to 40 chars
    ResourceType string  // e.g., "aws:ec2:Instance"
    Provider     string  // e.g., "aws"
    Monthly      float64
    Delta        float64
    Currency     string
    HasError     bool
    ErrorMsg     string
}
```

**Derived From**: `engine.CostResult`

**Transformation**:

```go
func NewResourceRow(result engine.CostResult) ResourceRow {
    name := fmt.Sprintf("%s/%s", result.ResourceType, result.ResourceID)
    if len(name) > 40 {
        name = name[:37] + "..."
    }
    provider := extractProvider(result.ResourceType) // "aws:ec2:Instance" → "aws"
    return ResourceRow{
        ResourceName: name,
        ResourceType: result.ResourceType,
        Provider:     provider,
        Monthly:      result.Monthly,
        Delta:        result.Delta,
        Currency:     result.Currency,
        HasError:     strings.HasPrefix(result.Notes, "ERROR:"),
        ErrorMsg:     result.Notes,
    }
}
```

### 6. SortField

**Location**: `internal/tui/cost_model.go`

Enum for table sort options.

```go
type SortField int

const (
    SortByCost SortField = iota
    SortByName
    SortByType
    SortByDelta
)
```

### 7. CostSummary

**Location**: Existing `internal/engine/types.go` (AggregatedResults.Summary)

Already defined, used directly for styled summary rendering.

```go
type CostSummary struct {
    TotalMonthly float64
    TotalHourly  float64
    Currency     string
    ByProvider   map[string]float64
    ByService    map[string]float64
    ByAdapter    map[string]float64
}
```

## Relationships

```text
┌────────────────────┐
│   CostViewModel    │
│   (Bubble Tea)     │
├────────────────────┤
│ - state            │
│ - results[]        │◀──────────┐
│ - aggregated       │           │
│ - table            │           │
│ - loading          │           │
└────────────────────┘           │
         │                       │
         │ contains              │ derives from
         ▼                       │
┌────────────────────┐   ┌───────┴────────┐
│   LoadingState     │   │  CostResult    │
├────────────────────┤   │  (engine)      │
│ - spinner          │   ├────────────────┤
│ - plugins{}        │   │ + Delta        │ ◀── NEW FIELD
│ - message          │   └────────────────┘
└────────────────────┘
         │
         │ tracks
         ▼
┌────────────────────┐
│   PluginStatus     │
├────────────────────┤
│ - Name             │
│ - Done             │
│ - Count            │
│ - Error            │
└────────────────────┘
```

## Message Types (Bubble Tea)

```go
// Spinner tick message
type tickMsg time.Time

// Plugin completed message
type pluginDoneMsg struct {
    Name  string
    Count int
    Error error
}

// All loading complete message
type loadingCompleteMsg struct {
    Results []engine.CostResult
    Errors  []engine.ErrorDetail
}

// Window size changed
type windowSizeMsg tea.WindowSizeMsg

// Quit message
type quitMsg struct{}
```

## JSON Compatibility

The Delta field addition maintains full backward compatibility:

**Before (existing output)**:

```json
{
  "resourceType": "aws:ec2:Instance",
  "resourceId": "i-1234567890abcdef0",
  "monthly": 150.00,
  "currency": "USD"
}
```

**After (with delta)**:

```json
{
  "resourceType": "aws:ec2:Instance",
  "resourceId": "i-1234567890abcdef0",
  "monthly": 150.00,
  "currency": "USD",
  "delta": 25.50
}
```

**Without delta (omitempty)**:

```json
{
  "resourceType": "aws:ec2:Instance",
  "resourceId": "i-1234567890abcdef0",
  "monthly": 150.00,
  "currency": "USD"
}
```
