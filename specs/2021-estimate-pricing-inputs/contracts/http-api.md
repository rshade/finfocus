---
title: "Estimate pricing inputs HTTP contract"
description: "Additive fields and the identity-override rule on the Estimate endpoints"
layout: "docs"
---

No new endpoint. The existing Estimate endpoints gain additive `display` fields
and one new rejection. The page computes no classification.

## Go methods the hook calls

| Method | Called by | Purpose |
| --- | --- | --- |
| `viewmodel.BuildEstimatePropertyPlan` | `viewmodel.BuildEstimateDisplay`, `baselineFailureDisplay`, `tui.EstimateModel.initializeProperties` | Classify, order, and tag rows; build details and note |
| `viewmodel.BuildEstimateDisplay` | `Server.serveEstimate` (baseline and recalculate) | Fill `display`, now including the plan |
| `viewmodel.RejectIdentityOverrides` | `Server.handleEstimateRecalculate`, `tui.EstimateModel` before recalculation | Refuse identity keys (takes the override map only) |
| exported form of `engine.redactedProperties` | `BuildEstimatePropertyPlan` | Redaction on both transports, no size cap (research R9) |
| `(*engine.Engine).DiscoverPricingSpec` | `Server.serveEstimate` (now on baseline and recalculate), `tui.EstimateModel` | Usage hints for classification; cached per type |
| `(*engine.Engine).EstimateBaseline`, `EstimateCost` | `Server.serveEstimate` (unchanged) | Pricing |

`BuildEstimatePropertyRows` stays as the row builder the plan calls.

## `display.properties[]` (additive)

| Field | Type | Meaning |
| --- | --- | --- |
| `key`, `originalValue`, `currentValue`, `delta` | string | Unchanged |
| `class` | `pricing` \| `other` | Identity rows are never here |
| `provenance` | `sku` \| `region` \| `plugin-input` \| `usage` | Present only for `pricing` |
| `editable` | bool | Always true for rows in this list |
| `defaultVisible` | bool | Show without Show all properties |
| `overridden` | bool | `currentValue` differs from `originalValue` |

Order: pricing inputs in mapping order, then other rows sorted by key.

## `display` (additive)

| Field | Type | Meaning |
| --- | --- | --- |
| `details` | `[{key, value}]` | Read-only identity metadata, sorted by key |
| `curated` | bool | The type has a mapping with at least one reviewed key present |
| `note` | string | Fallback text when `curated` is false, else empty |

The baseline-failure response (`status: "unavailable"`) carries the same fields,
with no deltas, so an unpriced resource keeps its editor.

## `POST /api/estimate/recalculate`

Request body unchanged: `{urn, overrides, pricingMode}`.

New rule: if any key in `overrides` is identity metadata for the resource, the
server answers before any pricing call:

```json
{"error": "identity property \"arn\" cannot be overridden", "code": "identity_override"}
```

Status 400. The message names the first offending key in sorted order and never
echoes the submitted value. The check sits in `handleEstimateRecalculate` with
the other request validation (`invalid_query`), so it runs before the resource
lookup: an identity override against an unknown URN answers 400, not 404. The
`invalid_pricing_mode` 400 and the 404 for an unknown resource are unchanged.

## `GET /api/estimate/baseline`

No request change. A request cannot carry overrides, so no new rejection.

## Redaction

Properties pass `engine.BuildAttributes` before classification. A hidden key is
absent from `properties`, `details`, and `note`. The payload still passes the
server's `RedactJSON`. No second credential-name list is added.

## Frontend behavior

`static/views/estimate.js` renders `display.properties` where `defaultVisible`
is true, or all of them while the Show all properties checkbox is on, renders
`display.details` as a read-only list, and shows `display.note` in the status
line. The checkbox sits in the controls row beside Show all resources. Toggling
changes no request and no state: the list is rebuilt from the last response,
and the page restores the focused input by its `key`.
