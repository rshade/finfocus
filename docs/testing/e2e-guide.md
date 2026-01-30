---
layout: default
title: E2E Testing Guide
description: How to configure, run, and troubleshoot FinFocus end-to-end tests.
parent: Testing
nav_order: 1
---

## Overview

This guide details how to set up and run end-to-end (E2E) tests for FinFocus to
ensure the system works correctly with real cloud resources. E2E tests validate
the complete cost calculation pipeline using actual AWS services.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Running Tests](#running-tests)
- [Test Scenarios](#test-scenarios)
- [Related Documentation](#related-documentation)
- [Troubleshooting](#troubleshooting)

## Prerequisites

Before running end-to-end tests, ensure your environment meets the following requirements:

- **Go 1.25.6+**: Required for building the core and plugins.
- **AWS Credentials**: A valid AWS account with read permissions (for Cost Explorer) and resource creation permissions
  (for infrastructure tests).
- **FinFocus Core**: Installed locally.

## Quick Start

1. **Install Required Plugins** The E2E tests rely on the `aws-public` plugin for fallback pricing and `aws-costexplorer`
   for actual billing data validation.

   ```bash
   # Install public plugin
   finfocus plugin install github.com/rshade/finfocus-plugin-aws-public

   # (Optional) Install Cost Explorer plugin if testing actual costs
   finfocus plugin install github.com/rshade/finfocus-plugin-aws-costexplorer
   ```

2. **Configure Environment** Export your AWS credentials.

   ```bash
   export AWS_ACCESS_KEY_ID="testing-key"
   export AWS_SECRET_ACCESS_KEY="testing-secret"
   export AWS_REGION="us-east-1"
   ```

3. **Run Tests** Execute the E2E test suite using the Makefile target.

   ```bash
   make test-e2e
   ```

4. **View Results** Test results are summarized in JSON format.

   ```bash
   cat test-results/e2e-summary.json
   ```

## Running Tests

The `make test-e2e` command orchestrates the entire testing process. It:

1. Builds the core binary and plugins.
2. Sets up a temporary Pulumi project.
3. Runs the test suite which interacts with AWS.
4. Cleans up resources after tests complete.

### Custom Test Arguments

You can pass custom arguments to the test runner using the `TEST_ARGS` variable:

```bash
make test-e2e TEST_ARGS="-v -run TestProjectedCosts"
```

## Test Scenarios

The E2E suite covers several critical scenarios to ensure system reliability.

### 1. Projected Costs

Calculates estimated monthly costs based on the Pulumi plan before any resources are deployed. This validates the
`aws-public` plugin's pricing data lookup.

### 2. Actual Costs

After resources have been running (or using historical data), this scenario queries the AWS Cost Explorer API via the
`aws-costexplorer` plugin to verify actual spending matches expectations.

### 3. Cost Validation

Compares projected costs against actual costs (where applicable) to ensure accuracy within a defined tolerance.

### 4. Cleanup Verification

Ensures that all resources created during the test are destroyed, preventing accidental costs.

## Related Documentation

- [Plugin Ecosystem Guide](../architecture/plugin-ecosystem.md)
- [Troubleshooting Guide](../guides/troubleshooting.md)
- [Cost Calculation Guide](../guides/cost-calculation.md)
- [Plugin Compatibility Reference](../reference/plugin-compatibility.md)

## Troubleshooting

### "Plugin not found"

Ensure you ran `finfocus plugin install` before testing. You can list installed plugins with:

```bash
finfocus plugin list
```

### "Access Denied" or Credential Errors

Verify your AWS credentials have the necessary permissions. The test suite requires permissions to create/destroy EC2
instances, S3 buckets, and query Cost Explorer.

### Test Timeouts

E2E tests can be slow due to cloud resource provisioning. If tests timeout, try increasing the timeout duration:

```bash
go test -timeout 30m ./test/e2e/...
```
