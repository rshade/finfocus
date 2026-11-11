# Prometheus Usage Source

Reports historical Kubernetes CPU and memory usage for `finfocus cost cluster`.
This plugin is a usage source only. It does not allocate cost and it does not
price resources. The kubernetes plugin still allocates.

## Address

`FINFOCUS_PROMETHEUS_URL` is the Prometheus base URL. When that variable is
unset and the process is running in a cluster (`KUBERNETES_SERVICE_HOST` is
set), the plugin uses the Prometheus Operator service:

```text
http://prometheus-operated.monitoring.svc:9090
```

Any other chart sets `FINFOCUS_PROMETHEUS_URL`.

`FINFOCUS_PROMETHEUS_BEARER_TOKEN`, when set, is sent as a bearer credential.
The token is not written to logs or errors.

## Usage data

Each workload row carries `cpu_usage` and `mem_usage` integrated over the
window, in core-hours and GiB-hours. Each node row carries allocatable CPU
and memory integrated the same way. The source reports no `cpu_request` or
`mem_request`. The kubernetes allocator charges `max(request, usage)` (policy
`charge: max-request-usage`), so with this source a workload is charged for
what it consumed, not what it reserved. Node capacity that no workload
consumed stays on the node's `__idle__` row unless the policy sets
`idle: "share"`.

## Node labels

kube-state-metrics must publish node labels. The source reads
`kube_node_labels` and `kube_node_info` for a node's provider, instance type,
and region. A node that those series and the live API both cannot identify is
omitted.

## Pod annotations

The source reads pod labels from `kube_pod_labels` and does not read pod
annotations. `finfocus cost cluster --group-by pulumi-stack` therefore puts
every workload under `<none>` with this source, and JSON and NDJSON groups
have no `pulumi_urns`, even when kube-state-metrics exports the
`finfocus.dev/pulumi-urn` annotation through `--metric-annotations-allowlist`.
Group by Pulumi stack in run-rate mode (no `--from` or `--to`, and
`--usage-source kubernetes` when several sources are installed), where the
kubernetes source reads the annotation from the live pods.

## Install

`finfocus plugin install prometheus` waits on tag `prometheus-v0.1.0` and on
a registry entry added after that tag exists. Until then, install the binary
by copying it under `FINFOCUS_HOME`, the same way the kind test does.

## Logs

Stdout is the port handshake. Every log line goes to stderr.
