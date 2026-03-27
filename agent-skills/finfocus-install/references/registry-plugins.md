# Registry Plugins & Provider Mapping

## Embedded Registry

The embedded registry (`internal/registry/registry.json`) contains official plugins.

## Available Plugins

### aws-public

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus-plugin-aws-public` |
| Providers | `aws` |
| Capabilities | `cost_projection`, `pricing_specs`, `recommendations` |
| Security | official |
| Asset prefix | `finfocus-plugin-aws-public` |

Default plugin installed by `finfocus setup`. Supports region-specific binaries
via `--metadata="region=<region>"` and a multi-region router binary (default).

### kubecost

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus-plugin-kubecost` |
| Providers | `kubernetes` |
| Capabilities | `cost_retrieval`, `cost_projection` |
| Security | official |
| Asset prefix | `finfocus-plugin-kubecost` |

## Provider Detection → Plugin Mapping

| Detected Provider | Plugin to Install | Detection Signal |
|-------------------|-------------------|------------------|
| AWS | `aws-public` | `aws:` resource prefix, `~/.aws/` directory |
| Kubernetes | `kubecost` | `kubernetes:` resource prefix, kubeconfig |
| None detected | `aws-public` | Default fallback |

## Registry Capabilities

Capabilities validated for embedded registry entries
(`internal/registry/registry.json`):

| Capability | Description |
|------------|-------------|
| `cost_projection` | Projected cost estimation from resource specs |
| `cost_retrieval` | Historical cost data from cloud APIs |
| `pricing_specs` | Pricing specification/breakdown data |
| `recommendations` | Cost optimization suggestions |
| `projected` | Alias for projected cost support |
| `actual` | Alias for actual cost support |

Additional runtime capabilities (available via plugin `GetPluginInfo` but not
used in the embedded registry):

| Capability | Description |
|------------|-------------|
| `dry_run` | Field mapping inspection |
| `budgets` | Budget tracking and alerts |
| `batch_cost` | Multi-resource cost queries |
| `estimate_cost` | Quick cost estimation |

## Non-Registry Plugins

Install via GitHub URL:

```bash
finfocus plugin install github.com/owner/repo
finfocus plugin install github.com/owner/repo@v1.0.0
```
