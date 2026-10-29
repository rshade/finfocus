# Research: Dependency Blast Radius

All findings come from reading the code on `main` (a89a46b1) on 2026-10-28.
Nothing was run against a real stack; the one item that needs a real state file
to confirm is flagged in R1.

## R1. Where the dependency edges live today

**Decision**: Build the graph from the links Pulumi records on each resource
(`dependencies`, `propertyDependencies`, `provider`, `deletedWith`,
`replaceWith`; R12), read from the full deployment resource list (custom and
component) and from preview steps.

**Findings**:

- `ingest.StackExportResource` (`internal/ingest/state.go`) and
  `ingest.PulumiStep` / `ingest.PulumiState` / `ingest.PulumiResource`
  (`internal/ingest/pulumi_plan.go`) already carry `Parent`, `Dependencies`
  and `PropertyDependencies`.
- Both are dropped on the way to the overview. `stackload.convertStateResources`
  builds `engine.StateResource` without them, and `convertPlanSteps` builds
  `engine.PlanStep` without them. Outside `internal/ingest`, only
  `state.go` and `map_resource.go` read them, and only to build `Refs`
  (spec 618).
- `stackload.Snapshot` keeps `RawState` (custom resources only), `ComponentURNs`
  (URNs only, no parent or dependencies) and `Plan`. Components are therefore
  present at load time (`load.go` iterates `state.Deployment.Resources`) but
  their `Parent` and `Dependencies` are not retained.
- `engine.OverviewRowResult` already has `ParentURN` / `ChildURNs`, but those
  belong to Kubernetes cluster expansion. They must not be reused for
  dependencies.

**Rationale**: The snapshot is the one object both the terminal pipeline
(`OverviewPipelineResult.Snapshot`) and the web session
(`webData.snapshot`) already share, so the graph belongs there.

**Confirmed against a real state file (2026-10-28, T001)**: a Go program with
a component `grp` whose children `a` and `b` are custom resources, and a
resource `after` declared `dependsOn: [grp]`, was deployed to a local backend.
The exported state records `after.dependencies = [a, b]`. It does **not** list
`grp`. Pulumi expands a component into its members when it records the edge.
Components carry empty `dependencies`. Children carry `parent: grp`, and
`parent` creates no edge.

**Alternatives considered**: reading edges from the plugin lineage metadata
named in issue #366 (`CostAllocationLineage`). Rejected: the symbol does not
exist in `finfocus-spec` and never has, so it would add a cross-repo
dependency for data core already has.

## R2. What counts as a dependent

**Decision**: Resource A depends on resource B when A's `dependencies`,
`propertyDependencies`, `deletedWith` or `replaceWith` names B, or A's
`provider` reference names B (R12). Parent-child creates no edge, and there
is no ancestor walk. This is Pulumi's own `DependingOn` with
`includeChildren` false.

**Rationale**: R1 showed Pulumi already expands `dependsOn: [component]` into
the component's members, so a database inside a component is listed by every
resource that depends on the component. Walking `parent` as well would add
nothing for those, and it would break FR-005 for the stack, because
`pulumi:pulumi:Stack` is the parent of everything.

**Alternatives considered**: (a) walk ancestors of B (the original design,
dropped after R1). (b) treat every child as a dependent of its parent: floods
the tree with the whole stack. (c) `dependencies` and `propertyDependencies`
only, as first shipped: misses every resource reached through a provider
(ISS-001, R12).

## R3. Traversal and tree shape

**Decision**: Breadth-first search backward over the reverse index from the
selected URN. Each resource is visited once, at its shortest distance, so
diamonds list once and a cycle cannot loop. A resource's tree parent (`via`) is
the first resource that reached it in a deterministic order (sorted by URN
within a level).

**Components**: They are traversed and listed (flagged `component`, no price)
so the tree stays connected, but they are excluded from the count, the total
and the unpriced count.

**Rationale**: BFS gives the "shortest distance" display for free and visits
O(V+E). At a few hundred resources the cost is negligible, so nothing is
cached beyond the reverse index built once per snapshot.

**Alternatives considered**: depth-first with path tracking. Rejected: it
lists a diamond twice and needs separate cycle logic.

## R4. Which cost, and which dependents are summed

**Decision**: Sum `OverviewRow.ProjectedCost.MonthlyCost` (the Monthly column
basis). Counted: Active, Updating, Replacing, Creating. Not counted: Deleting
(no projected cost), unpriced rows, components. The result reports the number
left out per reason.

**Priced means**: `ProjectedCost != nil` and `!ProjectedPricingFailed`. A
`$0.00` price is a price (CLAUDE.md, router section). A row that only reports
`NO_COST_DATA` has no `ProjectedCost` and is unpriced.

**Rationale**: `MTDCost` and `MonthlyCost` are different units and must never
be summed together (CLAUDE.md, Overview Field Semantics). One basis keeps the
SC-003 invariant checkable.

**Currency**: Plugins report a currency per row. If the counted rows disagree,
the total is withheld and `mixedCurrency` is set; a blank currency is USD.

## R5. Where the computation lives

**Decision**: A pure function in `internal/engine`,
`ComputeBlastRadius(graph DependencyGraph, rows []OverviewRow, urn string)
BlastRadius`, next to `ComputeOverviewResult`. The terminal view and the web
handler both call it.

**Rationale**: `OverviewRowResult` is documented as "exactly one row schema"
shared by the table, JSON, TUI and web. `engine` cannot import `ingest` (an
import cycle: ingest already imports engine), so `stackload` converts
`ingest` types into an `engine.DependencyGraph`, the same way it already
converts state and steps.

**Alternatives considered**: computing in `internal/viewmodel` (used by the
web). Rejected: the TUI would need a second copy, which is the drift FR-009
forbids.

## R6. Graph construction through a preview

**Decision**: `Snapshot.Graph` is built at load from all deployment resources.
`Snapshot.WithPlan` rebuilds it by overlaying the preview: `create` adds a
node with the step's new-state edges; `update` and `replace` replace the
node's edges with the new state's; `delete` keeps the node, flagged so the
radius marks it deleting; `same` steps (shown only with `--show-sames`) add
nothing new. Unchanged resources missing from a default preview keep their
state edges.

**Rationale**: matches how `engine.MergeResourcesForOverview` already treats
state and steps, and `WithPlan` already returns a new snapshot, so the web's
on-demand preview picks up the new graph through the existing
`setPipelineSource` publish.

## R7. Terraform and OpenTofu

**Decision**: `Graph.Available` is false and the section shows "dependency
information is not available".

**Rationale**: Terraform plans parse through `pluginsdk.CostParseProvider`,
whose `ParseResponse` carries no dependencies (spec 1732). Adding them is a
spec change for another feature.

## R8. Terminal rendering

**Decision**: Append a "BLAST RADIUS" section to `renderDetailViewForDay`
(`internal/tui/overview_view.go`) after the cost sections and before the
budget block, using the existing `visibleDetail` scrolling and the
accessibility plain mode for tree glyphs.

**Constitution gate**: TUI changes need rendered output verification and
golden files (`internal/tui/testdata/detail_*.golden`). The plan adds goldens
for a populated tree, an empty radius, an unavailable graph and a mid-change
tree.

## R9. Web rendering

**Decision**: `GET /api/overview/resource` (`handleOverviewResource`) gains a
`blastRadius` object on `ResourceDetail`, computed by `ComputeBlastRadius`
from the session rows and the published snapshot's graph. `overview.js`
renders it in the detail panel as a `.detail-table`. No new route, no new
setting.

**Rationale**: the handler already returns the row and its display model for
one URN, and the detail panel is where the spec places the section.

## R10. Security and redaction

**Decision**: The payload carries URN, type, display name, status, distance,
`via`, monthly cost and flags. No properties, tags or inputs. It still passes
through the web JSON path that runs `RedactJSON`; there are no data-keyed maps
in it, so no `datakey` tag is needed.

## R11. Not needed

- No new setting, so no `config.KnownSettings` entry.
- No new persistent state, so the Persistence Model principle is not engaged.
- No protocol change and no `finfocus-spec` change.
- Non-interactive output is unchanged (FR-013).

## R12. Provider, `deletedWith` and `replaceWith` links (ISS-001)

**Decision**: Three more Pulumi links are edges, from state and from preview
step states, decided in the spec Clarifications of 2026-10-28:

- `provider`: Pulumi records it as `<provider-urn>::<provider-id>`. The edge
  names the URN, which is the reference up to its last `::`. URNs contain
  `::`, so the split must be on the last one (the rule Pulumi's
  `providers.ParseReference` uses). A reference with no `::` adds no edge.
  A preview's unknown-id sentinel (`04da6b54-...`) sits only in the id
  segment, so the URN is still usable; the graph never leaves core, so the
  sentinel is not sent anywhere.
- `deletedWith`: one URN.
- `replaceWith`: a list of URNs.

`DependsOn` stays one sorted, de-duplicated list of every link kind; the
engine does not learn which kind an edge came from (FR-012 shows no such
column). `Available` keeps its rule, any node with a non-empty `DependsOn`,
which now includes a stack whose only links are provider references.

A preview step state that omits `custom` falls back to the `pulumi:` type
prefix to mark a component (`isStepComponent`). Providers
(`pulumi:providers:*`) are custom resources, so the fallback must exclude
that prefix, or a provider created in a preview is labeled a component and
left out of the count. Real previews always carry `custom` (it has no
`omitempty` on `apitype.ResourceV3`), so this matters for fixtures and
hand-edited files.

**Rationale**: Pulumi's `DependencyGraph.DependingOn`, the graph behind
`pulumi destroy --target-dependents`, follows exactly these links plus
`dependencies` and `propertyDependencies`, and `parent` only on request
([`pkg/resource/graph/dependency_graph.go`](https://github.com/pulumi/pulumi/blob/a050d5fe0db89367c795338e7e18985567556fc3/pkg/resource/graph/dependency_graph.go#L55-L96);
the link set is `State.GetAllDependencies` in
[`pkg/resource/state.go`](https://github.com/pulumi/pulumi/blob/a050d5fe0db89367c795338e7e18985567556fc3/pkg/resource/state.go#L361-L392)).
On the `initial-30` stack, seven Kubernetes resources reach their cluster
only through the `civo-k8s` provider, so without the provider edge selecting
the cluster showed one dependent instead of eight. Both `deletedWith` and
`replaceWith` are present on `apitype.ResourceV3` at the pinned SDK
(v3.270.0), which is the shape of state exports and of preview step states.

Effect on the total: an added link changes the count or total only when it
reaches a resource no other edge reaches, because each dependent is listed
once (FR-002). The common `deletedWith: cluster` on Kubernetes resources is a
second route to rows the provider edge already reaches. A replaced resource
comes back, so `replaceWith` marks what a change touches without implying
savings.

**Alternatives considered**: (a) provider edges only for explicit providers,
skipping `default_<version>`: rejected in Clarifications; it hides real
dependents and a default provider is a real source of them. (b) leave
`deletedWith` and `replaceWith` out: keeps the same class of gap for rarer
stacks. (c) depend on `github.com/pulumi/pulumi/pkg/v3` to call
`GetAllDependencies` and `ParseReference` directly: pulls the Pulumi engine
module into core for one string split and two fields.
