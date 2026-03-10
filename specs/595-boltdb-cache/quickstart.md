# Quickstart: BoltDB Cache Backend

**Feature**: 595-boltdb-cache

## Prerequisites

- Go 1.25.8+
- `go.etcd.io/bbolt` dependency added to `go.mod`

## Basic Usage

### Cache initialization (CLI layer)

The cache is initialized in `internal/cli/common_execution.go` via `initCacheFromConfig()`. The configuration precedence is unchanged:

1. CLI flag: `--cache-ttl`
2. Environment: `FINFOCUS_CACHE_TTL`
3. Config file: `config.yaml` → `cost.cache.ttlSeconds`
4. Default: 0 (disabled)

When enabled, the cache creates a `cache.db` file in the project's `.finfocus/` directory (resolved by walking up from CWD to find `Pulumi.yaml`). Falls back to `~/.finfocus/cache.db` when no project context is available.

### Engine integration

```go
// Engine uses cache via the Cache interface - no changes needed
eng := engine.New(clients, loader).
    WithCache(cacheStore)

// Projected costs: cached per-resource
results, err := eng.GetProjectedCost(ctx, resources)

// Actual costs: cached per-query
results, err := eng.GetActualCostWithOptions(ctx, request)
```

### Targeted invalidation

```go
// Invalidate all AWS EC2 projected costs
count, err := store.InvalidateByPrefix("projected/aws/ec2:Instance/")

// Invalidate all projected costs for a provider
count, err := store.InvalidateByPrefix("projected/aws/")

// Invalidate all actual cost queries
count, err := store.InvalidateByPrefix("actual/")

// Clear everything
count, err := store.InvalidateByPrefix("")
```

### Cache key format

Keys are structured and human-readable:

```text
projected/aws/ec2:Instance/us-east-1/t3.micro
actual/aws/ec2:Instance/2025-01-01/2025-02-01/a3f2b1c4
recommendations/multi/ec2:Instance+rds:DBInstance
```

## Development

### Running tests

```bash
# Unit tests for cache package
go test -v ./internal/engine/cache/...

# Engine cache integration tests
go test -v ./internal/engine/ -run TestCache

# Full test suite
make test

# With race detector
make test-race
```

### Inspecting the cache

```bash
# View database stats
bbolt stats .finfocus/cache.db

# List all keys in a bucket
bbolt keys .finfocus/cache.db projected

# View a specific entry
bbolt get .finfocus/cache.db projected "aws/ec2:Instance/us-east-1/t3.micro"
```

### Configuration

```bash
# Enable caching with 1-hour TTL
finfocus cost projected --cache-ttl 3600 --pulumi-json plan.json

# Override via environment
export FINFOCUS_CACHE_TTL=7200    # 2 hours
export FINFOCUS_CACHE_DIR=/tmp/finfocus-cache

# Disable caching
finfocus cost projected --cache-ttl 0 --pulumi-json plan.json
```
