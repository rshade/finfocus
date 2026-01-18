---
layout: default
title: CLI Commands Reference
description: Complete reference for all FinFocus CLI commands
---

Complete command reference for FinFocus.

## Commands Overview

```bash
finfocus                 # Help
finfocus cost            # Cost commands
finfocus cost projected  # Estimate costs from plan
finfocus cost actual     # Get actual historical costs
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
finfocus analyzer serve  # Start the analyzer gRPC server
```

## cost projected

Calculate estimated costs from Pulumi plan.

### Usage

```bash
finfocus cost projected --pulumi-json <file> [options]
```

### Options

| Flag            | Description                               | Default  |
| --------------- | ----------------------------------------- | -------- |
| `--pulumi-json` | Path to Pulumi preview JSON               | Required |
| `--filter`      | Filter resources (tag:key=value, type=\*) | None     |
| `--output`      | Output format: table, json, ndjson        | table    |
| `--utilization` | Assumed resource utilization (0.0-1.0)    | 1.0      |
| `--help`        | Show help                                 |          |

### Examples

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

## cost actual

Get actual historical costs from plugins.

### Usage

```bash
finfocus cost actual [options]
```

### Options

| Flag         | Description                                              | Default    |
| ------------ | -------------------------------------------------------- | ---------- |
| `--from`     | Start date (YYYY-MM-DD or RFC3339)                       | 7 days ago |
| `--to`       | End date (YYYY-MM-DD or RFC3339)                         | Today      |
| `--filter`   | Filter resources (tag:key=value, type=\*)                | None       |
| `--group-by` | Group results (resource, type, provider, daily, monthly) | resource   |
| `--output`   | Output format: table, json, ndjson                       | table      |
| `--help`     | Show help                                                |            |

### Examples

```bash
# Last 7 days
finfocus cost actual

# Specific date range
finfocus cost actual --from 2024-01-01 --to 2024-01-31

# By day
finfocus cost actual --group-by daily --from 2024-01-01 --to 2024-01-31

# By provider
finfocus cost actual --group-by provider

# Filter by tag
finfocus cost actual --filter "tag:env=prod"

# JSON output
finfocus cost actual --output json --from 2024-01-01
```

## plugin init

Initialize a new FinFocus plugin project.

### Usage

```bash
finfocus plugin init <plugin-name> --author <name> --providers <list> [options]
```

### Options

| Flag          | Description                             | Default    |
| ------------- | --------------------------------------- | ---------- |
| `--author`    | Author name for the plugin              | (required) |
| `--providers` | Comma-separated list of cloud providers | (required) |
| `--help`      | Show help                               |            |

### Examples

```bash
# Initialize a new AWS plugin
finfocus plugin init my-aws-plugin --author "Your Name" --providers aws
```

## plugin install

Install a FinFocus plugin from a registry or URL.

### Usage

```bash
finfocus plugin install <plugin-name> [--version <version>] [--url <url>] [options]
```

### Options

| Flag        | Description                                        | Default           |
| ----------- | -------------------------------------------------- | ----------------- |
| `--version` | Specify plugin version to install                  | latest            |
| `--url`     | URL to plugin binary (for custom installs)         | (registry lookup) |
| `--force`   | Force overwrite existing plugin installation       | false             |
| `--clean`   | Remove all other versions after successful install | false             |
| `--no-save` | Don't add plugin to config file                    | false             |
| `--help`    | Show help                                          |                   |

### Examples

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

### Usage

```bash
finfocus plugin update <plugin-name> [options]
```

### Options

| Flag        | Description                                 | Default |
| ----------- | ------------------------------------------- | ------- |
| `--version` | Specify target version (defaults to latest) | latest  |
| `--all`     | Update all installed plugins                | false   |
| `--help`    | Show help                                   |         |

### Examples

```bash
# Update the Vantage plugin to the latest version
finfocus plugin update vantage

# Update all installed plugins
finfocus plugin update --all
```

## plugin remove

Remove an installed FinFocus plugin.

### Usage

```bash
finfocus plugin remove <plugin-name> [options]
```

### Options

| Flag     | Description                  | Default |
| -------- | ---------------------------- | ------- |
| `--all`  | Remove all installed plugins | false   |
| `--help` | Show help                    |         |

### Examples

```bash
# Remove the Vantage plugin
finfocus plugin remove vantage

# Remove all installed plugins
finfocus plugin remove --all
```

## plugin list

List installed plugins.

### Usage

```bash
finfocus plugin list [options]
```

### Options

| Flag     | Description |
| -------- | ----------- |
| `--help` | Show help   |

### Examples

```bash
# List all plugins
finfocus plugin list

# Output:
# NAME      VERSION   SPEC    PATH
# vantage   0.1.0     0.4.14  /Users/me/.finfocus/plugins/vantage/v0.1.0/finfocus-plugin-vantage
# kubecost  0.2.0     0.4.14  /Users/me/.finfocus/plugins/kubecost/v0.2.0/finfocus-plugin-kubecost
```

## plugin inspect

Inspect a plugin's capabilities and field mappings.

### Usage

```bash
finfocus plugin inspect <plugin-name> <resource-type> [options]
```

### Options

| Flag        | Description                       | Default |
| ----------- | --------------------------------- | ------- |
| `--version` | Specify plugin version to inspect | latest  |
| `--json`    | Output in JSON format             | false   |
| `--help`    | Show help                         |         |

### Examples

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

### Usage

```bash
finfocus plugin validate [options]
```

### Options

| Flag     | Description |
| -------- | ----------- |
| `--help` | Show help   |

### Examples

```bash
# Validate all plugins
finfocus plugin validate

# Output:
# vantage (0.1.0): OK
# kubecost (0.2.0): OK
```

## plugin conformance

Run conformance tests against a plugin binary to verify protocol compliance.

### Usage

```bash
finfocus plugin conformance <plugin-path> [options]
```

### Options

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

### Examples

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

### Usage

```bash
finfocus plugin certify <plugin-path> [options]
```

### Options

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

### Examples

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

### Usage

```bash
finfocus analyzer serve [options]
```

### Options

| Flag              | Description                                  | Default     |
| ----------------- | -------------------------------------------- | ----------- |
| `--logtostderr`   | Log messages to stderr rather than log files | false       |
| `--v`             | Log level for V-logging (verbose logging)    | 0           |
| `--pulumilogfile` | Pulumi log file name (internal use)          | (generated) |
| `--help`          | Show help                                    |             |

### Examples

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
