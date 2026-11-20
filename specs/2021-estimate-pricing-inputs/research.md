---
title: "Estimate pricing inputs research"
description: "Decisions behind the What-If property policy"
layout: "docs"
---

## R1. Where the policy lives

**Decision**: `internal/viewmodel`, next to `BuildEstimatePropertyRows`.

**Rationale**: Both `internal/tui/estimate_model.go` and
`internal/webui/handlers_estimate.go` already call it, so one change reaches
both. `viewmodel` imports `engine`, so it can reuse `BuildAttributes` redaction
and `PricingMetricHint`.

**Alternatives**: A policy in `engine` mixes presentation with pricing. A policy
in the page breaks the "JavaScript only renders" rule and TUI parity.

## R2. What counts as a pricing input

**Decision**: A property is a pricing input only when a reviewed mapping names it
with provenance. Three provenance kinds:

1. `sku`: a key `resolveSKUAndRegion` reads for a SKU. The SDK order is
   `instanceType`, `instanceClass`, `type`, `volumeType`, then core's
   `dbInstanceClass`, `sku`, `tier`.
2. `region`: `region`, then `availabilityZone`. An ARN region and the provider
   region come from keys the editor does not show, so they are not inputs.
3. `plugin-input`: a name the aws-public plugin documents as accepted for that
   service (for example EBS `size`, DynamoDB `billingMode`).

**Rationale**: The issue says a field may be claimed only when the selected path
consumes it. The first two kinds are checkable against core code.
`plugin-input` rows come from the plugin README and are checked against the
installed plugin during the render check.

**Alternatives**: Learning inputs from deltas fails on free tiers and combined
deltas (out of scope). Reading plugin DryRun field mappings describes FOCUS
output, not inputs.

## R3. Launch type list

**Decision**: Reviewed mappings for `aws:ec2/instance:Instance`,
`aws:ebs/volume:Volume`, `aws:lambda/function:Function`,
`aws:dynamodb/table:Table`, `aws:lb/loadBalancer:LoadBalancer`,
`aws:elasticache/cluster:Cluster`, `aws:s3/bucket:Bucket`, and
`aws:ec2/natGateway:NatGateway`, all matched through
`resourcetype.NormalizeProvider` and the lowercased module segment (the same
form `skus.ResolveSKU` uses). `aws:alb/loadBalancer:LoadBalancer` maps like `lb`.

**Rationale**: These are the services the aws-public README lists as priced.
RDS is a stub there (returns $0), so RDS uses the fallback: its fields would
otherwise be claimed as pricing inputs that the path does not read.

**Alternatives**: Mapping every type with a SKU key overstates what the plugin
reads. Starting with EC2 alone leaves the issue's "storage or usage dimensions"
unmet.

## R4. Usage hints

**Decision**: A `PricingMetricHint.Metric` marks a property as `usage` only on an
exact key match with a descriptor property. `Server.serveEstimate` calls
`(*engine.Engine).DiscoverPricingSpec` before it builds `display` on both the
baseline and the recalculate path, and passes the hints to
`BuildEstimatePropertyPlan`. The terminal model already holds discovery
(`WithPricingDiscovery`) and feeds the same hints. The plan takes the hints as a
parameter, so classification is the same on every request for one resource.

**Rationale**: Metric names, SKU fields, and resource inputs are not
interchangeable (issue Technical Notes). Exact match is the only safe link.
Discovery is cached per resource type inside the engine, so the recalculate path
pays only the first call, bounded by the existing 5 second deadline. The "opening
the picker calls no plugin" rule (spec 630 SC-005) covers the resource list, not
the estimate request, which already calls the plugin.

The engine caches a discovery answer per resource type, an empty one included
(a cancelled context is the exception), so a type classifies the same way on
every web request in a session. A failed or empty discovery gives no hints: a
`usage` row is then `other`, and a test covers that case.

In the terminal, discovery arrives after the model is built
(`estimatePricingSpecMsg`). The model then re-plans, keeping the cursor row and
every typed or committed edit by key, and a row the user has edited never
disappears (an edited row is an override, so it stays `DefaultVisible`).

**Alternatives**: Fuzzy name matching misclassifies a tag. Hints only when
already known changes classification between requests. Deferring hints leaves the
issue's usage-input criterion to `plugin-input` rows alone.

**Decided**: 2026-11-19, by the issue owner during plan review.

## R5. Identity metadata

**Decision**: Two rules, both exact and case-sensitive, never a substring match:

1. Any key in the `pulumi:` or `terraform:` namespace is identity metadata. These
   are the bookkeeping keys core adds (`pulumi:urn`, `pulumi:cloudId`,
   `pulumi:arn`, `pulumi:created`, `pulumi:external`, `terraform:cloudId`,
   `terraform:arn`).
2. A short exact list of provider output names: `id`, `arn`, `urn`.

**Rationale**: `static/views/estimate.js` already drops every `terraform:` key in
the browser (`renderRows`). That is classification in JavaScript, so it moves into
this policy and the filter is deleted from the page. A user tag value or an input
such as `subnetId` stays editable. The implementation step checks the list
against a real descriptor and adds only keys that occur; `createdAt`,
`lastModified`, and `ownerId` were considered and are left out until a fixture
shows them.

**Alternatives**: Suffix matching (`*Id`, `*Arn`) catches user inputs such as
`subnetId`, which a developer may want to change.

## R6. Which rows the server sends

**Decision**: The server sends every safe row, ordered, each with `class`,
`editable`, and `defaultVisible`. The page shows rows where `defaultVisible` is
true, or all non-identity rows when Show all properties is on. Identity rows go
to a separate `details` list.

`defaultVisible` is true for a pricing input, for any row whose current value
differs from its original (an existing override), and for every row when the type
is not curated.

**Rationale**: The toggle becomes a client-side filter. No request, so no stale
late response and no lost edit (FR-007). Visibility is still decided in Go.

**Alternatives**: A `showAllProperties` request parameter refetches on every
toggle and races with a recalculation.

## R7. Identity override enforcement

**Decision**: `viewmodel.RejectIdentityOverrides(overrides)` takes only the
override map, because identity is decided by key (R5), not by resource type. It
returns an error naming the first key in sorted order. The web recalculate
handler calls it in `handleEstimateRecalculate`, beside the other request
validation and before the resource lookup, and answers 400 `identity_override`.
In the terminal, identity values sit in a read-only block that has no cursor, so
no edit can start; the TUI also calls the function before recalculating, as a
guard for the override map it builds.

**Rationale**: FR-006 asks for a server-side rule. A disabled input is not one.
`cost estimate --property` is unchanged: the CLI user typed the key
deliberately, and the issue scopes enforcement to "either UI".

**Alternatives**: Enforcing in `engine.EstimateCost` would change CLI and
`--modify` behavior, which the issue does not ask for.

## R8. Fallback note

**Decision**: When the type has no mapping, or a mapped type carries none of its
reviewed keys, the plan sets `Curated` false and `Note` to fixed text:
`Pricing inputs are not curated for this resource type; all safe properties are
shown.` The page and the TUI show `Note` as given.

**Rationale**: Incomplete metadata then behaves like an unknown type, as the
acceptance criteria require, and no field is lost.

## R9. Redaction runs inside the policy, on both transports

**Finding**: The web path redacts with `engine.BuildAttributes`, but
`tui.EstimateModel.initializeProperties` passes `resource.Properties` straight to
`BuildEstimatePropertyRows`, so the terminal editor today shows credential-like
keys and Pulumi secret values. `BuildAttributes` also returns nil, and so zero
rows, when the encoded attributes pass `pluginsdk.MaxAttributesBytes`; the web
editor would then say "No editable properties" while the descriptor has many.

**Decision**: `BuildEstimatePropertyPlan` takes the raw descriptor properties and
redacts them itself, so the terminal gets FR-009 for free and the two transports
cannot differ. It uses an exported form of `engine.redactedProperties`
(credential-like and `__` keys, Pulumi secrets, unrepresentable values, no size
cap), the rule `EstimateCost` already applies to what it sends. A test with a
credential-like key and a Pulumi secret runs through both transports.

**Rationale**: One rule, no second credential list (constitution and CLAUDE.md).
No size cap means a large descriptor still opens an editor. The editor then shows
what `EstimateCost` would send.

**Alternatives**: Redacting in each caller is what let the terminal drift.

## R10. Verified plugin inputs (aws-public v0.2.5)

**Method**: `finfocus plugin inspect aws-public <type>` cannot verify inputs.
`plugin_inspect.go` sends a `DryRunRequest` with no `Provider`, so the plugin
answers `InvalidArgument: provider and resource_type are required for DryRun`
for every type (a separate core defect, out of scope here), and DryRun reports
FOCUS output fields, not accepted inputs (R2). The names below come from the
plugin source at tag `v0.2.5`: `internal/plugin/attr_aliases.go`
(`serviceAliases`, `deriveSKU`), `projected.go` (`extractAWSSKU`,
`extractAWSRegion`, the `estimate*` readers), and `ec2_attrs.go`.

| Type | Reviewed keys (plugin-input unless noted) | Kind |
| --- | --- | --- |
| `aws:ec2/instance:Instance` | `instanceType` (sku), `region`, `availabilityZone` (region), `tenancy` | sku, region, plugin-input |
| `aws:ebs/volume:Volume` | `volumeType` (sku), `size` | sku, plugin-input |
| `aws:lambda/function:Function` | `memorySize`, `architectures`, `requestsPerMonth`, `avgDurationMs` | plugin-input |
| `aws:dynamodb/table:Table` | `billingMode`, `readCapacity`, `writeCapacity`, `storageGb`, `readRequestsPerMonth`, `writeRequestsPerMonth` | plugin-input |
| `aws:lb/loadBalancer:LoadBalancer`, `aws:alb/loadBalancer:LoadBalancer` | `loadBalancerType`, `lcuPerHour`, `nlcuPerHour`, `capacityUnits` | plugin-input |
| `aws:elasticache/cluster:Cluster` | `nodeType`, `numCacheNodes`, `engine` | plugin-input |
| `aws:s3/bucket:Bucket` | `storageClass` | plugin-input |
| `aws:ec2/natGateway:NatGateway` | `dataProcessedGb` | plugin-input |

Findings that change the mapping list T007 implements:

- **EC2 `platform` is dropped.** The plugin reads a `platform` tag, but
  `aws:ec2/instance:Instance` has no `platform` input (it is an output), so the
  editor would offer a field a Pulumi program cannot set. Keep `tenancy` only.
- **EC2 `region`.** The plugin reads `region`, then `availabilityZone`, through
  `mapping.ExtractAWSRegion`; core's `resolveSKUAndRegion` does the same. A
  descriptor with neither key gets no region row (the policy emits no row for
  an absent reviewed key). The plugin also rejects a region other than its own
  binary's, so a region edit prices only through the router's per-region child.
- **Derived SKUs.** `memorySize`, `billingMode`, `loadBalancerType`, and
  `nodeType` are not core SKU keys. The plugin derives a SKU from them
  (`deriveSKU`), so they are `plugin-input`, not `sku`. `elbSKU` accepts
  `application`, `network`, and `gateway`; `gateway` is unpriced ($0).
- **Aliases are the contract.** Each camelCase key above is an alias of a
  canonical snake_case key (`memorySize` to `memory_mb`, `numCacheNodes` to
  `num_nodes`). The mapping uses the Pulumi-style camelCase names core
  forwards; `volumeSize` and `volume_size` are also accepted for EBS but are
  not Pulumi inputs, so they are left out.
- **R3's RDS premise is stale.** R3 says RDS is a stub that returns $0, but
  v0.2.5 has `estimateRDS` reading `engine`, `storageType`, `allocatedStorage`,
  and `multiAz`, with a SKU from `instanceClass`. RDS stays on the fallback for
  this change (the launch list in R3 and the tasks does not include it), and
  adding it is a follow-up decision for the issue owner, not made here.

## R11. Descriptor keys the editor receives (identity rule check)

**Method**: `../finfocus-demo` is not present on this machine, so no live
`GET /api/estimate/baseline` was captured. The keys come from the code that
builds descriptors (`ingest.MapStateResource`, `terraform_map.go`) and from the
EC2 state fixtures in `test/fixtures/state/imported-resources.json` and
`test/e2e/fixtures/web/estimate-picker-state.json`. A live capture should
replace this when the demo stack is available (T028 renders against it).

| Namespace or key | Occurs | Source |
| --- | --- | --- |
| `pulumi:cloudId`, `pulumi:urn` | always (cloud id when the state id is set) | `MapStateResource` |
| `pulumi:arn` | when the merged properties hold a string `arn` | `MapStateResource` |
| `pulumi:created`, `pulumi:modified`, `pulumi:external` | when the state carries the timestamp or flag | `MapStateResource` |
| `terraform:id`, `terraform:arn`, `terraform:cloudId`, `terraform:module`, `terraform:provider`, `terraform:unknownProperties` | Terraform input only | `terraform_map.go`, `terraform_plan.go` |
| `arn` | EC2 state outputs (fixture) | provider output |
| `id`, `urn` | not in `Properties`; they are `ID` and the URN on the descriptor | `StackExportResource` top level |
| `__providerRegion` | AWS resources with a provider region | `stackload/provider_region.go` |

Consequences for R5:

- The namespace rule (`pulumi:` and `terraform:` prefix) covers every
  bookkeeping key above, including `pulumi:modified` and
  `terraform:module`, `terraform:provider`, `terraform:unknownProperties`,
  which R5's example list omitted. No key needs its own entry.
- Bare `arn` occurs in real state. Bare `id` and `urn` do not appear in
  `Properties` for state input, but a Terraform plan or a plan-only resource
  can carry an `id` output, so the exact list `id`, `arn`, `urn` stays.
- `region` is an EC2 *input* only when the program or the fixture sets it
  (the e2e picker fixture does; a deployed stack usually relies on the
  provider region). A descriptor without it gets no region row, which is what
  the policy already does for any absent reviewed key.
- `createdAt`, `lastModified`, and `ownerId` stay out until a live capture
  shows them, as R5 says.
