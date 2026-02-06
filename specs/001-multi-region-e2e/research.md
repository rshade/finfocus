# Research: Multi-Region E2E Testing

**Date**: 2026-01-21
**Phase**: 0 (Outline & Research)
**Status**: Complete

## Overview

This document consolidates research findings for implementing multi-region E2E testing support in FinFocus. The research addresses test fixture design, cost validation strategies, failure handling patterns, and integration with existing E2E infrastructure.

## Research Tasks Completed

### 1. Multi-Region Test Fixture Design

**Decision**: Parameterized Pulumi programs with region-specific configuration

**Rationale**:
- Existing E2E test infrastructure (`test/e2e/`) uses Pulumi JSON plans and Pulumi Automation API
- Parameterized fixtures allow code reuse across regions while varying resource configurations
- Each region fixture includes explicit expected cost values for validation
- Structure follows established pattern in `test/e2e/fixtures/ec2/`

**Alternatives Considered**:
- **Static JSON plans only**: Rejected because actual cost testing requires deployed resources via Pulumi Automation API
- **Single fixture with region parameter**: Rejected because different regions may need different resource configurations to demonstrate pricing differences
- **Hardcoded resource definitions per region**: Rejected because it reduces maintainability and increases duplication

**Implementation Approach**:

```text
fixtures/multi-region/<region>/
  ├── Pulumi.yaml          # YAML runtime with 5-10 resources across 3-4 types
  └── expected-costs.json  # Expected cost ranges per resource (±5% tolerance)
```

### 2. Cost Variance Validation Strategy

**Decision**: Tolerance-based validation with explicit expected cost ranges

**Rationale**:
- ±5% variance tolerance (from clarifications) handles minor pricing fluctuations
- Expected cost values stored in `expected-costs.json` per region
- Validation logic extends existing `validator.go` with multi-region support
- Prevents test brittleness from cloud provider pricing updates

**Alternatives Considered**:
- **Exact cost matching**: Rejected due to pricing volatility and test brittleness
- **Percentage-only validation**: Rejected because it doesn't validate absolute cost correctness
- **No baseline expectations**: Rejected because it wouldn't catch regional pricing errors

**Implementation Pattern**:
```go
type ExpectedCost struct {
    ResourceType string  `json:"resource_type"`
    MinCost      float64 `json:"min_cost"`
    MaxCost      float64 `json:"max_cost"`
    Region       string  `json:"region"`
}

func ValidateWithinTolerance(actual, expected, tolerance float64) bool {
    lower := expected * (1 - tolerance)
    upper := expected * (1 + tolerance)
    return actual >= lower && actual <= upper
}
```

### 3. Failure Handling Patterns for E2E Tests

**Decision**: Strict failure semantics with retry logic for transient errors

**Rationale**:
- E2E tests validate production readiness - failures must be explicit (from clarification)
- Missing pricing data → immediate test failure (no graceful degradation)
- Network failures → retry 3x with exponential backoff, then fail
- Distinguishes transient (recoverable) from persistent (fatal) failures

**Alternatives Considered**:
- **Graceful degradation**: Rejected per clarification - E2E tests must fail fast
- **Skip on errors**: Rejected because it masks real issues
- **No retry logic**: Rejected because it increases false negatives from transient issues

**Implementation Pattern**:
```go
func RetryWithBackoff(ctx context.Context, attempts int, operation func() error) error {
    var err error
    for i := 0; i < attempts; i++ {
        err = operation()
        if err == nil {
            return nil
        }
        if !isTransientError(err) {
            return fmt.Errorf("non-transient error: %w", err)
        }
        backoff := time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond
        time.Sleep(backoff)
    }
    return fmt.Errorf("failed after %d attempts: %w", attempts, err)
}
```

### 4. Integration with Existing E2E Infrastructure

**Decision**: Extend existing `test/e2e/` patterns with minimal refactoring

**Rationale**:
- `config.go` already supports `AWSRegion` parameter
- `validator.go` provides cost validation primitives
- `pricing.go` has pricing data validation helpers
- New test files follow established naming: `multi_region_*_test.go`

**Alternatives Considered**:
- **New test package**: Rejected because it fragments E2E infrastructure
- **Refactor existing tests**: Rejected as out of scope; focus on additive changes
- **Separate test runner**: Rejected because `make test-e2e` should run all E2E tests

**Integration Points**:
- Reuse `Config` struct with region parameterization
- Extend `validator.go` with `ValidateMultiRegionCosts()` function
- Add `multi_region_helpers.go` for region-specific utilities
- New test files use `//go:build e2e` tag like existing tests

### 5. Resource Type Selection for Test Fixtures

**Decision**: EC2 (compute), EBS (storage), VPC/NAT Gateway (network), RDS (database)

**Rationale**:
- Covers 4 resource types as specified (compute, storage, network, database)
- All have well-documented regional pricing differences
- Supported by finfocus-plugin-aws-public
- 5-10 instances total keeps execution under 5-minute target

**Alternatives Considered**:
- **Lambda, S3, CloudFront**: Rejected because some have complex pricing models that complicate variance validation
- **10+ resource types**: Rejected due to execution time and maintenance overhead
- **Fewer types**: Rejected because it wouldn't adequately test pricing variations

**Resource Distribution per Region**:
- 2x EC2 instances (different instance types)
- 2x EBS volumes (gp3, io2)
- 2x Network resources (NAT Gateway, VPC Endpoint)
- 2x RDS instances (different engine types)
- Total: 8 resources per region × 3 regions = 24 total test resources

### 6. Dependency Coordination Strategy

**Decision**: Documented prerequisites with blocking checks in test setup

**Rationale**:
- #177 (Pulumi Automation API) must complete before actual cost tests can run
- #24 (GetActualCost fallback) required for multi-region actual cost validation
- Test setup checks for plugin availability before execution

**Alternatives Considered**:
- **Skip tests if dependencies missing**: Rejected because it masks incomplete implementation
- **Mock dependencies**: Rejected because E2E tests must use real implementations
- **Parallel development**: Rejected due to integration risk

**Implementation Pattern**:
```go
func TestMain(m *testing.M) {
    // Check #177 dependency
    if !checkPulumiAutomationAPISupport() {
        log.Fatal("Pulumi Automation API support required (issue #177)")
    }

    // Check #24 dependency
    if !checkGetActualCostSupport() {
        log.Fatal("GetActualCost fallback required (issue #24)")
    }

    os.Exit(m.Run())
}
```

## Best Practices Applied

### Go Testing Best Practices
- Use `testify/require` for setup assertions (fail fast)
- Use `testify/assert` for validation checks (report all failures)
- Table-driven tests for multi-region scenarios
- Parallel test execution where possible (`t.Parallel()`)
- Clear test naming: `TestMultiRegion_<CostType>_<Scenario>`

### E2E Test Patterns
- Build tag isolation: `//go:build e2e`
- External binary execution (test actual CLI behavior)
- Fixture-based testing (Pulumi programs as fixtures)
- Environment-based configuration (`FINFOCUS_E2E_*` variables)
- Cleanup in `defer` statements

### Cost Validation Patterns
- Tolerance-based comparisons for financial data
- Explicit expected values per region
- Currency validation (all costs in USD)
- Aggregation validation across resources

## Technology Stack Summary

- **Go** (1.25.7): Test implementation language
- **testify** (v1.11.1): Assertion and require helpers
- **Pulumi SDK** (v3.210.0+): Automation API for resource deployment
- **AWS SDK Go v2** (latest): (Indirect) Plugin uses for actual costs
- **FinFocus CLI** (local build): Binary under test (`bin/finfocus`)

## Open Questions & Resolutions

**Q**: How to handle region-specific plugin binary paths?
**A**: Plugin host (`internal/pluginhost`) already handles version selection; region-specific plugins are differentversions installed via `plugin install` command.

**Q**: Should tests deploy real AWS resources?
**A**: Yes for actual cost tests (requires #177); projected cost tests use static JSON plans initially, then migrate to deployed resources post-#177.

**Q**: How to prevent test flakiness from pricing updates?
**A**: ±5% tolerance handles minor updates; `expected-costs.json` requires manual update for major pricing changes (acceptable for E2E tests).

**Q**: What occurs when region configuration is invalid? (Edge case from spec)
**A**: Test framework validates region parameter in setup; invalid regions fail immediately with clear error message before resource deployment.

## References

- Existing E2E test infrastructure: `test/e2e/`
- FinFocus CLAUDE.md: Testing section
- Pulumi Automation API docs: https://www.pulumi.com/docs/guides/automation-api/
- AWS regional pricing docs: https://aws.amazon.com/ec2/pricing/on-demand/
- testify documentation: https://github.com/stretchr/testify

---

## User Story 4: Unified Multi-Region Fixture Research

**Added**: 2026-01-22
**Status**: In Progress (Discovery phase)

### 7. Unified Multi-Region Fixture Design

**Decision**: Pulumi YAML runtime with explicit provider aliases per region

**Rationale**:

- Real-world Pulumi programs often deploy to multiple regions simultaneously
- YAML runtime requires no Go code - simpler to maintain than Go fixtures
- Explicit provider aliases (`options.provider`) clearly map resources to regions
- Tests FinFocus's ability to detect regions from plan JSON provider configuration

**Alternatives Considered**:

- **Go runtime with multiple providers**: Rejected due to increased complexity; YAML is sufficient for test fixtures
- **TypeScript/Python runtime**: Rejected because Go/YAML are already used in existing fixtures
- **Separate stacks per region**: Rejected because that's what US1-3 already test; US4 specifically tests unified deployments

**Implementation Approach**:

```yaml
# Pulumi.yaml with YAML runtime
name: multi-region-test-unified
runtime: yaml

resources:
  # Explicit provider per region
  aws-us-east-1:
    type: pulumi:providers:aws
    properties:
      region: us-east-1

  # Resources reference specific providers
  web-us-east-1:
    type: aws:ec2:Instance
    options:
      provider: ${aws-us-east-1}
```

### 8. Plugin Behavior with Unified Fixtures

**Status**: NEEDS CLARIFICATION (Task T035)

**Question**: How does `aws-public` plugin handle multi-region plans?

**Options**:

- **Option A**: Single `aws-public` instance with autodetection
  - Pros: Simpler config, dynamic
  - Cons: Requires region extraction logic
- **Option B**: Per-region plugin declarations
  - Pros: Explicit, predictable
  - Cons: More config, known regions only

**Discovery Approach**:

1. Create minimal unified YAML fixture with resources in 2+ regions
2. Run `pulumi preview --json > plan.json`
3. Execute `finfocus cost projected --pulumi-json plan.json --debug`
4. Observe:
   - How many plugin processes spawn?
   - Are regions correctly extracted from plan JSON?
   - What errors occur if autodetection fails?

**Expected Findings**:

- `aws-public` likely uses region from resource properties or provider configuration
- Plan JSON includes provider URN that can be traced to region
- Ingestion layer (`internal/ingest/pulumi_plan.go`) may need enhancement for provider resolution

### 9. Region Extraction from Plan JSON

**Decision**: Extract region from provider configuration in plan JSON steps

**Rationale**:

- Plan JSON structure includes provider resources with `properties.region`
- Resources reference providers via `provider` field (URN format)
- Region can be resolved by: Resource → Provider URN → Provider Properties → Region

**Plan JSON Structure Example**:

```json
{
  "steps": [
    {
      "op": "create",
      "urn": "urn:pulumi:dev::project::pulumi:providers:aws::aws-us-east-1",
      "newState": {
        "type": "pulumi:providers:aws",
        "inputs": {
          "region": "us-east-1"
        }
      }
    },
    {
      "op": "create",
      "urn": "urn:pulumi:dev::project::aws:ec2/instance:Instance::web-us-east-1",
      "newState": {
        "type": "aws:ec2/instance:Instance",
        "provider": "urn:pulumi:dev::project::pulumi:providers:aws::aws-us-east-1",
        "inputs": {
          "ami": "ami-xxx",
          "instanceType": "t3.micro"
        }
      }
    }
  ]
}
```

**Region Resolution Algorithm**:

```go
func ResolveResourceRegion(step PlanStep, providerMap map[string]string) string {
    // 1. Check resource inputs for explicit region
    if region, ok := step.NewState.Inputs["region"].(string); ok {
        return region
    }

    // 2. Check resource properties for availabilityZone
    if az, ok := step.NewState.Inputs["availabilityZone"].(string); ok {
        return extractRegionFromAZ(az) // us-east-1a -> us-east-1
    }

    // 3. Resolve from provider
    if providerURN := step.NewState.Provider; providerURN != "" {
        if region, ok := providerMap[providerURN]; ok {
            return region
        }
    }

    return "" // Unknown region - plugin will use default
}
```

### 10. Cost Aggregation for Unified Fixtures

**Decision**: Per-resource validation + aggregate total validation

**Rationale**:

- Individual resource costs must match regional pricing expectations
- Aggregate total validates overall accuracy within ±5% tolerance
- Both levels catch different error classes

**Validation Structure**:

```json
{
  "fixture_type": "unified",
  "resources": [
    {"resource_name": "web-us-east-1", "region": "us-east-1", "min_cost": 6.66, "max_cost": 7.36},
    {"resource_name": "web-eu-west-1", "region": "eu-west-1", "min_cost": 7.13, "max_cost": 8.47},
    {"resource_name": "web-ap-northeast-1", "region": "ap-northeast-1", "min_cost": 7.33, "max_cost": 8.83}
  ],
  "aggregate_validation": {
    "total_min_cost": 21.12,
    "total_max_cost": 24.66
  }
}
```

**Validation Logic**:

1. For each resource: `actual_cost >= min_cost && actual_cost <= max_cost`
2. For aggregate: `sum(actual_costs) >= total_min && sum(actual_costs) <= total_max`
3. Test passes only if both levels pass

### Open Questions for User Story 4

- **Plugin spawning behavior**: Status: Pending T028. Resolution: Discovery testing with decision tree
- **Region extraction from plan JSON**: Status: Partially understood. Resolution: Verify in ingestion code
- **YAML runtime AMI handling**: Status: Resolved. Resolution: Use placeholder AMI IDs (e.g., `ami-placeholder-us-east-1`); E2E tests run `pulumi preview` only, not actual deployment, so AMI validity is not required

### Best Practices Applied for User Story 4

- **YAML Runtime Testing**: First use of YAML runtime in test fixtures; validates runtime-agnostic cost calculation
- **Provider Alias Pattern**: Standard Pulumi pattern for multi-region deployments
- **Aggregate Validation**: Financial data requires both micro and macro validation levels
- **Discovery-First**: Task T035 runs before implementation to resolve unknowns
