# Quickstart: price Kubernetes workloads in a Pulumi plan

```bash
make install-kubernetes
export FINFOCUS_KUBERNETES_CPU_HOURLY_RATE=0.04
export FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE=0.005
export FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT=4      # optional
export FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH=10      # optional

pulumi preview --json > plan.json
finfocus cost projected --pulumi-json plan.json
```

Without the rates, each workload shows `NO_COST_DATA` with a note naming the
missing variable. For a running cluster, use `finfocus cost cluster`.

## Verify

```bash
go test ./internal/engine/... ./internal/proto/...
make test-kubernetes
go test -run 'TestKubernetes' ./test/integration/...
```
