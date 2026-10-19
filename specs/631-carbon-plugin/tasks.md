---
title: "Carbon Plugin Tasks"
description: "Test-first tasks for AWS EC2 carbon estimation and enrichment."
layout: "docs"
---

**Input**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), and [plugin contract](contracts/plugin.md).
**Status**: Implementation and verification complete; PR #1842 open.

All Go tests use testify. Run each new behavioral test before implementation to
establish its intended failure, then implement and rerun until it passes. Minimum
coverage is 80%, with 95% on critical calculation and communication paths.

## Phase 1 — Setup

- [x] T001 Initialize independent module `plugins/carbon/go.mod` with root Go/SDK/testify pins and resolve
  `plugins/carbon/go.sum`.

## Phase 2 — Foundational Model Data

- [x] T002 Add failing coverage and provenance tests in `plugins/carbon/data_test.go` for 34 exact identifiers, 38
  regions, missing-data lookup, and t3.nano memory correction.
- [x] T003 Embed the pinned CCF inputs using `plugins/carbon/data.go` and `plugins/carbon/data/aws.json`; document
  units, modifications, and retain LICENSE/NOTICE in `plugins/carbon/data/`.

## Phase 3 — US1: Compare EC2 Regional Emissions

**Goal**: Offline estimates match the independent CCF-derived numerical oracle.
**Independent test**: Four families, two regions, utilization boundaries, unknown
inputs, and CCF memory correction all pass without cloud credentials.

- [x] T004 [US1] Add independent literal numerical fixtures to `plugins/carbon/testdata/estimates.json` using
  research.md, including 0/0.5/1 utilization.
- [x] T005 [US1] Add failing model/config tests in `plugins/carbon/model_test.go` and
  `plugins/carbon/config_test.go` for numerical fixtures, idle power, nil descriptors, provider/type/SKU/region
  declines, and non-finite configuration.
- [x] T006 [US1] Implement `ConfigFromEnv` in `plugins/carbon/config.go` and `EstimateResource` in
  `plugins/carbon/model.go` with 730 hours, utilization 0.5, PUE 1.135, and faithful baseline memory handling.
- [x] T007 [US1] Add failing Supports/metadata/RPC tests in `plugins/carbon/plugin_test.go` covering current impact
  kind/unit, cancellation, nil requests, expiry, unsupported inputs, and capability exclusions.
- [x] T008 [US1] Implement `Plugin`, `New`, and `Info` in `plugins/carbon/plugin.go` with carbon-only capability
  and existing SDK service contracts.
- [x] T009 [US1] Run `go -C plugins/carbon test -race -coverprofile=coverage.out ./...` and inspect coverage for
  `plugins/carbon/model.go` and `plugins/carbon/plugin.go`.

## Phase 4 — US2: Preserve Monetary Pricing

**Goal**: Add one carbon measurement to each eligible existing resource row.
**Independent test**: Both projected entry points and batch/cache paths preserve
monetary amounts, currency, source, count, and missing-monetary status.

- [x] T010 [US2] Add failing routing tests in `internal/engine/carbon_test.go` for carbon-only exclusion from
  projected/actual/hourly/batch pricing, router/no-router paths, unknown-capability compatibility, and scorer
  exclusion.
- [x] T011 [US2] Add failing enrichment tests in `internal/engine/carbon_test.go` for
  combined/carbon-only/no-source configurations, canonical/deprecated/zero precedence, invalid units/values,
  priorities, decline notes, timeout, and fallback-disabled failure.
- [x] T012 [US2] Implement `isCarbonOnly` and `enrichProjectedCarbon` in `internal/engine/carbon.go`; apply generic
  filtering in `internal/engine/engine.go` without provider-specific constants or logic.
- [x] T013 [US2] Add failing cache regressions in `internal/engine/engine_cache_test.go` for fresh carbon on
  monetary cache hits, changed settings/data/source, no stored enrichment mutation, and uncached missing-monetary
  placeholders.
- [x] T014 [US2] Hook enrichment after monetary caching and on cache hits in both entry points in
  `internal/engine/engine.go`; preserve successful monetary rows and surface carbon errors independently.
- [x] T015 [US2] Add failing batch parity and batch-cache pre-check tests in
  `internal/engine/engine_batch_test.go`; ensure `internal/engine/engine_batch.go` never selects carbon-only
  clients for pricing.
- [x] T016 [US2] Add batch fresh/cache enrichment hooks and preserve partial results on cancellation in
  `internal/engine/engine.go`, including `GetProjectedCostWithErrors` result assembly before its cancellation
  return.
- [x] T017 [US2] Add current metric mapping and equivalency regressions in `internal/proto/adapter_test.go` and
  `internal/engine/carbon_test.go`; run targeted engine/proto tests with race and coverage.

## Phase 5 — US3: Install and Configure Carbon

**Goal**: Build, discover, configure, and invoke the plugin with production transport.
**Independent test**: A temporary plugin home discovers carbon-only metadata and
produces carbon beside a monetary fixture without credentials.

- [x] T018 [US3] Add failing real-transport installation tests in `test/integration/plugin/carbon_test.go` and
  real-response contract coverage in `internal/conformance/carbon_test.go`, following existing
  runner/temporary-home patterns.
- [x] T019 [US3] Implement signal-aware `plugins/carbon/cmd/main.go` and `plugins/carbon/plugin.manifest.json`;
  test handshake, settings delivery, version agreement, combined output, carbon-only NO_COST_DATA, and existing
  equivalencies.
- [x] T020 [US3] Add build/install/test/lint-carbon targets in `Makefile`; include carbon in
  build-all/test/lint/boundary targets and stamp manifest versions consistently.
- [x] T021 [US3] Add carbon unit/race/lint/tidy/version/boundary/platform-build gates in
  `.github/workflows/ci.yml`, preserving existing plugin checks.
- [x] T022 [US3] Register independent carbon releases in `release-please-config.json` and catalog metadata in
  `internal/registry/registry.json`; cover tag/asset metadata in `internal/registry/catalog_test.go` without
  editing the release manifest.
- [x] T023 [US3] Update `README.md`, `plugins/carbon/README.md`, and `docs/src/content/docs/plugins/carbon.md` with
  installation/settings, coverage, sources, operational limitations, and standalone missing-cost behavior.

## Phase 6 — Verification and Review

- [x] T024 Ran quickstart transport/conformance checks, module tidy/vet, and five platform builds; recorded outcomes
  in `specs/631-carbon-plugin/quickstart.md`.
- [x] T025 Ran make validate/test/lint/test-race/docs-lint, explicit changed-Markdown lint, boundaries, and
  govulncheck; fixed change-attributable failures and recorded toolchain limitations in
  `specs/631-carbon-plugin/quickstart.md`.
- [x] T026 Reviewed the full diff from base `837bc5a` with code-review and scout, fixed substantiated findings, and
  updated `spec.md` and `checklists/requirements.md`; remaining scout observations are recorded in
  `specs/631-carbon-plugin/review.md`.
- [x] T027 Prepared the scoped commit and PR closing #688, rechecked claim ownership and PR overlap, and released
  the claim after PR creation; the PR body includes validation and remaining limitations.

## Dependencies and Execution

T001 -> T002/T003 -> US1 -> US2 -> US3 -> final verification/review.
The user may select native execution or delegation before implementation.

Parallel opportunities after their prerequisites:

- US1 model/config tests can be written separately from RPC metadata tests once
  the data contract is fixed; implementation remains test-first.
- US2 cache tests and routing tests touch separate test files; integration hooks
  must have one owner to avoid shared `engine.go` edits.
- US3 documentation and release/catalog metadata can proceed alongside transport
  verification after the configuration/manifest contract is implemented.

MVP delivery requires US1 and US2 together; a carbon source that can displace
monetary pricing is not a releasable increment. US3 finishes usable packaging.

## Planning Tooling Note

The setup-tasks script fails because `resolve_template` and
`resolve_template_content` are absent from its sourced helpers. Task generation
used the checked-in `.specify/templates/tasks-template.md` fallback; no unrelated
script changes are included in this feature.
