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

## Node labels

kube-state-metrics must publish node labels. The source reads
`kube_node_labels` and `kube_node_info` for a node's provider, instance type,
and region. A node that those series and the live API both cannot identify is
omitted.

## Install

`finfocus plugin install prometheus` waits on tag `prometheus-v0.1.0` and on
a registry entry added after that tag exists. Until then, install the binary
by copying it under `FINFOCUS_HOME`, the same way the kind test does.

## Logs

Stdout is the port handshake. Every log line goes to stderr.
