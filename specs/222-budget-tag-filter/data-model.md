# Data Model: Tag-Based Budget Filtering

**Feature**: 222-budget-tag-filter
**Date**: 2026-02-02

## Entity Changes

### BudgetFilterOptions (Modified)

**Location**: `internal/engine/budget.go`

**Current State**:

```go
// BudgetFilterOptions contains criteria for filtering budgets.
type BudgetFilterOptions struct {
    Providers []string // Filter by provider names (case-insensitive, OR logic)
}
```

**Updated State**:

```go
// BudgetFilterOptions contains criteria for filtering budgets.
type BudgetFilterOptions struct {
    Providers []string          // Filter by provider names (case-insensitive, OR logic)
    Tags      map[string]string // Filter by metadata tags (case-sensitive, AND logic, supports glob patterns)
}
```

**Field Details**:

| Field | Type | Logic | Case | Pattern Support |
|-------|------|-------|------|-----------------|
| `Providers` | `[]string` | OR (any match) | Insensitive | No |
| `Tags` | `map[string]string` | AND (all match) | Sensitive | Yes (`*` glob) |

**Validation Rules**:

- `nil` or empty `Tags` map: No tag filtering applied (all budgets pass)
- Tag key must be non-empty string
- Tag value may be empty (matches budgets with empty value for that key)
- Tag value may contain `*` for glob matching via `path.Match()`

---

### Filter Expression (New Concept)

**Location**: CLI layer (parsed from `--filter` flags)

**Format**: `<type>:<key>=<value>` or `<type>=<value>`

**Examples**:

| Input | Type | Key | Value |
|-------|------|-----|-------|
| `provider=kubecost` | provider | - | kubecost |
| `tag:namespace=production` | tag | namespace | production |
| `tag:env=prod-*` | tag | env | prod-* |
| `tag:cluster=us-east-1` | tag | cluster | us-east-1 |

**Parsing Rules**:

1. If starts with `tag:` → tag filter
2. If starts with `provider=` → provider filter
3. Must contain `=` for tag filters
4. Key after `tag:` must be non-empty

---

## State Transitions

No state transitions apply. This is a stateless filtering operation.

---

## Relationships

```text
┌─────────────────────┐
│ CLI --filter flags  │
└──────────┬──────────┘
           │ parseBudgetFilters()
           ▼
┌─────────────────────┐
│ BudgetFilterOptions │
│ - Providers []str   │
│ - Tags map[str]str  │
└──────────┬──────────┘
           │ GetBudgets()
           ▼
┌─────────────────────┐     ┌─────────────────┐
│ pbc.Budget          │────▶│ Budget.Metadata │
│ (from plugins)      │     │ (tag storage)   │
└─────────────────────┘     └─────────────────┘
           │
           │ FilterBudgets() / matchesBudgetTags()
           ▼
┌─────────────────────┐
│ Filtered Budgets    │
└─────────────────────┘
```

---

## Data Volume Considerations

| Metric | Expected Range | Notes |
|--------|----------------|-------|
| Budgets per query | 1 - 10,000 | SC-001 performance target |
| Tags per budget | 0 - 50 | Typical cloud metadata |
| Filter tags | 1 - 10 | Practical CLI usage |
| Filter evaluation | O(b × t × f) | b=budgets, t=tags/budget, f=filter tags |

**Performance**: For 10,000 budgets with 50 tags each and 10 filter tags:

- Worst case: 10,000 × 50 × 10 = 5M comparisons
- With early exit on mismatch: typically much less
- Target: < 3 seconds (SC-001)

---

## Existing Entity References

### pbc.Budget (Protobuf)

**Location**: `github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1`

```protobuf
message Budget {
    string id = 1;
    string name = 2;
    string source = 3;              // Provider (e.g., "kubecost", "aws-budgets")
    // ... other fields
    map<string, string> metadata = 10;  // Tag storage location
}
```

**Metadata Key Convention**:

- Direct keys: `namespace`, `cluster`, `environment`
- Prefixed keys: `tag:env`, `tag:team` (legacy format)

### pbc.BudgetFilter (Protobuf)

**Location**: `github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1`

```protobuf
message BudgetFilter {
    repeated string providers = 1;
    repeated string regions = 2;
    repeated string resource_types = 3;
    map<string, string> tags = 4;
}
```

**Note**: The existing `FilterBudgets()` function already supports this protobuf filter. The implementation will convert `BudgetFilterOptions` to `pbc.BudgetFilter` for filtering.

---

## Migration Notes

**Backward Compatibility**:

- Existing code using `BudgetFilterOptions{Providers: []string{...}}` continues to work
- `Tags: nil` or missing field behaves identically to pre-change behavior
- No database migrations required (in-memory filtering only)
