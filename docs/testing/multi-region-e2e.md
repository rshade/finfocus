---
title: Multi-Region E2E Testing Guide
description: Comprehensive guide for running and maintaining multi-region E2E tests that validate cost calculations across AWS regions
layout: default
---

## Overview

This guide provides comprehensive documentation for running and maintaining
multi-region E2E tests in FinFocus. These tests validate cost calculations
across different AWS regions (us-east-1, eu-west-1, ap-northeast-1).
They ensure regional pricing differences are correctly reflected in both
projected and actual cost calculations.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Test Architecture](#test-architecture)
3. [Configuration](#configuration)
4. [Test Fixtures](#test-fixtures)
5. [Running Tests](#running-tests)
6. [Validation Strategy](#validation-strategy)
7. [Failure Handling](#failure-handling)
8. [Troubleshooting](#troubleshooting)
9. [Extending Tests](#extending-tests)
10. [CI/CD Integration](#cicd-integration)

## Quick Start

```bash
# Build the FinFocus binary
make build

# Run multi-region tests (default: us-east-1 only)
make test-e2e

# Test all three regions
export FINFOCUS_E2E_REGIONS="us-east-1,eu-west-1,ap-northeast-1"
go test -v -tags e2e ./test/e2e -run TestMultiRegion
```

**Expected Duration**: ~5 minutes per region (projected + actual costs)

## Test Architecture

### Components

Multi-region E2E tests consist of several key components:

1. **Test Fixtures** (`test/e2e/fixtures/multi-region/`)
   - Pulumi programs for each region
   - Expected cost data with tolerance ranges
   - Region-specific configurations

2. **Test Implementations** (`test/e2e/multi_region_*_test.go`)
   - `multi_region_projected_test.go`: Projected cost validation
   - `multi_region_actual_test.go`: Actual cost validation with deployed resources
   - `multi_region_fallback_test.go`: Plugin fallback and error handling

3. **Helper Functions** (`test/e2e/multi_region_helpers.go`)
   - Configuration management
   - Cost validation logic
   - Retry mechanisms
   - Plugin version tracking

### Test Flow

```text
┌─────────────────────────────────────────────────────────────┐
│ 1. Setup & Configuration                                     │
│    - Load RegionTestConfig                                   │
│    - Validate region, tolerance, timeout                     │
│    - Log plugin versions                                     │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│ 2. Fixture Preparation                                       │
│    - Load expected costs from JSON                           │
│    - Generate/use Pulumi plan                                │
│    - Validate fixture structure                              │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│ 3. Cost Calculation                                          │
│    - Projected: Run finfocus cost projected                  │
│    - Actual: Deploy resources + Run finfocus cost actual     │
│    - Parse JSON output                                       │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│ 4. Validation                                                │
│    - Compare actual vs expected costs                        │
│    - Check tolerance (±5% default)                           │
│    - Validate plugin loading                                 │
│    - Record results                                          │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│ 5. Assertion & Reporting                                     │
│    - Assert all validations passed                           │
│    - Log execution time                                      │
│    - Report failures with context                            │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Environment Variables

| Variable                             | Default     | Description                             | Example                              |
| ------------------------------------ | ----------- | --------------------------------------- | ------------------------------------ |
| `FINFOCUS_E2E_REGIONS`               | `us-east-1` | Comma-separated list of regions to test | `us-east-1,eu-west-1,ap-northeast-1` |
| `FINFOCUS_E2E_TOLERANCE`             | `0.05`      | Cost variance tolerance (±5%)           | `0.10` for ±10%                      |
| `FINFOCUS_E2E_TIMEOUT`               | `10m`       | Maximum test execution time per region  | `15m`                                |
| `FINFOCUS_E2E_SKIP_ACTUAL_COSTS`     | `false`     | Skip actual cost tests                  | `true`                               |
| `FINFOCUS_E2E_SKIP_DEPENDENCY_CHECK` | `false`     | Skip dependency validation              | `true`                               |
| `AWS_PROFILE`                        | (none)      | AWS credentials profile to use          | `finfocus-testing`                   |
| `AWS_REGION`                         | (none)      | Default AWS region for credentials      | `us-east-1`                          |

### RegionTestConfig Structure

```go
type RegionTestConfig struct {
    Region            string        // AWS region code
    FixturePath       string        // Path to Pulumi program
    ExpectedCostsPath string        // Path to expected-costs.json
    Tolerance         float64       // Cost variance tolerance
    Timeout           time.Duration // Maximum execution time
    PluginVersion     string        // Expected plugin version
}
```

## Test Fixtures

### Directory Structure

```text
test/e2e/fixtures/multi-region/
├── us-east-1/
│   ├── Pulumi.yaml          # YAML runtime with inline resources (8 resources)
│   └── expected-costs.json  # Expected cost ranges
├── eu-west-1/
│   ├── Pulumi.yaml          # YAML runtime (region: eu-west-1)
│   └── expected-costs.json
├── ap-northeast-1/
│   ├── Pulumi.yaml          # YAML runtime (region: ap-northeast-1)
│   └── expected-costs.json
└── unified/                  # User Story 4 - Multi-region in single fixture
    ├── Pulumi.yaml          # YAML runtime with explicit provider aliases
    └── expected-costs.json  # Per-resource + aggregate cost expectations
```

### Resource Distribution

Each region fixture includes:

- **2x EC2 Instances** (t3.micro, t3.small) - Compute
  - Different instance types to test pricing variations
- **2x EBS Volumes** (gp3 100GB, io2 50GB) - Storage
  - Different volume types and IOPS configurations
- **2x Network Resources** (NAT Gateway, VPC Endpoint) - Network
  - Regional network pricing differences
- **2x RDS Instances** (db.t3.micro PostgreSQL, MySQL) - Database
  - Database engine pricing variations

**Total**: 8 resources per region × 3 regions = 24 resources validated

### Expected Costs Format

`expected-costs.json` structure:

```json
{
  "region": "us-east-1",
  "cost_type": "projected",
  "resources": [
    {
      "resource_type": "aws:ec2:Instance",
      "resource_name": "web-server-t3-micro",
      "min_cost": 6.5,
      "max_cost": 7.5,
      "region": "us-east-1",
      "cost_type": "projected"
    },
    ...
  ]
}
```

## Running Tests

### Basic Usage

```bash
# Run all E2E tests (includes multi-region)
make test-e2e

# Run only multi-region tests
go test -v -tags e2e ./test/e2e -run TestMultiRegion

# Run specific region test
go test -v -tags e2e ./test/e2e -run TestMultiRegion_Projected_USEast1
```

### Advanced Usage

```bash
# Test all three regions with custom tolerance
export FINFOCUS_E2E_REGIONS="us-east-1,eu-west-1,ap-northeast-1"
export FINFOCUS_E2E_TOLERANCE=0.10
go test -v -tags e2e ./test/e2e -run TestMultiRegion

# Skip actual cost tests (for development)
export FINFOCUS_E2E_SKIP_ACTUAL_COSTS=true
go test -v -tags e2e ./test/e2e -run TestMultiRegion_Projected

# Run with increased timeout
export FINFOCUS_E2E_TIMEOUT=15m
go test -timeout 20m -v -tags e2e ./test/e2e -run TestMultiRegion

# Run specific test suite
go test -v -tags e2e ./test/e2e -run TestMultiRegion_Fallback
```

### Parallel Execution

Tests are designed to run in parallel for faster execution:

```bash
# Run with parallelism (default behavior)
go test -v -tags e2e -parallel 3 ./test/e2e -run TestMultiRegion_Projected
```

## Validation Strategy

### Tolerance-Based Validation

Multi-region tests use **tolerance-based validation** to handle pricing variability:

1. **Expected Cost Range**: Each resource has `min_cost` and `max_cost` values
2. **Default Tolerance**: ±5% to accommodate minor pricing fluctuations
3. **Validation Logic**: Actual cost must fall within [min_cost, max_cost]
4. **Variance Calculation**: Percentage difference from midpoint

```go
midpoint := (expected.MinCost + expected.MaxCost) / 2
withinTolerance := actual.MonthlyCost >= expected.MinCost &&
                   actual.MonthlyCost <= expected.MaxCost
variance := (actual.MonthlyCost - midpoint) / midpoint * 100
```

### Plugin Validation

Tests validate plugin loading and versioning:

1. **Plugin Availability**: Verify AWS plugin is loaded
2. **Version Check**: Validate plugin version matches expected
3. **Binary Path**: Confirm plugin binary exists at reported path
4. **Region-Specific**: Check correct plugin is used for each region

## Failure Handling

### Error Classification

Multi-region tests implement strict failure semantics:

| Error Type                | Detection                                            | Action                   | Retry |
| ------------------------- | ---------------------------------------------------- | ------------------------ | ----- |
| **Missing Pricing Data**  | "no pricing data" in error                           | Fail immediately         | No    |
| **Network Failure**       | "connection refused", "timeout"                      | Retry 3x with backoff    | Yes   |
| **Invalid Region**        | Region not in [us-east-1, eu-west-1, ap-northeast-1] | Fail immediately         | No    |
| **Cost Out of Tolerance** | Actual cost outside expected range                   | Record failure, continue | No    |

### Retry Logic

Network failures use exponential backoff:

```go
attempt 1: 100ms delay
attempt 2: 200ms delay
attempt 3: 400ms delay
final: fail with error
```

### Fallback Behavior

When region-specific plugins are unavailable:

1. **Detection**: Plugin not found or fails to load
2. **Fallback**: Use public pricing data (issue #24)
3. **Warning**: Log fallback occurrence
4. **Validation**: Use looser tolerance (±10% instead of ±5%)

## Troubleshooting

### Common Issues

#### 1. Test Fails: "No pricing data available"

**Symptom**:

```text
FAIL: No pricing data available for eu-west-1/aws:ec2:Instance
```

**Cause**: Plugin doesn't have pricing data for the region

**Solution**:

```bash
# Verify plugin installation
./bin/finfocus plugin list

# Install region-specific plugin if missing
./bin/finfocus plugin install aws-public --version <version>
```

#### 2. Test Fails: "Network failure after 3 retries"

**Symptom**:

```text
FAIL: Network failure after 3 retries: connection refused
```

**Cause**: Plugin gRPC communication failed

**Solution**:

1. Check plugin process: `ps aux | grep finfocus-plugin`
2. Verify no firewall blocking localhost
3. Retry test (may be transient)
4. Check plugin logs: `~/.finfocus/logs/`

#### 3. Cost Variance Exceeds Tolerance

**Symptom**:

```text
FAIL: Resource web-server cost $58.50 outside expected range [$50.00, $55.00]
```

**Cause**: AWS pricing changed or resource configuration differs

**Solution**:

1. Verify pricing change on AWS pricing page
2. Update `expected-costs.json` with new ranges
3. Temporarily increase tolerance: `export FINFOCUS_E2E_TOLERANCE=0.10`

#### 4. Test Timeout

**Symptom**:

```text
panic: test timed out after 10m0s
```

**Cause**: Test execution exceeded timeout (likely actual cost tests)

**Solution**:

```bash
# Increase timeout
export FINFOCUS_E2E_TIMEOUT=15m
go test -timeout 20m -tags e2e ./test/e2e -run TestMultiRegion

# Or skip actual cost tests
export FINFOCUS_E2E_SKIP_ACTUAL_COSTS=true
```

#### 5. Missing Dependencies (Issue #177 or #24)

**Symptom**:

```text
FAIL: Pulumi Automation API support required (issue #177)
```

**Cause**: Test depends on incomplete feature

**Solution**:

1. Check issue status: `gh issue view 177`
2. Skip actual cost tests: `export FINFOCUS_E2E_SKIP_ACTUAL_COSTS=true`
3. Wait for dependency to merge

## Extending Tests

### Adding a New Region

1. **Create Fixture Directory**:

   ```bash
   mkdir -p test/e2e/fixtures/multi-region/us-west-2
   ```

2. **Create Pulumi YAML Program** (`Pulumi.yaml`):

   ```yaml
   name: multi-region-test-us-west-2
   runtime: yaml
   description: Multi-region E2E test fixture for us-west-2

   config:
     aws:region:
       value: us-west-2
     pulumi:tags:
       value:
         Environment: e2e-test
         ManagedBy: finfocus
         TestSuite: multi-region

   resources:
     # EC2 Instance example
     web-server-t3-micro:
       type: aws:ec2:Instance
       properties:
         ami: ami-placeholder-us-west-2
         instanceType: t3.micro
         tags:
           Name: web-server-t3-micro

     # Add additional resources as needed (2 EC2, 2 EBS, 2 Network, 2 RDS)
   ```

3. **Create Expected Costs** (`expected-costs.json`):

   ```json
   {
     "region": "us-west-2",
     "cost_type": "projected",
     "resources": [...]
   }
   ```

4. **Add Test Function** (in `multi_region_projected_test.go`):

   ```go
   func TestMultiRegion_Projected_USWest2(t *testing.T) {
       if shouldSkipRegion(t, "us-west-2") {
           t.Skip("Skipping us-west-2 test")
       }
       testMultiRegionProjected(t, "us-west-2")
   }
   ```

5. **Update Configuration Validation** (in `multi_region_helpers.go`):

   ```go
   validRegions := []string{"us-east-1", "eu-west-1", "ap-northeast-1", "us-west-2"}
   ```

### Adding New Resource Types

1. **Update Fixture** (`test/e2e/fixtures/multi-region/<region>/Pulumi.yaml`):

   ```yaml
   # Add new resource (e.g., Lambda function)
   test-function:
     type: aws:lambda:Function
     properties:
       runtime: nodejs18.x
       handler: index.handler
       role: ${test-role.arn}
       # ... other properties
   ```

2. **Update Expected Costs** (`expected-costs.json`):

   ```json
   {
     "resource_type": "aws:lambda:Function",
     "resource_name": "test-function",
     "min_cost": 0.2,
     "max_cost": 0.3,
     "region": "us-east-1",
     "cost_type": "projected"
   }
   ```

3. **Verify All Regions**: Add the same resource to all region fixtures

## Unified Multi-Region Fixtures (User Story 4)

### Overview

In addition to per-region fixtures, the unified fixture tests a **single Pulumi program**
that deploys resources across multiple regions - a common real-world pattern.

### Structure

```yaml
# test/e2e/fixtures/multi-region/unified/Pulumi.yaml
name: multi-region-test-unified
runtime: yaml

resources:
  # Explicit provider per region
  aws-us-east-1:
    type: pulumi:providers:aws
    properties:
      region: us-east-1

  aws-eu-west-1:
    type: pulumi:providers:aws
    properties:
      region: eu-west-1

  # Resources reference specific providers
  web-us-east-1:
    type: aws:ec2:Instance
    properties:
      ami: ami-placeholder
      instanceType: t3.micro
    options:
      provider: ${aws-us-east-1}

  web-eu-west-1:
    type: aws:ec2:Instance
    properties:
      ami: ami-placeholder
      instanceType: t3.micro
    options:
      provider: ${aws-eu-west-1}
```

### Validation

Unified fixtures validate:

1. **Per-resource costs** reflect region-specific pricing
2. **Aggregate total** equals sum of individual region costs within ±5% tolerance
3. **Region attribution** from provider configuration in plan JSON

### Running Unified Tests

```bash
go test -v -tags e2e ./test/e2e -run TestMultiRegion_Unified_Projected
```

### Troubleshooting Unified Fixtures

**Issue: "Region not detected for resource"**

- Ensure provider is explicitly declared with `region` property
- Verify resource uses `options.provider` to reference the provider
- Check plan JSON contains provider URN with region

**Issue: "Aggregate total outside tolerance"**

- Tolerance compounds: 3 resources at +5% each = ~+15% total
- Update `aggregate_validation` bounds in `expected-costs.json` if needed

## CI/CD Integration

### GitHub Actions

Multi-region tests run automatically via `make test-e2e`:

```yaml
- name: Run E2E Tests
  run: make test-e2e
  env:
    FINFOCUS_E2E_REGIONS: us-east-1
    FINFOCUS_E2E_SKIP_ACTUAL_COSTS: true
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
```

### Performance Considerations

- **Projected tests**: ~2-3 minutes per region
- **Actual tests**: ~4-5 minutes per region (includes deployment)
- **Parallel execution**: Reduces total time by ~50%
- **CI optimization**: Run us-east-1 only by default, full multi-region on schedule

### Failure Notification

Tests report failures with detailed context:

```text
Cost validation failed for web-server-t3-micro:
  Actual: $58.50
  Expected: [$50.00, $55.00]
  Variance: +15.00%
  Region: eu-west-1
  Cost Type: projected
```

## Best Practices

1. **Regular Updates**: Update `expected-costs.json` when AWS pricing changes
2. **Tolerance Adjustment**: Use ±5% for normal tests, ±10% for fallback scenarios
3. **Fixture Maintenance**: Keep fixtures synchronized across regions
4. **Dependency Tracking**: Monitor issues #177 and #24 for test prerequisites
5. **Log Review**: Check plugin version logs to verify correct plugin usage
6. **Performance Monitoring**: Watch for tests exceeding 5-minute target
7. **Error Classification**: Ensure errors are properly classified for correct retry behavior

## References

- [Feature Specification](../../specs/001-multi-region-e2e/spec.md)
- [Implementation Plan](../../specs/001-multi-region-e2e/plan.md)
- [Quickstart Guide](../../specs/001-multi-region-e2e/quickstart.md)
- [Test Fixtures](../../test/e2e/fixtures/multi-region/)
- [E2E Test README](../../test/e2e/README.md)
- [CLAUDE.md - Testing Section](../../CLAUDE.md#testing)
