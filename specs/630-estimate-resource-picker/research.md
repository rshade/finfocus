---
title: "Estimate picker research"
description: "Evidence and classification tradeoffs"
layout: "docs"
---

## Decisions

- Decision: Reuse engine-local cached discovery with the full existing key.
  Rationale: SKU, region, and property edits invalidate the lookup naturally;
  listing never probes. Alternatives: probing every candidate adds latency and
  warnings; a type catalogue duplicates plugin knowledge.
- Decision: Require metadata agreement across effective projected-cost clients.
  Rationale: an unrelated plugin's zero-cost mode must not hide a priced resource.
  EstimateCost tries engine clients before projected fallback, so metadata must
  agree across engine clients and any routed projected fallback clients. Missing, unknown, or conflicting
  modes produce unknown. No Supports RPC is added for classification.
- Decision: Share candidate filtering and preferred initial selection in engine.
  Rationale: both transports can use one policy; stable source order is retained.
  Positive known projected baselines may improve initial ranking only.
- Decision: List filtering belongs after session preview overlays and CLI merge
  and launch filters. Baseline lookup always uses the complete safe source list.
  Rationale: prevents stale classification and preserves direct access.

## Resolved Questions

No protocol change or dependency issue is required. Issue #1732 property selection
is separate. Interactive plan mode has no dropdown: select the shared preferred
candidate and explain an empty list; explicit CLI mode still addresses any type.
