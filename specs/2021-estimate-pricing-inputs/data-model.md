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
| `ResourceSegment` | Lowercased module/resource segment, such as `ec2/instance` |
| `Inputs` | Ordered list of `{Key, Provenance}` |

Rules: keys are unique per mapping; order is the display order; a `sku` or
`region` key must be read by `proto.ResolveSKUAndRegion` (provenance test).

## EstimatePropertyRow (extended)

Existing: `Key`, `OriginalValue`, `CurrentValue`, `CostDelta`. Added:

| Field | Meaning |
| --- | --- |
| `Class` | `PropertyClass` |
| `Provenance` | `InputProvenance`, empty unless `pricing` |
| `Editable` | False for `identity` |
| `DefaultVisible` | See research R6 |
| `Overridden` | `CurrentValue` differs from `OriginalValue` |

## EstimateDetail

`Key`, `Value`: one read-only identity row.

## EstimatePropertyPlan

| Field | Meaning |
| --- | --- |
| `Rows` | Editable rows: pricing inputs first, then other, each group sorted by key except pricing, which follows mapping order |
| `Details` | Identity rows, sorted by key |
| `Curated` | True when the type has a mapping and a reviewed key is present |
| `Note` | Fallback text when `Curated` is false, else empty |

## IdentityOverrideError

Carries the offending `Key`. `Error()` reads `identity property "<key>" cannot be overridden`.

## Invariants

- Union of `Rows` and `Details` keys equals the redacted input keys.
- `Rows` never contains an identity key.
- A pricing row is never demoted because its delta is zero.
- Redaction happens before classification, inside `BuildEstimatePropertyPlan`
  (research R9); no row or detail carries a hidden key, on either transport.
- A key in the `pulumi:` or `terraform:` namespace is always identity.
