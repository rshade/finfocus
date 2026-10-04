# Contract: kubernetes plugin projected cost

## Environment

| Variable | Unit | Required for |
| --- | --- | --- |
| `FINFOCUS_KUBERNETES_CPU_HOURLY_RATE` | USD per vCPU-hour | all five kinds |
| `FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE` | USD per GiB-hour | all five kinds |
| `FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT` | pods | DaemonSet |
| `FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH` | hours | Job, CronJob |

An invalid value never stops the plugin; projected pricing declines with a
reason naming the variable.

## Supports

- `Supported: true` exactly when `Estimate` returns `Priced`.
- Otherwise `Supported: false` with `Reason` at most 160 characters (core
  truncates longer reasons), naming the fix.

## GetProjectedCost

| Field | Value |
| --- | --- |
| `cost_per_month` | `pods × podHourly × hours` (hours = 730, or the Job variable) |
| `unit_price` | `podHourly = cpuCores × cpuRate + memGiB × memRate` |
| `currency` | `USD` |
| `billing_detail` | method, pod count and source, rate source, "not a real node price" |

When `Estimate` declines (only reachable if `Supports` was skipped), the RPC
returns `codes.FailedPrecondition` with the decline reason.

## Worked example

Deployment, `replicas: 3`, one container `cpu: 500m`, `memory: 1Gi`, rates
0.04 and 0.005: `podHourly = 0.5 × 0.04 + 1 × 0.005 = 0.025`;
`cost_per_month = 3 × 0.025 × 730 = 54.75`.
