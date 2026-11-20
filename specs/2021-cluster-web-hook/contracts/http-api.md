# HTTP API: Cluster Allocation

Both endpoints are authenticated like every other dashboard POST: session
cookie, same `Origin`, `Content-Type: application/json`, `Sec-Fetch-Site`
same-origin. Responses pass `RedactJSON`.

## `POST /api/cluster/query`

### Request

```json
{
  "context": "prod",
  "namespace": "",
  "selector": ["team=payments"],
  "groupBy": "namespace",
  "from": "2026-11-01",
  "to": "2026-11-08",
  "usageSource": "",
  "allocator": "",
  "policy": "{ \"idle\": \"share\" }"
}
```

Every field is optional. Each takes the syntax of the `cost cluster` flag
of the same name. `policy` is HuJSON text; the server never reads a path
from the request, and a body with an unknown field such as `policyPath` is
400 `invalid_request`. The whole body is capped at 1 MiB like every
dashboard POST, so policy text must fit inside that with its JSON escaping;
the CLI's own 1 MiB policy limit therefore admits slightly larger files than
the page can send. Omitted `groupBy` is `namespace`, the flag's default.
Omitted `policy` uses the project, then global,
`allocation.hujson`, then the allocator's defaults. A discovered file is
named by its tier (`project allocation.hujson` or `global
allocation.hujson`) in `policy.source` and in errors, never by its path.

The query has no deadline of its own: each plugin call carries the CLI's
deadline (`engine.ClusterCallTimeouts`), and closing the request cancels
it.

### Response 200, report

```json
{
  "status": "ok",
  "mode": "run-rate",
  "period": "monthly",
  "currency": "USD",
  "group_by": "namespace",
  "total": 300,
  "idle": 60,
  "namespace_scoped": false,
  "selector_scoped": false,
  "incomplete": false,
  "groups": [{"key": "payments", "cpu_cost": 120, "mem_cost": 120, "total_cost": 240, "rows": 3}],
  "capacity": [],
  "priced": [{"kind": "node", "id": "n1", "resource_type": "aws:ec2/instance:Instance", "sku": "m5.large", "monthly": 300, "priced": true}],
  "policy": {"source": "built-in defaults", "digest": "abc…", "effective": {}},
  "display": {
    "total": "$300.00 USD",
    "idle": "$60.00 USD",
    "idleShare": "20.0%",
    "mode": "run-rate (monthly, 730 h)",
    "policy": "built-in defaults · abc123def456",
    "groups": [{"key": "payments", "cpu": "$120.00 USD", "memory": "$120.00 USD", "total": "$240.00 USD", "share": "80.0%"}],
    "capacity": [],
    "warnings": [],
    "incompleteCauses": [],
    "unpriced": []
  }
}
```

`display` lists follow the report's order. `scope` is `namespace`,
`selector`, or `namespace and selector` when the report is scoped, and then
`idle` and `idleShare` are absent. `warnings` are the summarized lines the
CLI prints on stderr (the report's `warnings` field keeps every one).
`incompleteCauses` are what the usage source reported missing and
`unpriced` names each resource that could not be priced, with its note.
When `incomplete` is true and both lists are empty, the warnings are the
explanation; the CLI's "see the warnings on stderr" line has no web
equivalent.

Fields other than `status` and `display` are exactly the
`cost cluster --output json` document for the same arguments, except that a
discovered policy file is named by tier instead of path. Every group,
including `__idle__`, `__cluster__`, and names that look like credentials,
passes redaction: `ClusterGroup.Key` is tagged `datakey:"true"`.

### Response 200, unavailable

```json
{
  "status": "unavailable",
  "code": "cluster_plugin_missing",
  "capability": "usage_stats",
  "error": "no installed plugin provides usage_stats; install one with: finfocus plugin install kubernetes"
}
```

### Errors

| Status | `code` | When | `error` text |
| --- | --- | --- | --- |
| 400 | `invalid_request` | Body is not JSON | Server's standard text |
| 400 | `invalid_cluster_query` | Any input `cost cluster` rejects with exit 2: group-by, selector, window, plugin choice, policy syntax or allocator rejection of the policy, usage source rejecting the request, several sources serving the mode | The CLI's message |
| 503 | `not_ready` | Session engine not loaded yet | `cluster data is loading` |
| 502 | `cluster_unavailable` | Any other plugin or allocation failure, including a conservation failure | `cluster data unavailable` |

## `POST /api/cluster/policy`

### Policy request

```json
{"allocator": "", "policy": ""}
```

### Policy response 200

```json
{
  "status": "ok",
  "source": "built-in defaults",
  "digest": "abc123def4567890",
  "effective": {},
  "display": {"source": "built-in defaults", "digest": "abc123def456"}
}
```

`source`, `digest`, and `effective` are the
`cost cluster --show-policy --output json` document. The unavailable
answer and the errors are those of `/api/cluster/query`; only the
allocation capability is required.

## Go methods the hook calls

| Step | Method | Shared with the CLI |
| --- | --- | --- |
| Validate the query | `cli.parseClusterQuery` (`engine.ValidateClusterGroupBy`, `cli.parseSelectors`, `cli.resolveClusterWindow`) | yes, `runCostCluster` |
| Resolve the policy | `config.ParseAllocationPolicy` (text) or `config.ResolveDiscoveredAllocationPolicy` (web); the CLI calls `config.ResolveAllocationPolicy` | same parser and limits |
| Choose plugins | `cli.selectUsageSource`, `cli.selectCapablePlugin`, `cli.clusterAllocator` | yes |
| Check a supplied policy | `engine.ShowAllocationPolicy` | yes |
| Build the report | `cli.buildClusterReport` | yes, `runCostCluster` |
| Run allocation | `engine.RunClusterAllocation` (conservation via `engine.VerifyConservation`) | yes |
| Group rows | `engine.GroupClusterRows` | yes |
| Build the document | `engine.NewClusterReport` | yes, `renderClusterResult` |
| Show the policy | `cli.buildClusterPolicy` -> `engine.ShowAllocationPolicy` | yes, `--show-policy` |
| Display text | `viewmodel.BuildClusterReportDisplay` (`viewmodel.ClusterModeLine`, `viewmodel.ClusterPolicyLine`, `viewmodel.ClusterShare`, `viewmodel.SummarizeClusterWarnings`, `viewmodel.ClusterUnpriced`) | the CLI table uses the same mode, policy, share, warning, and unpriced helpers |

The session callbacks are `cli.(*webData).cluster` and
`cli.(*webData).clusterPolicy`, wired as `webui.SessionOptions.Cluster` and
`webui.SessionOptions.ClusterPolicy`. Both run under the session query gate
with the dashboard engine (`(*webData).queryEngine`) and its open clients.
Errors are typed by the builder: `*engine.ClusterInputError` (CLI exit 2,
web 400) and `*engine.ClusterPluginMissingError` (CLI exit 1, web
"unavailable").
