---
title: "Estimate picker data model"
description: "Billability classes and selection transitions"
layout: "docs"
---

## Billability

String values: `non_billable`, `potentially_billable`, `unknown`.
Pulumi bookkeeping is non-billable. Only unanimous explicit zero_cost metadata
from effective providers can establish cloud non-billability. A non-zero-cost
billing mode means potentially billable regardless of its rate. Unavailable,
unsupported, malformed, and conflicting metadata remain unknown.

## Candidate List

Existing ResourceDescriptor identity and properties stay intact. Classifications
are returned separately keyed by ID. Hidden count describes only policy-hidden
resources after launch filters. Preferred URN ranks eligible positive baselines,
then potentially billable, then unknown, then non-billable when all are shown.
Selection is preserved while still visible; removed selections reset the editor.
