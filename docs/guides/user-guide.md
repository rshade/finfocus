---
layout: default
title: User Guide
description: Complete guide for end users - install, configure, and use FinFocus
---

This guide is for anyone who wants to **use FinFocus** to see costs for their Pulumi
infrastructure.

## Table of Contents

1. [What is FinFocus?](#what-is-finfocus)
2. [Installation](#installation)
3. [Quick Start](#quick-start)
4. [Cost Types](#cost-types)
5. [Common Workflows](#common-workflows)
6. [Configuration](#configuration)
7. [Budget Management](#budget-management)
8. [Output Formats](#output-formats)
9. [Filtering and Grouping](#filtering-and-grouping)
10. [Debugging and Logging](#debugging-and-logging)
11. [Logging Configuration](#logging-configuration)
12. [Troubleshooting](#troubleshooting)

---

## What is FinFocus?

FinFocus is a command-line tool that calculates cloud infrastructure costs from your Pulumi infrastructure definitions.

**Key Features:**

- 📊 **Projected Costs** - Estimate costs before deploying
- 💰 **Actual Costs** - See what you're actually paying
- 🔌 **Multiple Cost Sources** - Works with Vantage, local specs, and more
- 🎯 **Flexible Filtering** - Filter by resource type, tags, or custom criteria
- 📈 **Cost Aggregation** - Group costs by provider, type, date, or tags
- 📱 **Multiple Formats** - Table, JSON, or NDJSON output

---

## Installation

### Prerequisites

- **Pulumi CLI** installed and working
- **Go 1.25.7+** (if building from source)
- **Cloud credentials** configured (AWS, Azure, GCP, etc.)

### Option 1: Download Binary (Recommended)

Coming soon - prebuilt binaries for Linux, macOS, and Windows.

### Option 2: Build from Source

```bash
git clone https://github.com/rshade/finfocus
cd finfocus
make build
./bin/finfocus --help
```

### Verify Installation

```bash
finfocus --version
finfocus --help
```

---

## Quick Start

### 1. Generate Pulumi Plan

```bash
cd your-pulumi-project
pulumi preview --json > plan.json
```

### 2. View Projected Costs

```bash
finfocus cost projected --pulumi-json plan.json
```

**Output:**

```text
RESOURCE                          TYPE                MONTHLY   CURRENCY
aws:ec2/instance:Instance         aws:ec2:Instance    $7.50     USD
aws:s3/bucket:Bucket              aws:s3:Bucket       $0.00     USD
aws:rds/instance:Instance         aws:rds:Instance    $0.00     USD

Total: $7.50 USD
```

### 3. (Optional) View Actual Costs

Requires plugin configuration. See [Configuration](#configuration).

```bash
finfocus cost actual --pulumi-json plan.json --from 2024-01-01
```

---

## Cost Types

### Projected Costs

**What it is:** Estimated costs based on your infrastructure definitions

**When to use:**

- Before deploying infrastructure
- During planning and design phases
- Comparing different infrastructure options

**Command:**

```bash
finfocus cost projected --pulumi-json plan.json
```

### Actual Costs

**What it is:** Real costs from your cloud provider's billing system

**When to use:**

- After infrastructure is deployed and running
- Cost optimization and analysis
- Budget tracking and reporting

**Command:**

```bash
finfocus cost actual --pulumi-json plan.json --from 2024-01-01 --to 2024-01-31
```

**Note:** Requires plugin setup (Vantage, Kubecost, etc.)

---

## Automatic Pulumi Integration

When you run `finfocus cost projected` or `finfocus cost actual` inside a Pulumi
project directory, FinFocus automatically detects the project and runs the appropriate
Pulumi CLI command. No flags required.

### Simplified Workflow

```bash
# Just cd into your Pulumi project and run
cd my-pulumi-project/
finfocus cost projected

# For actual costs
finfocus cost actual
```

This replaces the manual two-step workflow:

```bash
# Old workflow (still supported)
pulumi preview --json > plan.json
finfocus cost projected --pulumi-json plan.json
```

### How Auto-Detection Works

1. FinFocus looks for `Pulumi.yaml` or `Pulumi.yml` in the current directory and
   parent directories
2. It verifies the `pulumi` CLI is available in your PATH
3. It determines the active stack (or uses `--stack` if provided)
4. For projected costs: runs `pulumi preview --json`
5. For actual costs: runs `pulumi stack export` and auto-detects the date range from
   resource timestamps

### Stack Selection

Use `--stack` to target a specific stack without changing your active stack:

```bash
finfocus cost projected --stack production
finfocus cost actual --stack staging
```

### Precedence Rules

- Explicit file flags always take priority: `--pulumi-json` or `--pulumi-state`
- When no file flag is provided, auto-detection is attempted
- `--stack` only applies during auto-detection (ignored with file flags)

### Requirements

- The `pulumi` CLI must be installed and available in PATH
- You must be inside a Pulumi project directory (or a subdirectory)
- Environment variables for your cloud provider and Pulumi backend must be configured

### Auto-Detection Errors

If auto-detection fails, you'll see actionable error messages:

- **"pulumi CLI not found"**: Install from <https://www.pulumi.com/docs/install/>
- **"no Pulumi project found"**: Run from a directory containing `Pulumi.yaml`
- **"no active Pulumi stack"**: Use `--stack` to specify one, or run `pulumi stack select`

---

## Zero-Click Cost Estimation (Analyzer)

FinFocus can integrate directly with the Pulumi CLI as an Analyzer, providing instant
cost estimates during `pulumi preview`. This eliminates the need for a separate
`finfocus` command to see projected costs.

For detailed setup instructions, refer to the
[Analyzer Setup Guide](../getting-started/analyzer-setup.md).

---

### Cross-Provider Aggregation

FinFocus supports aggregating costs across multiple cloud providers and services, allowing
you to get a holistic view of your infrastructure spending. This feature is particularly
powerful when combining actual cost data from various plugins.

### Daily Cost Trends

View daily cost trends across all configured providers for a specific period:

```bash
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --to 2025-01-31 --group-by daily
```

### Monthly Comparison

Generate a monthly cost comparison. You can output this as JSON for further processing:

```bash
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --group-by monthly --output json
```

## Sustainability Metrics

FinFocus now supports sustainability metrics, allowing you to estimate the carbon footprint of your infrastructure.

### Carbon Footprint

The table output includes a "CO₂" column showing the estimated carbon emissions for supported resources.

**Example Output:**

```text
RESOURCE                      TYPE              MONTHLY   CURRENCY  CO₂
aws:ec2/instance:Instance     aws:ec2:Instance  $7.50     USD       12.5 kg
```

### Utilization Rate

You can adjust the assumed utilization rate for sustainability calculations using the
`--utilization` flag. The default is 1.0 (100%).

```bash
finfocus cost projected --pulumi-json plan.json --utilization 0.8
```

---

## Common Workflows

### 1. Check Cost Before Deploying

```bash
# Generate plan
pulumi preview --json > plan.json

# Check projected costs
finfocus cost projected --pulumi-json plan.json

# Review output and make decisions
```

### 2. Compare Costs of Different Configurations

```bash
# Try one configuration
pulumi preview --json > config1.json
finfocus cost projected --pulumi-json config1.json

# Switch configuration
# ... modify Pulumi code ...

# Try another configuration
pulumi preview --json > config2.json
finfocus cost projected --pulumi-json config2.json

# Compare outputs
```

### 3. Track Historical Spending

```bash
# View last 7 days
finfocus cost actual --from 2024-01-24

# View last month
finfocus cost actual --from 2024-01-01 --to 2024-01-31

# View by day
finfocus cost actual --from 2024-01-01 --to 2024-01-31 --group-by daily
```

### 4. Find Expensive Resources

```bash
# Sort by cost (output shows highest first)
finfocus cost projected --pulumi-json plan.json --output json | jq '.resources | sort_by(.estimatedCost) | reverse'

# Or filter to specific resource type
finfocus cost projected --pulumi-json plan.json --filter "type=aws:rds*"
```

### 5. Cost by Environment

```bash
# Assuming resources are tagged with 'env' tag
finfocus cost actual --filter "tag:env=prod" --from 2024-01-01

finfocus cost actual --filter "tag:env=dev" --from 2024-01-01
```

---

## Configuration

### Using Vantage Plugin

Vantage provides unified cost data from multiple cloud providers.

**Setup:**

1. Get Vantage API key from [Vantage](https://vantage.sh)
2. Configure plugin (see [Vantage Plugin Setup](../plugins/vantage/setup.md))
3. Run commands with Vantage data

**Commands:**

```bash
finfocus cost actual --from 2024-01-01 --to 2024-01-31
```

### Using Local Pricing Specs

Use local YAML files for cost estimates without external services.

**Setup:**

1. Create YAML spec file: `~/.finfocus/specs/my-specs.yaml`
2. Add resource pricing definitions
3. FinFocus automatically uses them

**Example spec file:**

```yaml
---
resources:
  aws:ec2/instance:Instance:
    t3.micro:
      monthly: 7.50
      currency: USD
      notes: Linux on-demand
    t3.small:
      monthly: 15.00
      currency: USD
```

---

## Budget Management

FinFocus allows you to set spending limits and receive alerts when your cloud costs
approach or exceed configured thresholds. This helps you stay in control of your
infrastructure spending.

### Configuring a Budget

Add a budget configuration to your `~/.finfocus/config.yaml`:

```yaml
cost:
  budgets:
    amount: 1000.00
    currency: USD
    period: monthly
    alerts:
      - threshold: 50
        type: actual
      - threshold: 80
        type: actual
      - threshold: 100
        type: forecasted
```

**Configuration Fields:**

| Field      | Description                      | Example      |
| ---------- | -------------------------------- | ------------ |
| `amount`   | Total spend limit for the period | `1000.00`    |
| `currency` | ISO 4217 currency code           | `USD`, `EUR` |
| `period`   | Budget period (default: monthly) | `monthly`    |
| `alerts`   | List of threshold alerts         | See below    |

**Alert Configuration:**

| Field       | Description                              | Values                 |
| ----------- | ---------------------------------------- | ---------------------- |
| `threshold` | Percentage of budget that triggers alert | `0` - `1000`           |
| `type`      | What spend to check against threshold    | `actual`, `forecasted` |

### Alert Types

**Actual Alerts** (`type: actual`)

Triggers when your current spending exceeds the threshold percentage of your budget.

```yaml
alerts:
  - threshold: 80
    type: actual # Alert when actual spend reaches 80% of budget
```

**Forecasted Alerts** (`type: forecasted`)

Triggers when your projected end-of-period spend exceeds the threshold. Uses linear
extrapolation based on your current daily spending rate.

```yaml
alerts:
  - threshold: 100
    type: forecasted # Alert when forecasted spend will exceed budget
```

### Viewing Budget Status

When a budget is configured, FinFocus automatically displays budget status after
cost calculations:

**Terminal Output (TTY):**

```text
╭──────────────────────────────────────────╮
│ BUDGET STATUS                            │
│ ────────────────────────────────────     │
│                                          │
│ Budget: $1,000.00/monthly                │
│ Current Spend: $850.00 (85.0%)           │
│                                          │
│ ██████████████████████████░░░░ 85%       │
│                                          │
│ ⚠ WARNING - spend exceeds 80% threshold  │
│                                          │
│ Forecasted: $1,240.00 (124.0%)           │
╰──────────────────────────────────────────╯
```

**CI/CD Output (Non-TTY):**

```text
BUDGET STATUS
=============
Budget: $1000.00/monthly
Current Spend: $850.00 (85.0%)
Status: WARNING - Exceeds 80% threshold
Forecasted: $1240.00 (124.0%)
```

### Progress Bar Colors

The progress bar color indicates budget health:

| Color  | Meaning                        |
| ------ | ------------------------------ |
| Green  | Under 80% of budget            |
| Yellow | Between 80% and 100% of budget |
| Red    | Over 100% of budget            |

### Alert Status Levels

| Status      | Indicator | Description                           |
| ----------- | --------- | ------------------------------------- |
| OK          | (none)    | Spend is below threshold              |
| APPROACHING | ◉         | Within 5% of threshold (e.g., 75-80%) |
| EXCEEDED    | ⚠         | At or above threshold                 |

### Forecasting Logic

Forecasted spend uses linear extrapolation:

```text
Forecasted Spend = (Current Spend / Current Day) × Total Days in Month
```

**Example:**

- Current day: 15th of a 31-day month
- Current spend: $600
- Daily rate: $600 / 15 = $40/day
- Forecasted spend: $40 × 31 = $1,240

### Common Budget Configurations

**Basic Budget with Single Alert:**

```yaml
cost:
  budgets:
    amount: 500.00
    currency: USD
    alerts:
      - threshold: 80
        type: actual
```

**Comprehensive Budget with Multiple Alerts:**

```yaml
cost:
  budgets:
    amount: 2000.00
    currency: USD
    period: monthly
    alerts:
      - threshold: 50
        type: actual # Heads-up at 50%
      - threshold: 80
        type: actual # Warning at 80%
      - threshold: 100
        type: actual # Critical at 100%
      - threshold: 100
        type: forecasted # Proactive: warn if forecast exceeds budget
```

**Proactive Forecasting Only:**

```yaml
cost:
  budgets:
    amount: 1000.00
    currency: USD
    alerts:
      - threshold: 90
        type: forecasted # Warn if forecast will hit 90%
      - threshold: 100
        type: forecasted # Critical if forecast will exceed budget
```

### Disabling Budgets

Set the amount to 0 to disable budget tracking:

```yaml
cost:
  budgets:
    amount: 0 # Budget disabled
```

### Multi-Currency Support

FinFocus supports multiple currencies for budget display:

| Currency | Symbol |
| -------- | ------ |
| USD      | $      |
| EUR      | €      |
| GBP      | £      |
| JPY      | ¥      |
| CAD      | C$     |
| AUD      | A$     |
| CHF      | CHF    |

**Note:** Budget currency must match your actual cost currency. A currency mismatch
will result in an error.

---

## Output Formats

### Table (Default)

```bash
finfocus cost projected --pulumi-json plan.json
```

**Output:**

```text
RESOURCE                      TYPE              MONTHLY   CURRENCY
aws:ec2/instance:Instance     aws:ec2:Instance  $7.50     USD
aws:s3/bucket:Bucket          aws:s3:Bucket     $0.00     USD
```

### JSON

```bash
finfocus cost projected --pulumi-json plan.json --output json
```

**Output:**

```json
{
  "summary": {
    "totalMonthly": 7.5,
    "currency": "USD"
  },
  "resources": [
    {
      "type": "aws:ec2:Instance",
      "estimatedCost": 7.5,
      "currency": "USD"
    }
  ]
}
```

### NDJSON (Newline-Delimited JSON)

Useful for streaming and pipeline processing.

```bash
finfocus cost projected --pulumi-json plan.json --output ndjson
```

**Output:**

```text
{"type": "aws:ec2:Instance", "estimatedCost": 7.50}
{"type": "aws:s3:Bucket", "estimatedCost": 0.00}
```

---

## Filtering and Grouping

### Filtering by Resource Type

```bash
# EC2 instances only
finfocus cost projected --pulumi-json plan.json --filter "type=aws:ec2*"

# RDS databases
finfocus cost projected --pulumi-json plan.json --filter "type=aws:rds*"
```

### Filtering by Tags

```bash
# Production resources
finfocus cost actual --filter "tag:env=prod" --from 2024-01-01

# Team resources
finfocus cost actual --filter "tag:team=platform" --from 2024-01-01

# Multiple conditions
finfocus cost actual --filter "tag:env=prod AND tag:team=platform" --from 2024-01-01
```

### Grouping by Dimension

```bash
# By provider (AWS, Azure, GCP)
finfocus cost actual --group-by provider --from 2024-01-01

# By resource type
finfocus cost actual --group-by type --from 2024-01-01

# By date (daily breakdown)
finfocus cost actual --group-by daily --from 2024-01-01 --to 2024-01-31

# By tag
finfocus cost actual --group-by "tag:env" --from 2024-01-01
```

---

## Debugging and Logging

### Using Debug Mode

FinFocus includes a `--debug` flag that enables verbose logging to help troubleshoot issues:

```bash
# Enable debug output for any command
finfocus cost projected --debug --pulumi-json plan.json

# Debug output shows:
# - Command start/stop with duration
# - Resource ingestion details
# - Plugin lookup attempts
# - Cost calculation decisions
# - Fallback behavior when plugins don't return data
```

**Example Debug Output:**

```text
2025-01-15T10:30:45Z INF command started command=projected trace_id=01HQ7X2J3K4M5N6P7Q8R9S0T1U component=cli
2025-01-15T10:30:45Z DBG loading Pulumi plan plan_path=plan.json component=ingest
2025-01-15T10:30:45Z DBG extracted 3 resources from plan component=ingest
2025-01-15T10:30:45Z DBG querying plugin for projected cost resource_type=aws:ec2:Instance plugin=vantage component=engine
2025-01-15T10:30:46Z DBG plugin returned cost data monthly_cost=7.50 component=engine
2025-01-15T10:30:46Z INF projected cost calculation complete result_count=3 duration_ms=245 component=engine
```

### Environment Variables for Logging

Configure logging behavior via environment variables:

```bash
# Set log level (trace, debug, info, warn, error)
export FINFOCUS_LOG_LEVEL=debug

# Set log format (json, text, console)
export FINFOCUS_LOG_FORMAT=json

# Inject external trace ID for correlation with other systems
export FINFOCUS_TRACE_ID=my-pipeline-trace-12345

# Example: Debug with JSON format for log aggregation
FINFOCUS_LOG_LEVEL=debug FINFOCUS_LOG_FORMAT=json \
  finfocus cost projected --pulumi-json plan.json 2> debug.log
```

### Configuration Precedence

Log settings are applied in this order (highest priority first):

1. **CLI flags** (`--debug`)
2. **Environment variables** (`FINFOCUS_LOG_LEVEL`)
3. **Config file** (`~/.finfocus/config.yaml`)
4. **Defaults** (info level, text format)

### Trace ID for Debugging

Every command generates a unique trace ID that appears in all log entries.
This helps correlate log entries for a single operation:

```bash
# Use external trace ID for pipeline correlation
FINFOCUS_TRACE_ID=jenkins-build-123 finfocus cost projected --debug --pulumi-json plan.json

# All logs will include: trace_id=jenkins-build-123
```

---

## Logging Configuration

FinFocus provides comprehensive logging capabilities for debugging, monitoring, and auditing.

### Configuration File

Create or edit `~/.finfocus/config.yaml` to configure logging:

```yaml
logging:
  # Log level: trace, debug, info, warn, error (default: info)
  level: info

  # Log format: json, text, console (default: console)
  format: json

  # Log to file (optional - defaults to stderr)
  file: /var/log/finfocus/finfocus.log

  # Audit logging for compliance (optional)
  audit:
    enabled: true
    file: /var/log/finfocus/audit.log
```

### Log Output Locations

**Default Behavior:**

- Without configuration: logs go to stderr in console format
- With `--debug` flag: forces debug level, console format, and stderr output

**File Logging:**

When file logging is configured, FinFocus displays the log location at startup:

```bash
$ finfocus cost projected --pulumi-json plan.json
Logging to: /var/log/finfocus/finfocus.log
COST SUMMARY
============
...
```

**Fallback Behavior:**

If the configured log file cannot be written (permissions, disk full), FinFocus:

1. Falls back to stderr
2. Displays a warning with the reason

```bash
$ finfocus cost projected --pulumi-json plan.json
Warning: Could not write to log file, falling back to stderr (permission denied)
COST SUMMARY
============
...
```

### Audit Logging

Audit logging tracks all cost queries for compliance and analysis.

**Enable Audit Logging:**

```yaml
logging:
  audit:
    enabled: true
    file: /var/log/finfocus/audit.log
```

**Audit Log Entry Example:**

```json
{
  "time": "2025-01-15T10:30:45Z",
  "level": "info",
  "audit": true,
  "command": "cost projected",
  "trace_id": "01HQ7X2J3K4M5N6P7Q8R9S0T1U",
  "duration_ms": 245,
  "success": true,
  "result_count": 3,
  "total_cost": 7.5,
  "parameters": {
    "pulumi_json": "plan.json",
    "output": "table"
  }
}
```

**Audit Entry Fields:**

| Field          | Description                                                  |
| -------------- | ------------------------------------------------------------ |
| `command`      | CLI command executed (e.g., "cost projected", "cost actual") |
| `trace_id`     | Unique request identifier for correlation                    |
| `duration_ms`  | Command execution time in milliseconds                       |
| `success`      | Whether the command completed successfully                   |
| `result_count` | Number of resources processed                                |
| `total_cost`   | Sum of all calculated costs                                  |
| `parameters`   | Command parameters (sensitive values redacted)               |

**Security:** Sensitive parameter values (API keys, passwords, tokens) are automatically redacted in audit logs.

### Log Rotation

FinFocus does not perform log rotation internally. Use external tools:

**Linux (logrotate):**

```text
# /etc/logrotate.d/finfocus
/var/log/finfocus/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
}
```

**systemd journald:**

If running as a service, logs go to journald automatically:

```bash
journalctl -u finfocus --since today
```

---

## Troubleshooting

### "No cost data available"

**Problem:** No pricing information found for resources

**Solutions:**

- Check if plugin is configured correctly
- Verify API credentials are valid
- Some resources may not have pricing data - this is normal
- Check troubleshooting guide: [Troubleshooting](../support/troubleshooting.md)

### "Invalid date format"

**Problem:** Date format not recognized

**Solutions:**

- Use format: `YYYY-MM-DD` (e.g., `2024-01-01`)
- Or RFC3339: `2024-01-01T00:00:00Z`
- Example: `--from 2024-01-01 --to 2024-01-31`

### "Plugin not found"

**Problem:** Cost source plugin not installed

**Solutions:**

```bash
# List installed plugins
finfocus plugin list

# Validate installations
finfocus plugin validate

# See plugin setup guide for your cost source
# - Vantage: docs/plugins/vantage/setup.md
```

### Getting Help

- **FAQ:** [Frequently Asked Questions](../support/faq.md)
- **Troubleshooting:** [Detailed Troubleshooting Guide](../support/troubleshooting.md)
- **Report Issue:** [GitHub Issues](https://github.com/rshade/finfocus/issues)

---

## Next Steps

- **Quick Start:** [5-Minute Quickstart](../getting-started/quickstart.md)
- **Installation:** [Detailed Installation Guide](../getting-started/installation.md)
- **Vantage Setup:** [Setting up Vantage Plugin](../plugins/vantage/setup.md)
- **CLI Reference:** [Complete CLI Commands](../reference/cli-commands.md)
- **Examples:** [Practical Examples](../getting-started/examples/)

---

**Last Updated:** 2025-10-29
