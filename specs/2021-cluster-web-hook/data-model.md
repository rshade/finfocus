# Data Model: Cluster Allocation Web Hook

## ClusterQuery (web request, `internal/webui`)

| Field | Type | CLI flag | Default |
| --- | --- | --- | --- |
| `context` | string | `--context` | current context |
| `namespace` | string | `--namespace` | all |
| `selector` | string[] | `--selector` (repeatable) | none |
| `groupBy` | string | `--group-by` | `namespace` |
| `from` | string | `--from` | run rate |
| `to` | string | `--to` | now when `from` is set |
| `usageSource` | string | `--usage-source` | automatic |
| `allocator` | string | `--allocator` | automatic |
| `policy` | string (HuJSON text, optional) | `--policy` (as a file) | discovered file |

Validation is the CLI's: group-by in the allowed set or `label:<key>`;
selectors `key=value` with a non-empty key; `to` requires `from`; window
rules from `ParseTimeRange`; policy text valid HuJSON within the size limit.

## ClusterPolicyQuery

`allocator` and `policy`, as above.

## clusterInput (internal, `internal/cli`)

Validated form shared by CLI and web: context, namespace, selector map,
group-by, from/to times, usage-source and allocator names,
`config.AllocationPolicy`.

## engine.ClusterReport

Moved unchanged from `clusterOutput`: `mode`, `period`, `currency`,
`group_by`, `total`, `idle` (omitted when scoped), `namespace_scoped`,
`selector_scoped`, `incomplete`, `groups`, `capacity`, `priced`, `policy`,
`warnings`.

## engine.ClusterPolicy

`source` (path, `built-in defaults`, or `request body`), `digest`,
`effective`.

## Errors

- `engine.ClusterInputError{Err}`: the caller's input; CLI exit 2, web 400
  `invalid_cluster_query`. `Error()` is the inner message.
- `engine.ClusterPluginMissingError{Capability}`: no installed plugin
  declares the capability; CLI exit 1 with the existing message, web 200
  `status: "unavailable"`.

## viewmodel.ClusterReportDisplay

`total`, `idle`, `idleShare`, `scope`, `mode`, `policy`, `groups[]`
(`key`, `cpu`, `memory`, `total`, `share`), `capacity[]` (`capacityType`,
`total`, `share`, `idle`), `warnings[]`, `incompleteCauses[]`,
`unpriced[]`. Lists follow the report's order and are never null.
