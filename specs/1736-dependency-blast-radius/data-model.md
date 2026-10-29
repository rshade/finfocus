# Data Model: Dependency Blast Radius

All types live in `internal/engine` (pure data, no I/O) unless noted. None is
persisted.

## DependencyGraph

The relationships of one stack load, converted from `ingest` types by
`internal/stackload`.

| Field | Type | Meaning |
| --- | --- | --- |
| `Available` | bool | False when the source carries no dependency data (Terraform, or a state with no link of any kind); a provider reference alone makes it true |
| `Nodes` | map of URN to `DependencyNode` | Every deployment and preview resource, including components |

`DependencyNode`:

| Field | Type | Meaning |
| --- | --- | --- |
| `URN` | string | Resource identity |
| `Type` | string | Resource type, shown for a dependent with no overview row (a component, or a create the overview has not priced) |
| `DependsOn` | list of string | Union of `dependencies`, every list in `propertyDependencies`, `deletedWith`, every `replaceWith` entry, and the provider URN from `provider` (research R12), de-duplicated and sorted |
| `Component` | bool | True for a non-custom resource; a preview step uses its state's `custom` field, falling back to `pulumi:` types when absent |
| `Deleting` | bool | True when the preview deletes it |
| `Creating` | bool | True when the preview creates it; labels a dependent `new` when no overview row exists |

Derived once per graph, not stored in the snapshot: a reverse index from
each URN to the URNs that list it in `DependsOn`.

**Validation**: a URN in `DependsOn` with no entry in `Nodes` is
kept and counted as unresolved when traversal meets it. `Nodes` never contains
the empty URN.

**Provider reference**: `<provider-urn>::<provider-id>`, split on the last
`::`. A reference with no `::` contributes no edge. The id segment, including
a preview's unknown-id sentinel, is discarded. A provider URN missing from
`Nodes` is unresolved like any other reference.

**Source fields**: `ingest.StackExportResource` and `ingest.PulumiState`
gain `DeletedWith string` (`json:"deletedWith,omitempty"`) and
`ReplaceWith []string` (`json:"replaceWith,omitempty"`), matching
`apitype.ResourceV3`. Both already carry `Provider`. The step-level fallback
(a step with no state) reads `PulumiStep.Provider`, `Dependencies` and
`PropertyDependencies`; `PulumiStep` gains no new fields.

## BlastRadius

Result of `ComputeBlastRadius` for one selected URN.

| Field | Type | Meaning |
| --- | --- | --- |
| `URN` | string | The selected resource |
| `Available` | bool | Copy of `DependencyGraph.Available` |
| `Dependents` | list of `BlastDependent` | Tree rows in display order (by distance, then URN) |
| `Count` | int | Dependents excluding components |
| `MonthlyTotal` | `*float64` | Sum of counted dependents' projected monthly cost; nil when withheld |
| `Currency` | string | Currency of the total; blank means USD |
| `MixedCurrency` | bool | True when counted rows disagree on currency (total withheld) |
| `LeftOut` | `BlastLeftOut` | Counts excluded from the total, by reason |
| `SummaryDisplay` | string | Count and total line, such as `2 dependents, $10.00/month` |
| `NoteDisplay` | string | What the total leaves out and why; blank when nothing is left out |

`BlastLeftOut`: `Deleting` (int), `Unpriced` (int), `Unresolved` (int).

`BlastDependent`:

| Field | Type | Meaning |
| --- | --- | --- |
| `URN` | string | Dependent identity |
| `Type` | string | Resource type |
| `DisplayName` | string | Rightmost URN component |
| `Status` | `ResourceStatus` | Active, Updating, Replacing, Creating, Deleting |
| `Depth` | int | Shortest distance from the selected resource, 1 for direct |
| `Via` | string | URN of the tree parent, the first resource that reached this one |
| `MonthlyCost` | `*float64` | Projected monthly cost; nil when unpriced, deleting or a component |
| `Priced` | bool | `ProjectedCost != nil` and not `ProjectedPricingFailed` |
| `Component` | bool | Listed for tree continuity; no price, not counted |
| `MonthlyDisplay` | string | Cost cell: the formatted cost, `no price`, or `-` (deleting or component) |
| `StatusLabel` | string | `new`, `deleting`, `replacing`, `updating`, `component`, or blank when active |

The display strings are filled by `ComputeBlastRadius` with
`FormatOverviewCurrency`, as for the overview rows, so the terminal and the
web render the same text and the browser formats no number (FR-011b).

**Invariants** (these are the test oracles for SC-003):

1. Each URN appears at most once in `Dependents`, and the selected URN never
   appears in it.
2. `Count` equals the number of non-component dependents.
3. `MonthlyTotal` equals the sum of `MonthlyCost` over dependents that are
   priced, not deleting and not components.
4. `LeftOut.Deleting` equals the number of deleting dependents;
   `LeftOut.Unpriced` equals the number of non-component, non-deleting
   dependents that are not priced.
5. For every dependent, `Via` is the selected URN (depth 1) or a URN present
   earlier in `Dependents` (depth greater than 1), so the list renders as a
   tree in one pass.

**State transitions**: none. A new `BlastRadius` is computed per call from the
current rows and graph; the web recomputes it after a preview replaces the
snapshot.

## Relationship to existing types

- Rows come from `engine.OverviewRow` (`URN`, `Type`, `Status`,
  `ProjectedCost`, `ProjectedPricingFailed`). Nothing is added to
  `OverviewRow` or `OverviewRowResult`.
- `stackload.Snapshot` gains `Graph engine.DependencyGraph` and a
  `Graph()` accessor, rebuilt by `WithPlan`.
- `webui.ResourceDetail` gains `BlastRadius *engine.BlastRadius`.
