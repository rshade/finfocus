# FinFocus

[![CI](https://github.com/rshade/finfocus/actions/workflows/ci.yml/badge.svg)](https://github.com/rshade/finfocus/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-61%25-yellow)](https://github.com/rshade/finfocus/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/rshade/finfocus)](https://goreportcard.com/report/github.com/rshade/finfocus)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Cloud cost analysis for Pulumi infrastructure** - Calculate projected and actual infrastructure costs without modifying your Pulumi programs.

FinFocus Core is a CLI tool that analyzes Pulumi infrastructure definitions to provide accurate cost estimates, budget enforcement, and historical cost tracking through a flexible plugin-based architecture.

## Key Features

- **📊 [Projected Costs](docs/reference/cli-commands.md#cost-projected)**: Estimate monthly costs before deploying infrastructure
- **💰 [Budgets & Alerts](docs/guides/budgets.md)**: Configure spending limits, alerts, and CI/CD thresholds
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

Download the latest release or build from source:

```bash
# Download latest release (coming soon)
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-amd64 -o finfocus
chmod +x finfocus

# Or build from source
git clone https://github.com/rshade/finfocus
cd finfocus
make build
./bin/finfocus --help
```

### 2. Generate Pulumi Plan

Export your infrastructure plan to JSON:

```bash
cd your-pulumi-project
pulumi preview --json > plan.json
```

### 3. Calculate Costs

**Projected Costs** - Estimate costs before deployment:

```bash
finfocus cost projected --pulumi-json plan.json
```

**Check Budget** - Verify if plan fits within budget:

```bash
# Set budget first
export FINFOCUS_BUDGET_AMOUNT=500

# Check projected cost against budget
finfocus cost projected --pulumi-json plan.json
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

### Actual Cost Analysis (FUTURE)

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

- **Cost Plugins**: Query cloud provider APIs (Kubecost, Vantage, AWS Cost Explorer, etc.)
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
# AWS credentials
export FINFOCUS_PLUGIN_AWS_ACCESS_KEY_ID="your-access-key"
export FINFOCUS_PLUGIN_AWS_SECRET_ACCESS_KEY="your-secret-key"

# Vantage API
export FINFOCUS_PLUGIN_VANTAGE_API_TOKEN="your-token"
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

## Plugin Management

### List & Install Plugins

```bash
# List available
finfocus plugin list

# Install Vantage plugin
finfocus plugin install vantage

# Inspect capabilities
finfocus plugin inspect vantage aws:ec2/instance:Instance
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
- [🔌 Available Plugins](docs/plugins/) - Vantage, Kubecost, and more
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
- [finfocus-plugin-kubecost](https://github.com/rshade/finfocus-plugin-kubecost) - Kubecost integration plugin
- [finfocus-plugin-vantage](https://github.com/rshade/finfocus-plugin-vantage) - Vantage cost intelligence plugin

---

**Getting Started**: Try the [examples](examples/) directory for sample Pulumi plans and pricing specifications.
