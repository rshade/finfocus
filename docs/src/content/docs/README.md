---
title: Project README
description: Cloud cost analysis for Pulumi infrastructure with projected costs, budgets, and plugin architecture.
---

<!-- markdownlint-disable MD013 -->

*Cloud cost analysis for Pulumi infrastructure*

[![CI](https://github.com/rshade/finfocus/actions/workflows/ci.yml/badge.svg)](https://github.com/rshade/finfocus/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/rshade/finfocus/graph/badge.svg)](https://codecov.io/gh/rshade/finfocus)
[![Go Report Card](https://goreportcard.com/badge/github.com/rshade/finfocus)](https://goreportcard.com/report/github.com/rshade/finfocus)
[![Go Version](https://img.shields.io/github/go-mod/go-version/rshade/finfocus)](https://github.com/rshade/finfocus/blob/main/go.mod)
[![Release](https://img.shields.io/github/v/release/rshade/finfocus)](https://github.com/rshade/finfocus/releases/latest)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](https://github.com/rshade/finfocus/blob/main/LICENSE)

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

- **🔭 [Unified Overview](../commands/overview/)**: Interactive dashboard combining actual costs, projected costs, drift analysis, and recommendations in a single view
- **📊 [Projected Costs](../reference/cli-commands/#cost-projected)**: Estimate monthly costs before deploying infrastructure
- **💰 [Budgets & Alerts](../guides/budgets/)**: Hierarchical budgets (global, provider, tag, type) with CI/CD thresholds
- **💡 [Recommendations](../guides/recommendations/)**: Actionable cost optimization insights and savings opportunities
- **♿ [Accessibility](../guides/accessibility/)**: High-contrast, plain text, and adaptive terminal UI modes
- **💰 [Actual Costs](../reference/cli-commands/#cost-actual)**: Track historical spending with detailed breakdowns
- **🔌 [Plugin-Based](../plugins/)**: Extensible architecture supporting multiple cost data sources
- **🧪 [E2E Testing](../testing/e2e-guide/)**: Comprehensive guide for validating infrastructure costs against real cloud resources
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
curl -L https://github.com/rshade/finfocus/releases/download/v0.3.3/finfocus-v0.3.3-linux-amd64.tar.gz -o finfocus.tar.gz
tar -xzf finfocus.tar.gz
chmod +x finfocus
sudo mv finfocus /usr/local/bin/

curl -L https://github.com/rshade/finfocus/releases/download/v0.3.3/finfocus-v0.3.3-macos-arm64.tar.gz -o finfocus.tar.gz
tar -xzf finfocus.tar.gz && chmod +x finfocus && sudo mv finfocus /usr/local/bin/

curl -L https://github.com/rshade/finfocus/releases/download/v0.3.3/finfocus-v0.3.3-macos-amd64.tar.gz -o finfocus.tar.gz
tar -xzf finfocus.tar.gz && chmod +x finfocus && sudo mv finfocus /usr/local/bin/
```

```powershell
# Windows - installs to a user-local directory, no admin rights required
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
pulumi stack export > state.json
pulumi preview --json > plan.json

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

Full documentation: [docs/commands/overview.md](../commands/overview/)

### 4. Calculate Costs

**Projected Costs** - Estimate costs before deployment:

```bash
finfocus cost projected --pulumi-json plan.json
```

**Check Budget** - Verify if plan fits within budget:

```bash
finfocus cost projected --pulumi-json plan.json

finfocus cost projected --pulumi-json plan.json --exit-on-threshold

finfocus cost projected --pulumi-json plan.json --exit-on-threshold --exit-code 2

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

See [Budget Guide](../guides/budgets/) for full configuration details.

### Environment Variables for Secrets

For sensitive values like API keys and credentials, use environment variables:

```bash
export FINFOCUS_PLUGIN_AWS_ACCESS_KEY_ID="your-access-key"
export FINFOCUS_PLUGIN_AWS_SECRET_ACCESS_KEY="your-secret-key"

export FINFOCUS_PLUGIN_AZURE_SUBSCRIPTION_ID="your-subscription-id"
```

The naming convention is: `FINFOCUS_PLUGIN_<PLUGIN_NAME>_<KEY_NAME>` in uppercase.

### Cost Allocation Tags

For accurate month-long cost tracking — especially when resources are replaced
or destroyed mid-month — FinFocus can query cloud billing APIs by **resource
tags**. This requires your Pulumi-managed resources to carry consistent tags.

> **Why this matters**: When a resource is replaced (e.g., EC2 instance swap),
> the old cloud ID disappears from Pulumi state. Without tags, FinFocus can
> only see costs for the *current* resource. With tags like `pulumi:project`
> on both the old and new resources, billing APIs return the full month's costs.
>
> **Important**: Pulumi does **not** automatically tag cloud resources with
> `pulumi:project` or `pulumi:stack`. These are stack-level metadata only.
> You must explicitly configure resource tagging using one of the methods below.

<details>
<summary><strong>AWS — Provider Default Tags</strong></summary>

**Option A: Stack configuration (static values)**

```yaml
config:
  aws:defaultTags:
    tags:
      pulumi:project: my-app
      pulumi:stack: dev
      Environment: development
      CostCenter: "12345"
```

**Option B: Explicit provider in code (dynamic values)**

TypeScript:

```typescript
const provider = new aws.Provider("tagged", {
  defaultTags: {
    tags: {
      "pulumi:project": pulumi.getProject(),
      "pulumi:stack": pulumi.getStack(),
      "Environment": "dev",
    },
  },
});

// Use this provider for all resources
const bucket = new aws.s3.Bucket("data", {}, { provider });
```

Go:

```go
provider, _ := aws.NewProvider(ctx, "tagged", &aws.ProviderArgs{
    DefaultTags: &aws.ProviderDefaultTagsArgs{
        Tags: pulumi.StringMap{
            "pulumi:project": pulumi.String(ctx.Project()),
            "pulumi:stack":   pulumi.String(ctx.Stack()),
            "Environment":    pulumi.String("dev"),
        },
    },
})

// Use this provider for all resources
bucket, _ := s3.NewBucket(ctx, "data", nil, pulumi.Provider(provider))
```

Python:

```python
provider = aws.Provider("tagged", default_tags={
    "tags": {
        "pulumi:project": pulumi.get_project(),
        "pulumi:stack": pulumi.get_stack(),
        "Environment": "dev",
    },
})

bucket = aws.s3.Bucket("data", opts=pulumi.ResourceOptions(provider=provider))
```

**After tagging**: Activate `pulumi:project` and `pulumi:stack` as
[Cost Allocation Tags](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/cost-alloc-tags.html)
in the AWS Billing Console. Tags take ~24 hours to appear in Cost Explorer.

</details>

<details>
<summary><strong>GCP — Provider Default Labels</strong></summary>

> GCP labels must be lowercase with hyphens/underscores only (no colons).
> Use `pulumi_project` instead of `pulumi:project`.

**Option A: Stack configuration**

```yaml
config:
  gcp:defaultLabels:
    pulumi_project: my-app
    pulumi_stack: dev
    environment: development
    cost_center: "12345"
```

**Option B: Explicit provider in code**

TypeScript:

```typescript
const provider = new gcp.Provider("tagged", {
  defaultLabels: {
    pulumi_project: pulumi.getProject(),
    pulumi_stack: pulumi.getStack(),
    environment: "dev",
  },
});
```

Go:

```go
provider, _ := gcp.NewProvider(ctx, "tagged", &gcp.ProviderArgs{
    DefaultLabels: pulumi.StringMap{
        "pulumi_project": pulumi.String(ctx.Project()),
        "pulumi_stack":   pulumi.String(ctx.Stack()),
        "environment":    pulumi.String("dev"),
    },
})
```

Python:

```python
provider = gcp.Provider("tagged", default_labels={
    "pulumi_project": pulumi.get_project(),
    "pulumi_stack": pulumi.get_stack(),
    "environment": "dev",
})
```

> **Note**: GCP v8.0+ automatically adds the `goog-pulumi-provisioned` label
> to all resources. Opt out with `gcp:addPulumiAttributionLabel = false`.

</details>

<details>
<summary><strong>Azure — Stack Transformations</strong></summary>

Azure Native does not have a provider-level `defaultTags` option. Use a
stack transformation to inject tags into all taggable resources:

TypeScript:

```typescript
pulumi.runtime.registerStackTransformation((args) => {
  if (args.props["tags"] !== undefined) {
    args.props["tags"] = {
      ...args.props["tags"],
      "pulumi:project": pulumi.getProject(),
      "pulumi:stack": pulumi.getStack(),
      "Environment": "dev",
    };
  }
  return { props: args.props, opts: args.opts };
});
```

Go:

```go
ctx.RegisterStackTransformation(func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
    if tags, ok := args.Props["tags"]; ok {
        if tagMap, ok := tags.(pulumi.StringMap); ok {
            tagMap["pulumi:project"] = pulumi.String(ctx.Project())
            tagMap["pulumi:stack"] = pulumi.String(ctx.Stack())
            tagMap["Environment"] = pulumi.String("dev")
            args.Props["tags"] = tagMap
        }
    }
    return &pulumi.ResourceTransformationResult{Props: args.Props, Opts: args.Opts}
})
```

Python:

```python
def auto_tags(args: pulumi.ResourceTransformationArgs):
    if "tags" in args.props:
        args.props["tags"] = {
            **args.props["tags"],
            "pulumi:project": pulumi.get_project(),
            "pulumi:stack": pulumi.get_stack(),
            "Environment": "dev",
        }
    return pulumi.ResourceTransformationResult(args.props, args.opts)

pulumi.runtime.register_stack_transformation(auto_tags)
```

</details>

**FinFocus configuration** for tag-based cost queries:

```yaml
cost:
  allocation:
    enabled: true
    tags:
      - "pulumi:project"
      - "pulumi:stack"
      - "Environment"
      - "CostCenter"
```

### Resource History

FinFocus maintains a resource history database (`~/.finfocus/history/history.db`)
that tracks which cloud resource IDs existed over time. This enables accurate
month-long cost queries even when resources are replaced or destroyed mid-month.

**How it works**: Every time FinFocus runs, it snapshots the current state of your
resources. Over time, this builds a timeline of all cloud IDs each logical resource
has had. When you query actual costs for a full month, FinFocus queries billing APIs
for *all* historical IDs — not just the current one.

> **Important**: Unlike the cache (`cache.db`), the history database is **not safe
> to delete**. Deleting it loses the resource identity timeline, reducing cost
> accuracy for past periods. Back up `~/.finfocus/history/` if you back up your
> infrastructure configuration.

```yaml
cost:
  history:
    enabled: true          # default: true
    retention_days: 90     # how long to keep entries (default: 90)
```

## Advanced Usage

### Resource Filtering

```bash
finfocus cost projected --pulumi-json plan.json --filter "type=aws:ec2/instance"
```

### Output Formats

```bash
finfocus cost projected --pulumi-json plan.json --output table

finfocus cost projected --pulumi-json plan.json --output json

finfocus cost projected --pulumi-json plan.json --output ndjson
```

## Configuration Management

FinFocus provides commands to manage configuration:

```bash
finfocus config init [--force]

finfocus config set cost.budgets.amount 500.00
finfocus config set output.format json

finfocus config get cost.budgets.amount

finfocus config list [--format json|yaml]

finfocus config validate [--verbose]

finfocus config routes list [--output table|json]

finfocus config routes test aws:ec2:Instance [region] [--output table|json]
```

## Multi-Plugin Routing

FinFocus intelligently routes resources to appropriate plugins based on provider, resource patterns, and feature capabilities.

### Automatic Routing (Zero Configuration)

Resources automatically route to plugins based on their supported providers:

```bash
finfocus plugin install aws-public
finfocus plugin install gcp-public

finfocus plugin list --verbose

finfocus cost projected --pulumi-json plan.json
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

#
#
```

See the [Routing Configuration Guide](../guides/routing/) for detailed examples and troubleshooting.

## Plugin Management

### List & Install Plugins

```bash
finfocus plugin list

finfocus plugin list --verbose

finfocus plugin install aws-public
finfocus plugin install vantage

finfocus plugin inspect aws-public

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
finfocus analyzer serve [--debug]
```

When integrated with Pulumi, costs are automatically calculated and displayed as advisory diagnostics during preview, without modifying your Pulumi programs. The analyzer uses ADVISORY enforcement and never blocks deployments.

## Debugging

Enable debug output for troubleshooting:

```bash
finfocus --debug cost projected --pulumi-json plan.json

export FINFOCUS_LOG_LEVEL=debug
export FINFOCUS_LOG_FORMAT=json    # json or console
```

## Documentation

Complete documentation is available in the [docs/](../) directory:

- **👤 End Users**: [User Guide](../guides/user-guide/) - How to install and use FinFocus
- **💰 Budgets**: [Budget Guide](../guides/budgets/) - Configure alerts and thresholds
- **📜 Resource History**: [Resource History Guide](../guides/resource-history/) - Accurate month-long costs
- **💡 Recommendations**: [Recommendations Guide](../guides/recommendations/) - Optimization insights
- **♿ Accessibility**: [Accessibility Guide](../guides/accessibility/) - UI configuration
- **🛠️ Engineers**: [Developer Guide](../guides/developer-guide/) - How to extend and contribute
- **🏗️ Architects**: [Architect Guide](../guides/architect-guide/) - System design and integration
- **🧪 E2E Testers**: [E2E Testing Guide](../testing/e2e-guide/) - Setup and execution
- **💼 Business/CEO**: [Business Value](../guides/business-value/) - ROI and competitive advantage

**Quick Links:**

- [🚀 5-Minute Quickstart](../getting-started/quickstart/)
- [📖 Full Documentation Index](../)
- [🔌 Available Plugins](../plugins/) - AWS Public Pricing and more
- [🛠️ Plugin Development](../plugins/plugin-development/)
- [🏗️ System Architecture](../architecture/system-overview/)
- [💬 FAQ & Support](../support/faq/)

## Contributing

We welcome contributions! See our development documentation:

- [CONTRIBUTING.md](../support/contributing/) - Development setup and guidelines
- [CLAUDE.md](CLAUDE.md) - AI assistant development context
- [Architecture Documentation](internal/) - Internal package documentation

## License

Apache-2.0 - See [LICENSE](https://github.com/rshade/finfocus/blob/main/LICENSE) for details.

## Agent Skills

FinFocus ships [agent skills](agent-skills/) for AI coding assistants
(Claude Code, Gemini CLI, etc.) that automate common workflows:

| Skill | Description |
|-------|-------------|
| [finfocus-install](agent-skills/finfocus-install/) | Install CLI, detect providers, setup plugins and config |
| [finfocus-analyzer-setup](agent-skills/finfocus-analyzer-setup/) | Configure Pulumi Analyzer for inline cost estimation |
| [finfocus-routing](agent-skills/finfocus-routing/) | Configure plugin routing with priority and fallback |

See [agent-skills/README.md](agent-skills/README.md) for the full list and
planned skills.

## Related Projects

- [finfocus-spec](https://github.com/rshade/finfocus-spec) - Protocol definitions and schemas
- [finfocus-plugin-aws-public](https://github.com/rshade/finfocus-plugin-aws-public) - AWS public pricing plugin
- [agent-skills](https://github.com/rshade/agent-skills) - Generic cost workflow skills (multi-tool)

---

**Getting Started**: Try the [examples](examples/) directory for sample Pulumi plans and pricing specifications.
