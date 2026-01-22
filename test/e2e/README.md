# End-to-End Tests

This directory contains E2E tests for FinFocus Core.

## Running Tests

Prerequisites:

- `finfocus` binary built and available in `bin/` or `PATH`.
- Fixtures available in `test/fixtures/plans/`.
- AWS credentials configured (for multi-region and actual cost tests).

Run all E2E tests:

```bash
go test -v -tags e2e ./test/e2e/...
```

Run specific test suites:

```bash
# Projected cost tests only
go test -v -tags e2e ./test/e2e -run Projected

# Multi-region tests
go test -v -tags e2e ./test/e2e -run TestMultiRegion

# Actual cost tests (requires AWS credentials)
go test -v -tags e2e ./test/e2e -run Actual
```

## Test Structure

- `projected_cost_test.go`: Validates projected cost workflow.
- `output_*.go`: Validates different output formats.
- `errors_test.go`: Validates error handling.
- `*_test.go`: Provider-specific tests.
- `actual_cost_test.go`: Validates actual cost workflow with Pulumi state.
- `multi_region_projected_test.go`: Multi-region projected cost validation.
- `multi_region_actual_test.go`: Multi-region actual cost validation.
- `multi_region_fallback_test.go`: Multi-region plugin fallback scenarios.
- `multi_region_helpers.go`: Helper functions for multi-region testing.

## Multi-Region Testing

Multi-region E2E tests validate cost calculations across different AWS regions (us-east-1, eu-west-1, ap-northeast-1) to ensure regional pricing differences are correctly reflected.

### Quick Start

```bash
# Run multi-region tests for default region (us-east-1)
make test-e2e

# Test all three regions
export FINFOCUS_E2E_REGIONS="us-east-1,eu-west-1,ap-northeast-1"
go test -v -tags e2e ./test/e2e -run TestMultiRegion
```

### Configuration

Environment variables for multi-region testing:

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `FINFOCUS_E2E_REGIONS` | `us-east-1` | Comma-separated list of regions to test |
| `FINFOCUS_E2E_TOLERANCE` | `0.05` | Cost variance tolerance (±5%) |
| `FINFOCUS_E2E_TIMEOUT` | `10m` | Maximum test execution time per region |
| `FINFOCUS_E2E_SKIP_ACTUAL_COSTS` | `false` | Skip actual cost tests (useful during development) |

### Test Fixtures

Multi-region test fixtures are located in `fixtures/multi-region/`. Fixtures use YAML runtime (no Go code):

```text
fixtures/multi-region/
├── us-east-1/
│   ├── Pulumi.yaml          # YAML program (region: us-east-1) with 16 resources
│   └── expected-costs.json  # Expected cost ranges (±5%)
├── eu-west-1/
│   ├── Pulumi.yaml          # YAML program (region: eu-west-1) with 16 resources
│   └── expected-costs.json
├── ap-northeast-1/
│   ├── Pulumi.yaml          # YAML program (region: ap-northeast-1) with 16 resources
│   └── expected-costs.json
└── unified/
    ├── Pulumi.yaml          # Multi-region unified fixture
    └── expected-costs.json
```

Each regional fixture contains:

- **2x EC2 Instances** (t3.micro, t3.small) - Compute
- **2x EBS Volumes** (gp3, io2) - Storage
- **2x Network Resources** (NAT Gateway, VPC Endpoint) - Network
- **2x RDS Instances** (db.t3.micro) - Database

### Validation Strategy

Multi-region tests use **tolerance-based validation**:

- Each resource has expected cost range in `expected-costs.json`
- Default tolerance: ±5% to handle minor pricing fluctuations
- Tests fail if actual cost exceeds expected range
- Expected costs are region-specific to validate pricing differences

### Failure Handling

The tests implement strict failure semantics:

- **Missing pricing data**: Immediate test failure (no graceful degradation)
- **Network failures**: Retry 3x with exponential backoff (100ms, 200ms, 400ms), then fail
- **Invalid region config**: Fail immediately in setup phase
- **Cost out of tolerance**: Record failure and continue (reports all failures)

### Expected Performance

- **Projected cost tests**: ~2-3 minutes per region
- **Actual cost tests**: ~4-5 minutes per region (includes resource deployment)
- **All three regions**: ~15 minutes total

### Troubleshooting

**Test fails with "No pricing data available":**

```bash
# Verify plugin installation
./bin/finfocus plugin list

# Install region-specific plugin if missing
./bin/finfocus plugin install aws-public --version <version>
```

**Test timeout:**

```bash
# Increase timeout for slower environments
export FINFOCUS_E2E_TIMEOUT=15m
go test -timeout 20m -v -tags e2e ./test/e2e -run TestMultiRegion
```

**Cost variance exceeds tolerance:**

```bash
# Verify AWS pricing hasn't changed
# Update expected-costs.json if pricing has been updated
# Or temporarily increase tolerance for investigation
export FINFOCUS_E2E_TOLERANCE=0.10  # ±10%
```

For detailed multi-region testing documentation, see:

- [Feature Spec](../../specs/001-multi-region-e2e/spec.md)
- [Quickstart Guide](../../specs/001-multi-region-e2e/quickstart.md)
- [Multi-Region E2E Testing Guide](../../docs/testing/multi-region-e2e.md)

## Helpers

The tests rely on `findFinFocusBinary()` to locate the executable.
Tests use `exec.Command` to run the binary against fixture plans.
Multi-region tests use helper functions in `multi_region_helpers.go` for:

- Fixture loading and validation
- Cost tolerance checking
- Retry logic with exponential backoff
- Error classification (network vs. missing data)
