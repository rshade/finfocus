---
layout: default
title: Vantage Plugin Troubleshooting
description: Common issues, solutions, and support paths for the FinFocus Vantage plugin
---

This guide helps diagnose and resolve common issues with the Vantage plugin
for FinFocus.

## Table of Contents

1. [Authentication Issues](#authentication-issues)
2. [Configuration Errors](#configuration-errors)
3. [Data Retrieval Problems](#data-retrieval-problems)
4. [Performance Issues](#performance-issues)
5. [Plugin Integration Issues](#plugin-integration-issues)
6. [Getting Help](#getting-help)

---

## Authentication Issues

### Issue: 401 Unauthorized

**Symptoms:**

```text
Error: 401 Unauthorized
Failed to authenticate with Vantage API
```

**Causes:**

- API token not set or empty
- Token expired or revoked
- Token lacks required permissions
- Wrong token type used

**Solutions:**

1. Verify token is set:

   ```bash
   echo $FINFOCUS_VANTAGE_TOKEN
   # Should output your token (not empty)
   ```

2. Test token validity:

   ```bash
   curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
     https://api.vantage.sh/costs
   # Expected: 200 OK or 400 (bad params), NOT 401
   ```

3. Regenerate token:
   - Log into Vantage console
   - Navigate to **Settings** → **API Tokens**
   - Generate new token
   - Update `FINFOCUS_VANTAGE_TOKEN` environment variable

4. Verify token permissions:
   - Token must have **read-only cost access**
   - Cost Report tokens must have access to specified report

**See Also:** [Authentication Guide](authentication.md)

---

### Issue: 403 Forbidden

**Symptoms:**

```text
Error: 403 Forbidden
Insufficient permissions to access cost data
```

**Causes:**

- Token has wrong permissions
- Cost Report Token doesn't have access to report
- Workspace Token doesn't have cost access

**Solutions:**

1. Verify token permissions in Vantage console
2. Ensure token has **Read-only cost access**
3. For Cost Report Token, verify report access
4. Generate new token with correct permissions

---

## Configuration Errors

### Issue: Configuration File Not Found

**Symptoms:**

```text
Error: config file not found at: config.yaml
```

**Solutions:**

1. Verify file exists:

   ```bash
   ls -la ~/.finfocus/plugins/vantage/config.yaml
   ```

2. Check file path:

   ```bash
   finfocus-vantage pull --config /full/path/to/config.yaml
   ```

3. Create configuration if missing:

   ```bash
   mkdir -p ~/.finfocus/plugins/vantage
   cp config.example.yaml ~/.finfocus/plugins/vantage/config.yaml
   ```

---

### Issue: Invalid YAML Syntax

**Symptoms:**

```text
Error: failed to parse config.yaml
YAML syntax error at line 10
```

**Solutions:**

1. Validate YAML syntax:

   ```bash
   yamllint ~/.finfocus/plugins/vantage/config.yaml
   ```

2. Check required fields:

   ```yaml
   version: 0.1 # Required
   source: vantage # Required
   credentials:
     token: ${...} # Required
   params:
     cost_report_token: 'cr_...' # Required
     granularity: 'day' # Required
   ```

3. Common YAML mistakes:
   - Missing colons after keys
   - Incorrect indentation (use spaces, not tabs)
   - Unquoted special characters
   - Missing quotes around strings with colons

---

### Issue: Environment Variable Not Set

**Symptoms:**

```text
Error: FINFOCUS_VANTAGE_TOKEN environment variable not set
```

**Solutions:**

1. Set environment variable:

   ```bash
   export FINFOCUS_VANTAGE_TOKEN="your_token"
   ```

2. Verify it's set:

   ```bash
   echo $FINFOCUS_VANTAGE_TOKEN
   ```

3. Persist across sessions:

   ```bash
   echo 'export FINFOCUS_VANTAGE_TOKEN="your_token"' >> ~/.bashrc
   source ~/.bashrc
   ```

---

## Data Retrieval Problems

### Issue: No Cost Data Returned

**Symptoms:**

```text
No cost records found for specified date range
```

**Causes:**

- Date range has no data in Vantage
- Cost Report Token doesn't have data for date range
- Filters excluding all data
- Data not yet available (lag)

**Solutions:**

1. Verify date range has data in Vantage console

2. Check Cost Report Token is valid:

   ```yaml
   params:
     cost_report_token: 'cr_valid_token_here'
   ```

3. Ensure Cost Report has data for selected providers

4. Account for late posting:
   - Cost data lags 2-3 days
   - Use historical dates (not today/yesterday)
   - Wait 3+ days for final reconciliation

5. Review filters:

   ```yaml
   params:
     group_bys:
       - provider
       - service
     # Ensure not over-filtering
   ```

---

### Issue: Rate Limit Exceeded

**Symptoms:**

```text
Error: 429 Too Many Requests
Rate limit exceeded
```

**How It Works:**

The plugin automatically retries 429 responses with exponential backoff:

- Attempt 1 → Wait 1s → Retry
- Attempt 2 → Wait 2s → Retry
- Attempt 3 → Wait 4s → Retry
- Continues up to `max_retries` (default: 5)

**Solutions:**

1. Wait for automatic retry (30-60 seconds)

2. Increase retry attempts:

   ```yaml
   params:
     max_retries: 10
     request_timeout_seconds: 120
   ```

3. Reduce request frequency:

   ```yaml
   params:
     page_size: 10000 # Larger pages = fewer requests
   ```

4. Reduce data dimensionality:

   ```yaml
   params:
     group_bys:
       - provider
       - service
       # Remove: resource_id, tags (high cardinality)
   ```

5. Schedule off-peak:
   - Run backfills during low-usage hours
   - Daily incremental syncs less likely to hit limits

---

### Issue: Partial or Incomplete Data

**Symptoms:**

- Missing cost records for some resources
- Gaps in date range
- Null values in cost fields

**Causes:**

- Late posting (data still arriving)
- Metric not available for provider
- Tag not present in cost report
- Data not yet finalized

**Solutions:**

1. Wait for data to finalize (3+ days after usage)

2. Verify metrics available:

   ```yaml
   params:
     metrics:
       - cost # Available for all
       - usage # Available for all
       - amortized_cost # AWS, GCP, Azure only
       - taxes # AWS, Azure only
   ```

3. Check tag configuration:

   ```yaml
   params:
     group_bys:
       - tags # Must be present for tags
   ```

4. Review Vantage console for data availability

---

## Performance Issues

### Issue: Slow Sync Performance

**Symptoms:**

- Sync takes >10 minutes
- High memory usage
- Multiple API timeouts

**Solutions:**

1. Increase page size:

   ```yaml
   params:
     page_size: 10000 # Max: 10,000
   ```

2. Reduce dimensions:

   ```yaml
   params:
     group_bys:
       - provider
       - service
       # Remove high-cardinality: resource_id, tags
   ```

3. Use monthly granularity for long ranges:

   ```yaml
   params:
     granularity: 'month'
   ```

4. Chunk large imports:

   ```bash
   # Import monthly instead of yearly
   for m in {01..12}; do
     finfocus-vantage pull \
       --config config.yaml \
       --start-date "2024-$m-01" \
       --end-date "2024-$m-31"
   done
   ```

5. Filter tags:

   ```yaml
   params:
     tag_prefix_filters:
       - 'user:'
       - 'cost-center:'
   ```

---

### Issue: High Memory Usage

**Symptoms:**

- Out-of-memory (OOM) errors
- Adapter crashes during sync
- System slowdown

**Solutions:**

1. Reduce page size:

   ```yaml
   params:
     page_size: 1000 # Conservative (vs 5000 default)
   ```

2. Reduce dimensions:

   ```yaml
   params:
     group_bys:
       - provider
       - service
       # Remove: account, region, resource_id, tags
   ```

3. Monitor resources:

   ```bash
   watch -n 1 'ps aux | grep finfocus-vantage'
   ```

4. Use monthly granularity:

   ```yaml
   params:
     granularity: 'month'
   ```

---

## Plugin Integration Issues

### Issue: Plugin Not Recognized by FinFocus

**Symptoms:**

```text
Error: plugin 'vantage' not found
```

**Solutions:**

1. Verify plugin installation:

   ```bash
   finfocus plugin list
   # Should show: vantage v0.1.0
   ```

2. Check plugin directory:

   ```bash
   ls -la ~/.finfocus/plugins/vantage/
   ```

3. Reinstall plugin:

   ```bash
   finfocus plugin install vantage
   ```

4. Verify binary executable:

   ```bash
   chmod +x ~/.finfocus/plugins/vantage/finfocus-vantage
   ```

---

### Issue: Plugin Communication Timeout

**Symptoms:**

```text
Error: plugin timeout after 10 seconds
Failed to communicate with vantage plugin
```

**Causes:**

- Plugin process not starting
- gRPC communication failure
- Network issues
- Plugin binary corrupted

**Solutions:**

1. Test plugin directly:

   ```bash
   ~/.finfocus/plugins/vantage/finfocus-vantage --version
   ```

2. Check plugin logs:

   ```bash
   finfocus cost actual --plugin vantage --debug
   ```

3. Restart plugin host:

   ```bash
   pkill -f finfocus
   finfocus cost actual --plugin vantage
   ```

4. Reinstall plugin if corrupted

---

### Issue: Cost Data Doesn't Match Vantage Console

**Symptoms:**

- Different cost totals
- Missing resources
- Unexpected cost values

**Causes:**

- Date range differences
- Granularity mismatch
- Filtering differences
- Late posting
- Currency differences

**Solutions:**

1. Verify date range matches:

   ```yaml
   params:
     start_date: '2024-01-01'
     end_date: '2024-01-31'
   ```

2. Check granularity:

   ```yaml
   params:
     granularity: 'day' # Must match Vantage console view
   ```

3. Verify filters match:

   ```yaml
   params:
     group_bys:
       - provider
       - service
     # Same dimensions as Vantage report
   ```

4. Account for timing:
   - Cost data lags 2-3 days
   - Check when Vantage data finalized
   - Wait 3+ days for final reconciliation

5. Verify currency (default: USD)

---

## Common Error Messages

### Error: "cursor expired"

**Cause:** Pagination cursor became stale during long sync

**Solution:**

1. Reduce page size:

   ```yaml
   params:
     page_size: 1000
   ```

2. Increase timeout:

   ```yaml
   params:
     request_timeout_seconds: 120
   ```

3. Use smaller date ranges

---

### Error: "invalid date format"

**Cause:** Date not in ISO 8601 format

**Solution:**

Use `YYYY-MM-DD` format:

```yaml
params:
  start_date: '2024-01-01' # Correct
  # start_date: "01/01/2024"  # WRONG
```

---

### Error: "tag cardinality limit exceeded"

**Cause:** Too many unique tags causing performance issues

**Solution:**

Use tag prefix filters:

```yaml
params:
  tag_prefix_filters:
    - 'user:'
    - 'cost-center:'
    # Limit high-cardinality tags
```

---

## Debugging Tips

### Enable Debug Logging

```bash
# Set debug environment variable
export VANTAGE_DEBUG=1

# Run plugin
finfocus cost actual --plugin vantage --start-date 2024-01-01
```

### Check Plugin Status

```bash
# List installed plugins
finfocus plugin list

# Validate plugin
finfocus plugin validate vantage
```

### Test API Connectivity

```bash
# Test Vantage API directly
curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
  https://api.vantage.sh/costs
```

### Verify Configuration

```bash
# Validate YAML
yamllint ~/.finfocus/plugins/vantage/config.yaml

# Test configuration loading
finfocus-vantage pull --config config.yaml --dry-run
```

---

## Getting Help

### Self-Service Resources

1. **Documentation**
   - [Setup Guide](setup.md)
   - [Authentication Guide](authentication.md)
   - [Features Guide](features.md)
   - [Cost Mapping Guide](cost-mapping.md)

2. **Vantage Resources**
   - [Vantage API Documentation](https://docs.vantage.sh/api)
   - [Vantage Support](https://support.vantage.sh/)
   - [Vantage Status Page](https://status.vantage.sh/)

3. **FinFocus Resources**
   - [FinFocus Documentation](../../README.md)
   - [Plugin Development Guide](../plugin-development.md)
   - [GitHub Issues](https://github.com/rshade/finfocus/issues)

### Support Channels

#### Community Support

- **GitHub Issues**: Report bugs or request features
- **GitHub Discussions**: Ask questions and share experiences

#### Commercial Support

- **Vantage Support**: For Vantage API issues
  - Email: <support@vantage.sh>
  - Portal: <https://support.vantage.sh/>

- **FinFocus Support**: For plugin integration issues
  - GitHub Issues: <https://github.com/rshade/finfocus/issues>

### Reporting Issues

When reporting issues, include:

1. **Error Message** (redact tokens):

   ```text
   Error: 401 Unauthorized
   Failed to authenticate with Vantage API
   ```

2. **Configuration** (redact sensitive data):

   ```yaml
   version: 0.1
   source: vantage
   params:
     cost_report_token: 'cr_REDACTED'
     granularity: 'day'
   ```

3. **Steps to Reproduce**:

   ```bash
   1. Set environment: export FINFOCUS_VANTAGE_TOKEN="..."
   2. Run command: finfocus cost actual --plugin vantage
   3. Observe error: ...
   ```

4. **Debug Logs** (redact tokens):

   ```bash
   VANTAGE_DEBUG=1 finfocus cost actual --plugin vantage 2>&1 | \
     sed 's/Bearer .*/Bearer REDACTED/' > debug.log
   ```

5. **Environment Information**:
   - Plugin version: `finfocus-vantage --version`
   - FinFocus version: `finfocus --version`
   - OS: `uname -a`
