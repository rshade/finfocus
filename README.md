# FinFocus

*Cloud cost analysis for Pulumi infrastructure*


[![CI](https://github.com/rshade/finfocus/actions/workflows/ci.yml/badge.svg)](https://github.com/rshade/finfocus/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/rshade/finfocus/graph/badge.svg)](https://codecov.io/gh/rshade/finfocus)
[![Go Report Card](https://goreportcard.com/badge/github.com/rshade/finfocus)](https://goreportcard.com/report/github.com/rshade/finfocus)
[![Go Version](https://img.shields.io/github/go-mod/go-version/rshade/finfocus)](https://github.com/rshade/finfocus/blob/main/go.mod)
[![Release](https://img.shields.io/github/v/release/rshade/finfocus)](https://github.com/rshade/finfocus/releases/latest)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)

**Cloud cost analysis for Pulumi infrastructure** - Calculate projected and actual infrastructure costs without modifying your Pulumi programs.

FinFocus is a CLI tool that analyzes Pulumi infrastructure definitions to provide accurate cost estimates, budget enforcement, and historical cost tracking through a flexible plugin-based architecture.

## Why FinFocus?

Cloud cost surprises are the norm. Teams deploy infrastructure with Pulumi but have no visibility into what it will cost until the bill arrives. FinFocus closes that gap:

- **Shift-left on costs** — See projected costs *before* you deploy, directly from `pulumi preview` output
- **No code changes required** — Works with any existing Pulumi project via JSON export
- **Budget guardrails** — Enforce spending limits in CI/CD pipelines with non-zero exit codes
- **Plugin architecture** — Swap pricing sources without changing your workflow
- **Single dashboard** — Interactive TUI combining actual spend, projected costs, drift analysis, and recommendations

## Key Features

- **🔭 [Unified Overview](docs/commands/overview.md)**: Interactive dashboard combining actual costs, projected costs, drift analysis, and recommendations in a single view
- **📊 [Projected Costs](docs/reference/cli-commands.md#cost-projected)**: Estimate monthly costs before deploying infrastructure
- **💰 [Budgets & Alerts](docs/guides/budgets.md)**: Hierarchical budgets (global, provider, tag, type) with CI/CD thresholds
- **💡 [Recommendations](docs/guides/recommendations.md)**: Actionable cost optimization insights and savings opportunities
- **♿ [Accessibility](docs/guides/accessibility.md)**: High-contrast, plain text, and adaptive terminal UI modes
- **💰 [Actual Costs](docs/reference/cli-commands.md#cost-actual)**: Track historical spending with detailed breakdowns
- **🔌 [Plugin-Based](docs/plugins/README.md)**: Extensible architecture supporting multiple cost data sources
- **🧪 [E2E Testing](docs/testing/e2e-guide.md)**: Comprehensive guide for validating infrastructure costs against real cloud resources
- **📈 Advanced Analytics**: Resource grouping, filtering, and aggregation
- **📱 Multiple Formats**: Table, JSON, and NDJSON output options
- **🔍 Smart Filtering**: Filter by resource type, tags, or custom expressions
- **🏗️ No Code Changes**: Works with existing Pulumi projects via JSON output

## Quick Start

### 1. Installation

**Install script (recommended)** -- auto-detects OS/architecture and downloads the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

**Build from source:**

```bash
git clone https://github.com/rshade/finfocus
cd finfocus
make build
./bin/finfocus --help
```

<details>
<summary>Manual download (pin to a specific version)</summary>

```bash
# Linux (amd64)
curl -L https://github.com/rshade/finfocus/releases/download/v0.3.3/finfocus-v0.3.3-linux-amd64.tar.gz -o finfocus.tar.gz
tar -xzf finfocus.tar.gz
chmod +x finfocus
sudo mv finfocus /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/rshade/finfocus/releases/download/v0.3.3/finfocus-v0.3.3-macos-arm64.tar.gz -o finfocus.tar.gz
tar -xzf finfocus.tar.gz && chmod +x finfocus && sudo mv finfocus /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/rshade/finfocus/releases/download/v0.3.3/finfocus-v0.3.3-macos-amd64.tar.gz -o finfocus.tar.gz
tar -xzf finfocus.tar.gz && chmod +x finfocus && sudo mv finfocus /usr/local/bin/

# Windows (PowerShell) - installs to a user-local directory, no admin rights required
Invoke-WebRequest -Uri "https://github.com/rshade/finfocus/releases/download/v0.3.3/finfocus-v0.3.3-windows-amd64.zip" -OutFile finfocus.zip
Expand-Archive finfocus.zip -DestinationPath .
$installDir = "$env:LocalAppData\Programs\finfocus"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Move-Item finfocus.exe "$installDir\finfocus.exe"
# Add to PATH for the current session (add to $PROFILE for a permanent effect)
$env:PATH = "$installDir;$env:PATH"
```

</details>

### 2. Generate Pulumi Plan

Export your infrastructure plan to JSON:

```bash
cd your-pulumi-project
pulumi preview --json > plan.json
```

### 3. Unified Overview

See all costs at a glance with the interactive dashboard. From inside any Pulumi
project directory, just run finfocus with no arguments:

```bash
finfocus
```

Or invoke the subcommand directly:

```bash
finfocus overview
```

For CI/CD or when you manage the export step yourself:

```bash
# Pre-export state and plan
pulumi stack export > state.json
pulumi preview --json > plan.json

# Launch overview with pre-exported files
finfocus overview --pulumi-state state.json --pulumi-json plan.json --plain --yes
```

Example plain text output:

```text
Resource                          Type                    Status  Actual(MTD)  Projected   Delta    Drift%  Recs
my-instance                       aws:ec2/instance:I...   ✓       $12.40       $15.00      $2.60    +8%     2
my-bucket                         aws:s3/bucket:Bucket    ✓       $0.83        $1.00       $0.17    0%      0
my-db                             aws:rds/instance:I...   ✓       $48.20       $50.00      $1.80    -3%     1

Total Actual (MTD): $61.43    Projected Monthly: $66.00    Potential Savings: $45.00
```

Full documentation: [docs/commands/overview.md](docs/commands/overview.md)

### 4. Calculate Costs

**Projected Costs** - Estimate costs before deployment:

```bash
finfocus cost projected --pulumi-json plan.json
```

**Check Budget** - Verify if plan fits within budget:

```bash
# Configure budget in ~/.finfocus/config.yaml (see Configuration section)
# Then check projected cost against budget
finfocus cost projected --pulumi-json plan.json

# Exit with non-zero code if budget exceeded (CI/CD integration)
finfocus cost projected --pulumi-json plan.json --exit-on-threshold

# Custom exit code (default: 1)
finfocus cost projected --pulumi-json plan.json --exit-on-threshold --exit-code 2

# Filter budget scope (global, provider:<name>, tag:<key>=<value>, type:<resource-type>)
finfocus cost projected --pulumi-json plan.json --budget-scope "provider:aws"
```

**View Recommendations** - Find savings opportunities:

```bash
finfocus cost recommendations --pulumi-json plan.json
```

## Example Output

### Projected Cost Analysis

```bash
$ finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json

Budget: $500.00 (75% used)
[=====================>......] $375.00 / $500.00

RESOURCE                          ADAPTER     MONTHLY   CURRENCY  NOTES
aws:ec2/instance:Instance         aws-spec    $375.00   USD       t3.xlarge
aws:s3/bucket:Bucket             none        $0.00     USD       No pricing info
```

### Actual Cost Analysis

```bash
$ finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --group-by type --output json
{
  "summary": {
    "totalMonthly": 45.67,
    "currency": "USD",
    "byProvider": {"aws": 45.67},
    "byService": {"ec2": 23.45, "s3": 12.22, "rds": 10.00}
  },
  "resources": [...]
}
```

## Core Concepts

### Resource Analysis Flow

1. **Export** - Generate Pulumi plan JSON with `pulumi preview --json`
2. **Parse** - Extract resource definitions and properties
3. **Query** - Fetch cost data via plugins or local specifications
4. **Aggregate** - Calculate totals with grouping and filtering options
5. **Output** - Present results in table, JSON, or NDJSON format

### Plugin Architecture

FinFocus uses plugins to fetch cost data from various sources:

- **Cost Plugins**: Query cloud provider APIs (AWS Public Pricing, AWS Cost Explorer, Azure, etc.)
- **Spec Files**: Local YAML/JSON pricing specifications as fallback
- **Plugin Discovery**: Automatic detection from `~/.finfocus/plugins/`

## Configuration

FinFocus is configured via `~/.finfocus/config.yaml`.

### Budget Configuration

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
cost:
  budgets:
    amount: 500.00
    currency: USD
    alerts:
      - threshold: 80
        type: actual
      - threshold: 100
        type: forecasted
```

See [Budget Guide](docs/guides/budgets.md) for full configuration details.

### Environment Variables for Secrets

For sensitive values like API keys and credentials, use environment variables:

```bash
# AWS credentials (for aws-public and aws-ce plugins)
export FINFOCUS_PLUGIN_AWS_ACCESS_KEY_ID="your-access-key"
export FINFOCUS_PLUGIN_AWS_SECRET_ACCESS_KEY="your-secret-key"

# Azure credentials (for azure-public plugin)
export FINFOCUS_PLUGIN_AZURE_SUBSCRIPTION_ID="your-subscription-id"
```

The naming convention is: `FINFOCUS_PLUGIN_<PLUGIN_NAME>_<KEY_NAME>` in uppercase.

## Advanced Usage

### Resource Filtering

```bash
# Filter by resource type
finfocus cost projected --pulumi-json plan.json --filter "type=aws:ec2/instance"
```

### Output Formats

```bash
# Table format (default)
finfocus cost projected --pulumi-json plan.json --output table

# JSON for API integration
finfocus cost projected --pulumi-json plan.json --output json

# NDJSON for streaming/pipeline processing
finfocus cost projected --pulumi-json plan.json --output ndjson
```

## Configuration Management

FinFocus provides commands to manage configuration:

```bash
# Initialize configuration (creates ~/.finfocus/config.yaml)
finfocus config init [--force]

# Set configuration values
finfocus config set cost.budgets.amount 500.00
finfocus config set output.format json

# Get configuration values
finfocus config get cost.budgets.amount

# List all configuration
finfocus config list [--format json|yaml]

# Validate configuration
finfocus config validate [--verbose]

# Inspect effective routing configuration
finfocus config routes list [--output table|json]

# Simulate plugin routing for a resource type
finfocus config routes test aws:ec2:Instance [region] [--output table|json]
```

## Multi-Plugin Routing

FinFocus intelligently routes resources to appropriate plugins based on provider, resource patterns, and feature capabilities.

### Automatic Routing (Zero Configuration)

Resources automatically route to plugins based on their supported providers:

```bash
# Install multiple plugins
finfocus plugin install aws-public
finfocus plugin install gcp-public

# View plugin capabilities
finfocus plugin list --verbose
# NAME        VERSION  PROVIDERS  CAPABILITIES                  SPEC    PATH
# aws-public  1.0.0    aws        ProjectedCosts, ActualCosts   0.5.5   ~/.finfocus/plugins/aws-public/1.0.0/...
# gcp-public  1.0.0    gcp        ProjectedCosts, ActualCosts   0.5.5   ~/.finfocus/plugins/gcp-public/1.0.0/...

# Cost calculation automatically routes resources
finfocus cost projected --pulumi-json plan.json
# AWS resources → aws-public
# GCP resources → gcp-public
```

### Declarative Routing (Advanced Configuration)

For advanced control, configure plugin routing in `~/.finfocus/config.yaml`:

```yaml
routing:
  plugins:
    # Route recommendations to AWS Cost Explorer (higher accuracy)
    - name: aws-ce
      features:
        - Recommendations
      priority: 20
      fallback: true

    # Route projected costs to AWS Public (no credentials needed)
    - name: aws-public
      features:
        - ProjectedCosts
        - ActualCosts
      priority: 10
      fallback: true

    # Route EKS resources to specialized plugin (highest priority)
    - name: eks-costs
      patterns:
        - type: glob
          pattern: "aws:eks:*"
      priority: 30
```

**Key Features:**

- **Priority-Based Selection**: Higher priority plugins are queried first (default: 0)
- **Automatic Fallback**: If a plugin fails, automatically try the next priority
- **Pattern Matching**: Use glob or regex patterns to route specific resource types
- **Feature Routing**: Assign different plugins for different capabilities

### Validate Routing Configuration

```bash
finfocus config validate

# Output (success):
# ✓ Configuration valid
#
# Discovered plugins:
#   aws-ce: Recommendations (priority: 20)
#   aws-public: ProjectedCosts, ActualCosts (priority: 10)
#
# Routing rules:
#   aws:eks:* → eks-costs (pattern)
#   aws:* → aws-public (provider)
```

See the [Routing Configuration Guide](docs/guides/routing.md) for detailed examples and troubleshooting.

## Plugin Management

### List & Install Plugins

```bash
# List installed plugins
finfocus plugin list

# List with detailed capabilities
finfocus plugin list --verbose

# Install plugins
finfocus plugin install aws-public
finfocus plugin install vantage

# Inspect plugin capabilities
finfocus plugin inspect aws-public

# Validate plugin installation
finfocus plugin validate
```

### Available Plugins

| Plugin | Status | Description |
|--------|--------|-------------|
| `aws-public` | Available | AWS public pricing data |
| `aws-ce` | In Development | AWS Cost Explorer integration |
| `azure-public` | In Development | Azure public pricing data |
| `kubecost` | Planned | Kubernetes cost analysis |

## Pulumi Analyzer Integration

FinFocus provides zero-click cost estimation during `pulumi preview` via the Pulumi Analyzer protocol:

```bash
# Start the analyzer server (prints port to stdout for Pulumi handshake)
finfocus analyzer serve [--debug]
```

When integrated with Pulumi, costs are automatically calculated and displayed as advisory diagnostics during preview, without modifying your Pulumi programs. The analyzer uses ADVISORY enforcement and never blocks deployments.

## Debugging

Enable debug output for troubleshooting:

```bash
# Global debug flag
finfocus --debug cost projected --pulumi-json plan.json

# Environment variable
export FINFOCUS_LOG_LEVEL=debug
export FINFOCUS_LOG_FORMAT=json    # json or console
```

## Documentation

Complete documentation is available in the [docs/](docs/) directory:

- **👤 End Users**: [User Guide](docs/guides/user-guide.md) - How to install and use FinFocus
- **💰 Budgets**: [Budget Guide](docs/guides/budgets.md) - Configure alerts and thresholds
- **💡 Recommendations**: [Recommendations Guide](docs/guides/recommendations.md) - Optimization insights
- **♿ Accessibility**: [Accessibility Guide](docs/guides/accessibility.md) - UI configuration
- **🛠️ Engineers**: [Developer Guide](docs/guides/developer-guide.md) - How to extend and contribute
- **🏗️ Architects**: [Architect Guide](docs/guides/architect-guide.md) - System design and integration
- **🧪 E2E Testers**: [E2E Testing Guide](docs/testing/e2e-guide.md) - Setup and execution
- **💼 Business/CEO**: [Business Value](docs/guides/business-value.md) - ROI and competitive advantage

**Quick Links:**

- [🚀 5-Minute Quickstart](docs/getting-started/quickstart.md)
- [📖 Full Documentation Index](docs/README.md)
- [🔌 Available Plugins](docs/plugins/) - AWS Public Pricing and more
- [🛠️ Plugin Development](docs/plugins/plugin-development.md)
- [🏗️ System Architecture](docs/architecture/system-overview.md)
- [💬 FAQ & Support](docs/support/faq.md)

## Contributing

We welcome contributions! See our development documentation:

- [CONTRIBUTING.md](CONTRIBUTING.md) - Development setup and guidelines
- [CLAUDE.md](CLAUDE.md) - AI assistant development context
- [Architecture Documentation](internal/) - Internal package documentation

## License

Apache-2.0 - See [LICENSE](LICENSE) for details.

## Related Projects

- [finfocus-spec](https://github.com/rshade/finfocus-spec) - Protocol definitions and schemas
- [finfocus-plugin-aws-public](https://github.com/rshade/finfocus-plugin-aws-public) - AWS public pricing plugin

---

**Getting Started**: Try the [examples](examples/) directory for sample Pulumi plans and pricing specifications.
