---
title: "Carbon Plugin Contract"
description: "Current SDK metadata, RPC, and enrichment rules."
layout: "docs"
---

## Metadata and Packaging

Module: `github.com/rshade/finfocus/plugins/carbon`.
Binary: `finfocus-plugin-carbon`. Registry name: `carbon`.
Use the root Go and finfocus-spec versions; do not import core packages.

`Info(version string) *pluginsdk.PluginInfo` declares provider `aws` and the
`PLUGIN_CAPABILITY_CARBON` capability only. The carbon-only source serves the
existing `GetProjectedCost` RPC but does not advertise monetary projected or
actual costs, batch, energy, recommendations, usage, or allocation capabilities.
This is intentional: capability describes the result domain, not just the RPC.

Advertise one optional string setting, `utilization`, with default `0.5`, env
`FINFOCUS_CARBON_UTILIZATION`, and a description specifying a [0, 1] fraction.

## RPCs

`Supports` accepts supported AWS EC2 descriptors and rejects everything else
with an actionable reason. `GetProjectedCost` uses the same model validation;
unsupported descriptors return `FailedPrecondition`, malformed requests return
`InvalidArgument`, and canceled contexts return the appropriate cancellation
status. Nil requests must not panic.

The response has one `ImpactMetric` of kind
`METRIC_KIND_CARBON_FOOTPRINT`, monthly total value in `gCO2e`, and a
`billing_detail` identifying the operational-only model, utilization, monthly
hours, PUE, and pinned data revision. The protocol's monetary fields are zero;
use `USD` only as a transport-valid currency where SDK validation requires it.
Core must never interpret these fields as free infrastructure. Do not place a
`/month` suffix into the mass unit; the response represents a monthly total.

`BasePlugin` supplies unsupported service methods. Startup follows the existing
`pluginsdk.Serve` pattern with stderr logging and signal cancellation.

## Routing

Carbon discovery requires explicit carbon capability, including no-router and
router-fallback paths. Carbon-only clients are excluded from monetary projected,
actual, pricing-spec, and batch selection, even when an explicit route names
one. Unknown-capability legacy clients retain existing monetary behavior.
Combined monetary/carbon clients remain eligible for their advertised domains.

For enrichment, use existing priority and fallback routing. Stop at the first
valid carbon result. Honor fallback-disabled failures and decline reasons.
Reuse the prepared resource descriptor and existing credential/secret filtering.
Do not transmit arbitrary extra properties or new credential-bearing attributes.

## Partial Results

A carbon failure never erases a successful monetary result. Both projected-cost
entry points retain completed rows if cancellation occurs while enriching;
return cancellation alongside partial data. Preserve deterministic input order.
The error-aware entry point includes carbon RPC failures in `Errors`; the regular
entry point records them through notes and structured warnings.
