---
title: "Carbon Plugin Review"
description: "Implementation review findings and deferred repository observations."
layout: "docs"
---

## Implementation Review

The Standards and Spec reviews found no code-standard issue. The Spec review raised
one batch-latency concern and one release-version assumption. Batch carbon enrichment
now runs in a bounded worker pool after monetary batch workers finish; a regression
test confirms two slow carbon calls run concurrently. The installation integration
test now derives the install path and manifest assertion from its explicit test
version. Both regressions were reproduced before their fixes and passed afterward.

No actionable code-review findings remain. The full change was reviewed against the
approved issue scope and repository standards. No separate proto change was needed:
the production transport test verifies the existing adapter maps the impact metric
and renders the carbon equivalency.

## Scout Observations

These are pre-existing opportunities in files touched by this work. They do not affect
the carbon implementation and remain outside this issue's scope.

- **Score 16/25**: `Makefile`'s plugin-boundary check can mask a `go list` failure in
  its pipeline and report success without checking the intended package list.
- **Score 15/25**: `internal/registry/registry_json_test.go` has older raw `t.Fatal`
  assertions and repeated schema checks where setup assertions could be clearer.
- **Score 10/25**: `make docs-serve` help text says port 4000 while the docs server uses
  port 4321.

## Verification Limit

`govulncheck` found Go 1.27.1 standard-library vulnerabilities, which are fixed in
Go 1.27.2. The unchanged main branch has 13 standard-library findings; the carbon
module has 8 reachable standard-library findings and 5 imported-package findings
without call traces. No vulnerable required module was reported. Updating the shared
Go pin was not part of issue #688.
