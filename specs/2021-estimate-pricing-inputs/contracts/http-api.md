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
| `viewmodel.BuildEstimatePropertyPlan` | `viewmodel.BuildEstimateDisplay`, `baselineFailureDisplay`, `tui.EstimateModel.initializeProperties` | Classify, order, and tag rows, including usage rows; build details and note |
| `viewmodel.NewEstimateInputContext` | `Server.serveEstimate`, `baselineFailureDisplay`, `tui.EstimateModel` (on discovery and on a mode switch) | Hints, discovered plugins, and the selected mode's plugin (research R14) |
| `viewmodel.EstimatePropertyRow.SetCurrentValue` | the plan builder, `tui.EstimateModel` edit commit | One rule for `overridden` and `defaultVisible` |
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
| `usage` | bool | A usage row: a reviewed `plugin-input` key the resource lacks. `originalValue` is `""`; the row is editable and default-visible |

Order: pricing inputs in mapping order (usage rows take their mapping position
after the present keys), then other rows sorted by key.

Which rows appear depends on the selected `pricingMode`: a mapping's
`plugin-input` and usage rows appear only when the mapping's plugin
(`aws-public`) serves the estimate, meaning it is the selected mode's plugin,
or, with no mode selected, one of the plugins the discovery reports. `sku` and
`region` rows appear for every mode. An override whose key the resource lacks
and whose usage row does not apply still has a row (`class: other`,
`overridden: true`), unless the key is identity (rejected) or hidden.

## `display` (additive)

| Field | Type | Meaning |
| --- | --- | --- |
| `details` | `[{key, value}]` | Read-only identity metadata, sorted by key |
| `curated` | bool | The type has a mapping and at least one reviewed row applies: a present `sku` or `region` key, a present `plugin-input` whose plugin serves, or a usage row whose plugin serves (FR-012) |
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

A usage row renders an empty input. The page sends an input as an override
only when its value differs from `originalValue`, so an empty usage row sends
nothing and a filled one sends its value. Changing the pricing mode re-requests
with the current overrides, and the server rebuilds the rows for that mode.
