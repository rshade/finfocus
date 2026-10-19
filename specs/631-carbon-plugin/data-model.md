---
title: "Carbon Plugin Data Model"
description: "Static model inputs and generic enrichment results."
layout: "docs"
---

## Plugin Model

`Config` contains `Utilization float64`, a fraction in the closed range [0, 1].
`ConfigFromEnv(getenv func(string) string) (Config, error)` defaults to 0.5;
`FINFOCUS_CARBON_UTILIZATION` is the sole configurable input. Empty means default;
non-numeric, infinite, NaN, and out-of-range values are errors.

`Instance` contains vCPU count, memory GiB, processor group, and whether memory
power is already included in the compute measurement. Instance identifiers are
an explicit allowlist, not a size-name extrapolation.

`Region` contains a region identifier and grams CO2e per kWh. Absence is distinct
from a valid zero coefficient. Regions not present in the pinned dataset decline.

`Estimate` contains monthly grams CO2e, monthly kWh, and an assumptions note.
`EstimateResource(resource *pbc.ResourceDescriptor, cfg Config) (Estimate, error)`
validates provider, EC2 type, SKU, region, and config before calculating. There is
no mutable model state, persistent store, provider client, or runtime download.

## Enrichment

`enrichProjectedCarbon(ctx context.Context, resource ResourceDescriptor,
results []CostResult) []ErrorDetail` modifies only the first existing result's
missing carbon metric and carbon-specific notes. It clones a sustainability map
before mutation. It never appends a resource row, copies monetary values from a
carbon response, or changes existing currency, pricing source, or cost fields.

Canonical or deprecated existing carbon keys both suppress enrichment. A zero
canonical measurement is present data, not a cache miss. Invalid incoming carbon
values or units are rejected and visible as a carbon failure.

Without monetary pricing, the existing `NO_COST_DATA` placeholder remains and
receives a carbon measurement; it remains ineligible for the monetary cache.

Carbon decline produces a note with its actionable reason. RPC failures also
produce an `ErrorDetail` in the error-aware entry point. Log events use the
context logger and `component`/`operation` fields. No configured carbon source
means no note or behavior change.

## Cache Lifetime

The existing monetary result is serialized to cache before enrichment. Every
cache hit is enriched again, including hits in the batch pre-check. Carbon-only
responses set `expires_at` to the current timestamp to discourage direct host
reuse. No separately persisted carbon cache is introduced. A monetary source's
own carbon measurement keeps its existing response lifetime and precedence.
