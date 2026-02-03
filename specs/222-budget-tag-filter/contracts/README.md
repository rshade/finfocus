# Contracts: Tag-Based Budget Filtering

**Feature**: 222-budget-tag-filter
**Date**: 2026-02-02

## No API Contract Changes Required

This feature does not introduce new API contracts or modify existing ones.

### Rationale

1. **Existing Protobuf Support**: The `pbc.BudgetFilter` message already includes a `tags` field
2. **Internal Extension Only**: Changes are limited to the Go-native `BudgetFilterOptions` struct
3. **No gRPC Changes**: Plugin communication protocol remains unchanged
4. **CLI Syntax Reuse**: Uses existing `--filter` flag pattern from other commands

### Existing Contracts Referenced

| Contract | Location | Status |
|----------|----------|--------|
| `pbc.BudgetFilter` | `finfocus-spec/proto/finfocus/v1/budget.proto` | Used (no changes) |
| `pbc.Budget.metadata` | `finfocus-spec/proto/finfocus/v1/budget.proto` | Used (no changes) |

### Internal API Extensions

The following Go types are extended (not protobuf):

```go
// internal/engine/budget.go
type BudgetFilterOptions struct {
    Providers []string          // Existing
    Tags      map[string]string // NEW: Added for tag filtering
}
```

This is an internal implementation detail, not a cross-repository contract.
