# Quickstart: Historical cluster cost

**Date**: 2026-10-04
**Spec**: [spec.md](spec.md)

This is the check a developer runs after implementation. It is not a second
spec.

## What you need

- A kubeconfig context for the cluster.
- Prometheus scraping cAdvisor (`container_cpu_usage_seconds_total`,
  `container_memory_working_set_bytes`) and kube-state-metrics
  (`kube_node_status_allocatable`, `kube_node_labels`, `kube_node_info`,
  `kube_pod_owner`, `kube_pod_labels`, `kube_pod_info`).
- The kubernetes plugin installed (it still allocates).
- The prometheus plugin installed and selected.
- An actual-cost plugin that can price the node descriptors. Without one,
  the report is incomplete rather than a projected month.

## Ask for last week

```bash
export FINFOCUS_PROMETHEUS_URL=http://127.0.0.1:9090
finfocus cost cluster \
  --usage-source prometheus \
  --allocator kubernetes \
  --from 2026-09-28 \
  --to 2026-10-05 \
  --output json
```

Expect `mode` `historical`, `period` covering that window, and group totals
that sum to `total`. `priced[].monthly` is the window spend. A namespace that
used half the CPU it reserved is charged on the used hours.

No `--from` still prints the run-rate report (`mode` `run-rate`,
`period` `monthly`).

## Failure checks

```bash
finfocus cost cluster --from 2026-10-05 --to 2026-09-28
finfocus cost cluster --usage-source kubernetes --from 2026-09-28 --to 2026-10-05
finfocus cost cluster --usage-source prometheus
```

The first fails before any plugin (end is not after start). The second fails
with the kubernetes run-rate-only error. The third fails because the
Prometheus source is historical only.

Unset `FINFOCUS_PROMETHEUS_URL` outside a cluster and the error names that
variable. Set `FINFOCUS_PROMETHEUS_BEARER_TOKEN` to a distinctive value and
confirm it does not appear in the error.

## Local cluster

`make test-e2e-kind` runs the existing run-rate kind test, then installs
Prometheus on that cluster, writes a fixed usage fixture, and runs the
windowed command against a home that contains only the allocator, the
prometheus plugin, and the fixed-cost test plugin. It does not need AWS
credentials. The assertion is conservation within 1e-6 and the fixture's
resource-hours.

## Not in this change

`finfocus plugin install prometheus` does not work until tag
`prometheus-v0.1.0` exists and a follow-up adds the registry entry. Until
then, install the binary the way the kind test does, by copying it under
`FINFOCUS_HOME`.
