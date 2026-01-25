# Data Model: Budget Threshold Exit Codes

**Feature Branch**: `124-budget-exit-codes`
**Date**: 2026-01-24

## Entity Changes

### BudgetConfig (Extended)

**Location**: `internal/config/budget.go`

**Current Structure**:

```go
type BudgetConfig struct {
    Amount   float64       `yaml:"amount"           json:"amount"`
    Currency string        `yaml:"currency"         json:"currency"`
    Period   string        `yaml:"period,omitempty" json:"period,omitempty"`
    Alerts   []AlertConfig `yaml:"alerts,omitempty" json:"alerts,omitempty"`
}
```

**New Fields**:

| Field             | Type   | YAML Key            | Default | Validation      |
|-------------------|--------|---------------------|---------|-----------------|
| `ExitOnThreshold` | `bool` | `exit_on_threshold` | `false` | None (boolean)  |
| `ExitCode`        | `int`  | `exit_code`         | `1`     | Must be 0-255   |

**Extended Structure**:

```go
type BudgetConfig struct {
    Amount          float64       `yaml:"amount"                      json:"amount"`
    Currency        string        `yaml:"currency"                    json:"currency"`
    Period          string        `yaml:"period,omitempty"            json:"period,omitempty"`
    Alerts          []AlertConfig `yaml:"alerts,omitempty"            json:"alerts,omitempty"`
    ExitOnThreshold bool          `yaml:"exit_on_threshold,omitempty" json:"exit_on_threshold,omitempty"`
    ExitCode        int           `yaml:"exit_code,omitempty"         json:"exit_code,omitempty"`
}
```

**New Methods**:

```go
// GetExitCode returns the configured exit code, defaulting to 1 if not set.
func (b BudgetConfig) GetExitCode() int

// ShouldExitOnThreshold returns true if exit behavior is enabled.
func (b BudgetConfig) ShouldExitOnThreshold() bool
```

**Validation Rules**:

1. `exit_code` must be in range 0-255 (inclusive)
2. Negative exit codes are rejected with `ErrExitCodeOutOfRange`
3. Exit codes > 255 are rejected with `ErrExitCodeOutOfRange`
4. Validation only runs when `exit_on_threshold: true`

### BudgetStatus (Extended)

**Location**: `internal/engine/budget_cli.go`

**Current Structure**:

```go
type BudgetStatus struct {
    Budget             BudgetConfig
    CurrentSpend       float64
    Percentage         float64
    ForecastedSpend    float64
    ForecastPercentage float64
    Alerts             []ThresholdStatus
    Currency           string
}
```

**New Methods** (no new fields):

```go
// ShouldExit determines if the CLI should exit with non-zero code.
// Returns true if:
//   - Budget has ExitOnThreshold enabled, AND
//   - Any alert threshold has been exceeded (EXCEEDED status)
func (s *BudgetStatus) ShouldExit() bool

// GetExitCode returns the exit code to use when ShouldExit() is true.
// Returns 0 if ShouldExit() would return false.
// Returns Budget.GetExitCode() otherwise.
func (s *BudgetStatus) GetExitCode() int

// ExitReason returns a human-readable explanation for the exit code.
// Used for --debug logging and error messages.
func (s *BudgetStatus) ExitReason() string
```

## New Error Types

**Location**: `internal/config/budget.go`

```go
var (
    // ErrExitCodeOutOfRange is returned when exit_code is not in 0-255 range.
    ErrExitCodeOutOfRange = errors.New("exit_code must be between 0 and 255")
)
```

## Configuration Examples

### YAML Config File

```yaml
# ~/.finfocus/config.yaml
cost:
  budgets:
    amount: 100
    currency: USD
    alerts:
      - threshold: 80
        type: actual
      - threshold: 100
        type: actual
    exit_on_threshold: true
    exit_code: 2
```

### Environment Variables

| Variable                            | Type   | Description                                |
|-------------------------------------|--------|--------------------------------------------|
| `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD` | string | "true", "1" enables; "false", "0" disables |
| `FINFOCUS_BUDGET_EXIT_CODE`         | string | Integer value 0-255                        |

### CLI Flags

| Flag                  | Type | Description                       |
|-----------------------|------|-----------------------------------|
| `--exit-on-threshold` | bool | Enable exit on threshold exceeded |
| `--exit-code`         | int  | Exit code to use (default: 1)     |

## State Transitions

Exit code evaluation is stateless—it reads current configuration and budget status, then determines exit behavior:

```text
[BudgetStatus] → ShouldExit() → true/false
                      │
                      ├── ExitOnThreshold: false → false
                      ├── ExitOnThreshold: true, No alerts exceeded → false
                      └── ExitOnThreshold: true, Alert exceeded → true
                                │
                                └── GetExitCode() → 0-255
```

## Precedence Rules

Configuration values are merged with the following precedence (highest to lowest):

1. **CLI Flags**: `--exit-on-threshold`, `--exit-code`
2. **Environment Variables**: `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD`, `FINFOCUS_BUDGET_EXIT_CODE`
3. **Config File**: `cost.budgets.exit_on_threshold`, `cost.budgets.exit_code`
4. **Defaults**: `exit_on_threshold: false`, `exit_code: 1`

## Relationships

```text
Config
  └── CostConfig
        └── BudgetConfig
              ├── Amount, Currency, Period
              ├── Alerts[]
              ├── ExitOnThreshold (NEW)
              └── ExitCode (NEW)

Engine
  └── BudgetStatus
        ├── Budget (BudgetConfig)
        ├── Alerts[] (ThresholdStatus)
        ├── ShouldExit() (NEW method)
        ├── GetExitCode() (NEW method)
        └── ExitReason() (NEW method)
```
