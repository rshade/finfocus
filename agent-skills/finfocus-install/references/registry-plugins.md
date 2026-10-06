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

### azure-public

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus-plugin-azure-public` |
| Providers | `azure` (covers the `azure` and `azure-native` Pulumi packages) |
| Capabilities | `cost_projection`, `cost_retrieval`, `pricing_specs` |
| Security | official |
| Asset prefix | `finfocus-plugin-azure-public` |

Queries the public Azure Retail Prices API, so it needs no Azure credentials.
Actual cost is a list-price projection scaled by hours, not billed spend.
`finfocus setup` does not install it; run `finfocus plugin install azure-public`.

### opencost

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus-plugin-opencost` |
| Providers | `kubernetes` |
| Capabilities | `cost_projection`, `cost_retrieval`, `pricing_specs` |
| Security | official |
| Asset prefix | `finfocus-plugin-opencost` |

Reads Kubernetes allocation cost over HTTP from an OpenCost (or, experimentally,
a Kubecost) endpoint, so it needs `KUBECOST_BASE_URL` pointing at a reachable
allocation API. `finfocus setup` does not install it; run
`finfocus plugin install opencost`.

### jev

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus` (monorepo, release tags `jev-vX.Y.Z`) |
| Providers | `*` (any provider) |
| Capabilities | `recommendation_scoring` |
| Security | official |
| Asset prefix | `finfocus-plugin-jev` |

Opt-in recommendation scorer backed by TypeSafe AI's Jev model. It rates risk,
false positives, worth and priority and groups duplicate recommendations. It is
not part of provider detection and `finfocus setup` does not install it.

- Requires `TYPESAFE_API_KEY` (from the TypeSafe console) and scoring enabled in
  config; without them it does nothing. Without the key every scoring call
  returns `UNAUTHENTICATED`, shown as a scoring warning.
- Enable in this order, because each config change is validated:
  `finfocus config set scoring.plugin jev`, then
  `finfocus config set scoring.enabled true`.
- Sends recommendation data to TypeSafe. Identifiers are pseudonymized by default.
- Setup and data handling: see the
  [plugin README](https://github.com/rshade/finfocus/tree/main/plugins/jev) and
  the Recommendation Scoring guide (`docs/src/content/docs/guides/recommendation-scoring.md`
  in the repo).

## Provider Detection → Plugin Mapping

| Detected Provider | Plugin to Install | Detection Signal |
|-------------------|-------------------|------------------|
| AWS | `aws-public` | `aws:` resource prefix, `~/.aws/` directory |
| Azure | `azure-public` | `azure:` or `azure-native:` resource prefix, `~/.azure/` directory |
| Kubernetes | `opencost` | `kubernetes:` resource prefix, kubeconfig |
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
| `recommendation_scoring` | Scores and groups recommendations from other plugins |
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
