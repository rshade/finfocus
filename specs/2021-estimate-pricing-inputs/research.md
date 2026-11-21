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

> Superseded for RDS by R17: aws-public v0.2.5 prices RDS, and RDS now has a
> reviewed mapping.

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
| `aws:ebs/volume:Volume` | `type` (sku), `size` | sku, plugin-input |
| `aws:lambda/function:Function` | `memorySize`, `architectures`, `requestsPerMonth`, `avgDurationMs` | plugin-input |
| `aws:dynamodb/table:Table` | `billingMode`, `readCapacity`, `writeCapacity`, `storageGb`, `readRequestsPerMonth`, `writeRequestsPerMonth` | plugin-input |
| `aws:lb/loadBalancer:LoadBalancer`, `aws:alb/loadBalancer:LoadBalancer` | `loadBalancerType`, `lcuPerHour`, `nlcuPerHour`, `capacityUnits` | plugin-input |
| `aws:elasticache/cluster:Cluster` | `nodeType`, `numCacheNodes`, `engine` | plugin-input |
| `aws:s3/bucket:Bucket` | `storageClass` | plugin-input |
| `aws:ec2/natGateway:NatGateway` | `dataProcessedGb` | plugin-input |

Findings that change the mapping list T007 implements:

- **EBS SKU is `type`, not `volumeType`.** `volumeType` is the plugin's
  internal attribute name; a Pulumi `aws:ebs/volume:Volume` input is `type`,
  which `mapping.ExtractAWSSKU` reads (`instanceType`, `instanceClass`, `type`,
  then `volumeType`). The T028 render against `../finfocus-demo` showed the
  volume type hidden behind Show all properties. A live recalculation on that
  volume moved the monthly cost for `type: io2` and left it unchanged for
  `volumeType: io2`. Same rule as dropping EC2 `platform`: a reviewed key must
  be a property the Pulumi program sets.

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

## R12. Reviewed keys are Pulumi inputs; plugin-only names become usage rows (ISS-001)

- **Decision**: A `sku` or `region` key MUST be a property the Pulumi resource
  type carries (EBS `type`, not the plugin's `volumeType`). A `plugin-input` key
  is a name the mapping's plugin reads; when the descriptor lacks it, the
  policy emits a usage row (spec FR-012). The corrected table:

  | Type | Present-key inputs | Usage rows when absent |
  | --- | --- | --- |
  | `aws:ec2/instance:Instance` | `instanceType` (sku), `region`, `availabilityZone` (region), `tenancy` | `tenancy` |
  | `aws:ebs/volume:Volume` | `type` (sku), `size` | `size` |
  | `aws:rds/instance:Instance` | `instanceClass` (sku), `region`, `availabilityZone` (region), `engine`, `storageType`, `allocatedStorage`, `multiAz` | the four plugin-input keys |
  | `aws:lambda/function:Function` | `memorySize`, `architectures` | those two, `requestsPerMonth`, `avgDurationMs` |
  | `aws:dynamodb/table:Table` | `billingMode`, `readCapacity`, `writeCapacity` | those three, `storageGb`, `readRequestsPerMonth`, `writeRequestsPerMonth` |
  | `aws:lb/loadBalancer:LoadBalancer`, `aws:alb/…` | `loadBalancerType` | `loadBalancerType`, `lcuPerHour`, `nlcuPerHour`, `capacityUnits` |
  | `aws:elasticache/cluster:Cluster` | `nodeType`, `numCacheNodes`, `engine` | the same three |
  | `aws:s3/bucket:Bucket` | none | `storageClass` |
  | `aws:ec2/natGateway:NatGateway` | none | `dataProcessedGb` |

  Every usage key is a `serviceAliases` entry in aws-public v0.2.5
  (`internal/plugin/attr_aliases.go`), checked one by one, so each row lands
  on a key the plugin reads.
- **Rationale**: The clarification chose usage rows (spec Clarifications,
  Q1). A key no program can set is reachable only as a row the user fills in.
- **Alternatives**: Drop plugin-only keys (S3 and NAT lose curation); keep
  them inert (docs overstate coverage). Both rejected in clarification.

## R13. A usage value reaches the plugin unchanged

- **Decision**: No engine change. `mergePropertiesWithOverrides`
  (`internal/engine/estimate.go`) already adds an override for an absent key,
  as a string because `coerceOverrideValue` has no original to copy a type
  from. Both estimate paths send the merged map: `EstimateCost` and the
  projected fallback (`estimateSelectedFallback`).
- **Rationale**: aws-public flattens every attribute to a string
  (`pricingTags` → `scalarString`) and parses numbers itself
  (`validateNonNegativeInt64` and friends), and its `EstimateCost` reads
  attributes "through the same lookup the projected-cost path uses". A string
  `"1000000"` for `requestsPerMonth` therefore prices.
- **Check**: an implementation task recalculates a real Lambda or S3
  descriptor with a usage value and asserts the modified cost moves, the same
  live proof R10's EBS finding used.

## R14. Which plugin serves the estimate

- **Decision**: An `InputMapping` names its plugin (`aws-public`). The policy
  receives the plugins the estimate's pricing-spec discovery reports
  (`engine.PricingDiscovery.Modes[].Plugin`) and the selected mode's plugin
  (the mode whose `ID()` the request carries; empty for Automatic). The
  mapping's `plugin-input` and usage rows apply when its plugin is selected,
  or, under Automatic, when it is among the discovered plugins. `sku` and
  `region` rows apply regardless.
- **Plugin names**: `Mode.Plugin` is `pluginhost.Client.Name`, the name the
  plugin reports (`finfocus-plugin-aws-public`), while the install directory
  and `plugin list` use `aws-public`. The policy compares after trimming the
  `finfocus-plugin-` prefix, the same rule `registry/entry.go` uses to derive
  a plugin name from a repository name.
- **Determinism (FR-010)**: discovery is the call the estimate already makes,
  cached per resource type, so the rows are the same on every request for one
  resource and selection. No new plugin call, no protocol field.
- **Discovery empty** (no plugin implements `GetPricingSpec`, or it timed
  out): no plugin serves, so only `sku`/`region` rows apply. EC2, EBS, and RDS
  stay curated through their SKU key; S3 and NAT take the fallback. Recorded
  as accepted: the fallback still shows every safe property.
- **Recommendations** route by capability to every plugin that advertises
  them and never read this policy (spec Assumptions).

## R15. Rows for overrides the descriptor lacks

- **Decision**: `BuildEstimatePropertyRows` drops an override whose key the
  descriptor lacks. The policy adds two kinds of such rows:
  1. A usage row (R12) when its mapping applies, with the override, if any,
     as its current value.
  2. An `other`, overridden row for any remaining override key, so an entry
     made in a usage row survives a mode switch that removes the row (spec
     edge case, FR-007).
  Identity keys are rejected before this (FR-006). An override key that
  `engine.IsHiddenPropertyKey` hides gets no row (FR-009), the same name rule
  redaction uses.
- **Invariant change**: rows and details now cover the redacted input keys
  plus usage keys and non-hidden override keys.

## R16. Terminal mode switch replans

- **Decision**: The terminal always selects a concrete mode once discovery
  arrives (`pricingModes[pricingMode]`). Selecting another mode now replans
  with that mode's plugin before recalculating, as discovery arrival already
  does (`replan`). The web page already re-requests on a mode change and
  sends its overrides, so the server builds the new rows.

## R17. RDS (corrects R3)

- **Decision**: Add the reviewed RDS mapping in R12's table. R3's premise
  (RDS is a $0 stub) is superseded by R10: aws-public v0.2.5 `estimateRDS`
  reads `engine`, the SKU from `instanceClass`, and the aliases `storageType`,
  `allocatedStorage`, and `multiAz`, all `aws:rds/instance:Instance` inputs.
