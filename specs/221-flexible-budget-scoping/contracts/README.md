# API Contracts: Flexible Budget Scoping

**Feature Branch**: `221-flexible-budget-scoping`
**Date**: 2026-01-24

## Status: No New Contracts Required

This feature does not introduce new API contracts because:

1. **Configuration-Based Feature**: Scoped budgets are defined in
   `~/.finfocus/config.yaml`, not via API endpoints.

2. **Existing Plugin APIs**: Cost data is retrieved using existing
   `GetProjectedCost` and `GetActualCost` plugin APIs. No protocol
   buffer changes needed.

3. **Internal Processing**: Scope matching and cost allocation happen
   entirely within the core engine, using existing data structures.

## Existing Contracts Used

| Contract                  | Location               | Usage                                |
| ------------------------- | ---------------------- | ------------------------------------ |
| `GetProjectedCostRequest` | `finfocus-spec/proto/` | Fetch projected costs from plugins   |
| `GetActualCostRequest`    | `finfocus-spec/proto/` | Fetch actual costs from plugins      |
| `BudgetFilter`            | `finfocus-spec/proto/` | Filter costs by provider/region/tags |

## Future Considerations

If scoped budgets need to be managed via API (e.g., for a web UI), a new
`BudgetConfigurationService` would be defined in `finfocus-spec`. This is
explicitly out of scope for the MVP.
