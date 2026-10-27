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

### aws-ce

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus-plugin-aws-ce` |
| Providers | `aws` |
| Capabilities | `cost_retrieval`, `cost_projection`, `recommendations`, `budgets` |
| Security | official |
| Asset prefix | `finfocus-plugin-aws-ce` |

Reads billed AWS cost from Cost Explorer by account, service, or resource, plus
AWS Budgets and AWS recommendations. Projected cost is a forecast for
`aws:account` and `aws:service` only; route single-resource projections to
`aws-public`. It needs AWS credentials, and each paginated Cost Explorer
request costs $0.01. `finfocus setup` does not install it; run
`finfocus plugin install aws-ce`.

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

### vantage

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus-plugin-vantage` |
| Providers | `aws`, `azure`, `gcp`, `kubernetes` |
| Capabilities | `cost_retrieval` |
| Security | official |
| Asset prefix | `finfocus-plugin-vantage` |

Reads imported actual cost from a Vantage Cost Report, so it needs
`FINFOCUS_VANTAGE_TOKEN` and `FINFOCUS_VANTAGE_COST_REPORT_TOKEN`. It has no
projected cost. `finfocus setup` does not install it; run
`finfocus plugin install vantage`.

### flexera

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus-plugin-flexera` |
| Providers | `aws`, `azure`, `gcp` |
| Capabilities | `cost_projection`, `cost_retrieval`, `pricing_specs` |
| Security | official |
| Asset prefix | `finfocus-plugin-flexera` |

Reads billed cost from Flexera One Bill Analysis, so it needs
`FLEXERA_ORG_ID`, `FLEXERA_BILLING_CENTER_IDS`, and a refresh token or service
account. `finfocus setup` does not install it; run
`finfocus plugin install flexera`.

### opencost

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus-plugin-opencost` |
| Providers | `kubernetes` |
| Capabilities | `cost_projection`, `cost_retrieval`, `pricing_specs` |
| Security | official |
| Asset prefix | `finfocus-plugin-opencost` |

Reads Kubernetes allocation cost over HTTP from an OpenCost or Kubecost
endpoint, so it needs `KUBECOST_BASE_URL` pointing at a reachable allocation
API. There is no separate `kubecost` plugin: for Kubecost, install `opencost`
and set `OPENCOST_PROFILE=kubecost`. `finfocus setup` does not install it; run
`finfocus plugin install opencost`.

### jev

| Field | Value |
|-------|-------|
| Repository | `rshade/finfocus` (monorepo, release tags `jev-vX.Y.Z`) |
| Providers | `*` (any provider) |
| Capabilities | `recommendation_scoring` |
| Security | official |
| Asset prefix | `finfocus-plugin-jev` |

Legacy opt-in Jev scorer. `finfocus plugin install jev` continues installing
the last `jev-v*` release separately; it does not install the renamed scorer or
migrate settings. Historical tags and assets remain available. The registry
entry stays until new scorer release assets exist.

### scorer (source install; public release pending)

The renamed plugin supports `SCORER_BACKEND=jev|clef` (default `jev`). Build and
install it from this repository with `make install-scorer`. After scorer assets
and its registry entry are published, use `finfocus plugin install scorer`.
Neither scorer is installed by `finfocus setup` or provider detection.

- Use `SCORER_API_KEY` for the selected backend token. Clef also requires a
  32-character hexadecimal `SCORER_CLEF_ACCOUNT_ID` when a key is supplied.
- Defaults: `SCORER_MODEL=jev-1.13.0` for Jev or `clef` for Clef;
  `SCORER_BASE_URL=https://api.typesafe.ai` or `https://api.cloudflare.com`.
  The base URL is an origin; the Clef adapter appends the account path.
- `SCORER_TIMEOUT=60s`, `SCORER_BATCH_SIZE=25` and
  `SCORER_DUPLICATE_THRESHOLD=0.5` retain the legacy defaults.
- Enable in order: `finfocus config set scoring.plugin scorer`, then
  `finfocus config set scoring.enabled true`. Without a key scoring calls
  return `UNAUTHENTICATED` and recommendations remain unscored.
- Hosted Jev and Clef send recommendation data to TypeSafe AI and Cloudflare
  respectively. Identifiers are pseudonymized by default; inspect the dry run
  and narrow data with `scoring.field_allowlist`. Scores rank work for review
  and never approve or apply changes. Local compatible endpoints may avoid
  hosted calls, but Clef self-hosting and vLLM compatibility are untested.
- See the [scorer README](https://github.com/rshade/finfocus/tree/main/plugins/scorer)
  and Recommendation Scoring guide
  (`docs/src/content/docs/guides/recommendation-scoring.md`).

The new plugin does not read legacy environment variables. Migrate explicitly:

| Removed setting | Replacement |
| --- | --- |
| `TYPESAFE_API_KEY` | `SCORER_API_KEY` |
| `JEV_MODEL` | `SCORER_MODEL` |
| `JEV_BASE_URL` | `SCORER_BASE_URL` |
| `JEV_TIMEOUT` | `SCORER_TIMEOUT` |
| `JEV_BATCH_SIZE` | `SCORER_BATCH_SIZE` |
| `JEV_DUPLICATE_THRESHOLD` | `SCORER_DUPLICATE_THRESHOLD` |
| `scoring.plugin: jev` | `scoring.plugin: scorer` |

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
| `budgets` | Budget tracking and alerts |
| `recommendations` | Cost optimization suggestions |
| `recommendation_scoring` | Scores and groups recommendations from other plugins |
| `projected` | Alias for projected cost support |
| `actual` | Alias for actual cost support |

Additional runtime capabilities (available via plugin `GetPluginInfo` but not
used in the embedded registry):

| Capability | Description |
|------------|-------------|
| `dry_run` | Field mapping inspection |
| `batch_cost` | Multi-resource cost queries |
| `estimate_cost` | Quick cost estimation |

## Non-Registry Plugins

Install via GitHub URL:

```bash
finfocus plugin install github.com/owner/repo
finfocus plugin install github.com/owner/repo@v1.0.0
```
