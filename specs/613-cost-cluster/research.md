# Research: `finfocus cost cluster`

**Date**: 2026-09-24 (converted 2026-09-29)
**Source**: SP3 plan review focus and global constraints, plus design-doc §8
amendments that affect the command. See
`specs/612-k8s-cost-allocation/research.md` for the plugin-side decisions.

## Decisions

### D1: Plugin selection is explicit flag or single capable plugin

- **Decision**: `--usage-source` / `--allocator` flags take precedence;
  otherwise the single installed plugin declaring `usage_stats` / `allocation`
  is used. Zero or ambiguous candidates are fatal errors listing candidates.
  Config routing for these capabilities is **deferred** (§8.2).
- **Rationale**: Router features are a fixed list in
  `internal/router/features.go`; extending routing is a separate feature. With
  exactly one usage source and one allocator shipping, flag-or-singleton keeps
  behavior predictable.
- **Alternatives rejected**: wiring the router now (scope creep; the router's
  feature list is deliberately fixed).

### D2: `$0` price = unpriced

- **Decision**: A priceable whose projected monthly cost is `<= 0` or carries a
  `StructuredError` is treated as unpriced (§8.5).
- **Rationale**: aws-public answers unknown instance types with `$0` and a
  "not found in pricing data" note instead of an error; a real node is never
  free. Without this rule every workload on such a node looks free.

### D3: Conservation is verified in core via SDK helpers

- **Decision**: `VerifyConservation` wraps
  `pluginsdk.ValidateAllocateResponse` + `pluginsdk.CheckConservation` at
  `pluginsdk.DefaultConservationEpsilon`; violations are fatal
  (`ErrConservation`, `PLUGIN_ERROR`).
- **Rationale**: The conformance suite checks allocators, but third-party
  allocators exist; one shared SDK rule keeps core, allocators, and the
  conformance suite in agreement (§8.14). A failure must never render as a
  plausible number.

### D4: Policy resolution order and fatal parse errors

- **Decision**: `--policy` > `$PROJECT/.finfocus/allocation.hujson` >
  `<ResolveConfigDir()>/allocation.hujson` > none. First found wins; files do
  not merge with each other. HuJSON is standardized via `ax.ParseConfig` (the
  project's HuJSON path, §8.3), not `hujson.Standardize`. A discovered file
  that fails to parse is fatal; an explicit `--policy` that is missing is
  fatal.
- **Rationale**: Allocation policy is organizational policy meant to be
  committed and reviewed, deliberately separate from `config.hujson` (whose
  shallow top-level merge would clobber the whole `plugins` key). Silent
  fallback to defaults would hide policy mistakes.

### D5: Validate flags before loading plugins or state

- **Decision**: `--output` (via `resolveOutputFormat`, honoring an explicitly
  set flag first) and `--group-by` are validated before `openPlugins` runs.
- **Rationale**: CLAUDE.md gotcha — commands that early-return must reject
  unknown formats first, or an invalid `--output` silently exits 0.

### D6: Grouping is generic string-map aggregation

- **Decision**: Group keys come from the subject map (`namespace`,
  `namespace/controller_kind/controller`, `namespace/pod`, `node`,
  `label.<k>`), with `<none>` for missing values and fixed placement for
  `__idle__`/`__cluster__` rows.
- **Rationale**: Core stays Kubernetes-agnostic — the same aggregator works
  for any usage source whose subjects follow the documented keys.

### D7: Results join by `(type, id)`, never slice index

- **Decision**: Priced results are matched back to priceable descriptors by
  `(resource_type, id)`.
- **Rationale**: The engine may reorder or skip results; index-based joins
  silently misattribute costs.

### D8: MCP exposure by default

- **Decision**: `cost cluster` is exposed as an MCP tool (read-only, bounded,
  no positional args, so ax-go lists it automatically); `tools.golden` is
  regenerated.
- **Rationale**: Consistent with other read-only cost commands; exclusion
  would need a reason and an `mcpExcludedCommands` entry instead.

### D9: kind E2E derives expectations from reported prices

- **Decision**: Nodes are labeled with `kubectl label` (instance type, region,
  `finfocus.dev/provider=aws`); aws-public is installed from its release with
  `--metadata region=us-east-1`; the expected Deployment cost is computed from
  live node allocatable and the node price the command itself reports
  (`priced[]`), with only a plausibility band on the m5.large price (§8.13).
- **Rationale**: Hardcoding aws-public's price table would make the E2E test
  the pricing table's owner; deriving from reported prices keeps the test
  about the pipeline, not the price list.
