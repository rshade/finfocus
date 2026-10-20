---
title: Troubleshooting
description: Solutions for common FinFocus installation, configuration, and runtime issues.
parent: Guides
nav_order: 5
---

## Overview

This guide addresses common issues encountered when using FinFocus, including
installation problems, AWS credential errors, cost calculation issues, and E2E
test failures.

## Table of Contents

- [Installation Failures](#installation-failures)
- [AWS Credential Problems](#aws-credential-problems)
- [Cost Calculation Errors](#cost-calculation-errors)
- [E2E Test Timeouts](#e2e-test-timeouts)
- [Resource Cleanup Issues](#resource-cleanup-issues)
- [ESC environment skipped](#esc-environment-skipped)

## Installation Failures

### Plugin Installation Fails

**Symptom**: `finfocus plugin install` returns an error or hangs.

**Solutions**:

1. **Network**: Check your internet connection. Plugins are downloaded from GitHub Releases.
2. **Rate Limits**: GitHub API rate limits may apply. Try setting `GITHUB_TOKEN`.
3. **Permissions**: Ensure the `~/.finfocus/plugins/` directory is writable.

```bash
mkdir -p ~/.finfocus/plugins
chmod -R 755 ~/.finfocus/plugins
```

### Binary Not Found

**Symptom**: `command not found: finfocus`

**Solution**: Add the installation directory to your PATH.

```bash
export PATH=$PATH:$HOME/go/bin
# or where you installed it
```

## AWS Credential Problems

### Access Denied

**Symptom**: `AccessDeniedException` when running `actual` cost queries.

**Solution**:

The configured AWS credentials must have `ce:GetCostAndUsage` permission. Ensure your IAM policy includes:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["ce:GetCostAndUsage", "ce:GetDimensionValues", "ce:GetTags"],
      "Resource": "*"
    }
  ]
}
```

See [AWS Cost Explorer API Permissions](https://docs.aws.amazon.com/cost-management/latest/userguide/ce-access.html) for
additional guidance on Cost Explorer access and IAM policy best practices.

### Profile Not Found

**Symptom**: `Profile 'default' not found`

**Solution**:

Check `~/.aws/credentials` or export credentials directly:

```bash
export AWS_PROFILE=my-profile
```

## Cost Calculation Errors

### Zero Costs Reported

**Symptom**: Projected costs show $0.00 for supported resources.

**Causes**:

1. **Unsupported Region**: The resource is in a region not supported by the embedded database.
2. **Missing Properties**: Required properties (like `instanceType`) are missing from the Pulumi plan.
3. **Plugin Issue**: The `aws-public` plugin might not be installed or discovered.

**Debug**:

Run with debug logging to see plugin selection:

```bash
finfocus --debug cost projected --pulumi-json plan.json
```

### Mismatch Between Projected and Actual

**Symptom**: Actual costs are significantly higher than projected.

**Explanation**:

- **Taxes/Credits**: Actual costs include taxes and credits; projected do not.
- **Data Transfer**: Data transfer costs are hard to predict and are usually excluded from projections.
- **Storage**: S3 API requests and tiered storage costs accumulate over time.

## E2E Test Timeouts

### Test Suite Hangs

**Symptom**: `make test-e2e` runs for >20 minutes.

**Solution**:

1. **Check Resources**: Cloud resource creation (e.g., RDS) can be slow.
2. **Increase Timeout**:

   ```bash
   go test -timeout 30m ./test/e2e/...
   ```

3. **Region Latency**: Switch `AWS_REGION` to a closer region (e.g., `us-east-1` -> `us-west-2`).

## Resource Cleanup Issues

### Leftover Resources

**Symptom**: E2E tests failed but resources remain in AWS.

**Solution**:

Use the `cleanup` utility or manually delete resources tagged with `finfocus-e2e`.

```bash
# Example manual cleanup
aws ec2 terminate-instances --instance-ids i-xxxx
```

## ESC environment skipped

FinFocus reads plugin environment variables from Pulumi ESC. When that read
fails, the command still finishes. Plugins keep the values from the shell and
from `config.hujson`. Stderr shows one `Warning: ESC …` line for each
environment and category. The warning names the environment reference and the
category. It never includes a variable value.

| Category | Cause | Fix |
| --- | --- | --- |
| `not_logged_in` | No usable Pulumi Cloud token or organization. A keychain-encrypted login has no token finfocus can read. | Set `PULUMI_ACCESS_TOKEN`, or run `pulumi login`. Set `PULUMI_DEFAULT_ORGANIZATION` when the reference has no organization. |
| `not_found` | The environment does not exist, or this identity cannot see it. The warning says "not found or no access". | Check the reference and who can open that environment. |
| `access_denied` | Pulumi Cloud returned 403. | Ask for access to the environment, or sign in as an identity that has it. |
| `unsupported_backend` | State is not on Pulumi Cloud. `file://`, `s3://`, `gs://`, `azblob://`, and non-loopback `http://` cannot host ESC. A `backend.url` in `Pulumi.yaml` also does not receive `PULUMI_ACCESS_TOKEN`. | Use Pulumi Cloud, or set `PULUMI_BACKEND_URL` to the Pulumi Cloud API and log in there. `http://127.0.0.1` is accepted for a local API. |
| `unsupported_definition` | The stack's `environment:` block is not a list of imports. Inline `values:` and per-import merge options are not read. | Import an environment, or set `esc.environments` to a list of references. |
| `timed_out` | The ESC service did not answer inside the five-second wait. | Retry when the network is available. The command already continued with shell and config values. |
| `unavailable` | The service, the network, or the client failed in some other way. | Check connectivity to the Pulumi Cloud API and try again. |

An empty `esc.environments` list turns ESC off and prints no warning. See
[Pulumi ESC credentials](esc-credentials.md).
