# Quickstart Guide: CLI Pagination and Performance Optimizations

**Feature**: CLI Pagination and Performance Optimizations
**Branch**: `122-cli-pagination`
**Date**: 2026-01-20

## Overview

This guide shows how to use the new pagination, sorting, caching, and virtual scrolling features in FinFocus CLI and TUI.

---

## CLI Pagination

### Basic Usage

**Limit results**:
```bash
# Show only first 10 recommendations
finfocus cost recommendations --limit 10
```

**Page-based pagination**:
```bash
# Show page 2 with 20 items per page
finfocus cost recommendations --page 2 --page-size 20

# Show page 1 (default page size: 20)
finfocus cost recommendations --page 1
```

**Offset-based pagination**:
```bash
# Skip first 40 items, show next 20
finfocus cost recommendations --offset 40 --limit 20

# Skip first 100 items, show rest
finfocus cost recommendations --offset 100
```

**Important**: `--page` and `--offset` are mutually exclusive. Use one or the other, not both.

---

## Sorting

**Sort by savings (descending)**:
```bash
finfocus cost recommendations --sort savings:desc
```

**Sort by cost (ascending)**:
```bash
finfocus cost recommendations --sort cost:asc
```

**Sort by resource name (alphabetical)**:
```bash
finfocus cost recommendations --sort name:asc
```

**Valid sort fields**:
- `savings` - Annual savings amount
- `cost` - Current monthly cost
- `name` - Resource name
- `resourceType` - Resource type (e.g., aws:ec2:Instance)
- `provider` - Cloud provider (aws, azure, gcp)
- `actionType` - Recommendation action (RIGHTSIZE, TERMINATE, etc.)

**Invalid field error**:
```bash
$ finfocus cost recommendations --sort price:desc
Error: invalid sort field "price". Valid fields: actionType, cost, name, provider, resourceType, savings
```

---

## Combining Flags

**Pagination + Sorting**:
```bash
# Top 10 highest-savings recommendations
finfocus cost recommendations --sort savings:desc --limit 10

# Page 2 of cost-sorted recommendations
finfocus cost recommendations --sort cost:asc --page 2 --page-size 20
```

**Pagination + Output Format**:
```bash
# JSON output with pagination metadata
finfocus cost recommendations --page 2 --page-size 20 --output json

# NDJSON streaming (no pagination metadata)
finfocus cost recommendations --limit 50 --output ndjson
```

---

## Streaming Output (NDJSON)

**Basic streaming**:
```bash
# Output each recommendation as a JSON line
finfocus cost recommendations --output ndjson
```

**Pipeline integration**:
```bash
# Show only first 5 recommendations
finfocus cost recommendations --output ndjson | head -n 5

# Filter by action type
finfocus cost recommendations --output ndjson | grep RIGHTSIZE

# Extract savings with jq
finfocus cost recommendations --output ndjson | jq .savings

# Sum total savings
finfocus cost recommendations --output ndjson | jq -s 'map(.savings) | add'
```

**Line-by-line processing**:
```bash
# Process each recommendation immediately
finfocus cost recommendations --output ndjson | while read line; do
    resource=$(echo "$line" | jq -r .resource)
    savings=$(echo "$line" | jq -r .savings)
    echo "Resource $resource can save \$$savings/year"
done
```

**NDJSON Example Output**:
```json
{"resource":"aws:ec2:Instance/web-server-1","savings":1800.00,"monthlyCost":150.00,"actionType":"RIGHTSIZE"}
{"resource":"aws:rds:Instance/db-prod-2","savings":3600.00,"monthlyCost":300.00,"actionType":"TERMINATE"}
{"resource":"aws:s3:Bucket/logs-archive","savings":300.00,"monthlyCost":25.00,"actionType":"DELETE_UNUSED"}
```

---

## JSON Output with Pagination Metadata

**Basic JSON output**:
```bash
finfocus cost recommendations --page 2 --page-size 10 --output json
```

**Example Output**:
```json
{
  "recommendations": [
    {
      "resource": "aws:ec2:Instance/web-1",
      "savings": 1800.00,
      "monthlyCost": 150.00,
      "actionType": "RIGHTSIZE"
    }
  ],
  "pagination": {
    "page": 2,
    "page_size": 10,
    "total_items": 47,
    "total_pages": 5,
    "has_next_page": true,
    "has_prev_page": true
  }
}
```

**Extract pagination info**:
```bash
# Get total items
finfocus cost recommendations --page 1 --output json | jq .pagination.total_items

# Check if more pages exist
finfocus cost recommendations --page 3 --output json | jq .pagination.has_next_page
```

---

## Edge Cases

### Out-of-Bounds Page

When requesting a page beyond available pages, you get an empty result with metadata:

```bash
$ finfocus cost recommendations --page 10 --page-size 20 --output json
{
  "recommendations": [],
  "pagination": {
    "page": 10,
    "page_size": 20,
    "total_items": 47,
    "total_pages": 3,
    "has_next_page": false,
    "has_prev_page": true
  }
}
```

### Zero Results

If no recommendations exist, pagination metadata reflects empty state:

```bash
{
  "recommendations": [],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total_items": 0,
    "total_pages": 0,
    "has_next_page": false,
    "has_prev_page": false
  }
}
```

---

## TUI Mode

### Virtual Scrolling

**Launch TUI**:
```bash
finfocus tui
```

**Navigation**:
- `↑`/`k`: Move selection up
- `↓`/`j`: Move selection down
- `PgUp`: Jump up one page
- `PgDn`: Jump down one page
- `Home`: Go to first item
- `End`: Go to last item

**Performance**:
- Lists with 10,000+ items load instantly
- Scrolling remains smooth (60fps)
- Only visible rows are rendered

### Lazy Loading

**Detail View**:
1. Navigate to a recommendation
2. Press `Enter` to open detail view
3. Cost history loads asynchronously

**Loading State**:
```
┌─ Resource Details ─────────────────────────┐
│ Resource: aws:ec2:Instance/web-1           │
│ Cost History:                              │
│   Loading cost history...                  │
└────────────────────────────────────────────┘
```

**Loaded State**:
```
┌─ Resource Details ─────────────────────────┐
│ Resource: aws:ec2:Instance/web-1           │
│ Cost History:                              │
│   Last 7 days: $35.42                      │
│   Last 30 days: $152.18                    │
└────────────────────────────────────────────┘
```

### Error Recovery

**Network Failure**:
```
┌─ Resource Details ─────────────────────────┐
│ Resource: aws:ec2:Instance/web-1           │
│ Cost History:                              │
│   ❌ Error: network timeout                │
│   [Press 'r' to retry]                     │
└────────────────────────────────────────────┘
```

Press `r` to retry loading without leaving the detail view.

---

## Caching

### Configuration

**Config file** (`~/.finfocus/config.yaml`):
```yaml
cache:
  enabled: true
  ttl_seconds: 3600  # 1 hour
  directory: ~/.finfocus/cache
  max_size_mb: 100
```

**Environment variables**:
```bash
# Override TTL
export FINFOCUS_CACHE_TTL_SECONDS=7200  # 2 hours

# Disable caching
export FINFOCUS_CACHE_ENABLED=false
```

**CLI flag**:
```bash
# Override TTL for this command
finfocus cost recommendations --cache-ttl 1800  # 30 minutes
```

### Cache Behavior

**First run** (cache miss):
```bash
$ time finfocus cost recommendations
# ... fetches from plugins ...
real    0m2.341s
```

**Subsequent run** (cache hit):
```bash
$ time finfocus cost recommendations
# ... reads from cache ...
real    0m0.042s
```

**Cache expiration**:
After TTL expires (default: 1 hour), next run fetches fresh data.

**Manual cache clear**:
```bash
# Clear all expired cache entries
finfocus cache clear

# Clear all cache entries (force refresh)
finfocus cache clear --all
```

---

## Performance Optimization

### Batch Processing

Commands that process large datasets (1000+ resources) automatically use batch processing:

**Progress indicator**:
```
Processing resources... [300/1000] (30%)
```

**Batch size**: 100 items per batch (balances memory vs. API calls)

**Memory usage**: ~80MB for 1000 items (under 100MB target)

### Progress Indicators

**Long-running queries**:
Commands taking >500ms show progress:

```bash
$ finfocus cost recommendations
⠋ Fetching recommendations...
# ... spinner animates until complete ...
```

**Batch progress**:
```bash
$ finfocus cost projected --pulumi-json large-plan.json
Processing resources... [247/1000] (24%)
```

---

## Scripting Examples

### Iterate Through Pages

**Bash script**:
```bash
#!/bin/bash
page=1
while true; do
    response=$(finfocus cost recommendations --page "$page" --page-size 20 --output json)
    has_next=$(echo "$response" | jq -r .pagination.has_next_page)

    # Process recommendations
    echo "$response" | jq -r '.recommendations[] | "\(.resource): $\(.savings)"'

    # Stop if no more pages
    if [ "$has_next" = "false" ]; then
        break
    fi

    page=$((page + 1))
done
```

### Export to CSV

**Using jq**:
```bash
# Header
echo "Resource,Savings,MonthlyCost,ActionType"

# Data
finfocus cost recommendations --output ndjson | \
    jq -r '[.resource,.savings,.monthlyCost,.actionType] | @csv'
```

### Top N Highest Savings

**One-liner**:
```bash
finfocus cost recommendations --sort savings:desc --limit 10 --output ndjson | \
    jq -r '"\(.resource): $\(.savings)/year"'
```

**Output**:
```
aws:rds:Instance/db-prod-2: $3600.00/year
aws:ec2:Instance/web-server-1: $1800.00/year
aws:elasticache:Cluster/cache-1: $1200.00/year
...
```

---

## Troubleshooting

### Flag Conflicts

**Error**: Cannot use both --page and --offset
```bash
$ finfocus cost recommendations --page 2 --offset 40
Error: cannot specify both --page and --offset
```

**Solution**: Use one or the other:
```bash
# Page-based
finfocus cost recommendations --page 2 --page-size 20

# Offset-based
finfocus cost recommendations --offset 40 --limit 20
```

### Invalid Sort Field

**Error**: Invalid sort field
```bash
$ finfocus cost recommendations --sort price:desc
Error: invalid sort field "price". Valid fields: actionType, cost, name, provider, resourceType, savings
```

**Solution**: Use a valid field from the error message.

### Cache Issues

**Stale data**:
```bash
# Force fresh data
finfocus cache clear --all
finfocus cost recommendations
```

**Cache location**:
Check `~/.finfocus/cache/` for cache files (JSON format).

---

## Next Steps

- See [data-model.md](./data-model.md) for data structure details
- See [contracts/interfaces.md](./contracts/interfaces.md) for Go interface definitions
- See [plan.md](./plan.md) for implementation plan

For questions or issues, file a bug report on GitHub: <https://github.com/yourusername/finfocus/issues>
