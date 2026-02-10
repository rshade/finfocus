# Quickstart: Running E2E Cost Tests

This guide explains how to run the End-to-End (E2E) cost validation tests for FinFocus.

## Prerequisites

1. **Go 1.25.7+** installed.
2. **Pulumi CLI** installed.
3. **AWS Credentials** configured in your environment:
   - `AWS_ACCESS_KEY_ID`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_REGION` (default: `us-east-1`)
   - `AWS_SESSION_TOKEN` (if using MFA/SSO)

## Running Tests

The E2E tests are separated from unit tests using the `e2e` build tag.

### Using Make (Recommended)

```bash
# Run all E2E tests
make test-e2e

# Run a specific test
make test-e2e TEST_ARGS="-run TestProjectedCost"
```

### Using Go Command

```bash
# Run all tests in the e2e directory
go test -v -tags e2e ./test/e2e/...

# Run with a 60-minute timeout (recommended for infrastructure tests)
go test -v -tags e2e -timeout 60m ./test/e2e/...
```

## Configuration

| Variable           | Description                        | Default     |
| ------------------ | ---------------------------------- | ----------- |
| `E2E_REGION`       | AWS Region to use                  | `us-east-1` |
| `E2E_TIMEOUT_MINS` | Max duration before forced cleanup | `60`        |

## Troubleshooting

### Cleanup Failures

If a test crashes hard (e.g., process killed), resources might be left in AWS.

1. Log in to AWS Console.
2. Go to **CloudFormation**.
3. Look for stacks named `e2e-test-<random-string>`.
4. Delete them manually.

### Timeout Issues

Resources like EC2 instances can take minutes to provision. If tests timeout:

- Increase the timeout flag: `-timeout 90m`.
- Check AWS Service Quotas (e.g., vCPU limits) if provisioning hangs.

### Cost Discrepancies

If validation fails:

- Check `test/e2e/logs/` for detailed cost calculation output.
- Verify AWS pricing hasn't changed significantly for the region.
