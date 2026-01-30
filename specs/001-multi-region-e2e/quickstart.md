# Quickstart: Multi-Region E2E Testing

**Audience**: Developers implementing or running multi-region E2E tests
**Time to Complete**: 15-20 minutes
**Prerequisites**: Go 1.25.6, FinFocus built locally, AWS credentials configured

## Overview

This guide walks through running multi-region E2E tests to validate cost calculations across different AWS regions (us-east-1, eu-west-1, ap-northeast-1). Tests cover both projected and actual costs with ±5% variance tolerance.

## Quick Start

### 1. Build FinFocus Binary

```bash
cd /path/to/finfocus
make build
```

**Expected Output**:
```text
go build -o bin/finfocus ./cmd/finfocus
Build complete: bin/finfocus
```

**Verification**:
```bash
./bin/finfocus --version
# Should output: finfocus version 0.2.x
```

---

### 2. Run Multi-Region E2E Tests (Default: us-east-1 only)

```bash
# Run all multi-region tests
make test-e2e

# Or run specific multi-region test file
go test -v -tags e2e ./test/e2e -run TestMultiRegion
```

**Expected Output**:
```text
=== RUN   TestMultiRegion_Projected_USEast1
--- PASS: TestMultiRegion_Projected_USEast1 (45.2s)
=== RUN   TestMultiRegion_Actual_USEast1
--- PASS: TestMultiRegion_Actual_USEast1 (120.5s)
PASS
ok      github.com/rshade/finfocus/test/e2e     165.734s
```

---

### 3. Run Tests for All Regions

```bash
# Set environment variable to test all three regions
export FINFOCUS_E2E_REGIONS="us-east-1,eu-west-1,ap-northeast-1"
go test -v -tags e2e ./test/e2e -run TestMultiRegion
```

**Expected Duration**: ~15 minutes total (<5 minutes per region)

**Expected Output**:
```text
=== RUN   TestMultiRegion_Projected_USEast1
--- PASS: TestMultiRegion_Projected_USEast1 (42.1s)
=== RUN   TestMultiRegion_Projected_EUWest1
--- PASS: TestMultiRegion_Projected_EUWest1 (48.3s)
=== RUN   TestMultiRegion_Projected_APNortheast1
--- PASS: TestMultiRegion_Projected_APNortheast1 (51.7s)
=== RUN   TestMultiRegion_Actual_USEast1
--- PASS: TestMultiRegion_Actual_USEast1 (118.4s)
=== RUN   TestMultiRegion_Actual_EUWest1
--- PASS: TestMultiRegion_Actual_EUWest1 (125.9s)
=== RUN   TestMultiRegion_Actual_APNortheast1
--- PASS: TestMultiRegion_Actual_APNortheast1 (132.1s)
PASS
ok      github.com/rshade/finfocus/test/e2e     518.503s
```

---

### 4. Run Only Projected Cost Tests (Skip Actual Costs)

Useful when developing before issue #177 (Pulumi Automation API) is complete.

```bash
export FINFOCUS_E2E_SKIP_ACTUAL_COSTS=true
go test -v -tags e2e ./test/e2e -run TestMultiRegion_Projected
```

**Expected Duration**: ~2-3 minutes per region

---

### 5. Customize Cost Tolerance

```bash
# Use ±10% tolerance instead of default ±5%
export FINFOCUS_E2E_TOLERANCE=0.10
go test -v -tags e2e ./test/e2e -run TestMultiRegion
```

---

## Configuration Options

### Environment Variables

- `FINFOCUS_E2E_REGIONS` (default: `us-east-1`): Comma-separated list of regions to test
- `FINFOCUS_E2E_TOLERANCE` (default: `0.05`): Cost variance tolerance (±5%)
- `FINFOCUS_E2E_TIMEOUT` (default: `10m`): Maximum test execution time per region
- `FINFOCUS_E2E_SKIP_ACTUAL_COSTS` (default: `false`): Skip actual cost tests (pre-#177 development)
- `AWS_PROFILE` (default: none): AWS credentials profile to use
- `AWS_REGION` (default: none): AWS region for credentials (overridden by test configs)

### Example: Full Configuration

```bash
export FINFOCUS_E2E_REGIONS="us-east-1,eu-west-1,ap-northeast-1"
export FINFOCUS_E2E_TOLERANCE=0.05
export FINFOCUS_E2E_TIMEOUT=5m
export FINFOCUS_E2E_SKIP_ACTUAL_COSTS=false
export AWS_PROFILE=finfocus-testing

go test -v -tags e2e ./test/e2e -run TestMultiRegion -timeout 20m
```

---

## Test Fixtures

Multi-region test fixtures are located in `test/e2e/fixtures/multi-region/`. All fixtures use Pulumi YAML runtime (no Go code required):

```text
test/e2e/fixtures/multi-region/
├── us-east-1/
│   ├── Pulumi.yaml          # YAML runtime with 8 resources (2 EC2, 2 EBS, 2 Network, 2 RDS)
│   └── expected-costs.json  # Expected cost ranges (±5% built in)
├── eu-west-1/
│   ├── Pulumi.yaml          # YAML runtime with 8 resources and EU pricing
│   └── expected-costs.json
├── ap-northeast-1/
│   ├── Pulumi.yaml          # YAML runtime with 8 resources and APAC pricing
│   └── expected-costs.json
└── unified/
    ├── Pulumi.yaml          # YAML runtime with explicit provider aliases for all regions
    └── expected-costs.json  # Per-resource + aggregate cost expectations
```

### Resource Types in Fixtures

Each region fixture includes:
- **2x EC2 Instances** (t3.micro, t3.small) - Compute
- **2x EBS Volumes** (gp3 100GB, io2 50GB) - Storage
- **2x Network Resources** (NAT Gateway, VPC Endpoint) - Network
- **2x RDS Instances** (db.t3.micro PostgreSQL, MySQL) - Database

**Total**: 8 resources per region × 3 regions = 24 resources validated

---

## Troubleshooting

### Test Fails: "No pricing data available"

**Symptom**:
```text
FAIL: No pricing data available for eu-west-1/aws:ec2:Instance
```

**Cause**: Plugin does not have pricing data for the region

**Solution**:
1. Verify plugin version supports multi-region: `./bin/finfocus plugin list`
2. Check if region-specific plugin is installed: `ls ~/.finfocus/plugins/aws-public/*/`
3. If missing, install: `./bin/finfocus plugin install aws-public --version <version>`

---

### Test Fails: "Network failure after 3 retries"

**Symptom**:
```text
FAIL: Network failure after 3 retries: connection refused
```

**Cause**: Plugin gRPC communication failed (transient network issue)

**Solution**:
1. Check plugin process is running: `ps aux | grep finfocus-plugin`
2. Verify no firewall blocking localhost communication
3. Retry test (should succeed if transient)
4. If persistent, check plugin logs in `~/.finfocus/logs/`

---

### Test Fails: Cost Variance Exceeds Tolerance

**Symptom**:
```text
FAIL: Resource web-server cost $58.50 outside expected range [$50.00, $55.00] (±5%)
```

**Cause**: AWS pricing has changed, or resource configuration differs from expected

**Solution**:
1. **Verify Pricing Change**: Check AWS pricing page for the resource type in that region
2. **Update Expected Costs**: Edit `test/e2e/fixtures/multi-region/<region>/expected-costs.json`
3. **Adjust Tolerance** (temporary): `export FINFOCUS_E2E_TOLERANCE=0.10` for ±10%
4. **Review Fixture**: Ensure Pulumi program matches expected resource specs

---

### Test Timeout: "test timed out after 10m"

**Symptom**:
```text
panic: test timed out after 10m0s
```

**Cause**: Test execution exceeded timeout (likely actual cost tests with resource deployment)

**Solution**:
1. **Increase Timeout**: `export FINFOCUS_E2E_TIMEOUT=15m`
2. **Run Test with `-timeout` Flag**: `go test -timeout 20m -tags e2e ./test/e2e -run TestMultiRegion`
3. **Skip Slow Tests**: `export FINFOCUS_E2E_SKIP_ACTUAL_COSTS=true` to test only projected costs

---

### Missing Dependency: Issue #177 or #24

**Symptom**:
```text
FAIL: Pulumi Automation API support required (issue #177)
```

**Cause**: Test depends on incomplete feature

**Solution**:
1. **Check Issue Status**: `gh issue view 177` and `gh issue view 24`
2. **Skip Actual Cost Tests**: `export FINFOCUS_E2E_SKIP_ACTUAL_COSTS=true`
3. **Wait for Dependency**: Tests will automatically enable once dependencies merge

---

## Validating Changes

### Before Committing

```bash
# Run full test suite including multi-region
make test-e2e

# Check lint compliance
make lint

# Verify cross-region pricing differences
go test -v -tags e2e ./test/e2e -run TestMultiRegion_Projected | grep "Pricing difference detected"
```

### Expected Pricing Differences

Tests validate that costs vary across regions:

- **t3.micro EC2**: us-east-1: $7.01/month (baseline), eu-west-1: ~$7.50 (+7%), ap-northeast-1: ~$8.00 (+14%)
- **gp3 100GB EBS**: us-east-1: $8.00/month (baseline), eu-west-1: ~$9.60 (+20%), ap-northeast-1: ~$8.80 (+10%)
- **NAT Gateway**: us-east-1: $32.85/month (baseline), eu-west-1: ~$36.14 (+10%), ap-northeast-1: ~$39.42 (+20%)

*Note: Actual values may vary; tests use ±5% tolerance to accommodate pricing updates.*

---

---

## User Story 4: Unified Multi-Region Fixtures

### Overview

In addition to per-region fixtures (US1-3), User Story 4 tests a **unified Pulumi program** that deploys resources across multiple regions in a single stack - a common real-world pattern.

### Run Unified Multi-Region Test

```bash
# Run the unified fixture test specifically
go test -v -tags e2e ./test/e2e -run TestMultiRegion_Unified_Projected
```

**Expected Output**:
```text
=== RUN   TestMultiRegion_Unified_Projected
    multi_region_projected_test.go:XXX: Testing unified multi-region fixture
    multi_region_projected_test.go:XXX: Resource web-us-east-1 (us-east-1): $7.01 [PASS]
    multi_region_projected_test.go:XXX: Resource web-eu-west-1 (eu-west-1): $7.51 [PASS]
    multi_region_projected_test.go:XXX: Resource web-ap-northeast-1 (ap-northeast-1): $8.01 [PASS]
    multi_region_projected_test.go:XXX: Aggregate total: $22.53 within [$21.12, $24.66] [PASS]
--- PASS: TestMultiRegion_Unified_Projected (35.2s)
```

---

### Unified Fixture Structure

```text
test/e2e/fixtures/multi-region/unified/
├── Pulumi.yaml           # YAML runtime with multi-region providers
└── expected-costs.json   # Per-resource + aggregate cost expectations
```

### Key Differences from Per-Region Fixtures

- **Structure**: Per-region has 3 separate `Pulumi.yaml` files (one per region), Unified has 1 `Pulumi.yaml` with all regions
- **Providers**: Per-region uses single implicit AWS provider per fixture, Unified uses explicit provider aliases per region
- **Validation**: Per-region validates per-region totals only, Unified validates per-resource costs + aggregate total

---

### Unified Fixture Example

**Pulumi.yaml**:
```yaml
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
```

---

### Unified Expected Costs Format

```json
{
  "fixture_type": "unified",
  "total_regions": 3,
  "resources": [
    {
      "resource_name": "web-us-east-1",
      "resource_type": "aws:ec2:Instance",
      "region": "us-east-1",
      "min_cost": 6.66,
      "max_cost": 7.36,
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

---

### Troubleshooting Unified Fixtures

#### "Region not detected for resource"

**Symptom**:
```text
WARN: Region not detected for resource web-us-east-1, using default
```

**Cause**: FinFocus could not resolve region from provider configuration

**Solution**:
1. Ensure provider is explicitly declared with `region` property
2. Verify resource uses `options.provider` to reference the provider
3. Check plan JSON contains provider URN with region

#### Aggregate Total Outside Tolerance

**Symptom**:
```text
FAIL: Aggregate total $25.50 outside expected range [$21.12, $24.66]
```

**Cause**: Individual resource costs are within tolerance but sum exceeds aggregate bounds

**Solution**:
1. Recalculate `aggregate_validation` based on sum of individual ranges
2. Consider that tolerance compounds: 3 resources at +5% each = ~+15% total
3. Update `expected-costs.json` with wider aggregate tolerance if needed

---

## Next Steps

1. **Add New Regions**: Create new fixture in `test/e2e/fixtures/multi-region/<region>/`
2. **Customize Resources**: Edit `Pulumi.yaml` in fixture to test different resource types
3. **Update Expected Costs**: Adjust `expected-costs.json` when AWS pricing changes
4. **Run in CI**: Multi-region tests run automatically via `make test-e2e` in CI pipeline
5. **Create Unified Fixtures**: Add `unified/` subfolder with YAML runtime for cross-region tests

## Related Documentation

- [E2E Test Plan](../../E2E_TEST_PLAN.md) - Section 4.2 Multi-region testing
- [Feature Spec](./spec.md) - Full feature specification with User Stories 1-4
- [Implementation Plan](./plan.md) - Technical design details
- [CLAUDE.md](../../CLAUDE.md) - Testing section
