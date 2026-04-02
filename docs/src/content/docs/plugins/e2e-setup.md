---
title: E2E Test Setup
---
This guide explains how to set up your environment for running End-to-End
(E2E) tests against real cloud providers.

## Prerequisites

- Active Cloud Provider Account (AWS, Azure, or GCP)
- Pulumi CLI installed
- `finfocus` binary built (`make build`)

## AWS Setup

1. **Credentials**: Ensure you have AWS credentials in your environment.

   ```bash
   export AWS_ACCESS_KEY_ID=...
   export AWS_SECRET_ACCESS_KEY=...
   export AWS_SESSION_TOKEN=... # if using MFA
   ```

2. **Configuration**:

   ```bash
   export FINFOCUS_E2E_AWS_REGION=us-west-2
   export FINFOCUS_E2E_TOLERANCE=0.05  # 5% tolerance
   export FINFOCUS_E2E_TIMEOUT=15m
   ```

3. **Running Tests**:

   ```bash
   make test-e2e
   # or specific test
   go test -v -tags e2e ./test/e2e/aws/...
   ```

## Troubleshooting

- **Skipped Tests**: If tests are skipped with "missing credentials",
  check your environment variables.
- **Timeouts**: Increase `FINFOCUS_E2E_TIMEOUT` for complex stacks.
