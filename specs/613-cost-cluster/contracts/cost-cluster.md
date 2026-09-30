# Contracts: `finfocus cost cluster`

## CLI

```text
finfocus cost cluster
  [--context <kubecontext>]
  [--namespace <ns>] [--selector <k=v>]...
  [--group-by namespace|controller|pod|node|label:<key>]   # default: namespace
  [--policy <file>] [--show-policy]
  [--usage-source <plugin>] [--allocator <plugin>]
  [--output table|json|ndjson]
```

- Uses `resolveOutputFormat`; `--output` and `--group-by` are validated before
  plugins or state are loaded.
- All failures exit with code 1, consistent with other commands; error codes
  follow the design's error table (`VALIDATION_ERROR`, `PLUGIN_ERROR`,
  `NO_COST_DATA`, `TIMEOUT_ERROR`).
- Warnings go to stderr (`cmd.PrintErrln`), never stdout.

## JSON output (stable contract for agents and the E2E test)

```json
{
  "mode": "run-rate",
  "period": "monthly",
  "currency": "USD",
  "group_by": "namespace",
  "total": 1234.56,
  "idle": 345.67,
  "namespace_scoped": false,
  "incomplete": false,
  "groups": [
    {"key": "payments", "cpu_cost": 5.0, "mem_cost": 5.0, "total_cost": 10.0, "rows": 2}
  ],
  "priced": [
    {"kind": "node", "id": "n1", "resource_type": "aws:ec2/instance:Instance",
     "sku": "m5.large", "monthly": 70.08, "priced": true}
  ],
  "policy": {"source": ".finfocus/allocation.hujson", "digest": "<64 hex>", "effective": {}},
  "warnings": []
}
```

- `idle` is `null`/omitted when `namespace_scoped` is true.
- `policy.source` is the file path or `"built-in defaults"`.
- `groups[]` elements are `engine.ClusterGroup`; `priced[]` elements are
  `engine.PricedSummary`.

## NDJSON output

Mirrors `cost recommendations`: the first line is
`{"type":"summary", ...}` (the `clusterOutput` fields without `groups`),
followed by one `{"type":"group", ...ClusterGroup}` line per group.

## Table output

Tabwriter columns `GROUP  CPU  MEMORY  TOTAL  NOTES` with `%.2f` money, then a
footer:

```text
Mode:    run-rate (monthly, 730 h)
Total:   $1,234.56 USD
Idle:    $345.67 (28.0%)            ← or "Idle:    omitted (--namespace scoped)"
Policy:  .finfocus/allocation.hujson · 3f2a9c1b4d5e   ← or "built-in defaults · …"
Incomplete: 2 resources could not be priced (…)       ← only when incomplete
```

## MCP

`cost cluster` is exposed as an MCP tool (read-only, bounded, no positional
args). `internal/cli/testdata/mcp/tools.golden` is regenerated and the live
`tools/list` integration test
(`TestMCPServer_ToolListMatchesGoldenForBothEntryPoints`) must pass for both
entry points.

## Plugin RPCs

Consumed from finfocus-spec (see
`specs/612-k8s-cost-allocation/contracts/README.md`):
`UsageSourceService.GetStats` and `AllocatorService.Allocate`. Core verifies
every `AllocateResponse` with `pluginsdk.ValidateAllocateResponse` +
`pluginsdk.CheckConservation` at `pluginsdk.DefaultConservationEpsilon`.
