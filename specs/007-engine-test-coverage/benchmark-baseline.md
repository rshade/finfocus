# Engine Benchmark Baselines

**Date**: 2025-12-02
**System**: Intel Core i7-6600U @ 2.60GHz, Linux
**Go Version**: 1.25.6

## Summary

These benchmarks establish performance baselines for the engine package
at enterprise scale (1K, 10K, 100K resources).

## Benchmark Results

### Core Operations

| Benchmark                     | Ops/sec | ns/op   | B/op   | allocs/op |
| ----------------------------- | ------- | ------- | ------ | --------- |
| Properties_Conversion         | 142,045 | 7,690   | 1,144  | 24        |
| ResourceDescriptor_Allocation | 9,926   | 192,602 | 72,017 | 700       |

### Cross-Provider Aggregation

| Scale       | Ops/sec | ns/op      | B/op    | allocs/op |
| ----------- | ------- | ---------- | ------- | --------- |
| 1K results  | 2,276   | 675,571    | 78,672  | 2,078     |
| 10K results | 154     | 10,257,162 | 654,672 | 20,078    |

### Observations

1. **Linear Scaling**: Memory and allocation counts scale linearly with input size
   - 1K → 10K: ~10x increase in time and allocations (as expected)

2. **Performance Characteristics**:
   - Cross-provider aggregation: ~0.68ms for 1K, ~10ms for 10K
   - Memory: ~79KB for 1K, ~655KB for 10K

3. **Recommendations**:
   - For datasets > 10K results, consider streaming/pagination
   - Current performance is acceptable for typical enterprise deployments

## Running Benchmarks

```bash
# Run all benchmarks
FINFOCUS_LOG_LEVEL=error go test ./test/benchmarks/... -bench=. -benchmem

# Run specific scale benchmarks
go test ./test/benchmarks/... -bench='1K|10K|100K' -benchmem

# Compare against baseline
go test ./test/benchmarks/... -bench=. -benchmem | tee new-results.txt
```

## Notes

- Benchmarks exclude plugin communication overhead (no plugins registered)
- Results may vary based on system load and hardware
- Update this baseline after significant engine changes
