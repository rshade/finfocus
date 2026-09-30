# Data Model: `finfocus cost cluster`

**Date**: 2026-09-24 (converted 2026-09-29)
**Source**: SP3 plan Tasks 2–5. Wire types (`UsageRow`, `PricedResource`,
`AllocationRow`) are owned by finfocus-spec and documented in
`specs/612-k8s-cost-allocation/data-model.md`.

## Engine Types (`internal/engine/cluster.go`)

### ClusterRequest

| Field | Type | Notes |
| --- | --- | --- |
| `Scope` | `string` | Kubeconfig context / cluster id |
| `Namespace` | `string` | Empty = all namespaces |
| `Selector` | `map[string]string` | Label selectors |
| `PolicyJSON` | `[]byte` | Standardized JSON; nil = plugin defaults |

### ClusterRow

One allocator row in core terms.

| Field | Type | JSON | Notes |
| --- | --- | --- | --- |
| `Subject` | `map[string]string` | `subject` | `kind` ∈ {`workload`, `__idle__`, `__cluster__`} |
| `CPUCost` | `float64` | `cpu_cost` | |
| `MemCost` | `float64` | `mem_cost` | |
| `TotalCost` | `float64` | `total_cost` | |
| `Note` | `string` | `note,omitempty` | |

### PricedSummary

| Field | Type | JSON | Notes |
| --- | --- | --- | --- |
| `Kind` | `string` | `kind` | `node` or `cluster` |
| `ID` | `string` | `id` | Node name / cluster id |
| `ResourceType` | `string` | `resource_type` | |
| `SKU` | `string` | `sku` | |
| `Monthly` | `float64` | `monthly` | Projected monthly cost |
| `Priced` | `bool` | `priced` | False when monthly `<= 0` or structured error |
| `Note` | `string` | `note,omitempty` | |

### ClusterResult

| Field | Type | Notes |
| --- | --- | --- |
| `Mode` | `string` | `ModeRunRate = "run-rate"` (only mode this feature) |
| `Currency` | `string` | Resolved by `pluginsdk.ResolveCurrency` |
| `Rows` | `[]ClusterRow` | Ungrouped allocator rows |
| `Priced` | `[]PricedSummary` | One per priceable |
| `Total` | `float64` | Σ `priced=true` costs |
| `Idle` | `float64` | Σ `__idle__` rows |
| `NamespaceScoped` | `bool` | Idle/cluster rows omitted when true |
| `Incomplete` | `bool` | Any priceable unpriced |
| `PolicyDigest` | `string` | Hex SHA-256 of canonical effective policy |
| `EffectivePolicy` | `json.RawMessage` | Defaults + overrides as applied |
| `Warnings` | `[]string` | Rendered to stderr |

Constants/errors: `ModeRunRate`, `ErrConservation`
("allocator violated cost conservation"), `ErrHistoricalUnsupported`
("historical usage is not supported yet").

## Grouping (`internal/engine/cluster_group.go`)

### ClusterGroup

| Field | Type | JSON | Notes |
| --- | --- | --- | --- |
| `Key` | `string` | `key` | See key rules below |
| `CPUCost` | `float64` | `cpu_cost` | |
| `MemCost` | `float64` | `mem_cost` | |
| `TotalCost` | `float64` | `total_cost` | |
| `Rows` | `int` | `rows` | |
| `Notes` | `[]string` | `notes,omitempty` | De-duplicated, sorted |

Key rules: `__cluster__` rows → `__cluster__` for every dimension; `__idle__`
rows → node name when grouping by `node`, else `__idle__`; workload rows:
`namespace` → `namespace`; `controller` → `namespace/controller_kind/controller`;
`pod` → `namespace/pod`; `node` → `node`; `label:<k>` → the `label.<k>` value.
Missing value → `GroupKeyNone = "<none>"`. Groups sort by `TotalCost`
descending, then key ascending. `ValidateClusterGroupBy` rejects unknown
dimensions before plugins load.

## Policy Resolution (`internal/config/allocation_policy.go`)

```go
const AllocationPolicyFile = "allocation.hujson"

type AllocationPolicy struct {
    JSON   []byte // standard JSON; nil = plugin defaults
    Source string // path, or "" for built-in defaults
}

func ResolveAllocationPolicy(ctx context.Context, flagPath string) (AllocationPolicy, error)
```

Precedence: `--policy` > `$PROJECT/.finfocus/allocation.hujson` >
`<ResolveConfigDir()>/allocation.hujson` > none. First found wins; HuJSON is
standardized via `ax.ParseConfig`. A discovered file that fails to parse is
fatal; an explicit missing `--policy` is fatal.

## CLI Types

`NewCostClusterCmd()` registers under `cost`; `selectCapablePlugin(clients,
capability, explicit)` implements flag > singleton selection; rendering lives
in `internal/cli/cost_cluster_render.go`. The JSON output contract
(`clusterOutput`, `clusterPolicyOutput`, NDJSON line types) is specified in
[contracts/cost-cluster.md](contracts/cost-cluster.md).
