# Research: Kubernetes In-Cluster Cost Allocation

**Date**: 2026-09-24 (converted 2026-09-29)
**Source**: superpowers design doc decisions table and §8 plan-time amendments,
converted into Spec Kit research format. Each entry records the decision, its
rationale, and the alternatives rejected.

## Decisions

### D1: Allocation math lives in an allocator plugin, not core

- **Decision**: A new `AllocatorService.Allocate` RPC; core gathers usage and
  prices nodes, then delegates the split.
- **Rationale**: Core must stay Kubernetes-agnostic (Constitution Principle I).
  Third-party allocators (OpenCost-style, SP6) can plug in without core
  changes, and the conformance suite (`RunAllocatorConformance`) gives every
  allocator the same correctness bar.
- **Alternatives rejected**: allocation in core (couples core to Kubernetes
  semantics); extending `CostSourceService` (usage sources have no prices, so
  they would stub half the interface).

### D2: Usage sources and allocators are separate plugin capabilities

- **Decision**: `PLUGIN_CAPABILITY_USAGE_STATS = 14` and
  `PLUGIN_CAPABILITY_ALLOCATION = 15`; a plugin may implement any combination
  of `CostSourceService`, `UsageSourceService`, and `AllocatorService`.
- **Rationale**: Usage collection (cluster-specific: Kubernetes API,
  Prometheus, Datadog) evolves independently of allocation policy. Detection
  uses the existing `capabilities_enum` in `GetPluginInfo`.
- **Alternatives rejected**: one combined service (forces every usage source
  to also be an allocator).

### D3: SDK-owned allocation rules

- **Decision**: `pluginsdk.DecodePolicy`, `ValidateAllocateRequest`,
  `ValidateAllocateResponse`, `ResolveCurrency`, and `CheckConservation` (with
  `DefaultConservationEpsilon`, 1e-6 relative) are the single source of the
  shared rules (§8.14, from finfocus-spec 051/052 review): mixed currencies
  are `InvalidArgument`; an empty currency takes the others' single currency
  (all empty → `USD`); duplicate priced `(kind, id)` is rejected; unknown
  policy fields are reported with their JSON path; usage rows must carry
  `kind`; `kind=node` rows must name their node; duplicate `(subject, metric)`
  rows are invalid.
- **Rationale**: Core, allocators, and the conformance suite must apply one
  rule set, or third-party allocators drift from what core enforces.
- **Alternatives rejected**: implementing the rules in core and again in each
  allocator (guaranteed drift).

### D4: `$0` price means unpriced

- **Decision**: A priceable whose projected monthly cost is `<= 0` (or carries
  a structured error) is treated as unpriced (§8.5).
- **Rationale**: aws-public answers unknown instance types with `$0` and a
  "not found in pricing data" note instead of an error; a real node is never
  free. Without this rule every workload on an unknown instance type looks
  free and nothing is flagged.
- **Alternatives rejected**: changing aws-public to error (breaks its existing
  contract with `cost projected`).

### D5: `Supports` cache key and fail-open semantics

- **Decision**: `checkPluginSupports` sends provider, type, SKU, and region
  and caches per client+provider+type+region+SKU+feature. Fail-open (cached
  `true`) applies only to RPC errors and to the SDK's
  `DefaultSupportsNotImplementedReason` (plugins older than v0.6.2) (§8.7).
- **Rationale**: Since finfocus-spec v0.6.2 (#507/#510) a plugin's own
  `Supports` answer reaches the host. A region-bound plugin (aws-public) must
  be able to decline other regions, and a SKU-less resource must not poison
  the cached answer for every other SKU in the region.
- **Alternatives rejected**: keying on type only (wrong answers across
  regions); failing closed on the not-implemented reason (silently drops every
  pre-v0.6.2 plugin from routing).

### D6: pluginsdk serving is SDK work, not just protos

- **Decision**: The finfocus-spec issues included registration in
  `serveGRPC`, `serveConnect`, the grpchealth static checker, and harness
  support — not only proto definitions (§8.1).
- **Rationale**: `serveGRPC`/`serveConnect` hardcoded `CostSourceService`;
  protos alone would ship dead services.

### D7: Canonical version for prefixed tags

- **Decision**: For monorepo plugins, install directories, asset names,
  recorded versions, and update comparisons use the canonical version (tag
  minus prefix, e.g. `kubernetes-v0.1.0` → `v0.1.0`); prefixed "latest" scans
  100 stable releases and picks the highest semver (§8.9).
- **Rationale**: `ListLatestPlugins` requires a semver directory name; core
  releases outnumber plugin releases, so `/releases/latest` and small scans
  miss plugin tags; semver (not publish date) must win so a hotfix published
  after a newer tag still resolves correctly.
- **Alternatives rejected**: `/releases/latest` (repo-wide, dominated by CLI
  releases); publish-order selection (hotfix ordering breaks it).

### D8: v-leading tag prefixes are forbidden

- **Decision**: `tag_prefix` must match `^[a-uw-z0-9][a-z0-9-]*-$` (§8.9).
- **Rationale**: CLI workflows (`goreleaser.yml`, `nightly.yml`,
  `git describe --match 'v[0-9]*'`) treat `v*` tags as CLI releases; a
  v-leading plugin prefix would collide. Additionally, plugin releases are
  un-marked as the repo's "Latest" (the CLI release is re-marked), GoReleaser
  gets explicit current/previous CLI tags, and release-tag values reach `run:`
  only via `env:` (shell-injection guard).

### D9: `jq` pin for the plugin version

- **Decision**: Makefile reads the kubernetes plugin version with
  `jq -r '."plugins/kubernetes"' .release-please-manifest.json`; `jq` is a
  pinned tool (mise.toml) already required by the toolchain.
- **Rationale**: The release-please manifest is the single source of the
  plugin's current version; parsing JSON with `jq` avoids brittle sed/awk.

### D10: Policy schema and defaults are plugin-owned

- **Decision**: The kubernetes plugin owns the policy defaults and schema;
  user overrides live in `allocation.hujson`, delivered to the plugin as
  opaque standardized JSON (`policy_json` bytes). Strict decoding goes through
  `pluginsdk.DecodePolicy`; unknown `version` is an error; policy v1 has no
  list fields, so the "lists replace, not append" rule is documented but
  untested (§8.10).
- **Rationale**: Allocation policy is organizational policy meant to be
  committed and reviewed; `config.hujson` merges shallowly at top-level keys,
  so a project override under `plugins.kubernetes` would replace the entire
  global `plugins` key — a separate file avoids that trap.
- **Alternatives rejected**: policy inside `config.hujson` (shallow-merge
  hazard, mixes policy with per-user connection settings).

### D11: Run-rate allocation uses ratios only (no usage scaling)

- **Decision**: Usage rates are NOT multiplied by 730 hours; costs are monthly
  because nodes are priced with projected monthly cost (§8.4).
- **Rationale**: Allocation distributes each node's monthly cost by
  `request / allocatable` ratios, which are unitless; scaling usage would
  cancel out and only add rounding error.

### D12: Node descriptors are ordinary resources

- **Decision**: Nodes use `resource_type` `aws:ec2/instance:Instance` with
  `sku` and `region` passed as engine properties (read by
  `resolveSKUAndRegion`); the provider comes from the `finfocus.dev/provider`
  label, else the `providerID` scheme (§8.6).
- **Rationale**: Core never interprets Kubernetes semantics — nodes are priced
  by the existing projected-cost path like any other resource.

### D13: Plugin opts out of pricing via explicit capabilities and `Supports`

- **Decision**: The plugin declares only `USAGE_STATS` and `ALLOCATION`
  (inference always adds projected/actual/pricing/estimate, §8.8) and answers
  `Supports=false` for everything (§8.7).
- **Rationale**: `cost projected` asks every plugin; without the opt-out each
  resource would record a "not supported" plugin error.

### D14: finfocus-spec contract work is delivered as issues, one per RPC

- **Decision**: No finfocus-spec code is written from this effort; each RPC
  (plus the `Supports` default-registry fix) was a fully specified issue
  (#505, #506, #507), closed by the v0.6.2 release.
- **Rationale**: Protocol changes must originate in finfocus-spec with a
  version bump (Constitution Principle V, Multi-Repo Governance). A local
  `replace` directive was allowed during development and never merged.

### D15: Plugin location is the monorepo, for now

- **Decision**: `plugins/kubernetes/` is a nested Go module shipped via the
  registry from the monorepo; it graduates to its own repository once it has
  usage and outside contributors.
- **Rationale**: Tight iteration loop with core during slice 1; the
  `tag_prefix` registry work (D7/D8) makes monorepo releases a first-class
  distribution path.
