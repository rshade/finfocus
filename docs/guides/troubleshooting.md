---
layout: default
title: Troubleshooting
parent: Guides
nav_order: 5
---

This guide addresses common issues encountered when using FinFocus.

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

Run with verbose logging to see plugin selection:

```bash
finfocus cost projected --pulumi-json plan.json --verbose
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
