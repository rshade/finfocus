# Research: Overview Cluster Expansion

## Decision 1: Where expansion hooks into the overview pipeline

- **Decision**: After enrichment, before rendering, in both the plain path
  (`executeOverview`, after `EnrichOverviewRows` + `PopulateComputedDeltas`)
  and the TUI path (`overviewInitAndEnrich`, after
  `bridgeEnrichmentToTUI`, delivered via a new `OverviewExpansionReadyMsg`).
- **Rationale**: Projected children are ordinary rows that must be enriched
  (priced) first; live children are synthetic rows that must NOT go through
  enrichment (no plugin prices `finfocus:k8s/namespace:Allocation`). Running
  expansion last keeps `EnrichOverviewRows` untouched and avoids index
  drift in `OverviewResourceLoadedMsg`.
- **Alternatives considered**: expanding before enrichment was rejected
  because synthetic live rows would enter the pricing pipeline and because
  the TUI streams row updates by slice index.

## Decision 2: Cluster → kubeconfig context resolution without client-go in core

- **Decision**: Core never parses kubeconfig. It resolves a *scope string*
  and hands it to the usage-source plugin (`GetStatsRequest.Scope`), which
  already owns kubeconfig loading (plugins/kubernetes). Precedence:
  (a) `overview.cluster_contexts` config mapping keyed by cluster name or
  full URN; (b) the cluster's `name` property (or the final segment of its
  `arn` property) used verbatim as the context name; (c) single-cluster
  stacks fall back to empty scope (plugin's current context) with an
  "assumed" footnote. If the usage call fails, the cluster falls back to
  projected children.
- **Rationale**: `RunClusterAllocation` already takes `ClusterRequest.Scope`
  as an opaque string; the plugin interprets it. This keeps client-go out of
  core and matches the plugin-first constitution principle.
- **Alternatives considered**: parsing `~/.kube/config` in core to validate
  context names (rejected: new heavy dependency, duplicates plugin logic);
  requiring explicit config only (rejected: poor defaults for the common
  single-cluster case).

## Decision 3: Row shape for parent/child in JSON/NDJSON

- **Decision**: Flat `resources` list, unchanged order semantics. Children
  carry `parentUrn` + `expansionSource` (`"live"`|`"projected"`); cluster
  rows with children carry `childUrns` in display order. Live children are
  synthetic `OverviewRow`s with URN `<clusterURN>#ns/<namespace>` and type
  `finfocus:k8s/namespace:Allocation`. Suppressed/assumed notes ride on
  `StackContext.ExpansionNotes []string` (omitempty).
- **Rationale**: NDJSON streams one row per line, so child→parent references
  are mandatory; adding the parent's `childUrns` makes JSON consumption
  trivial. New fields are all `omitempty`, so unexpanded output is
  byte-identical to today (FR-008, SC-002).
- **Alternatives considered**: nested `children` arrays (rejected: breaks
  NDJSON's flat line convention and the `[]OverviewRow` schema consumers).

## Decision 4: Totals treatment

- **Decision**: `aggregateOverviewRows` skips rows whose
  `ExpansionSource == "live"`. Projected children keep contributing —
  they were flat rows contributing before expansion, so totals are unchanged
  on the no-plugins path. When live wins, suppressed projected rows are
  removed from the list and live rows are not summed; the footnote explains
  both.
- **Rationale**: Live allocation re-splits node cost already represented by
  node/cluster rows; summing it would double count.
- **Alternatives considered**: summing live children and excluding node
  rows (rejected: node rows are not reliably identifiable per cluster).

## Decision 5: Grouping projected workloads to clusters

- **Decision**: Single-cluster stacks group all five workload kinds under
  the cluster. Multi-cluster stacks leave workloads flat (live expansion
  still applies per cluster); the limitation is documented. The workload's
  Pulumi `provider` field is not currently propagated into
  `engine.StateResource`, so provider-chain attribution is future work.
- **Rationale**: Zero ambiguity for the common case; no wrong attribution
  for the rare multi-cluster case.

## Decision 6: TUI interaction

- **Decision**: Cluster rows with children show `▸`/`▾` in the Resource
  cell, start collapsed, and toggle with `e`, `→` (expand), `←` (collapse).
  Children render indented with a ↳ prefix immediately under the parent regardless
  of sort order. A display-order index maps the table cursor to the backing
  row so Enter/detail keeps working. When the filter box is active, rows
  render flat (expansion ignored) — matching the filter's existing flat
  substring semantics.
- **Rationale**: Keeps the existing flat sort/filter model intact for
  parents while making hierarchy explicit; `enter` already opens detail.
- **Alternatives considered**: always-expanded TUI (rejected: noisy for
  large clusters); tree component (rejected: disproportionate rework).

## Decision 7: Plain table rendering

- **Decision**: Children render as indented rows with a ↳ prefix directly
  beneath their parent (expansion function orders the slice parent-first,
  children immediately after). Footnotes print after the state-only
  footnote block: `†` notes from `StackContext.ExpansionNotes`.
- **Rationale**: Plain output is non-interactive, so always-expanded is the
  only faithful rendering; indentation preserves scannability.
