# Research: Testing Framework and Strategy

**Feature**: Testing Framework and Strategy
**Date**: 2025-11-06
**Purpose**: Validate technology choices and best practices for comprehensive Go testing

## Research Topics

### 1. Testing Frameworks for Go

**Decision**: Use Go standard `testing` package + `testify` for assertions

**Rationale**:

- Go's built-in `testing` package is industry standard and well-supported
- `testify` already present in codebase (`github.com/stretchr/testify`)
- Provides `assert` and `require` packages for readable test assertions
- Supports table-driven tests natively (Go idiom)
- No additional framework overhead or learning curve
- Excellent tooling integration (`go test`, `go test -cover`, IDE support)

**Alternatives Considered**:

- **Ginkgo/Gomega**: BDD-style testing framework
  - **Rejected**: Adds complexity without significant benefit for CLI tool testing
  - Go's native table-driven tests already provide excellent readability
- **GoConvey**: Web UI for tests with BDD syntax
  - **Rejected**: Web UI unnecessary for CI/CD context, additional dependency
- **Bare `testing` package only**: No assertion library
  - **Rejected**: Verbose error messages, less readable assertions

**Implementation Notes**:

- Use `assert` for non-critical checks (continues test execution)
- Use `require` for critical checks (stops test immediately on failure)
- Leverage table-driven test patterns for comprehensive scenario coverage

---

### 2. gRPC Mocking Strategies

**Decision**: Custom mock plugin server implementing CostSource protocol

**Rationale**:

- Full control over response behavior and timing
- Can simulate realistic plugin scenarios (latency, errors, partial failures)
- Reusable across unit, integration, and E2E tests
- Supports error injection for failure testing
- Can configure responses per-test for fine-grained control
- Educational value for future plugin developers

**Alternatives Considered**:

- **gomock with protoc-gen-gomock**: Auto-generated mocks from protobufs
  - **Rejected**: Less flexible for complex scenarios, harder to configure dynamically
  - Best for unit testing interfaces, not full gRPC services
- **grpc-mock library**: Generic gRPC mocking framework
  - **Rejected**: Adds dependency, less control over FinFocus-specific behavior
- **Test doubles as separate binaries**: Launch real plugin binaries configured for testing
  - **Rejected**: Slower test execution, harder to debug, more complex setup

**Implementation Notes**:

- Mock plugin runs as in-process gRPC server (no separate process needed)
- Configurable via function calls before test execution
- Supports both blocking and streaming RPC patterns
- Reset state between tests to prevent cross-test contamination

---

### 3. Coverage Tools and Reporting

**Decision**: Go native `go test -cover` with gocover-cobertura for CI reporting

**Rationale**:

- Built-in Go coverage already captures line-level coverage
- Existing CI pipeline (`.github/workflows/ci.yml`) already uses `go test -coverprofile`
- `gocover-cobertura` converts Go coverage to Cobertura XML for GitHub Actions
- No additional tools needed for local development
- Coverage reports automatically generated and uploaded in CI

**Alternatives Considered**:

- **gocov + gocov-html**: Alternative coverage HTML generation
  - **Rejected**: Existing pipeline already working with native tools
- **Codecov.io or Coveralls**: Third-party coverage hosting
  - **Rejected**: Unnecessary external dependency, GitHub Actions sufficient
- **go tool cover -html**: Manual HTML generation
  - **Rejected**: Already available, but CI needs XML format for PR comments

**Implementation Notes**:

- Run tests with `-coverprofile=coverage.out` flag
- Generate HTML reports locally: `go tool cover -html=coverage.out`
- CI converts to Cobertura: `gocover-cobertura < coverage.out > coverage.xml`
- Set coverage threshold in CI: fail if <80% overall, <95% for critical paths

---

### 4. CI Integration Points

**Decision**: Extend existing `.github/workflows/ci.yml` test job

**Rationale**:

- CI workflow already has comprehensive test job with coverage
- Already runs `go test -race -coverprofile=coverage.out ./...`
- Already uploads coverage artifacts
- Just need to add coverage threshold validation

**Current CI Test Job Analysis**:

```yaml
test:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/setup-go@v5
      with:
        go-version: '1.25.5'
        cache: true
    - run: go test -race -coverprofile=coverage.out -covermode=atomic ./...
    - run: go tool cover -func=coverage.out
    - uses: actions/upload-artifact@v4
      with:
        name: coverage
        path: coverage.out
```

**Enhancements Needed**:

1. Add coverage threshold check (fail if <80%)
2. Generate Cobertura XML for PR comments
3. Add separate jobs for integration and E2E tests (longer timeout)
4. Ensure critical path coverage validation (engine, CLI, pluginhost ≥95%)

**Alternatives Considered**:

- **Create new workflow file**: Separate testing workflow
  - **Rejected**: Fragmentation, harder to maintain, existing workflow sufficient
- **Use test matrix**: Run different test categories in matrix
  - **Considered**: May add later if execution time becomes issue

**Implementation Notes**:

- Add script to validate coverage thresholds
- Keep all tests in single workflow job initially (optimize later if needed)
- Use `go test ./test/unit/... ./test/integration/... ./test/e2e/...` for organization

---

### 5. Golden File Testing

**Decision**: Use `github.com/sebdah/goldie/v2` for golden file testing

**Rationale**:

- Well-maintained library specifically for Go golden file testing
- Simple API: `goldie.Assert(t, "test-name", actual)`
- Automatic golden file management (creates if missing, updates with flag)
- Supports custom differ functions for better failure messages
- Wide adoption in Go community (5k+ stars)

**Alternatives Considered**:

- **Custom implementation**: Write own golden file comparison
  - **Rejected**: Reinventing wheel, goldie already handles edge cases well
- **github.com/bradleyjkemp/cupaloy**: Alternative golden file library
  - **Rejected**: Less maintained, fewer features than goldie
- **Manual file comparison**: Load expected files and compare manually
  - **Rejected**: Verbose, error-prone, no auto-update mechanism

**Implementation Notes**:

- Store golden files in `test/fixtures/golden/` directory
- Use `-update` flag to regenerate golden files when output intentionally changes
- Test all output formats: table, JSON, NDJSON
- Include both success and error scenarios in golden files

---

### 6. Benchmark Patterns and Regression Detection

**Decision**: Go native benchmarks (`func BenchmarkXxx(b *testing.B)`) with manual baseline tracking

**Rationale**:

- Built-in Go benchmark support is excellent
- `go test -bench=. -benchmem` provides detailed metrics
- Can compare benchmark results with `benchcmp` or `benchstat`
- CI can store baseline and compare on each run

**Benchmark Metrics to Track**:

- **Execution time**: ns/op for cost calculations
- **Memory allocations**: allocs/op for resource usage
- **Memory bytes**: B/op for heap pressure
- **Regression threshold**: 20% degradation triggers failure

**Alternatives Considered**:

- **benchstat**: Statistical comparison of benchmark results
  - **Partially Adopted**: Will use for analyzing results, not automated yet
- **Third-party benchmark frameworks**: benchmarking libraries
  - **Rejected**: Native Go benchmarks sufficient for our needs
- **Continuous benchmarking services**: External platforms
  - **Rejected**: Overkill for current scale, can add later if needed

**Implementation Notes**:

- Create benchmarks for critical paths: cost calculation, JSON parsing, plugin communication
- Store baseline results in `test/benchmarks/baseline.txt`
- CI compares current run against baseline, fails if >20% slower
- Run benchmarks weekly or on performance-critical PRs (not every commit)

---

## Summary of Decisions

| Component         | Choice                                      | Key Reason                          |
| ----------------- | ------------------------------------------- | ----------------------------------- |
| Testing Framework | Go `testing` + `testify`                    | Already in use, industry standard   |
| gRPC Mocking      | Custom mock plugin server                   | Full control, reusable, educational |
| Coverage Tools    | Native `go test -cover` + gocover-cobertura | Already working in CI               |
| CI Integration    | Extend existing workflow                    | Avoid fragmentation                 |
| Golden Files      | `github.com/sebdah/goldie/v2`               | Well-maintained, simple API         |
| Benchmarks        | Go native with baseline tracking            | Built-in support sufficient         |

---

## Risk Mitigation

1. **Risk**: Test execution time grows too long
   - **Mitigation**: Use `go test -parallel` for concurrency, optimize slow tests

2. **Risk**: Flaky tests in CI
   - **Mitigation**: Mock plugin eliminates external dependencies, use `-race` to catch concurrency issues

3. **Risk**: Coverage threshold too strict (blocks valid PRs)
   - **Mitigation**: Allow package-level exceptions for infrastructure code, focus on critical paths

4. **Risk**: Golden files get out of sync with code
   - **Mitigation**: Update golden files in same PR as code changes, document update process

5. **Risk**: Benchmark noise on shared CI runners
   - **Mitigation**: Use statistical analysis (benchstat), run multiple iterations, focus on major regressions (>20%)

---

## Next Steps

All research topics resolved. Ready to proceed to Phase 1 (Design & Contracts).
