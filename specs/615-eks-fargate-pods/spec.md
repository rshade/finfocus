# Feature Specification: Price EKS Fargate pods

**Feature Branch**: `615-eks-fargate-pods`
**Created**: 2026-10-02
**Status**: Implemented (issue #1532)
**Input**: GitHub issue #1532. Supersedes the "Fargate pricing (reported as
`NO_COST_DATA`)" non-goal in `specs/612-k8s-cost-allocation/` and the Fargate
clause of `specs/613-cost-cluster/` for EKS pods only.

## Overview

A pod on an EKS Fargate virtual node
(`eks.amazonaws.com/compute-type=fargate`) is its own priceable resource.
The virtual node has no instance type and no spare capacity, so it still
contributes no capacity rows, no node descriptor, and no `__idle__` row.

The kubernetes plugin does not embed AWS prices. It asks aws-public, through
the normal projected-cost path, to price resource type
`aws:eks/fargate:Pod`. That work is
[finfocus-plugin-aws-public#409](https://github.com/rshade/finfocus-plugin-aws-public/issues/409).
Until that issue returns a positive monthly cost, the host treats
`Monthly <= 0` as unpriced and the pod row is `$0`.

### Non-goals

- ECS Fargate and Fargate Spot.
- Per-vCPU-hour or per-GiB-hour tables inside this repository.
- Folding a Fargate pod into another node's idle or into
  `system_workloads: share`. The pod's price is already its own cost.
- kind end-to-end coverage. kind cannot create Fargate nodes, so
  `make test-e2e-kind` does not exercise this path. Unit tests supply the
  fake priced response.

## Descriptor

`usage.FargatePodDescriptor` emits one `ResourceDescriptor` per running pod
whose node is in the Fargate region map:

| Field | Value |
| --- | --- |
| Provider | `aws` |
| ResourceType | `aws:eks/fargate:Pod` |
| Sku | `fargate` |
| Region | `topology.kubernetes.io/region`, else `failure-domain.beta.kubernetes.io/region` |
| Id | `<cluster>/<namespace>/<pod>` |
| Tags | `kind=fargate`, `cluster`, `namespace`, `pod`, `node`, `cpu` (cores), `memory_gib` |

`ResourceDescriptor` has no properties field. `cpu` and `memory_gib` are
tags. Core's `PriceableToResource` copies those tags onto the priced
resource's properties for aws-public.

An empty region, namespace, or pod name omits the descriptor. Collection
then records `fargate pod <namespace>/<pod> on <node>: region unknown; not priced`.

## Allocation

`groupPricedResources` keeps `kind=fargate` in its own bucket. It is not a
node, so `ValidateAllocateResponse` does not require an idle row, and it is
not a cluster row, so it does not emit `unallocated priced resource`.

For each priced entry:

1. Match an orphan workload on `namespace` + `pod` + `node`.
2. Assign `priced=true` cost to that workload. Split it into `cpu_cost` and
   `mem_cost` with the policy `node_split` unit prices times the pod's CPU
   and memory requests. Memory takes `cost - cpu_cost` so the parts sum to
   the cost. A non-positive cost, or both weights zero, stays entirely on
   `cpu_cost`.
3. Drop that workload from the orphan list so it is not also emitted at `$0`.
4. A second priced entry that matches an already-used workload still emits
   its own row from the tags, so the extra cost is not dropped.
5. `priced=false` emits `$0`. An empty note becomes `Fargate pod has no
   price`; a note from the pricer is kept (the host's
   `priced at $0; treated as unpriced` is the live note until #409 lands).

A pod whose node name starts with `fargate-` and that has no fargate priced
entry at all stays on the old orphan path: `$0` and
`Fargate pricing not supported yet`. Non-Fargate nodes allocate exactly as
before.
