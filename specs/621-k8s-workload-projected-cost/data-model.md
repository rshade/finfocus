# Data Model: Kubernetes Workload Projected Cost

## Core (generic)

### `proto.ResourceDescriptor` (`internal/proto/adapter.go`)

Adds one field:

| Field | Type | Rule |
| --- | --- | --- |
| `Attributes` | `*structpb.Struct` | Built by `engine.BuildAttributes`; nil when no properties survive redaction or the size cap is exceeded |

`PrepareProjectedDescriptor` copies it onto `pbc.ResourceDescriptor.Attributes`.

### Attributes (built from `engine.ResourceDescriptor.Properties`)

- Source: the raw ingested map (outputs merged under inputs, `ingest.MapResource`).
- Omitted at any depth: keys rejected by `skipDottedSegment`; values for which
  `history.IsPulumiSecret` is true.
- Omitted at the top level: `ref` and `ref.*`.
- Kept: Pulumi unknown values, and `tags`, `tagsAll`, `labels`, `annotations`.
- Bound: `proto.Size(attrs) <= pluginsdk.MaxAttributesBytes` (65536), else nil.

### Cache keys

| Key | Change |
| --- | --- |
| Projected cost | `…/tags-<d>[/refs-<d>][/attrs-<d>][/pricing-spec]` |
| Supports (in memory) | `plugin:provider:type:region:sku:feature[:attrs-<d>]` |

`<d>` is 16 hex characters (first 8 bytes of SHA-256). The attributes digest
hashes the deterministic protobuf encoding.

### BatchCost chunk

A chunk holds at most `maxBatchSize` resources and at most
`maxBatchRequestBytes` (3 MiB) of summed descriptor size. One resource always
forms a valid chunk.

## kubernetes plugin

### `Config` (from the environment at startup)

| Field | Env var | Valid |
| --- | --- | --- |
| `CPUHourlyRate` | `FINFOCUS_KUBERNETES_CPU_HOURLY_RATE` | finite, `>= 0` |
| `MemoryGiBHourlyRate` | `FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE` | finite, `>= 0` |
| `DaemonSetNodeCount` | `FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT` | integer `>= 1` |
| `JobHoursPerMonth` | `FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH` | finite, `> 0`, `<= 744` |

Each field records whether it was set, and the parse error when it was set
but invalid.

### `WorkloadShape`

| Field | Meaning |
| --- | --- |
| `Kind` | Deployment, StatefulSet, DaemonSet, Job, CronJob |
| `Pods`, `PodsSource` | pod count and where it came from (`spec.replicas`, default, node-count variable, `parallelism`) |
| `CPUCores`, `MemoryGiB` | effective per-pod request |
| `CPUDeclared`, `MemoryDeclared` | whether any container declared each |
| `UsedLimits` | a request fell back to a limit |

### `Estimate` (the one function `Supports` and `GetProjectedCost` share)

Either `Priced{PodHourly, Monthly, Hours, Note}` or `Declined{Reason}`.
Decline order (the first that applies wins):

1. Type is not one of the five kinds, or the provider is not `kubernetes`: today's reason.
2. No attributes on the descriptor: core sent no inputs.
3. A count or quantity is a Pulumi unknown: not known until deployment.
4. A quantity does not parse: names the field.
5. Neither CPU nor memory declared: no resource requests.
6. A rate, or the kind's hint, is unset or invalid: names the variable.
