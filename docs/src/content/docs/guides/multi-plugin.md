---
title: Running aws-public, Kubernetes, and Jev Together
description: End-to-end walkthrough of the three FinFocus plugins working together — aws-public pricing AWS resources, kubernetes splitting cluster costs, and jev scoring the recommendations.
---

## What Each Plugin Does

FinFocus plugins each do one job, and Core routes work between them. This
walkthrough runs a realistic day with all three plugins installed:

- **aws-public** prices AWS resources from public list pricing — both in a
  Pulumi plan and the live nodes of an EKS cluster. No AWS credentials needed.
- **kubernetes** reports what's running in the cluster and splits each priced
  node's cost across namespaces and workloads. It also prices Kubernetes
  workloads declared in a Pulumi plan.
- **jev** scores the cost recommendations the other plugins produce, so the
  list can be ordered by risk and priority instead of raw savings.

None of these plugins knows about the others. Core routes AWS resources to
aws-public, cluster state and `kubernetes:*` plan resources to the kubernetes
plugin, and the finished recommendations to jev.

## Setup

Install all three plugins and set the environment they need:

```bash
finfocus plugin install aws-public --metadata region=us-east-1
finfocus plugin install kubernetes
finfocus plugin install jev

# kubernetes: workload pricing rates for plan-time estimates
export FINFOCUS_KUBERNETES_CPU_HOURLY_RATE=0.04
export FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE=0.005

# jev: opt in to scoring
export TYPESAFE_API_KEY=...
finfocus config set scoring.plugin jev
finfocus config set scoring.enabled true
```

The kubernetes plugin also needs a reachable cluster via kubeconfig
(`KUBECONFIG` or `~/.kube/config`) for Step 2. Set `scoring.plugin` before
`scoring.enabled` — every config change is validated, and scoring cannot be
enabled without a plugin.

## Step 1: Price the Plan Before Deploying

The stack already runs an `m4.large` API server and a 200 GB gp2 data volume.
A pull request adds an RDS instance, an S3 bucket, and a `search-indexer`
Deployment (3 replicas, each requesting 0.5 vCPU and 1 GiB). One command
prices both providers — Core routes each resource to the plugin that claims
it:

```bash
pulumi preview --json > plan.json
finfocus cost projected --pulumi-json plan.json
```

```text
COST DIFF
Before     93.00 USD
After      202.61 USD
Change     +109.61 USD
Resources  5 (3 create, 0 update, 0 delete, 2 unchanged)

OP  RESOURCE                                  BEFORE  AFTER  CHANGE  CURRENCY
=   aws:ec2/instance:Instance/urn:pulumi:...  73.00   73.00  +0.00   USD
    On-demand Linux, Shared tenancy, 730 hrs/month [carbon_footprint: 2757.10 gCO2e]
=   aws:ebs/volume:Volume/urn:pulumi:prod...  20.00  20.00  +0.00  USD
    gp2 volume, 200 GB, $0.1000/GB-month [carbon_footprint: 147.20 gCO2e]
+   aws:rds/instance:Instance/urn:pulumi:...  0.00  54.86  +54.86  USD
    RDS db.t3.medium PostgreSQL, 730 hrs/month + 20GB gp2 storage (storage type defaulted, size defaulted to 20GB) [carbon_footprint: 3522.33 gCO2e]
+   kubernetes:apps/v1:Deployment/urn:pul...  0.00  54.75  +54.75  USD
    Estimated from declared requests × configured rates: 3 pods (spec.replicas) × $0.025/pod-hour (0.5 vCPU × 0.04 + 1 GiB × 0.005) × 730 h. Rates from FINFOCUS_KUBERNETES_CPU_HOURLY_RATE and FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE are plugin configuration, not a real node price.
+   aws:s3/bucket:Bucket/urn:pulumi:prod:...  0.00  0.00  +0.00  USD
    No pricing information available (declined by jev: jev plugin scores recommendations only; kubernetes: kubernetes plugin prices only kubernetes:* resources)

ERRORS
======
aws:s3/bucket:Bucket urn:pulumi:prod::shop::aws:s3/bucket:Bucket::assets finfocus-plugin-aws-public: plugin call failed: no cost data available
```

`=` rows are unchanged resources and `+` rows are creates, so the change is
the cost of this pull request: $109.61 a month. aws-public priced the EC2
instance, EBS volume, and RDS instance from its embedded pricing data. The
kubernetes plugin priced the Deployment from its declared requests, and its
note shows the arithmetic. The S3 bucket has no price from any installed
plugin; it is listed under `ERRORS` and counted as $0, which means "unknown",
not "free".

Neither plugin needed AWS credentials or a live cluster for this step, so it
runs safely in CI.

## Step 2: Allocate the Running Cluster's Cost

After the change ships, see what the cluster costs and who is spending it.
The kubernetes plugin lists nodes and workloads, aws-public prices the nodes,
and the kubernetes plugin's allocator splits each node's cost by workload
resource requests:

```bash
finfocus cost cluster
```

```text
GROUP               CPU    MEMORY  TOTAL  NOTES
payments            57.17  11.49   68.67
team-ci             21.44  5.75    27.19
__idle__            17.51  5.38    22.89
search              10.72  2.87    13.59
kube-system         7.50   0.32    7.82
local-path-storage  0.00   0.00    0.00

Mode:    run-rate (monthly, 730 h)
Total:   $140.16 USD
Idle:    $22.89 (16.3%)
Policy:  built-in defaults · 9ad579498af7
```

Two plugins contributed to this table. aws-public priced the two `m5.large`
nodes at $70.08 a month each, and the kubernetes plugin supplied the usage
data and did the allocation. The conservation invariant guarantees the rows
sum to the priced total of $140.16. The idle row is node capacity that no
workload requested, not a pricing gap. A namespace whose pods request nothing,
such as `local-path-storage` here, gets $0.

The `search` row is the same `search-indexer` Deployment from Step 1. In Step
1 it cost $54.75 at the configured plan-time rates; here it costs $13.59 as
its share of real node prices. Plan-time rates are configuration, so set them
close to your nodes' per-vCPU and per-GiB cost if you want the two to agree.

On EKS, the plugin also reports the control plane as a priceable resource
when the configured API server host matches the EKS endpoint pattern
(`<id>.<region>.eks.amazonaws.com`). A `__cluster__` row is added only when FinFocus
returns a price for it; if no installed plugin prices the control plane, no
row appears.

## Step 3: Score the Recommendations

aws-public also produces recommendations — here, moving the gp2 volume to gp3
and the `m4.large` instance to `m5.large`. With scoring enabled, Core sends
them to jev, which returns risk, false-positive, worth-acting, and priority
scores for each:

```bash
finfocus cost recommendations --pulumi-json plan.json --sort priority
```

```text
RECOMMENDATIONS SUMMARY
=======================
Total Recommendations: 2
Total Potential Savings: 6.92 USD

By Action Type:
  RECOMMENDATION_ACTION_TYPE_MODIFY: 2 (6.92 USD)

Scoring: jev/jev-1.13.0 (ranking_only): scored 2 of 2, 1 need review. Scores rank work for review; they never dismiss or apply a recommendation.

TOP 2 RECOMMENDATIONS BY PRIORITY
----------------------------------------
RESOURCE                                                      ACTION TYPE                        DESCRIPTION                                         SAVINGS   RISK  FALSE POS  WORTH  PRIORITY  REVIEW  GROUP
--------                                                      -----------                        -----------                                         -------   ----  ---------  -----  --------  ------  -----
urn:pulumi:prod::shop::aws:ebs/volume:Volume::data-volume     RECOMMENDATION_ACTION_TYPE_MODIFY  Upgrade 200GB gp2 volume to gp3 for ~20% cost s...  4.00 USD  0.21  0.12       0.76   2.35
urn:pulumi:prod::shop::aws:ec2/instance:Instance::api-server  RECOMMENDATION_ACTION_TYPE_MODIFY  Upgrade from m4.large to m5.large for better pe...  2.92 USD  0.26  0.12       0.46   2.12      review
```

The score columns sit at the right of the table. For this run they were:

| Recommendation      | Savings  | Risk | False pos | Worth | Priority | Review |
| ------------------- | -------- | ---- | --------- | ----- | -------- | ------ |
| gp2 → gp3 volume    | 4.00 USD | 0.21 | 0.12      | 0.76  | 2.35     |        |
| m4.large → m5.large | 2.92 USD | 0.26 | 0.12      | 0.46  | 2.12     | review |

jev's scores are `ranking_only`: they order the list, and they are not
probabilities. Priority is a relative score, so read it only against the
other rows in the same run. jev rated the gp2→gp3 change as both lower risk
and more worth doing, so it sorts first. It flagged the instance upgrade for
`review`, which the summary line counts as "1 need review". `GROUP` is empty
because jev found no duplicates. Scores can shift slightly between runs.

A score never approves, dismisses, or applies anything; that decision stays
with the reviewer.

## How They Fit Together

| Plugin       | Role                          | Invoked by                                             |
| ------------ | ----------------------------- | ------------------------------------------------------ |
| `aws-public` | AWS pricing, recommendations  | `cost projected`, `cost actual` (runtime estimate, not billing data), `cost cluster` (node pricing), `cost recommendations` |
| `kubernetes` | Cluster usage and allocation, Kubernetes workload pricing | `cost cluster`, `cost projected` (for `kubernetes:*` resources) |
| `jev`        | Recommendation scoring        | `cost recommendations` (when `scoring.enabled` is true) |

Because routing is per-resource and per-capability, the same pattern extends
to other plugins: install another plugin, such as Kubecost, and Core routes
to it without changing how these three behave.

## See Also

- [AWS Public Plugin](../plugins/aws-public.md)
- [Jev Scorer Plugin](../plugins/jev.md)
- [Kubernetes Cluster Cost Allocation](./cluster-costs.md)
- [Recommendation Scoring Guide](./recommendation-scoring.md)
- [Multi-Plugin Routing](./routing.md)
