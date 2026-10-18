---
title: "Estimate resource-list contract"
description: "Default filtering and complete-list escape hatch"
layout: "docs"
---

## GET /api/estimate/resources

Authenticated GET. Optional `showAll=true` includes confirmed non-billable
resources. Omitted or false applies the default; invalid boolean values return
400 `invalid_query`. Existing launch filters and safe redaction always apply.

200 response retains `resources` (array of descriptors) and adds:

- `billability`: ID-keyed map of non_billable, potentially_billable, unknown.
- `hiddenCount`: number hidden by default policy in this response.
- `preferredURN`: recommended initial ID, empty for an empty list.

Baseline and recalculation continue to accept any safe source URN regardless of
list visibility. No additional discovery RPC occurs during resource listing.
503 loading and 502 source errors retain existing contracts.
