---
title: "Estimate pricing inputs data model"
description: "Types for the shared property presentation"
layout: "docs"
---

All types live in `internal/viewmodel`.

## PropertyClass

`string` with three values: `pricing`, `identity`, `other`.

## InputProvenance

`string`: `sku`, `region`, `plugin-input`, or `usage`. Set only on `pricing` rows.

## InputMapping (reviewed data)

| Field | Meaning |
| --- | --- |
| `Provider` | Normalized billing provider, such as `aws` |
| `ResourceSegment` | Lowercased module/resource segment, such as `ec2/instance` |
| `Plugin` | The plugin whose input names the `plugin-input` keys are (`aws-public`) |
| `Inputs` | Ordered list of `{Key, Provenance}` |

Rules: keys are unique per mapping; order is the display order; a `sku` or
`region` key must be read by `proto.ResolveSKUAndRegion` (provenance test) and
must be an input the Pulumi type carries (research R12); a `plugin-input` key
must be one the mapping's plugin reads, and becomes a usage row when the
descriptor lacks it.

## EstimateInputContext

What the policy needs beyond the descriptor, built once per request by
`viewmodel.NewEstimateInputContext(discovery, pricingModeID)` for both
transports:

| Field | Meaning |
| --- | --- |
| `Hints` | Pricing-spec metric hints (research R4) |
| `Plugins` | Plugin names the discovery reports for the resource, `finfocus-plugin-` trimmed |
| `SelectedPlugin` | The selected mode's plugin, trimmed; empty for Automatic |

A mapping's plugin serves the estimate when it equals `SelectedPlugin`, or,
when `SelectedPlugin` is empty, when it is in `Plugins` (research R14).

## EstimatePropertyRow (extended)

Existing: `Key`, `OriginalValue`, `CurrentValue`, `CostDelta`. Added:

| Field | Meaning |
| --- | --- |
| `Class` | `PropertyClass` |
| `Provenance` | `InputProvenance`, empty unless `pricing` |
| `Editable` | False for `identity` |
| `DefaultVisible` | See research R6 |
| `Overridden` | `CurrentValue` differs from `OriginalValue` |
| `Usage` | A usage row: a reviewed `plugin-input` key the descriptor lacks; `OriginalValue` is empty |

## EstimateDetail

`Key`, `Value`: one read-only identity row.

## EstimatePropertyPlan

| Field | Meaning |
| --- | --- |
| `Rows` | Editable rows: pricing inputs first, then other, each group sorted by key except pricing, which follows mapping order |
| `Details` | Identity rows, sorted by key |
| `Curated` | True when the type has a mapping and at least one of its rows applies: a present reviewed key, or a usage row whose plugin serves the estimate |
| `Note` | Fallback text when `Curated` is false, else empty |

## IdentityOverrideError

Carries the offending `Key`. `Error()` reads `identity property "<key>" cannot be overridden`.

## Invariants

- Union of `Rows` and `Details` keys equals the redacted input keys, plus the
  usage keys of an applied mapping, plus override keys the descriptor lacks
  that are neither identity nor hidden (research R15).
- A usage row's override is sent only when its value is non-empty.
- `Rows` never contains an identity key.
- A pricing row is never demoted because its delta is zero.
- Redaction happens before classification, inside `BuildEstimatePropertyPlan`
  (research R9); no row or detail carries a hidden key, on either transport.
- A key in the `pulumi:` or `terraform:` namespace is always identity.
