# Data Model: Multi-Region E2E Testing

**Date**: 2026-01-21
**Phase**: 1 (Design & Contracts)
**Status**: Complete

## Overview

This document defines the data structures, validation rules, and state transitions for multi-region E2E testing infrastructure. Since this is a test infrastructure feature, the "data model" primarily consists of test configuration structures, expected cost definitions, and test result representations.

## Core Entities

### 1. RegionTestConfig

Represents configuration for a single region's E2E test execution.

**Fields**:

- `Region` (`string`): AWS region identifier. Must be valid AWS region code (us-east-1, eu-west-1, ap-northeast-1)
- `FixturePath` (`string`): Path to Pulumi program fixture directory. Must exist on filesystem
- `ExpectedCostsPath` (`string`): Path to expected-costs.json file. Must exist and be valid JSON
- `Tolerance` (`float64`): Cost variance tolerance (default: 0.05 for ±5%). Must be > 0 and ≤ 1.0
- `Timeout` (`time.Duration`): Maximum test execution time (default: 5 minutes). Must be > 0
- `PluginVersion` (`string`): Expected AWS plugin version (e.g., "0.1.0"). Semantic version format

**Relationships**:
- One-to-many with `ExpectedCost` (multiple expected costs per region)
- One-to-one with Pulumi program fixture directory

**Validation Rules**:
```go
func (r *RegionTestConfig) Validate() error {
    validRegions := []string{"us-east-1", "eu-west-1", "ap-northeast-1"}
    if !contains(validRegions, r.Region) {
        return fmt.Errorf("invalid region: %s (must be one of %v)", r.Region, validRegions)
    }

    if !fileExists(r.FixturePath) {
        return fmt.Errorf("fixture path does not exist: %s", r.FixturePath)
    }

    if r.Tolerance <= 0 || r.Tolerance > 1.0 {
        return fmt.Errorf("tolerance must be > 0 and ≤ 1.0, got: %f", r.Tolerance)
    }

    if r.Timeout <= 0 {
        return fmt.Errorf("timeout must be positive, got: %v", r.Timeout)
    }

    return nil
}
```

---

### 2. ExpectedCost

Defines expected cost range for a specific resource type in a region.

**Fields**:

- `ResourceType` (`string`): Resource type identifier (e.g., "aws:ec2:Instance"). Must match Pulumi resource type format (provider:module:type)
- `ResourceName` (`string`): Logical name of resource in Pulumi program. Non-empty
- `MinCost` (`float64`): Minimum acceptable monthly cost (USD). Must be ≥ 0
- `MaxCost` (`float64`): Maximum acceptable monthly cost (USD). Must be > MinCost
- `Region` (`string`): Region this expectation applies to. Must be valid AWS region code
- `CostType` (`CostType`): Type of cost calculation. Must be "projected" or "actual"

**Relationships**:

- Many-to-one with `RegionTestConfig` (grouped by region)
- One-to-one with specific resource in Pulumi program

**Validation Rules**:
```go
func (e *ExpectedCost) Validate() error {
    if e.ResourceType == "" {
        return fmt.Errorf("resource type is required")
    }

    if e.ResourceName == "" {
        return fmt.Errorf("resource name is required")
    }

    if e.MinCost < 0 {
        return fmt.Errorf("min cost must be non-negative, got: %f", e.MinCost)
    }

    if e.MaxCost <= e.MinCost {
        return fmt.Errorf("max cost (%f) must be greater than min cost (%f)", e.MaxCost, e.MinCost)
    }

    if e.CostType != CostTypeProjected && e.CostType != CostTypeActual {
        return fmt.Errorf("invalid cost type: %s", e.CostType)
    }

    return nil
}
```

**JSON Schema**:
```json
{
  "type": "object",
  "required": ["resource_type", "resource_name", "min_cost", "max_cost", "region", "cost_type"],
  "properties": {
    "resource_type": {
      "type": "string",
      "pattern": "^[a-z]+:[a-z0-9]+:[A-Za-z0-9]+"
    },
    "resource_name": {
      "type": "string",
      "minLength": 1
    },
    "min_cost": {
      "type": "number",
      "minimum": 0
    },
    "max_cost": {
      "type": "number",
      "exclusiveMinimum": 0
    },
    "region": {
      "type": "string",
      "enum": ["us-east-1", "eu-west-1", "ap-northeast-1"]
    },
    "cost_type": {
      "type": "string",
      "enum": ["projected", "actual"]
    }
  }
}
```

---

### 3. CostValidationResult

Captures the result of cost validation for a single resource.

**Fields**:

- `ResourceName` (`string`): Resource identifier. Non-empty
- `ResourceType` (`string`): Pulumi resource type. Non-empty
- `Region` (`string`): AWS region. Valid AWS region code
- `ActualCost` (`float64`): Actual calculated cost (USD). Must be ≥ 0
- `ExpectedMin` (`float64`): Minimum expected cost (USD). Must be ≥ 0
- `ExpectedMax` (`float64`): Maximum expected cost (USD). Must be > ExpectedMin
- `WithinTolerance` (`bool`): Whether actual cost is within expected range
- `Variance` (`float64`): Percentage variance from expected midpoint
- `Timestamp` (`time.Time`): When validation was performed
- `CostType` (`CostType`): Type of cost validated. Must be "projected" or "actual"

**Relationships**:

- Many-to-one with `MultiRegionTestResult` (aggregated test results)
- One-to-one with `ExpectedCost` (validation source)

**State Transitions**:
```text
Initial State: [Validation Pending]
    ↓
[Execute Cost Calculation]
    ↓
[Compare with Expected Range]
    ↓
Decision Point: Within Tolerance?
    ├─ YES → [Validation Passed] (WithinTolerance = true)
    └─ NO  → [Validation Failed] (WithinTolerance = false)
```

---

### 4. MultiRegionTestResult

Aggregates validation results across all regions and cost types.

**Fields**:

- `Regions` (`[]string`): List of tested regions. Must contain 1-3 regions
- `Results` (`[]CostValidationResult`): Individual validation results. Non-empty
- `TotalResources` (`int`): Total number of resources validated. Must match len(Results)
- `PassedValidations` (`int`): Number of resources within tolerance. Must be ≤ TotalResources
- `FailedValidations` (`int`): Number of resources outside tolerance. Must equal TotalResources - PassedValidations
- `ExecutionTime` (`time.Duration`): Total test execution time. Must be > 0
- `TestTimestamp` (`time.Time`): When test suite started
- `Success` (`bool`): True if all validations passed

**Relationships**:

- One-to-many with `CostValidationResult` (contains all validation results)
- One-to-many with `RegionTestConfig` (tested multiple regions)

**Validation Rules**:
```go
func (m *MultiRegionTestResult) Validate() error {
    if len(m.Results) == 0 {
        return fmt.Errorf("results cannot be empty")
    }

    if m.TotalResources != len(m.Results) {
        return fmt.Errorf("total resources (%d) must match results length (%d)",
            m.TotalResources, len(m.Results))
    }

    if m.PassedValidations + m.FailedValidations != m.TotalResources {
        return fmt.Errorf("passed (%d) + failed (%d) must equal total (%d)",
            m.PassedValidations, m.FailedValidations, m.TotalResources)
    }

    m.Success = (m.FailedValidations == 0)
    return nil
}
```

---

### 5. CostType

Enumeration for cost calculation types.

**Values**:
- `CostTypeProjected`: Projected costs from Pulumi preview
- `CostTypeActual`: Actual costs from deployed resources

**Validation**:
```go
type CostType string

const (
    CostTypeProjected CostType = "projected"
    CostTypeActual    CostType = "actual"
)

func (c CostType) IsValid() bool {
    return c == CostTypeProjected || c == CostTypeActual
}

func (c CostType) String() string {
    return string(c)
}
```

---

## Data Flow

```text
1. Test Setup:
   RegionTestConfig → Load ExpectedCost from JSON

2. Test Execution:
   RegionTestConfig → Deploy/Preview Pulumi → Calculate Costs

3. Validation:
   Actual Costs + Expected Costs → CostValidationResult

4. Aggregation:
   Multiple CostValidationResult → MultiRegionTestResult

5. Assertion:
   MultiRegionTestResult.Success → Test Pass/Fail
```

## File Formats

### expected-costs.json

```json
{
  "region": "us-east-1",
  "cost_type": "projected",
  "resources": [
    {
      "resource_type": "aws:ec2:Instance",
      "resource_name": "web-server",
      "min_cost": 50.0,
      "max_cost": 55.0,
      "region": "us-east-1",
      "cost_type": "projected"
    },
    {
      "resource_type": "aws:ebs:Volume",
      "resource_name": "data-volume",
      "min_cost": 8.0,
      "max_cost": 10.0,
      "region": "us-east-1",
      "cost_type": "projected"
    }
  ]
}
```

### Pulumi.yaml (fixture configuration - YAML runtime)

```yaml
name: multi-region-test-us-east-1
runtime: yaml
description: Multi-region E2E test fixture for us-east-1

config:
  aws:region: us-east-1

resources:
  # EC2 Instances
  web-server:
    type: aws:ec2:Instance
    properties:
      ami: ami-0123456789abcdef0
      instanceType: t3.micro
      tags:
        Environment: e2e-test
        ManagedBy: finfocus
        TestSuite: multi-region

  app-server:
    type: aws:ec2:Instance
    properties:
      ami: ami-0123456789abcdef0
      instanceType: t3.small
      tags:
        Environment: e2e-test
```

## Validation Matrix

- **Region code**: Scope: RegionTestConfig, Trigger: Test setup, Failure: Fail immediately
- **Fixture path exists**: Scope: RegionTestConfig, Trigger: Test setup, Failure: Fail immediately
- **Expected costs JSON valid**: Scope: ExpectedCost, Trigger: Load time, Failure: Fail immediately
- **Cost tolerance range**: Scope: CostValidationResult, Trigger: Comparison, Failure: Record failure in result
- **Plugin availability**: Scope: TestMain, Trigger: Before tests, Failure: Skip/Fail entire suite
- **Execution timeout**: Scope: Per-region test, Trigger: Timer, Failure: Fail test with timeout error
- **Network retry exhausted**: Scope: Plugin communication, Trigger: After 3 retries, Failure: Fail test with network error

## Error Handling

### Missing Pricing Data
- **Detection**: Plugin returns empty cost result
- **Action**: Fail test immediately with message: "No pricing data available for {region}/{resource_type}"
- **Rationale**: E2E tests require complete data (clarification Q4)

### Network Failure
- **Detection**: Plugin gRPC call fails with network error
- **Action**: Retry 3 times with exponential backoff (100ms, 200ms, 400ms)
- **Final Action**: Fail test with message: "Network failure after 3 retries: {error}"
- **Rationale**: Distinguish transient from persistent issues (clarification Q5)

### Invalid Region Configuration
- **Detection**: Region not in allowed list
- **Action**: Fail test in setup with message: "Invalid region: {region} (expected one of: us-east-1, eu-west-1, ap-northeast-1)"
- **Rationale**: Fail fast on configuration errors (edge case resolution)

## Concurrency Considerations

- Region tests can run in parallel (`t.Parallel()`)
- Projected and actual cost tests can run concurrently
- Shared fixtures are read-only (safe for parallel access)
- Plugin communication is stateless (safe for concurrent calls)

---

## User Story 4: Unified Multi-Region Fixtures

**Added**: 2026-01-22

### 6. UnifiedExpectedCosts

Defines expected cost structure for a unified multi-region fixture.

**Fields**:

- `FixtureType` (`string`): Fixture classification. Must be "unified"
- `TotalRegions` (`int`): Number of regions in fixture. Must be > 0 and ≤ 10
- `Resources` (`[]ExpectedCost`): Per-resource expected costs. Non-empty
- `AggregateValidation` (`AggregateExpectation`): Total cost expectations. Required

**Relationships**:

- One-to-many with `ExpectedCost` (contains expectations for all regions)
- One-to-one with unified Pulumi.yaml fixture

**JSON Schema**:
```json
{
  "type": "object",
  "required": ["fixture_type", "total_regions", "resources", "aggregate_validation"],
  "properties": {
    "fixture_type": {
      "type": "string",
      "const": "unified"
    },
    "total_regions": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10
    },
    "resources": {
      "type": "array",
      "minItems": 1,
      "items": { "$ref": "#/definitions/ExpectedCost" }
    },
    "aggregate_validation": {
      "$ref": "#/definitions/AggregateExpectation"
    }
  }
}
```

---

### 7. AggregateExpectation

Defines aggregate cost validation for unified fixtures.

**Fields**:

- `TotalMinCost` (`float64`): Minimum acceptable total cost (USD). Must be ≥ 0
- `TotalMaxCost` (`float64`): Maximum acceptable total cost (USD). Must be > TotalMinCost
- `Tolerance` (`float64`): Cost variance tolerance. Must be > 0 and ≤ 1.0

**Validation Rules**:
```go
func (a *AggregateExpectation) Validate() error {
    if a.TotalMinCost < 0 {
        return fmt.Errorf("total min cost must be non-negative, got: %f", a.TotalMinCost)
    }

    if a.TotalMaxCost <= a.TotalMinCost {
        return fmt.Errorf("total max cost (%f) must be greater than total min cost (%f)",
            a.TotalMaxCost, a.TotalMinCost)
    }

    if a.Tolerance <= 0 || a.Tolerance > 1.0 {
        return fmt.Errorf("tolerance must be > 0 and ≤ 1.0, got: %f", a.Tolerance)
    }

    return nil
}
```

---

### 8. UnifiedValidationResult

Captures validation results for a unified multi-region fixture.

**Fields**:

- `PerResourceResults` (`[]CostValidationResult`): Individual resource validations. Non-empty
- `TotalCost` (`float64`): Actual total cost (USD). Must be ≥ 0
- `TotalWithinTolerance` (`bool`): Whether total is within aggregate bounds
- `RegionBreakdown` (`map[string]float64`): Cost per region for analysis
- `Success` (`bool`): True if all validations passed

**Relationships**:

- One-to-many with `CostValidationResult` (contains per-resource results)
- One-to-one with `UnifiedExpectedCosts` (validation source)

**Validation Rules**:
```go
func (u *UnifiedValidationResult) Validate() error {
    if len(u.PerResourceResults) == 0 {
        return fmt.Errorf("per-resource results cannot be empty")
    }

    // All per-resource validations must pass
    for _, r := range u.PerResourceResults {
        if !r.WithinTolerance {
            u.Success = false
            return nil
        }
    }

    // Aggregate validation must also pass
    u.Success = u.TotalWithinTolerance
    return nil
}
```

---

### 9. ProviderRegionMapping

Maps provider URNs to regions for unified fixture processing.

**Fields**:

- `ProviderURN` (`string`): Pulumi provider URN. Must match URN format
- `Region` (`string`): AWS region for provider. Valid AWS region code
- `ProviderType` (`string`): Provider type identifier. Must be "pulumi:providers:aws"

**Purpose**:
- Built during plan JSON parsing
- Used to resolve resource regions from provider references
- Enables region-agnostic resource definitions with explicit provider assignment

**Example**:
```go
providerMap := map[string]string{
    "urn:pulumi:dev::project::pulumi:providers:aws::aws-us-east-1": "us-east-1",
    "urn:pulumi:dev::project::pulumi:providers:aws::aws-eu-west-1": "eu-west-1",
    "urn:pulumi:dev::project::pulumi:providers:aws::aws-ap-northeast-1": "ap-northeast-1",
}
```

---

## Unified Fixture Data Flow

```text
1. Fixture Load:
   UnifiedExpectedCosts → Parse from expected-costs.json

2. Plan Generation:
   Pulumi.yaml → pulumi preview --json → plan.json

3. Provider Mapping:
   plan.json → Extract provider resources → ProviderRegionMapping

4. Cost Calculation:
   finfocus cost projected → Plugin per region → Cost Results

5. Validation:
   Cost Results + UnifiedExpectedCosts → UnifiedValidationResult

6. Assertion:
   UnifiedValidationResult.Success → Test Pass/Fail
```

## File Formats (Unified Fixtures)

### expected-costs.json (Unified)

```json
{
  "fixture_type": "unified",
  "total_regions": 3,
  "resources": [
    {
      "resource_type": "aws:ec2:Instance",
      "resource_name": "web-us-east-1",
      "min_cost": 6.66,
      "max_cost": 7.36,
      "region": "us-east-1",
      "cost_type": "projected"
    },
    {
      "resource_type": "aws:ec2:Instance",
      "resource_name": "web-eu-west-1",
      "min_cost": 7.13,
      "max_cost": 8.47,
      "region": "eu-west-1",
      "cost_type": "projected"
    },
    {
      "resource_type": "aws:ec2:Instance",
      "resource_name": "web-ap-northeast-1",
      "min_cost": 7.33,
      "max_cost": 8.83,
      "region": "ap-northeast-1",
      "cost_type": "projected"
    }
  ],
  "aggregate_validation": {
    "total_min_cost": 21.12,
    "total_max_cost": 24.66,
    "tolerance": 0.05
  }
}
```

### Pulumi.yaml (Unified YAML Runtime)

```yaml
name: multi-region-test-unified
runtime: yaml
description: Unified multi-region E2E test fixture

resources:
  aws-us-east-1:
    type: pulumi:providers:aws
    properties:
      region: us-east-1

  aws-eu-west-1:
    type: pulumi:providers:aws
    properties:
      region: eu-west-1

  aws-ap-northeast-1:
    type: pulumi:providers:aws
    properties:
      region: ap-northeast-1

  web-us-east-1:
    type: aws:ec2:Instance
    properties:
      ami: ami-0123456789abcdef0
      instanceType: t3.micro
    options:
      provider: ${aws-us-east-1}

  web-eu-west-1:
    type: aws:ec2:Instance
    properties:
      ami: ami-0abcdef123456789a
      instanceType: t3.micro
    options:
      provider: ${aws-eu-west-1}

  web-ap-northeast-1:
    type: aws:ec2:Instance
    properties:
      ami: ami-0fedcba987654321f
      instanceType: t3.micro
    options:
      provider: ${aws-ap-northeast-1}

outputs:
  regions:
    - us-east-1
    - eu-west-1
    - ap-northeast-1
```
