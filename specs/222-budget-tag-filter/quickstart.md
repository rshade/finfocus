# Quickstart: Tag-Based Budget Filtering

**Feature**: 222-budget-tag-filter
**Date**: 2026-02-02

## Overview

This feature enables filtering budget results by metadata tags using the `--filter "tag:key=value"` CLI syntax. Tags are matched using AND logic with support for glob patterns.

## Usage Examples

### Basic Tag Filtering

Filter budgets by namespace:

```bash
finfocus cost actual --filter "tag:namespace=production"
```

### Glob Pattern Matching

Filter budgets matching a pattern:

```bash
# Match prod-us, prod-eu, prod-asia
finfocus cost actual --filter "tag:namespace=prod-*"

# Match team-a-production, team-b-production
finfocus cost actual --filter "tag:env=*-production"
```

### Multiple Tag Filters (AND Logic)

Filter budgets matching ALL specified tags:

```bash
finfocus cost actual \
  --filter "tag:namespace=production" \
  --filter "tag:cluster=us-east-1"
```

### Combined Provider and Tag Filters

```bash
finfocus cost actual \
  --filter "provider=kubecost" \
  --filter "tag:namespace=staging"
```

## Programmatic Usage

### Go API

```go
import "github.com/rshade/finfocus/internal/engine"

// Create filter options with tags
filter := &engine.BudgetFilterOptions{
    Providers: []string{"kubecost"},
    Tags: map[string]string{
        "namespace": "production",
        "cluster":   "us-east-*",  // Glob pattern
    },
}

// Get filtered budgets
result, err := eng.GetBudgets(ctx, filter)
if err != nil {
    return err
}

// Process filtered budgets
for _, budget := range result.Budgets {
    fmt.Printf("Budget: %s (provider: %s)\n", budget.GetName(), budget.GetSource())
}
```

### Filter Logic Summary

| Filter Type | Logic | Case Sensitivity | Patterns |
|-------------|-------|------------------|----------|
| Provider | OR (any match) | Insensitive | No |
| Tag | AND (all match) | Sensitive | `*` glob |

## Error Handling

### Malformed Filter Syntax

```bash
# Missing value - CLI exits with error
finfocus cost actual --filter "tag:namespace"
# Error: invalid filter syntax: missing '=' in "tag:namespace"

# Empty key - CLI exits with error
finfocus cost actual --filter "tag:=production"
# Error: invalid filter syntax: empty key in "tag:=production"
```

### No Matches

When no budgets match the filter, an empty result is returned (not an error):

```bash
finfocus cost actual --filter "tag:namespace=nonexistent"
# Returns empty table with headers
```

## Supported Metadata Keys

Common metadata keys populated by plugins:

| Key | Plugin | Example Values |
|-----|--------|----------------|
| `namespace` | kubecost | `production`, `staging`, `default` |
| `cluster` | kubecost | `us-east-1`, `eu-west-1` |
| `environment` | aws-budgets | `prod`, `dev`, `test` |
| `team` | vantage | `platform`, `frontend`, `backend` |

**Note**: Available keys depend on what plugins populate in budget metadata.

## Performance Considerations

- Filtering is performed in-memory after budgets are retrieved from plugins
- Performance target: 3 seconds for up to 10,000 budgets
- Glob pattern matching uses Go's `path.Match()` (efficient for simple patterns)

## Backward Compatibility

- Existing commands without `--filter` flags continue to work unchanged
- Provider-only filtering (`--filter "provider=aws"`) unchanged
- Empty filter options return all budgets (no filtering)
