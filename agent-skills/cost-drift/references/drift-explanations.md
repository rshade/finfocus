<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Drift Explanations by Resource Type

Per-resource-type likely causes and investigation prompts for cost
drift. Used by cost-drift to generate human-readable explanations
for flagged resources.

Match the `type` of each flagged row against the headings below, using the
Pulumi type token exactly as the CLI prints it. An older alias such as
`aws:elasticloadbalancingv2/loadBalancer:LoadBalancer` maps to the same entry
as `aws:lb/loadBalancer:LoadBalancer`.

Each entry has two parts:

- **Likely cause** — a short hint based on resource type and drift
  direction (overspend or underspend)
- **Investigate** — an actionable next step to confirm the cause

## AWS

### aws:ec2/instance:Instance (EC2)

**Overspend:**

- Likely cause: On-demand hours exceeding reservation coverage, or
  instance type larger than projected
- Investigate: Check EC2 reservation utilization and instance
  right-sizing in AWS Cost Explorer

**Underspend:**

- Likely cause: Instance stopped or terminated, or spot pricing
  lower than on-demand projection
- Investigate: Check instance uptime in CloudWatch and verify spot
  vs on-demand usage

### aws:rds/instance:Instance (RDS)

**Overspend:**

- Likely cause: Storage auto-scaling, increased IOPS, or Multi-AZ
  failover events
- Investigate: Review RDS storage metrics and allocated vs consumed
  IOPS in CloudWatch

**Underspend:**

- Likely cause: Reserved instance coverage applied, or lower
  utilization than projected
- Investigate: Check RDS reserved instance utilization in AWS Cost
  Explorer

### aws:s3/bucket:Bucket (S3)

**Overspend:**

- Likely cause: Higher request volume or data transfer than
  projected, or storage class transitions not reducing costs as
  expected
- Investigate: Check S3 request metrics and storage class
  distribution in S3 analytics

**Underspend:**

- Likely cause: Lifecycle policies deleting data faster than
  projected, or Intelligent-Tiering reducing costs
- Investigate: Check S3 lifecycle rules and storage class
  transitions

### aws:lambda/function:Function (Lambda)

**Overspend:**

- Likely cause: Higher invocation count or longer execution duration
  than projected
- Investigate: Check Lambda invocation count and duration metrics in
  CloudWatch

**Underspend:**

- Likely cause: Lower traffic than projected, or provisioned
  concurrency not fully utilized
- Investigate: Check Lambda invocation metrics and provisioned
  concurrency utilization

### aws:lb/loadBalancer:LoadBalancer (ALB/NLB)

**Overspend:**

- Likely cause: Higher LCU (Load Balancer Capacity Units) usage
  from increased traffic or connections
- Investigate: Check ALB/NLB consumed LCU metrics in CloudWatch

**Underspend:**

- Likely cause: Lower traffic than projected
- Investigate: Check load balancer request count and active
  connection metrics

## GCP

### gcp:compute/instance:Instance (Compute Engine)

**Overspend:**

- Likely cause: Committed use discount not applied, or sustained
  use discount lower than expected
- Investigate: Check committed use discount coverage in GCP Billing

**Underspend:**

- Likely cause: Instance stopped or preempted, or sustained use
  discount higher than projected
- Investigate: Check instance uptime and preemption events in
  Cloud Monitoring

### gcp:storage/bucket:Bucket (Cloud Storage)

**Overspend:**

- Likely cause: Higher egress or operations than projected
- Investigate: Check Cloud Storage request metrics and egress in
  Cloud Monitoring

**Underspend:**

- Likely cause: Lifecycle rules deleting objects or transitioning
  to cheaper storage classes
- Investigate: Check bucket lifecycle rules and storage class
  distribution

## Azure

### azure:compute/virtualMachine:VirtualMachine (also azure-native:compute:VirtualMachine)

**Overspend:**

- Likely cause: On-demand pricing without reservation, or disk
  costs exceeding projection
- Investigate: Check Azure Advisor for reservation recommendations
  and disk utilization

**Underspend:**

- Likely cause: VM deallocated or reservation discount applied
- Investigate: Check VM uptime and reservation utilization in
  Azure Cost Management

## Fallback

For resource types not listed above, use these generic explanations:

**Overspend:**

- Likely cause: Higher utilization or traffic than projected
- Investigate: Check the resource's usage metrics in the cloud
  provider's monitoring console

**Underspend:**

- Likely cause: Lower utilization than projected, or discount
  applied
- Investigate: Check the resource's usage metrics and verify
  discount/reservation coverage
