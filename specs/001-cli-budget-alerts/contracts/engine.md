# Internal Contract: Budget Engine

This contract defines the interface for the budget evaluation engine.

## Interface: `BudgetEngine`

```go
type BudgetEngine interface {
    // Evaluate compares current spend against the configured budget and alerts.
    // It returns a BudgetStatus or an error if evaluation fails (e.g. currency mismatch).
    Evaluate(budget config.BudgetConfig, currentSpend float64, currency string) (*BudgetStatus, error)
}
```

## Inputs
- `budget`: The `BudgetConfig` object from the configuration.
- `currentSpend`: The total cost calculated by the ingestion/analysis engine.
- `currency`: The currency of the `currentSpend`.

## Outputs
- `BudgetStatus`: A structured object containing percentages, forecasts, and alert states.
- `error`: Returned if:
    - `budget.Currency` != `currency` and no conversion available.
    - `budget.Amount` is negative.
    - Spend data is invalid.
