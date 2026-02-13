---
layout: default
title: CLI Commands Reference
description: Complete reference for all FinFocus CLI commands
---

Complete command reference for FinFocus.

## Commands Overview

```bash
finfocus                    # Help
finfocus overview           # Unified cost dashboard
finfocus cost               # Cost commands
finfocus cost projected     # Estimate costs from plan
finfocus cost actual        # Get actual historical costs
finfocus cost estimate      # What-if cost analysis
finfocus cost recommendations          # Get cost optimization recommendations
finfocus cost recommendations dismiss  # Dismiss a recommendation
finfocus cost recommendations snooze   # Snooze a recommendation
finfocus cost recommendations undismiss # Re-enable a dismissed recommendation
finfocus cost recommendations history  # View recommendation lifecycle history
finfocus config             # Configuration commands
finfocus config validate    # Validate routing configuration
finfocus plugin             # Plugin commands
finfocus plugin init        # Initialize a new plugin
finfocus plugin install     # Install a plugin
finfocus plugin update      # Update a plugin
finfocus plugin remove      # Remove a plugin
finfocus plugin list        # List installed plugins
finfocus plugin inspect     # Inspect plugin capabilities
finfocus plugin validate    # Validate plugin setup
finfocus plugin conformance # Run conformance tests
finfocus plugin certify     # Run certification tests
finfocus analyzer           # Analyzer commands
finfocus analyzer serve     # Start the analyzer gRPC server
```

## cost projected

Calculate estimated costs from Pulumi plan.

### Usage (cost projected)

```bash
finfocus cost projected --pulumi-json <file> [options]
```

### Options (cost projected)

| Flag            | Description                               | Default  |
| --------------- | ----------------------------------------- | -------- |
| `--pulumi-json` | Path to Pulumi preview JSON               | Required |
| `--filter`      | Filter resources (tag:key=value, type=\*) | None     |
| `--output`      | Output format: table, json, ndjson        | table    |
| `--utilization` | Assumed resource utilization (0.0-1.0)    | 1.0      |
| `--help`        | Show help                                 |          |

### Examples (cost projected)

```bash
# Basic usage
finfocus cost projected --pulumi-json plan.json

# JSON output
finfocus cost projected --pulumi-json plan.json --output json

# Filter by type
finfocus cost projected --pulumi-json plan.json --filter "type=aws:ec2*"

# NDJSON for pipelines
finfocus cost projected --pulumi-json plan.json --output ndjson
```

## cost recommendations

Display cost optimization recommendations from cloud providers.

### Usage (cost recommendations)

```bash
finfocus cost recommendations --pulumi-json <file> [options]
```

### Options (cost recommendations)

| Flag                  | Description                                                      | Default  |
| --------------------- | ---------------------------------------------------------------- | -------- |
| `--pulumi-json`       | Path to Pulumi preview JSON                                      | Required |
| `--filter`            | Filter expression (e.g., `action=RIGHTSIZE,TERMINATE`)           | None     |
| `--output`            | Output format: table, json, ndjson                               | table    |
| `--limit`             | Limit number of recommendations                                  | 0 (all)  |
| `--verbose`           | Show all recommendations with full details                       | false    |
| `--include-dismissed` | Show dismissed and snoozed recommendations alongside active ones | false    |
| `--sort`              | Sort expression (e.g., `savings:desc`)                           | None     |
| `--help`              | Show help                                                        |          |

### Subcommands (cost recommendations)

| Subcommand  | Description                                 |
| ----------- | ------------------------------------------- |
| `dismiss`   | Permanently dismiss a recommendation        |
| `snooze`    | Snooze a recommendation until a date        |
| `undismiss` | Re-enable a dismissed recommendation        |
| `history`   | View lifecycle history for a recommendation |

### Examples (cost recommendations)

```bash
# Interactive mode (default)
finfocus cost recommendations --pulumi-json plan.json

# Filter by action type
finfocus cost recommendations --pulumi-json plan.json --filter "action=RIGHTSIZE,TERMINATE"

# JSON output
finfocus cost recommendations --pulumi-json plan.json --output json

# Include dismissed and snoozed recommendations
finfocus cost recommendations --pulumi-json plan.json --include-dismissed
```

## cost recommendations dismiss

Permanently dismiss a recommendation with a reason.

### Usage (cost recommendations dismiss)

```bash
finfocus cost recommendations dismiss <recommendation-id> [options]
```

### Options (cost recommendations dismiss)

| Flag            | Description                                            | Default  |
| --------------- | ------------------------------------------------------ | -------- |
| `-r, --reason`  | Dismissal reason (required)                            | Required |
| `-n, --note`    | Free-text explanation (required for `other` reason)    | None     |
| `-f, --force`   | Skip confirmation prompt                               | false    |
| `--pulumi-json` | Path to Pulumi preview JSON (for plugin communication) | None     |
| `--adapter`     | Use specific adapter plugin                            | None     |

### Valid Reasons

| Reason                 | Description                          |
| ---------------------- | ------------------------------------ |
| `not-applicable`       | Recommendation doesn't apply         |
| `already-implemented`  | Already acted on this recommendation |
| `business-constraint`  | Business requirements prevent action |
| `technical-constraint` | Technical limitations prevent action |
| `deferred`             | Will address later                   |
| `inaccurate`           | Recommendation data is wrong         |
| `other`                | Custom reason (requires `--note`)    |

### Examples (cost recommendations dismiss)

```bash
# Dismiss with a reason
finfocus cost recommendations dismiss rec-123abc \
  --reason business-constraint --pulumi-json plan.json

# Dismiss with a custom note
finfocus cost recommendations dismiss rec-123abc \
  --reason other --note "Intentional oversizing" --pulumi-json plan.json

# Skip confirmation prompt
finfocus cost recommendations dismiss rec-123abc \
  --reason not-applicable --force --pulumi-json plan.json
```

## cost recommendations snooze

Temporarily dismiss a recommendation until a future date.

### Usage (cost recommendations snooze)

```bash
finfocus cost recommendations snooze <recommendation-id> [options]
```

### Options (cost recommendations snooze)

| Flag            | Description                                                                | Default    |
| --------------- | -------------------------------------------------------------------------- | ---------- |
| `--until`       | Snooze until date (required, YYYY-MM-DD or RFC3339; must be in the future) | Required   |
| `-r, --reason`  | Dismissal reason                                                           | `deferred` |
| `-n, --note`    | Free-text explanation                                                      | None       |
| `-f, --force`   | Skip confirmation prompt                                                   | false      |
| `--pulumi-json` | Path to Pulumi preview JSON (for plugin communication)                     | None       |
| `--adapter`     | Use specific adapter plugin                                                | None       |

### Examples (cost recommendations snooze)

```bash
# Snooze until a specific date (replace with a future date)
finfocus cost recommendations snooze rec-456def \
  --until YYYY-MM-DD --pulumi-json plan.json

# Snooze with reason and note (replace with a future date)
finfocus cost recommendations snooze rec-456def \
  --until YYYY-MM-DD --reason deferred \
  --note "Scheduled for Q2 review" --pulumi-json plan.json
```

## cost recommendations undismiss

Re-enable a previously dismissed or snoozed recommendation.

### Usage (cost recommendations undismiss)

```bash
finfocus cost recommendations undismiss <recommendation-id> [options]
```

### Options (cost recommendations undismiss)

| Flag          | Description              | Default |
| ------------- | ------------------------ | ------- |
| `-f, --force` | Skip confirmation prompt | false   |

### Examples (cost recommendations undismiss)

```bash
# Re-enable a dismissed recommendation
finfocus cost recommendations undismiss rec-123abc
```

## cost recommendations history

View the lifecycle history of a specific recommendation.

### Usage (cost recommendations history)

```bash
finfocus cost recommendations history <recommendation-id> [options]
```

### Options (cost recommendations history)

| Flag       | Description                        | Default |
| ---------- | ---------------------------------- | ------- |
| `--output` | Output format: table, json, ndjson | table   |

### Examples (cost recommendations history)

```bash
# View history in table format
finfocus cost recommendations history rec-123abc

# View history as JSON
finfocus cost recommendations history rec-123abc --output json

# View history as NDJSON (one JSON object per line)
finfocus cost recommendations history rec-123abc --output ndjson
```

## cost actual

Get actual historical costs from plugins.

### Usage (cost actual)

```bash
finfocus cost actual [options]
```

### Options (cost actual)

| Flag                    | Description                                              | Default    |
| ----------------------- | -------------------------------------------------------- | ---------- |
| `--pulumi-json`         | Path to Pulumi preview JSON                              |            |
| `--pulumi-state`        | Path to Pulumi state JSON from `pulumi stack export`     |            |
| `--from`                | Start date (YYYY-MM-DD or RFC3339)                       | 7 days ago |
| `--to`                  | End date (YYYY-MM-DD or RFC3339)                         | Today      |
| `--filter`              | Filter resources (tag:key=value, type=\*)                | None       |
| `--group-by`            | Group results (resource, type, provider, daily, monthly) | resource   |
| `--output`              | Output format: table, json, ndjson                       | table      |
| `--estimate-confidence` | Show confidence level for cost estimates                 | false      |
| `--help`                | Show help                                                |            |

### Confidence Levels

When `--estimate-confidence` is enabled, a Confidence column appears showing data reliability:

| Level  | Description                                                     |
| ------ | --------------------------------------------------------------- |
| HIGH   | Real billing data from plugin (AWS Cost Explorer, Kubecost)     |
| MEDIUM | Runtime estimate for Pulumi-created resources                   |
| LOW    | Runtime estimate for imported resources (creation time unknown) |

### Examples (cost actual)

```bash
# Estimate costs from Pulumi state (--from auto-detected from timestamps)
finfocus cost actual --pulumi-state state.json

# Estimate costs from state with explicit date range
finfocus cost actual --pulumi-state state.json --from 2025-01-01 --to 2025-01-31

# Get costs from Pulumi plan
finfocus cost actual --pulumi-json plan.json --from 2025-01-01

# Group by day
finfocus cost actual --pulumi-json plan.json --group-by daily --from 2025-01-01 --to 2025-01-31

# Group by provider
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --group-by provider

# Filter by tag
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --filter "tag:env=prod"

# JSON output
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --output json

# Show estimate confidence levels (useful for imported resources)
finfocus cost actual --pulumi-state state.json --estimate-confidence
```

## cost estimate

Perform what-if cost analysis on resources without modifying Pulumi code.

### Usage (cost estimate)

```bash
finfocus cost estimate [options]
```

### Modes

The command supports two mutually exclusive modes:

**Single-Resource Mode:**

- Specify `--provider` and `--resource-type` to estimate cost for a single resource
- Use `--property` to specify property overrides (repeatable)

**Plan-Based Mode:**

- Specify `--pulumi-json` to load resources from a Pulumi plan
- Use `--modify` to apply modifications to specific resources

### Options (cost estimate)

| Flag              | Description                              | Default |
| ----------------- | ---------------------------------------- | ------- |
| `--provider`      | Cloud provider (aws, gcp, azure)         |         |
| `--resource-type` | Resource type (e.g., ec2:Instance)       |         |
| `--property`      | Property override key=value (repeatable) |         |
| `--pulumi-json`   | Path to Pulumi preview JSON              |         |
| `--modify`        | Resource modification resource:key=value |         |
| `--region`        | Region for cost calculation              |         |
| `--interactive`   | Launch interactive TUI mode              | false   |
| `--output`        | Output format: table, json, ndjson       | table   |
| `--adapter`       | Specific plugin adapter to use           |         |
| `--help`          | Show help                                |         |

### Examples (cost estimate)

```bash
# Single resource estimation - estimate cost of changing instance type
finfocus cost estimate --provider aws --resource-type ec2:Instance \
  --property instanceType=m5.large

# Single resource with region
finfocus cost estimate --provider aws --resource-type ec2:Instance \
  --property instanceType=m5.large --region us-west-2

# Plan-based estimation - modify a specific resource in existing plan
finfocus cost estimate --pulumi-json plan.json \
  --modify "web-server:instanceType=m5.large"

# Plan-based with multiple modifications
finfocus cost estimate --pulumi-json plan.json \
  --modify "web-server:instanceType=m5.large" \
  --modify "api-server:instanceType=c5.xlarge"

# Interactive TUI mode
finfocus cost estimate --interactive

# Interactive mode with plan
finfocus cost estimate --pulumi-json plan.json --interactive

# JSON output for scripting
finfocus cost estimate --provider aws --resource-type ec2:Instance \
  --property instanceType=m5.large --output json
```

### Output Example

```text
What-If Cost Analysis
=====================

Resource: ec2:Instance (aws)
ID: estimate-resource

Baseline:  $8.32/mo (USD)
Modified:  $83.22/mo (USD)

Change:    +$74.90/mo

Property Changes:
-----------------
  instanceType: t3.micro -> m5.large (+$74.90/mo)
```

### Interactive Mode

The interactive TUI mode allows you to:

- Navigate through resource properties with arrow keys
- Edit property values inline (press Enter to edit)
- See live cost updates as you modify properties
- Press 'q' or Ctrl+C to exit

## config validate

Validate routing configuration for errors and warnings.

### Usage (config validate)

```bash
finfocus config validate [options]
```

### Options (config validate)

| Flag     | Description |
| -------- | ----------- |
| `--help` | Show help   |

### Examples (config validate)

```bash
# Validate routing configuration
finfocus config validate

# Success output:
# ✓ Configuration valid
#
# Discovered plugins:
#   aws-ce: Recommendations, ActualCosts (priority: 20)
#   aws-public: ProjectedCosts, ActualCosts (priority: 10)
#
# Routing rules:
#   aws:eks:* → eks-costs (pattern, priority: 30)
#   aws:* → aws-public (provider, priority: 10)

# Error output:
# ✗ Configuration invalid
#
# Errors:
#   - aws-ce: plugin not found
#   - patterns[0].pattern: invalid regex: missing closing bracket
#
# Warnings:
#   - aws-public: feature 'Carbon' not supported by plugin
#   - eks-costs: duplicate plugin configuration found
```

## plugin init

Initialize a new FinFocus plugin project.

### Usage (plugin init)

```bash
finfocus plugin init <plugin-name> --author <name> --providers <list> [options]
```

### Options (plugin init)

| Flag          | Description                             | Default    |
| ------------- | --------------------------------------- | ---------- |
| `--author`    | Author name for the plugin              | (required) |
| `--providers` | Comma-separated list of cloud providers | (required) |
| `--help`      | Show help                               |            |

### Examples (plugin init)

```bash
# Initialize a new AWS plugin
finfocus plugin init my-aws-plugin --author "Your Name" --providers aws
```

## plugin install

Install a FinFocus plugin from a registry or URL.

### Usage (plugin install)

```bash
finfocus plugin install <plugin-name> [--version <version>] [--url <url>] [options]
```

### Options (plugin install)

| Flag        | Description                                        | Default           |
| ----------- | -------------------------------------------------- | ----------------- |
| `--version` | Specify plugin version to install                  | latest            |
| `--url`     | URL to plugin binary (for custom installs)         | (registry lookup) |
| `--force`   | Force overwrite existing plugin installation       | false             |
| `--clean`   | Remove all other versions after successful install | false             |
| `--no-save` | Don't add plugin to config file                    | false             |
| `--help`    | Show help                                          |                   |

### Examples (plugin install)

```bash
# Install the latest Vantage plugin
finfocus plugin install vantage

# Install a specific version of a plugin
finfocus plugin install kubecost --version 0.2.0

# Install and remove all other versions (cleanup disk space)
finfocus plugin install kubecost --clean

# Install from a custom URL
finfocus plugin install my-plugin --url https://example.com/my-plugin-0.1.0.tar.gz
```

## plugin update

Update an installed FinFocus plugin.

### Usage (plugin update)

```bash
finfocus plugin update <plugin-name> [options]
```

### Options (plugin update)

| Flag        | Description                                 | Default |
| ----------- | ------------------------------------------- | ------- |
| `--version` | Specify target version (defaults to latest) | latest  |
| `--all`     | Update all installed plugins                | false   |
| `--help`    | Show help                                   |         |

### Examples (plugin update)

```bash
# Update the Vantage plugin to the latest version
finfocus plugin update vantage

# Update all installed plugins
finfocus plugin update --all
```

## plugin remove

Remove an installed FinFocus plugin.

### Usage (plugin remove)

```bash
finfocus plugin remove <plugin-name> [options]
```

### Options (plugin remove)

| Flag     | Description                  | Default |
| -------- | ---------------------------- | ------- |
| `--all`  | Remove all installed plugins | false   |
| `--help` | Show help                    |         |

### Examples (plugin remove)

```bash
# Remove the Vantage plugin
finfocus plugin remove vantage

# Remove all installed plugins
finfocus plugin remove --all
```

## plugin list

List installed plugins with optional capability details.

### Usage (plugin list)

```bash
finfocus plugin list [options]
```

### Options (plugin list)

| Flag        | Description                                     | Default |
| ----------- | ----------------------------------------------- | ------- |
| `--verbose` | Show detailed plugin capabilities and providers | false   |
| `--help`    | Show help                                       |         |

### Examples (plugin list)

```bash
# List all plugins
finfocus plugin list

# Output:
# NAME      VERSION   SPEC    PATH
# vantage   0.1.0     0.4.14  /Users/me/.finfocus/plugins/vantage/v0.1.0/finfocus-plugin-vantage
# kubecost  0.2.0     0.4.14  /Users/me/.finfocus/plugins/kubecost/v0.2.0/finfocus-plugin-kubecost

# List with detailed capabilities (routing-aware)
finfocus plugin list --verbose

# Output:
# NAME        VERSION  PROVIDERS    CAPABILITIES                 SPEC    PATH
# aws-public  1.0.0    [aws]        ProjectedCosts, ActualCosts  0.4.14  /Users/me/.finfocus/plugins/aws-public/v1.0.0/finfocus-plugin-aws-public
# aws-ce      1.0.0    [aws]        Recommendations, ActualCosts 0.4.14  /Users/me/.finfocus/plugins/aws-ce/v1.0.0/finfocus-plugin-aws-ce
# gcp-public  1.0.0    [gcp]        ProjectedCosts, ActualCosts  0.4.14  /Users/me/.finfocus/plugins/gcp-public/v1.0.0/finfocus-plugin-gcp-public
# eks-costs   0.5.0    [aws]        ProjectedCosts                 0.4.14  /Users/me/.finfocus/plugins/eks-costs/v0.5.0/finfocus-plugin-eks-costs
```

## plugin inspect

Inspect a plugin's capabilities and field mappings.

### Usage (plugin inspect)

```bash
finfocus plugin inspect <plugin-name> <resource-type> [options]
```

### Options (plugin inspect)

| Flag        | Description                       | Default |
| ----------- | --------------------------------- | ------- |
| `--version` | Specify plugin version to inspect | latest  |
| `--json`    | Output in JSON format             | false   |
| `--help`    | Show help                         |         |

### Examples (plugin inspect)

```bash
# Inspect field mappings for AWS EC2 Instance
finfocus plugin inspect aws-public aws:ec2/instance:Instance

# Output:
# Field Mappings:
# FIELD                STATUS     CONDITION
# -------------------- ---------- ------------------------------
# instanceType         MAPPED
# region               MAPPED
# tags                 IGNORED    Not used for pricing

# Inspect specific version
finfocus plugin inspect aws-public aws:ec2/instance:Instance --version v0.1.0

# Output as JSON
finfocus plugin inspect aws-public aws:ec2/instance:Instance --json
```

## plugin validate

Validate plugin installations.

### Usage (plugin validate)

```bash
finfocus plugin validate [options]
```

### Options (plugin validate)

| Flag     | Description |
| -------- | ----------- |
| `--help` | Show help   |

### Examples (plugin validate)

```bash
# Validate all plugins
finfocus plugin validate

# Output:
# vantage (0.1.0): OK
# kubecost (0.2.0): OK
```

## plugin conformance

Run conformance tests against a plugin binary to verify protocol compliance.

### Usage (plugin conformance)

```bash
finfocus plugin conformance <plugin-path> [options]
```

### Options (plugin conformance)

| Flag            | Description                                                            | Default |
| --------------- | ---------------------------------------------------------------------- | ------- |
| `--mode`        | Communication mode: tcp, stdio                                         | tcp     |
| `--verbosity`   | Output detail: quiet, normal, verbose, debug                           | normal  |
| `--output`      | Output format: table, json, junit                                      | table   |
| `--output-file` | Write output to file                                                   | stdout  |
| `--timeout`     | Global suite timeout                                                   | 5m      |
| `--category`    | Filter by category (repeatable): protocol, error, performance, context | all     |
| `--filter`      | Regex filter for test names                                            |         |
| `--help`        | Show help                                                              |         |

### Examples (plugin conformance)

```bash
# Basic conformance check
finfocus plugin conformance ./plugins/aws-cost

# Verbose output with JSON
finfocus plugin conformance --verbosity verbose --output json ./plugins/aws-cost

# Filter to protocol tests only
finfocus plugin conformance --category protocol ./plugins/aws-cost

# JUnit XML for CI
finfocus plugin conformance --output junit --output-file report.xml ./plugins/aws-cost

# Use stdio mode
finfocus plugin conformance --mode stdio ./plugins/aws-cost
```

## plugin certify

Run full certification tests and generate a certification report.

### Usage (plugin certify)

```bash
finfocus plugin certify <plugin-path> [options]
```

### Options (plugin certify)

| Flag           | Description                          | Default |
| -------------- | ------------------------------------ | ------- |
| `-o, --output` | Output file for certification report | stdout  |
| `--mode`       | Communication mode: tcp, stdio       | tcp     |
| `--timeout`    | Global certification timeout         | 10m     |
| `--help`       | Show help                            |         |

### Certification Requirements

A plugin is certified if all conformance tests pass:

- All protocol tests (Name, GetProjectedCost, GetActualCost)
- All error handling tests
- All context/timeout tests
- All performance tests

### Examples (plugin certify)

```bash
# Basic certification
finfocus plugin certify ./plugins/aws-cost

# Save report to file
finfocus plugin certify --output certification.md ./plugins/aws-cost

# Use stdio mode
finfocus plugin certify --mode stdio ./plugins/aws-cost

# Output:
# 🔍 Certifying plugin at ./plugins/aws-cost...
# Running conformance tests...
# ✅ CERTIFIED - Plugin passed all conformance tests
```

### Certification Report

The command generates a markdown report containing:

- Plugin name and version
- Certification status (CERTIFIED or FAILED)
- Test summary (total, passed, failed, skipped)
- List of issues (if any failed)

## analyzer serve

Starts the FinFocus analyzer gRPC server. This command is intended to be run by
the Pulumi CLI as part of the `pulumi preview` workflow, typically configured in
`Pulumi.yaml`.

### Usage (analyzer serve)

```bash
finfocus analyzer serve [options]
```

### Options (analyzer serve)

| Flag              | Description                                  | Default     |
| ----------------- | -------------------------------------------- | ----------- |
| `--logtostderr`   | Log messages to stderr rather than log files | false       |
| `--v`             | Log level for V-logging (verbose logging)    | 0           |
| `--pulumilogfile` | Pulumi log file name (internal use)          | (generated) |
| `--help`          | Show help                                    |             |

### Examples (analyzer serve)

```bash
# This command is typically not run directly by users.
# It's configured in Pulumi.yaml for zero-click cost estimation:
#
# plugins:
#   - path: finfocus
#     args: ["analyzer", "serve"]
```

## Global Options

```bash
finfocus [global options] command [command options]
```

| Option                 | Description                                  |
| ---------------------- | -------------------------------------------- |
| `--help`               | Show help                                    |
| `--version`            | Show version                                 |
| `--debug`              | Enable debug logging                         |
| `--verbose`            | Enable verbose output                        |
| `--no-color`           | Disable colored output                       |
| `--plain`              | Enable plain text mode (no TUI)              |
| `--high-contrast`      | Enable high contrast mode                    |
| `--skip-version-check` | Skip plugin spec version compatibility check |

## Date Formats

### Accepted Formats

```bash
# ISO 8601 (YYYY-MM-DD)
finfocus cost actual --from 2024-01-01

# RFC3339 (full timestamp)
finfocus cost actual --from 2024-01-01T00:00:00Z

# Relative (future)
finfocus cost actual --from "7 days ago"
```

## Output Formats

### Table (Default)

Human-readable table format:

```text
RESOURCE    TYPE       MONTHLY   CURRENCY
Instance1   ec2        $7.50     USD
Bucket1     s3         $0.50     USD
──────────────────────────────
Total                  $8.00     USD
```

### JSON

Machine-readable JSON format:

```json
{
  "summary": { "totalMonthly": 8.0, "currency": "USD" },
  "resources": [{ "name": "Instance1", "type": "ec2", "cost": 7.5 }]
}
```

### NDJSON

Newline-delimited JSON (one per line):

```text
{"name":"Instance1","type":"ec2","cost":7.50}
{"name":"Bucket1","type":"s3","cost":0.50}
```

## Exit Codes

| Code | Meaning           |
| ---- | ----------------- |
| 0    | Success           |
| 1    | General error     |
| 2    | Invalid arguments |

---

See [User Guide](../guides/user-guide.md) for workflow examples.
