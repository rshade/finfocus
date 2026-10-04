# Feature Specification: Kubernetes Workload Projected Cost

**Feature Branch**: `621-k8s-workload-projected-cost`
**Created**: 2026-10-03
**Status**: Implemented
**Input**: GitHub issue #1525

## Context

`cost projected` on a Pulumi plan that declares Kubernetes workloads returns
`NO_COST_DATA` for every one of them. The kubernetes plugin is the only plugin
that claims the `kubernetes` provider, and its `Supports` declines all pricing.
The result looks the same as "finfocus does not know this type", although the
real situation is "nothing can price this yet".

Two constraints shape this feature:

- **Kubernetes knowledge lives only in the kubernetes plugin.** finfocus-spec
  and core gain nothing shaped for Kubernetes: no proto fields for declared
  workloads, no core parsing of replicas or requests, no core flags for node
  rates. A change to either would make them plugin-specific.
- **Plugins cannot call other plugins.** The kubernetes plugin cannot ask
  aws-public what a node group's instance costs, so it cannot derive a node
  rate from a node group in the same plan.

A plugin cannot read a pod spec today. Tags are `map<string,string>`, values
are capped at 256 characters in batch validation and the conformance suite,
the dotted-tag flattening of spec 619 stops at 6 path segments and 50 entries
(`spec.template.spec.containers.0.resources.requests.cpu` is 8 segments), and
the collapsed `spec` tag is Go map text, not a parseable document. The feature
therefore depends on a generic finfocus-spec channel for a resource's inputs.
finfocus-spec v0.7.3 provides it: `ResourceDescriptor.attributes`
(rshade/finfocus-spec#617).

## Clarifications

### Session 2026-10-03

- Q: How is a DaemonSet's pod count found? → A: From a plugin-config
  node-count hint; without it, `NO_COST_DATA` with a reason.
- Q: How are Jobs and CronJobs priced? → A: From a plugin-config
  hours-per-month hint; without it, `NO_COST_DATA` with a reason.

## User Scenarios & Testing

### User Story 1 - Price a declared Deployment (Priority: P1)

A platform engineer runs `finfocus cost projected` on a plan that adds a
`kubernetes:apps/v1:Deployment` with 3 replicas, each requesting 500m CPU and
1Gi memory. They have configured the kubernetes plugin with a per-vCPU-hour
and a per-GiB-hour rate. The Deployment shows a monthly estimate, and its
notes say the estimate came from configured rates.

**Why this priority**: This is the case the issue was filed for. Deployments
are the most common declared workload.

**Independent Test**: Run `cost projected` on a fixture plan with one
Deployment and the plugin rates set. Assert the monthly cost equals
`3 × (0.5 × cpuRate + 1 × memRate) × 730` and the notes name the method.

**Acceptance Scenarios**:

1. **Given** a Deployment with `replicas: 3` and one container requesting
   `cpu: 500m, memory: 1Gi`, and rates of 0.04 per vCPU-hour and 0.005 per
   GiB-hour, **When** `cost projected` runs, **Then** the monthly cost is
   `3 × (0.5 × 0.04 + 1 × 0.005) × 730 = 54.75` USD and the notes state the
   per-request method and that the rates came from plugin configuration.
2. **Given** a Deployment with no `replicas` field, **When** it is priced,
   **Then** one pod is assumed and the notes say so.
3. **Given** a pod template with two containers, **When** it is priced,
   **Then** their CPU and memory requests are summed per pod.
4. **Given** a StatefulSet with the same shape, **When** it is priced,
   **Then** it is priced the same way as the Deployment.

---

### User Story 2 - Be told why a workload is not priced (Priority: P1)

The same engineer has not configured a rate, or the workload declares no
requests. The workload shows `NO_COST_DATA` with a note that says what is
missing and how to supply it, never a `$0` presented as a price.

**Why this priority**: A confident `$0` is worse than no answer. The issue
makes this an acceptance criterion in its own right.

**Independent Test**: Run `cost projected` on the same fixture with no rate
configured and assert `NO_COST_DATA` with a note that names the missing
configuration.

**Acceptance Scenarios**:

1. **Given** no rate is configured, **When** a Deployment is priced, **Then**
   the result is `NO_COST_DATA` with a note naming the configuration that
   supplies a rate and suggesting `finfocus cost cluster` for a live cluster.
2. **Given** a container with neither requests nor limits, **When** its pod is
   priced, **Then** the result is `NO_COST_DATA` with a note that the workload
   declares no resource requests.
3. **Given** `replicas` or a request is a Pulumi unknown value (computed at
   deploy time), **When** it is priced, **Then** the result is `NO_COST_DATA`
   with a note that the value is not known until deployment.

---

### User Story 3 - Leave everything else unchanged (Priority: P1)

Plans mix Kubernetes workloads with cloud resources and with other
`kubernetes:*` types (`ConfigMap`, `Service`, `Namespace`). Only the five
workload kinds change behavior.

**Why this priority**: The plugin must not start answering for types it cannot
price, and cloud resource costs must not move.

**Independent Test**: Run `cost projected` on a plan with a Deployment, a
`ConfigMap`, and an `aws:ec2/instance:Instance`; assert the ConfigMap is still
declined and the instance's cost matches a run without the Deployment.

**Acceptance Scenarios**:

1. **Given** a `kubernetes:core/v1:ConfigMap`, **When** routing asks the
   kubernetes plugin `Supports`, **Then** it declines with today's reason.
2. **Given** a plan containing an EC2 instance, **When** Kubernetes workload
   support ships, **Then** the instance's cost and notes are unchanged.

---

### User Story 4 - Price DaemonSets, Jobs, and CronJobs honestly (Priority: P2)

DaemonSets, Jobs, and CronJobs have no simple replica count. Each is either
priced from an explicit hint or reported as `NO_COST_DATA` with a reason,
never a silently wrong number.

**Why this priority**: These kinds are less common in plan-time review, and
their pod count or run time cannot be known from the plan alone.

**Independent Test**: Price each kind with and without its hint (FR-010,
FR-011) and assert the cost or the note.

**Acceptance Scenarios**:

1. **Given** a DaemonSet and a configured node count of 4, **When** it is
   priced, **Then** `podCount` is 4 and the notes say the count came from
   plugin configuration.
2. **Given** a DaemonSet and no node-count hint, **When** it is priced,
   **Then** the result is `NO_COST_DATA` with a note naming the hint.
3. **Given** a CronJob with `parallelism: 2` and a configured 10 hours per
   month, **When** it is priced, **Then** the monthly cost is
   `2 × perPodHourly × 10` and the notes say the hours came from plugin
   configuration.
4. **Given** a Job and no hours-per-month hint, **When** it is priced,
   **Then** the result is `NO_COST_DATA` with a note naming the hint.

---

### User Story 5 - Plugins receive a resource's full inputs (Priority: P1)

Any plugin, not only the kubernetes plugin, can read a resource's nested
Pulumi inputs as a structured document instead of a capped, flattened tag map.

**Why this priority**: Stories 1–4 cannot work without it. It is generic: the
same channel serves deep Azure or AWS inputs that spec 619's depth cap cuts.

**Independent Test**: Build the request for a resource with a 10-level nested
input, a password field, and a secret value; assert the plugin receives the
full structure without the password, the secret, or `__` keys.

**Acceptance Scenarios**:

1. **Given** a CronJob whose requests sit 10 levels deep, **When** core builds
   its projected-cost request, **Then** the structured inputs contain the full
   path.
2. **Given** an input under a credential-like or `__`-prefixed key, **When**
   the structured inputs are built, **Then** it is omitted (FR-004).
3. **Given** an input wrapped as a Pulumi secret, **When** the structured
   inputs are built, **Then** it is omitted.
4. **Given** two resources that differ only below spec 619's depth cap,
   **When** both are priced, **Then** they do not share a projected cache
   entry.

### Edge Cases

- `replicas: 0`: a real, priced `$0` (the workload is scaled to zero), with a
  note that says so. This is not "no data".
- A container sets limits but no requests: Kubernetes defaults requests to the
  limits, so the limits are used and the notes say so.
- Init containers, sidecars (`restartPolicy: Always`), and pod-level
  resources: per-pod requests follow the Kubernetes scheduling rule, the same
  rule `cost cluster` uses for live pods.
- Quantity formats such as `500m`, `0.5`, `2`, `1Gi`, `1G`, `512Mi`, and `1e9`
  are accepted. A value that does not parse is `NO_COST_DATA` with a note
  naming the field.
- Only one rate configured: the other resource is unpriced, so the workload is
  `NO_COST_DATA` with a note naming the missing rate, not a partial cost.
- A rate that is negative, not a number, or infinite: the plugin rejects the
  configuration and the note says why.
- The structured inputs exceed the size cap (FR-017): core omits them and logs
  a warning; the plugin then reports `NO_COST_DATA` because it has no spec.
- An older plugin that does not read the new field: unaffected; it still gets
  tags exactly as today.
- An older core that does not send the new field: the kubernetes plugin
  reports `NO_COST_DATA` with a note that core did not send inputs.
- Delete operations price the old inputs; update and replace price both
  sides, as `GetProjectedCostDiff` does for every other resource.

## Requirements

### Functional Requirements

#### finfocus-spec prerequisites (generic, released in v0.7.3)

- **FR-001**: `ResourceDescriptor` has an optional structured inputs field,
  `attributes` (field 12, `google.protobuf.Struct`), usable by every plugin
  and carrying no Kubernetes-specific shape. Done in v0.7.3.
- **FR-002**: The SDK's maximum tag value length is 2048 bytes in batch
  validation and the conformance suite. Done in v0.7.3.

#### Core (generic)

- **FR-003**: Core MUST populate the structured inputs from the same ingested
  inputs it already flattens into tags, for preview and state ingestion and
  for both sides of a diff.
- **FR-004**: Core MUST omit from the structured inputs, at any depth, keys
  that start with `__` and credential-like keys (spec 619's segment list). It
  MUST also omit the top-level key `ref` and keys starting `ref.`, which hold
  core's reference tags, not declared properties. Unlike dotted tags, the
  `tags`, `tagsAll`, `labels`, and `annotations` containers are kept.
- **FR-005**: Core MUST also omit any value wrapped as a Pulumi secret. This
  rule is new: dotted tags do not check for secrets today.
- **FR-006**: Projected cache keys MUST change when the structured inputs
  change, so two resources that differ only in deep inputs never share an
  entry.
- **FR-007**: Core MUST keep sending today's tags unchanged; the structured
  field is additive.
- **FR-008**: Core MUST NOT contain logic that depends on Kubernetes resource
  types, replicas, requests, or node rates.

#### kubernetes plugin

- **FR-009**: The plugin MUST support projected cost for exactly
  `kubernetes:apps/v1:Deployment`, `kubernetes:apps/v1:StatefulSet`,
  `kubernetes:apps/v1:DaemonSet`, `kubernetes:batch/v1:Job`, and
  `kubernetes:batch/v1:CronJob`, and MUST decline every other type with
  today's reason. Its declared capabilities MUST add projected cost and no
  other pricing capability.
- **FR-010**: A DaemonSet MUST be priced only when the plugin's configuration
  sets a node-count hint: `podCount` is that hint. Without the hint it MUST be
  `NO_COST_DATA` with a note naming the hint. The notes of a priced DaemonSet
  MUST say the pod count came from the configured node count.
- **FR-011**: A Job or CronJob MUST be priced only when the plugin's
  configuration sets an hours-per-month hint: the monthly cost uses that hint
  in place of 730 hours, and `podCount` is the pod template's `parallelism`
  (default 1; for a CronJob, the job template's). Without the hint it MUST be
  `NO_COST_DATA` with a note naming the hint. The notes of a priced Job or
  CronJob MUST say the run time came from the configured hours.
- **FR-012**: For Deployment and StatefulSet the plugin MUST compute
  `monthly = podCount × (cpuCores × cpuRate + memGiB × memRate) × 730`, where
  `podCount` is `spec.replicas` (default 1) and the per-pod request follows
  the edge cases above.
- **FR-013**: Rates MUST come only from the plugin's own configuration: a
  per-vCPU-hour rate and a per-GiB-hour rate, in USD.
- **FR-014**: When a required rate is missing or invalid, the plugin MUST
  return `NO_COST_DATA` with a note naming the configuration that fixes it.
  It MUST NOT return a `$0` price for a missing rate.
- **FR-015**: Every priced result's notes MUST name the method (declared
  requests × configured rates), the pod count and its source, and that the
  rates are plugin configuration, not the price of a real node.
- **FR-016**: The plugin MUST read only the structured inputs; projected cost
  MUST NOT need a live cluster, a kubeconfig, or network access.
- **FR-017**: Core MUST omit the structured inputs of a resource whose
  encoded size exceeds `pluginsdk.MaxAttributesBytes` (65536 bytes), log a
  warning, and never fail the run.
- **FR-018**: Core MUST split a `BatchCost` request whose encoded size would
  exceed the transport limit, as v0.7.3 requires of hosts, so large
  attributes never fail a batch that would fit as several requests.

### Key Entities

- **Structured inputs**: a resource's declared properties as a nested document
  on the descriptor, filtered by FR-004 and FR-005. Generic to every plugin.
- **Workload shape**: what the plugin reads from the inputs: kind, pod count
  and its source, and the effective per-pod CPU and memory request.
- **Node rate**: the plugin-configured per-vCPU-hour and per-GiB-hour prices.
- **Pricing hints**: the plugin-configured node count (DaemonSet) and
  hours per month (Job, CronJob). Each applies to every workload of its kinds.

## Success Criteria

### Measurable Outcomes

- **SC-001**: With rates and both hints configured, `cost projected` on a
  plan containing a Deployment, StatefulSet, DaemonSet, Job, and CronJob
  prices each to the cent by FR-010 to FR-012.
- **SC-002**: With no rate configured, every one of those workloads reports
  `NO_COST_DATA` with an actionable note; none reports `$0`.
- **SC-003**: Every other `kubernetes:*` type and every non-Kubernetes
  resource in the fixture plan reports the same cost and notes as before.
- **SC-004**: No credential-like key, `__` key, or secret value from the
  fixture's inputs reaches a plugin, verified with the recorder plugin.
- **SC-005**: Core contains no reference to Kubernetes workload types,
  replicas, or requests outside test fixtures.

## Assumptions

- Rates are USD; currency configuration is out of scope.
- The hints are global to the plugin. Per-workload hints (for example an
  annotation) are out of scope.
- 730 hours per month, the engine's `hoursPerMonth`.
- Requests, not usage, are the basis, as in `cost cluster` run-rate mode.
- The plugin reads its configuration from environment variables like its
  other settings; exact names are decided in planning.
- Users who want an authoritative number for a running cluster use
  `finfocus cost cluster`.

## Dependencies

- finfocus-spec v0.7.3, which provides FR-001 and FR-002
  ([rshade/finfocus-spec#617](https://github.com/rshade/finfocus-spec/issues/617)).
  Core and the kubernetes plugin move to it, with the `plugin upgrade` hop
  and guide that `TestHopsReachCoreSpecVersion` and `TestHopsMatchGuides`
  require (`internal/pluginupgrade`).
- Spec 619 (dotted tag keys): the key rules FR-004 reuses.
- Spec 612 (kubernetes plugin): the plugin this feature extends.

## Out of Scope

- Deriving a node rate from a node group or launch template in the same plan,
  which needs a price from another plugin.
- Live-cluster pricing (`cost cluster`, specs 612 and 613).
- HPA, VPA, and other autoscaling; declared values only.
- Fargate-backed pods (spec 615).
- Any `kubernetes:*` type outside the five workload kinds.
- `overview` cluster expansion (a separate follow-up).
