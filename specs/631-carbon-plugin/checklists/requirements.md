---
title: "Carbon Plugin Specification Checklist"
description: "Quality review of the carbon plugin specification."
layout: "docs"
---

**Feature**: [Carbon plugin](../spec.md)

## Content Quality

- [x] Intent, user value, constraints, and MVP boundaries are explicit.
- [x] User journeys have independently verifiable acceptance scenarios.
- [x] Edge cases and measurable success criteria are defined.
- [x] Current plugin examples and protocol drift are reconciled explicitly.
- [x] Core remains provider agnostic and the plugin operates offline.

## Readiness

- [x] Monetary preservation and carbon deduplication are required.
- [x] Actual emissions and later provider/service phases are excluded.
- [x] User has reviewed the proposed generic carbon enrichment design.
- [x] Planning pins coefficient revision and instance/region coverage.
- [x] Plan, tasks, and cross-artifact analysis are complete.
- [x] User has reviewed the implementation plan and selected execution method.

## Review Notes

The design reconciliation section intentionally records concrete integration
constraints alongside user-facing requirements. It prevents the obsolete issue
example from becoming an implementation contract. The approved MVP is implemented; see [quickstart validation](../quickstart.md) and [review notes](../review.md).
