# Feature Specification: Engine Test Coverage Completion

**Feature Branch**: `007-engine-test-coverage`
**Created**: 2025-12-02
**Status**: Draft
**Input**: Epic: Engine Test Coverage Completion - Complete the engine package test coverage to meet the 80% threshold with comprehensive edge case and integration testing.

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Core Calculation Reliability (Priority: P1)

As a developer maintaining the FinFocus engine, I need confidence that core cost calculation functions work correctly under all conditions so that users receive accurate cost estimates.

**Why this priority**: Cost calculation is the fundamental value proposition of FinFocus. Incorrect calculations directly impact user trust and financial planning decisions.

**Independent Test**: Can be validated by running `go test ./internal/engine/... -coverprofile=coverage.out && go tool cover -func=coverage.out | grep total` and verifying coverage meets 80% threshold with all tests passing.

**Acceptance Scenarios**:

1. **Given** empty cost results, **When** CreateCrossProviderAggregation is called, **Then** it returns ErrEmptyResults error
2. **Given** results with mixed currencies (USD and EUR), **When** aggregation is attempted, **Then** ErrMixedCurrencies is returned with descriptive message
3. **Given** a result with EndDate before StartDate, **When** validation runs, **Then** ErrInvalidDateRange is returned
4. **Given** results with all-zero costs, **When** aggregation calculates totals, **Then** zero totals are returned without divide-by-zero errors
5. **Given** a single cost result, **When** aggregated, **Then** the result is returned unchanged with correct period formatting

---

### User Story 2 - Edge Case Handling (Priority: P2)

As a developer, I need comprehensive edge case coverage in the engine so that unexpected input combinations don't cause crashes or incorrect behavior.

**Why this priority**: Edge cases represent real-world scenarios that users will encounter. Handling them gracefully prevents production issues.

**Independent Test**: Run edge case test suite with `go test ./test/unit/engine/... -run TestEdgeCase -v` and verify all edge case tests pass.

**Acceptance Scenarios**:

1. **Given** an empty currency field, **When** cross-provider aggregation runs, **Then** USD is used as default
2. **Given** very large cost values (near float64 max), **When** aggregated, **Then** no overflow occurs and totals are accurate
3. **Given** resource type without provider prefix, **When** extractProviderFromType runs, **Then** "unknown" is returned
4. **Given** properties with nil map, **When** resource validation runs, **Then** validation passes without nil pointer panic
5. **Given** results with DailyCosts array, **When** grouped by monthly, **Then** daily costs are correctly distributed across months

---

### User Story 3 - Performance at Scale (Priority: P3)

As a platform engineer using FinFocus for enterprise deployments, I need the engine to handle large resource sets efficiently so that cost analysis doesn't become a bottleneck.

**Why this priority**: Enterprise users may have thousands of resources. Performance degradation at scale limits adoption in large organizations.

**Independent Test**: Run benchmark suite with `go test ./test/benchmarks/... -bench=. -benchmem` and verify no significant regressions from baseline.

**Acceptance Scenarios**:

1. **Given** 1,000 resources, **When** GetProjectedCost is called, **Then** processing completes within acceptable time (benchmark established)
2. **Given** 10,000 resources, **When** GetActualCostWithOptions is called, **Then** memory usage scales linearly without excessive allocations
3. **Given** 100,000 resources, **When** cross-provider aggregation runs, **Then** results are returned without timeout or memory exhaustion

---

### User Story 4 - Integration Test Completeness (Priority: P2)

As a quality engineer, I need integration tests that verify engine components work together correctly so that individual unit tests don't miss interaction bugs.

**Why this priority**: Component interactions often reveal bugs that unit tests miss. Integration tests provide confidence in the system as a whole.

**Independent Test**: Run integration test suite with `go test ./test/integration/... -v` and verify all integration scenarios pass.

**Acceptance Scenarios**:

1. **Given** engine with mock plugin client, **When** GetProjectedCost fails on plugin, **Then** spec fallback is attempted
2. **Given** engine with tag filter request, **When** GetActualCostWithOptions runs, **Then** only matching resources are processed
3. **Given** engine with groupBy=daily, **When** actual cost results have DailyCosts, **Then** daily breakdown is correctly aggregated
4. **Given** engine with multiple plugins returning costs, **When** GetProjectedCost runs, **Then** all plugin results are collected

---

### Edge Cases

- What happens when all plugins return errors but spec fallback succeeds?
- How does the engine handle a context cancellation mid-processing?
- What happens when date range spans zero days (same start and end date)?
- How does GroupResults handle empty result slices?
- What happens when resource properties contain special characters in keys?
- How does currency validation handle case sensitivity (USD vs usd)?

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: Engine SHOULD achieve 80% or higher line coverage, but only through meaningful tests (quality over coverage)
- **FR-002**: Error types SHOULD have test coverage where they represent real failure modes users will encounter
- **FR-003**: CreateCrossProviderAggregation SHOULD be tested for meaningful boundary conditions (empty input, currency mismatch)
- **FR-004**: Benchmark tests SHOULD cover 1K, 10K, and 100K resource scenarios to establish performance baselines
- **FR-005**: Functions with 0% coverage SHOULD be evaluated: add tests only if the function has meaningful logic worth testing
- **FR-006**: Each test MUST have a distinct purpose and meaningful assertions - no tests just for coverage

### Coverage Analysis (for reference)

**Functions worth testing (have meaningful logic):**

- `tryStoragePricing` - Storage cost calculation with size multiplier
- `getDefaultMonthlyByType` - Type-based cost defaults
- `parseFloatValue` - Type conversion with multiple input types
- `distributeDailyCosts` - Daily cost distribution algorithm

**Functions that may not need unit tests:**

- `GetActualCost` - Simple delegation to `GetActualCostWithOptions`
- `getStorageSize` - Simple property lookup
- `tryFallbackNumericValue` - Last-resort fallback with minimal logic

### Key Entities

- **CostResult**: Core output structure containing calculated costs, currency, and metadata
- **ResourceDescriptor**: Input structure representing cloud resources with type, provider, and properties
- **CrossProviderAggregation**: Time-based aggregation structure for multi-provider cost analysis
- **GroupBy**: Type-safe enumeration for grouping strategies
- **ErrorDetail**: Structured error information for failed calculations

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: Engine package line coverage reaches 80% or higher (currently 78.5%)
- **SC-002**: All 5 defined error types have at least one explicit test case each
- **SC-003**: Each new test has a distinct, clear purpose documented in a comment
- **SC-004**: Benchmark tests cover resource counts of 1K, 10K, and 100K
- **SC-005**: Test suite runs complete within 60 seconds on standard hardware
- **SC-006**: No test flakiness observed across 10 consecutive runs

### Test Quality Constraints (Anti-Slop)

**Quality over quantity.** It is better to be below 80% coverage than to have AI slop tests.

**Signs of good tests (REQUIRED):**

- Each test has a distinct, clear purpose
- Table-driven tests for variations on the same behavior
- Simple setup, clear assertions
- Fast execution (< 1s for entire suite)
- Test names that describe what is being tested

**Signs of "AI slop" (PROHIBITED):**

- Redundant test cases testing same thing multiple ways
- Unused test structure fields (e.g., `expectError` when method never errors)
- Helper functions that don't reduce complexity
- Over-complicated extraction/assertion logic
- Generic test names like `TestFunction_Works`
- Tests that just exercise code paths without meaningful assertions

**What to test:**

- Pure transformation functions (data structure conversions)
- Stateless logic (diff calculations, validation)
- Error conditions that represent real failure modes
- Edge cases that users will actually encounter

**What NOT to test (at unit level):**

- Simple delegating methods that just call other tested functions
- Trivial getters/setters
- Code that primarily integrates external services (test as integration tests instead)

### Assumptions

- Go 1.25.7 is the target runtime environment
- Test coverage is measured using standard `go tool cover` tooling
- Benchmark baselines will be established during initial test implementation
- Mock plugin implementations from `test/mocks/plugin/` will be used for integration testing
- Coverage threshold excludes generated code (if any)
- **Coverage target is secondary to test quality** - reject any test that doesn't have clear value
