# Quickstart: Benchmark PR Reporting

**Date**: 2026-02-16
**Feature**: `594-ci-benchmark-reporting`

## What Changes

The CI pipeline's benchmark job is upgraded from a smoke-only check (no comparison, no reporting) to a full benchstat-based comparison workflow that posts results as PR comments.

## Files Modified

| File | Change |
| ---- | ------ |
| `.github/workflows/ci.yml` | Replace `benchmark` job (lines 213-234) with `benchmark-baseline` and `benchmark-compare` jobs |
| `.gitignore` | Add `test/benchmarks/baseline.txt`, `test/benchmarks/current.txt`, `test/benchmarks/comparison.txt` |

## Files Created

| File | Purpose |
| ---- | ------- |
| `scripts/ci-benchmark-filter.sh` | Filter zerolog noise from benchmark output for clean benchstat input |

## How It Works

### On push to `main`

1. CI runs all benchmarks (except 100K-scale) with `-count=5`
2. Output is filtered to benchstat-compatible format
3. Result is cached as the baseline for future PR comparisons

### On pull request

1. CI runs the same benchmarks on the PR branch
2. Restores the cached `main` baseline
3. Runs `benchstat baseline.txt current.txt` for statistical comparison
4. Parses delta percentages for severity classification
5. Posts/updates a PR comment with the comparison table

### Severity Indicators

| Delta | Level | CI Impact |
| ----- | ----- | --------- |
| > +50% | Error indicator | Job passes (informational) |
| +20% to +50% | Warning indicator | Job passes (informational) |
| < +20% | Normal | Job passes |
| Improvement | Positive note | Job passes |

## Local Development

The existing local comparison script continues to work unchanged:

```bash
# Local benchmark comparison (unchanged)
./scripts/compare-benchmarks.sh

# Reset baseline locally
./scripts/compare-benchmarks.sh --reset
```

## Verification

After implementation, verify by:

1. Push to `main` and confirm baseline cache is generated
2. Open a PR and confirm benchmark comparison comment appears
3. Push additional commits to the PR and confirm comment is updated (not duplicated)
4. Verify `make lint` passes (actionlint for YAML, shellcheck-compatible filter script)
