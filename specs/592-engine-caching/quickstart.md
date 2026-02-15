# Quickstart: Unified Engine Caching System

**Branch**: `592-engine-caching`

## Building

```bash
make build
```

## Using Caching

### Enable cache for projected costs

```bash
# First run: calculates from plugins, stores in cache
./bin/finfocus cost projected --pulumi-json plan.json --cache-ttl 3600

# Second run: returns cached results (look for "(cached)" in Adapter column)
./bin/finfocus cost projected --pulumi-json plan.json --cache-ttl 3600
```

### Enable cache for actual costs

```bash
# First run: queries cloud APIs
./bin/finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --to 2025-01-31 --cache-ttl 3600

# Second run: returns cached results
./bin/finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --to 2025-01-31 --cache-ttl 3600
```

### Disable cache

```bash
# Explicitly disable (default behavior)
./bin/finfocus cost projected --pulumi-json plan.json --cache-ttl 0
```

### Environment variable

```bash
# Enable caching for all commands without flags
export FINFOCUS_CACHE_TTL=3600
./bin/finfocus cost projected --pulumi-json plan.json
```

## Verifying Cache Behavior

### Check debug logs for cache hits/misses

```bash
./bin/finfocus cost projected --pulumi-json plan.json --cache-ttl 3600 --debug 2>&1 | grep cache
```

### Inspect cache files

```bash
ls ~/.finfocus/cache/
```

### Clear cache

```bash
rm ~/.finfocus/cache/*.json
```

## Running Tests

```bash
# All tests
make test

# Cache-specific tests
go test ./internal/engine/cache/...

# CLI cache wiring tests
go test ./internal/cli/... -run TestInitCache

# Engine cache integration tests
go test ./internal/engine/... -run TestCache

# Lint check
make lint
```
