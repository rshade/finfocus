# Quickstart: Engine Budget Logic

How to use the new Budget Filtering and Aggregation logic in the engine.

## Usage

```go
import (
    "context"

    "github.com/rshade/finfocus/internal/engine"
    pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func main() {
    ctx := context.Background()

    // 1. Create a filter
    filter := &pbc.BudgetFilter{
        Providers: []string{"aws", "gcp"},
        Tags:      map[string]string{"env": "prod"},
    }

    // 2. Filter Budgets
    // budgets := ... (get from plugins)
    filtered := engine.FilterBudgets(budgets, filter)

    // 3. Calculate Summary (uses context for trace propagation)
    summary := engine.CalculateBudgetSummary(ctx, filtered)

    fmt.Printf("Total: %d, Critical: %d\n", summary.TotalBudgets, summary.BudgetsCritical)
}
```

## Testing

Run unit tests for the engine package:

```bash
go test -v ./internal/engine/
```

```