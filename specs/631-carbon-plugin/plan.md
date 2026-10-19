---
title: "Carbon Plugin Implementation Plan"
description: "Implement EC2 carbon data and preserve monetary pricing."
layout: "docs"
---

> **For agentic workers:** Use `superpowers:executing-plans` for native execution,
> or `superpowers:subagent-driven-development` if the user selects delegation.
> Complete the test-first steps in [tasks.md](tasks.md) in order.

**Goal:** Deliver the approved AWS EC2 operational-carbon MVP alongside monetary
pricing, using a separately released in-repository plugin.

**Architecture:** A nested `plugins/carbon` module owns all data and calculations.
Core discovers carbon capability separately and enriches one existing resource
row after monetary caching. Existing protocol impact metrics and equivalency
rendering are reused.

**Spec:** [spec.md](spec.md), approved 2026-10-18.
**Branch:** `631-carbon-plugin`.
**Status:** Approved and implemented; verification complete.

## Technical Context

**Language/Version**: Go 1.27.1.
**Primary Dependencies**: finfocus-spec v0.7.6, gRPC, protobuf, zerolog, testify
v1.12.1; reuse existing versions and add no external runtime data client.
**Storage**: Embedded immutable coefficients; existing optional monetary cache.
**Testing**: Testify unit tests, real plugin transport integration, conformance,
race checks, Markdown lint, and cross-compilation.
**Target Platform**: Linux amd64/arm64, macOS amd64/arm64, Windows amd64.
**Project Type**: CLI host with a nested Go plugin module.
**Performance Goals**: Constant-time model lookups; reuse bounded per-resource
workers; no provider requests or runtime downloads.
**Scale/Scope**: 34 EC2 instance identifiers, 38 regions, one configurable
utilization fraction, projected operational carbon only.

## Global Constraints

- Keep AWS data and logic in the plugin; core changes are generic orchestration.
- Default utilization is 50 percent; use a 730-hour modeled month.
- Preserve monetary values, currency, pricing source, and resource count.
- Do not double-count a canonical or deprecated existing carbon measurement.
- Preserve explicit missing monetary data, including carbon-only installations.
- No actual carbon, storage, network, embodied, Azure, GCP, or Kubernetes allocation.
- Use current impact metrics and `carbon_footprint`; no protocol extension.
- Follow [research.md](research.md) for pinned data, sources, formula, and fixtures.
- Tests precede implementations; no stubs, TODO comments, or new lint exemptions.

## Review Focus

1. Batch and cached monetary responses must receive fresh carbon exactly once.
2. Carbon-only explicit routes must never reach monetary or hourly-rate lookup.
3. Zero carbon already supplied by a monetary source suppresses enrichment.
4. Cancellation during enrichment returns completed monetary rows with an error.
5. CCF baseline memory, t3 behavior, and tons-to-grams conversion must match fixtures.

## Constitution Check

Checked before research and rechecked against the contracts after research:

- [x] Plugin-first: provider logic stays in a separate module.
- [x] TDD: failing behavioral tests precede each implementation task; minimum
  80% changed-package coverage and 95% critical calculation/communication paths.
- [x] Cross-platform: standard Go implementation; all five target builds planned.
- [x] Documentation: plugin README, source attribution, user guide, root README,
  and spec status updates included with implementation.
- [x] Protocol stability: reuse existing SDK and capability enums unchanged.
- [x] Completeness: all shipped behavior covered; no placeholders in product code.
- [x] Persistence: no new state; existing monetary cache stays optional.
- [x] Multi-repo coordination: no upstream change required; PR #1819 overlap noted.
- [ ] Execution gates: validate, test, race, lint, security, and transport checks
  must pass before completion; these are not established by this plan.

No constitutional exceptions are requested. No TUI layout changes are planned;
existing equivalency rendering is exercised through output and transport tests.

## Project Structure and Responsibilities

| Files | Responsibility |
| --- | --- |
| `plugins/carbon/go.mod`, `go.sum` | Independent module using root dependency pins |
| `plugins/carbon/config.go`, `config_test.go` | Validated utilization setting |
| `plugins/carbon/data/aws.json`, `data.go`, `data_test.go` | Embedded versioned model inputs and provenance checks |
| `plugins/carbon/data/README.md`, `LICENSE`, `NOTICE` | Upstream attribution and modeling conventions |
| `plugins/carbon/model.go`, `model_test.go`, `testdata/estimates.json` | Pure operational calculation and numerical oracle |
| `plugins/carbon/plugin.go`, `plugin_test.go`, `cmd/main.go` | BasePlugin, explicit Info/settings, RPC validation, serving |
| `internal/engine/carbon.go`, `carbon_test.go` | Capability filtering and generic enrichment |
| `internal/engine/engine.go`, `engine_batch.go` | Shared filtering, cache/fresh/batch hooks, partial results |
| `internal/engine/engine_cache_test.go`, `engine_batch_test.go` | Carbon cache and batch regressions |
| `internal/proto/adapter_test.go` | Current impact-metric mapping regression |
| `internal/conformance/carbon_test.go` | Existing contract runner against real carbon responses |
| `test/integration/plugin/carbon_test.go` | Build, install, discovery, transport and combined-output checks |
| `Makefile`, `.github/workflows/ci.yml` | Build/install/test/lint/tidy/version/boundary/compile gates |
| `release-please-config.json`, `internal/registry/registry.json` | Independent release and catalog metadata |
| `README.md`, `plugins/carbon/README.md`, `docs/src/content/docs/plugins/carbon.md` | User guidance and limitations |

## Interfaces

Plugin model:

- `ConfigFromEnv(getenv func(string) string) (Config, error)` parses only
  `FINFOCUS_CARBON_UTILIZATION`; default 0.5, valid closed interval [0, 1].
- `EstimateResource(resource *pbc.ResourceDescriptor, cfg Config) (Estimate, error)`
  validates the documented AWS EC2 descriptor/SKU/region and calculates the total.
- `New(cfg Config) *Plugin` embeds `pluginsdk.BasePlugin`.
- `Info(version string) *pluginsdk.PluginInfo` advertises AWS and carbon only.
- RPC implementation and error contracts are in [contracts/plugin.md](contracts/plugin.md).

Core:

- `isCarbonOnly(client *pluginhost.Client) bool` identifies explicit carbon
  capability without monetary projected/actual/pricing-spec/batch capabilities.
- `enrichProjectedCarbon(ctx context.Context, resource ResourceDescriptor,
  results []CostResult) []ErrorDetail` adds only missing carbon and source notes.
- Apply capability filtering before Supports for router, fallback, and no-router
  selection. Carbon requests require explicit carbon metadata. Preserve scorer
  exclusions and existing unknown-capability monetary behavior.
- Reuse `getProjectedCostFromPlugin` and descriptor sanitation. Ignore carbon
  response cost fields. Validate carbon unit and finite nonnegative value.

## Task 1 — Model and Source Data

- [ ] Write `TestEstimateResource`, data coverage/provenance, and config tests
  using [research.md](research.md) numerical fixtures and `assert.InDelta` at
  1e-8 grams. Include all families, regional difference, 0/1 utilization,
  unknown sizes/regions, nil requests, non-finite config, and t3.nano correction.
- [ ] Run `go -C plugins/carbon test ./...`; establish the intended failure.
- [ ] Implement immutable embedded tables and the pure model; retain upstream
  LICENSE/NOTICE. Include family-specific baseline memory and all 34 identifiers.
- [ ] Run plugin unit tests and coverage; compare against independent fixtures.

## Task 2 — Plugin Transport and Settings

- [ ] Add failing metadata, Supports, nil/unsupported/canceled RPC, impact-unit,
  zero-dollar-transport, expiry, and environment-setting tests.
- [ ] Implement `Plugin`, `New`, `Info`, manifest, and signal-aware `cmd/main.go`.
  Emit monthly `gCO2e`, assumptions/source revision, and immediate expiry.
- [ ] Verify SDK validation and real transport handshake using the nested module.

## Task 3 — Generic Carbon Enrichment

- [ ] Add failing engine tests for combined monetary/carbon, carbon-only,
  no-carbon-source, legacy capability, explicit-route exclusion, priorities,
  fallback-disabled failure, existing canonical/deprecated/zero carbon,
  unsupported/invalid metrics, Supports failure, and context cancellation.
- [ ] Implement `carbon.go` and selection filtering. Add hooks after monetary
  cache reads and after cache writes in both projected entry points, batch fresh
  results, and batch cache pre-checks. Never cache the enrichment mutation.
- [ ] Add failing cache tests proving settings/source/data changes refresh carbon,
  stored monetary rows are unmodified, and placeholder rows remain uncached.
- [ ] Add batch parity and both-entry-point partial-cancellation regressions.
  Assemble `GetProjectedCostWithErrors` partial rows before returning cancellation.
- [ ] Run targeted engine/proto tests with race detection and coverage.

## Task 4 — Delivery Integration and Documentation

- [ ] Add integration/conformance tests using the real plugin binary with an
  isolated home and monetary fixture source; verify missing-cost status,
  canonical metric mapping, equivalent output, and zero credential dependencies.
- [ ] Add `build/install/test/lint-carbon` targets and integrate aggregate targets,
  CI tests/lint/tidy/version checks, boundaries, and platform builds. Stamp the
  installed manifest version consistently with the binary.
- [ ] Register `plugins/carbon` with release-please `initial-version: 0.1.0` and
  exclude it from root release paths. Do not edit the release manifest or changelog.
  Add catalog tag prefix `carbon-` and monorepo asset prefix metadata.
- [ ] Document installation, utilization, 34-size/38-region support, CCF sources,
  operational-only limitations, no runtime credentials, and fresh enrichment.
- [ ] Verify quickstart, full repository gates, attribution, cross-platform builds,
  security checks, and changed Markdown. Review diff with code-review and scout;
  fix substantiated findings without expanding scope. Update spec/task status.

## Execution and Handoff

Tasks 1 and 2 establish the plugin; Task 3 can be developed independently after
contracts are fixed; Task 4 needs both. Full minimum delivery includes all three
user stories because a carbon-only monetary source would violate US2.

Recommend native execution: the monetary selection/cache/batch changes share
interfaces and benefit from one implementer following them together. A delegated
review is still required by the repository's code-review workflow before PR.
The user may instead select subagent-driven execution when approving this plan.

Retain the #688 claim while waiting for plan review. Once implementation passes
its gates, prepare the Conventional Commit and PR against `main`, preserve
PR #1819 changes on rebase, and release the claim after the PR is open.
