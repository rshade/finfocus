---
title: "Carbon Estimation Plugin"
description: "AWS EC2 operational carbon estimates alongside monetary pricing."
layout: "docs"
---

**Feature Branch**: `631-carbon-plugin`
**Created**: 2026-10-18
**Status**: Implemented and verified; [PR #1842](https://github.com/rshade/finfocus/pull/1842) open
**Issue**: [#688](https://github.com/rshade/finfocus/issues/688)

## Intent and Scope

Provide infrastructure owners with projected operational carbon emissions for
AWS EC2 resources, alongside their existing monetary estimates. Follow the
current in-repository plugin format demonstrated by Kubernetes and Jev.
All AWS instance specifications, power models, and regional coefficients belong
in the carbon plugin. Core remains a provider-agnostic orchestrator.

The initial delivery is Phase 1 of the issue: EC2 instance families t3, m5,
c5, and r5, with explicitly documented supported sizes and regional data.
Storage, RDS, Azure, GCP, embodied emissions, and actual emissions are later
phases and are outside this implementation. Kubernetes and Jev are architectural
references; this issue does not add carbon allocation to Kubernetes or alter
recommendation scoring.

## User Scenarios & Testing

### User Story 1 — Compare EC2 Regional Emissions (Priority: P1)

An infrastructure owner estimates emissions for a declared EC2 instance and
compares the same instance across regions with different electricity mixes.

**Why this priority**: Regional comparisons provide the plugin's primary value.

**Independent Test**: Estimate a known instance in two supported regions and
check both results against independently calculated reference values.

**Acceptance Scenarios**:

1. **Given** a supported instance and region, **when** the user requests projected
   costs, **then** the result includes estimated monthly operational carbon and
   the assumptions needed to interpret it.
2. **Given** identical instance specifications in two regions, **when** their
   published carbon intensities differ, **then** the estimates reflect that
   difference rather than a global average.
3. **Given** no cloud credentials and no existing local state, **when** a
   supported resource is estimated, **then** the estimate succeeds offline.

### User Story 2 — Preserve Monetary Pricing (Priority: P1)

An owner installs carbon estimation alongside a monetary pricing plugin and
receives both outputs for the same resource.

**Why this priority**: Adding emissions must preserve existing cost estimates.

**Independent Test**: Run a resource through monetary-only, carbon-only, and
combined plugin configurations; compare currency and cost values.

**Acceptance Scenarios**:

1. **Given** a successful monetary estimate and a supported carbon resource,
   **when** both plugins are enabled, **then** the monetary amount, currency,
   pricing source, and resource count remain unchanged and carbon is added.
2. **Given** a monetary estimate and a carbon failure or decline, **when** the
   command completes, **then** monetary data remains available and the carbon
   failure is reported without a fabricated zero-carbon measurement.
3. **Given** a resource without monetary data, **when** carbon can be estimated,
   **then** carbon remains available and missing monetary data stays explicit.
4. **Given** carbon already supplied by the monetary source, **when** enrichment
   runs, **then** it does not double-count or overwrite that carbon measurement.

### User Story 3 — Install and Configure Carbon (Priority: P2)

An owner builds or installs the plugin using the same conventions as the other
in-repository plugins and adjusts utilization assumptions through settings.

**Why this priority**: The plugin must be discoverable and usable in real runs.

**Independent Test**: Build and install into an isolated plugin home, discover
metadata/settings, and request an estimate through the real plugin transport.

**Acceptance Scenarios**:

1. **Given** a clean plugin home, **when** the plugin is installed, **then** the
   CLI discovers its version, supported provider, capabilities, and settings.
2. **Given** a valid utilization setting, **when** an estimate is requested,
   **then** the configured assumption affects the estimate and is documented.
3. **Given** invalid configuration, **when** the plugin starts, **then** it
   reports an actionable validation error rather than silently substituting
   an unrelated default.

### Edge Cases

- Missing, unknown, or unsupported instance type, provider, or region is declined
  explicitly; no global regional fallback or invented instance specification.
- A region absent from the pinned source dataset is unsupported and documented.
- Zero utilization retains idle compute power and provisioned memory consumption.
- Negative, non-finite, or greater-than-100-percent utilization is rejected.
- Multiple carbon sources follow deterministic routing priority; only one carbon
  estimate contributes per resource.
- Cancellation and carbon timeouts preserve successful monetary estimates.
- Cache reuse must respect settings, source identity, dataset version, and units;
  settings changes must not return a previously cached carbon assumption.
- Generic and nested-module tests must not require AWS credentials, Kubernetes
  access, or a Jev API key.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST estimate monthly operational carbon for documented
  sizes in EC2 t3, m5, c5, and r5 families using region-specific coefficients.
- **FR-002**: The model MUST account for idle and utilized compute consumption,
  allocated memory, operating hours, and data-center power usage effectiveness.
- **FR-003**: Default projected utilization MUST be 50 percent and documented.
- **FR-004**: Every coefficient and instance specification MUST have a traceable
  source, pinned dataset revision, and documented unit conversion.
- **FR-005**: Unsupported inputs MUST yield an explicit decline rather than a
  fabricated zero or a global-average emission estimate.
- **FR-006**: Adding carbon MUST preserve monetary results and avoid duplicate
  resource rows and double-counted carbon metrics.
- **FR-007**: Missing monetary pricing MUST remain distinguishable from free
  infrastructure even when a carbon estimate is available.
- **FR-008**: Carbon failures MUST be visible without discarding successful
  monetary results.
- **FR-009**: Existing carbon equivalency presentation MUST consume the returned
  measurement with a compatible unit and reporting period.
- **FR-010**: Plugin build, installation, discovery, settings, independent tests,
  linting, release registration, and boundary checks MUST follow repository
  conventions without importing core packages into the plugin.
- **FR-011**: Estimation MUST require neither provider API calls nor prior local
  state, and MUST honor request cancellation.
- **FR-012**: Documentation MUST describe assumptions, coverage, provenance, and
  limitations; estimates MUST NOT be represented as measured or total lifecycle
  emissions.

### Key Entities

- **Instance specification**: Supported instance identifier, provisioned vCPU and
  memory quantities, and associated compute power model.
- **Regional coefficient**: Provider region, electricity carbon intensity, unit,
  dataset revision, and source citation.
- **Carbon estimate**: Operational emissions, normalized unit, monthly period,
  utilization assumption, and methodology revision.

## Design Reconciliation

The issue's sample response and metric key predate the current SDK. Implementation
will use the existing carbon impact-metric kind, mapped by core to the canonical
`carbon_footprint` key. A supported mass unit such as `gCO2e` will describe the
monthly total; `/month` will not be embedded into a mass unit that existing
normalization cannot parse. No protocol extension is required.

Use a separate `plugins/carbon` module with `BasePlugin`, `Info(version)`, explicit
capabilities/settings, `pluginsdk.Serve`, a manifest, and `cmd/main.go`, matching
Kubernetes and Jev. Core discovers carbon capability independently from monetary
pricing and adds missing carbon after monetary selection. This generic pass
must cover both projected-cost entry points and cache behavior, without AWS
knowledge in core or changes to actual-cost semantics.

Alternatives considered:

- A standalone projected-cost source is smaller, but current first-success
  selection can replace dollar pricing with a zero-dollar carbon response or
  skip carbon entirely. It does not satisfy the combined-output requirement.
- Adding emissions to an existing AWS pricing plugin avoids core enrichment but
  requires another repository and couples the new source to one pricing source.

The separate module plus generic enrichment is recommended. JEV's pending rename
in PR #1819 is independent; shared Makefile/release/CI edits must preserve its
migration when rebased.

## Assumptions and Methodology

The MVP uses an auditable snapshot of the
[Cloud Carbon Footprint methodology](https://www.cloudcarbonfootprint.org/docs/methodology/)
for operational compute and memory estimates. CCF includes embodied emissions in
its full methodology; this MVP explicitly excludes them. Exact data revisions,
instance sizes, available region coverage, and numerical fixtures must be resolved
and documented during implementation planning, before model code is written.

## Success Criteria

- **SC-001**: Independently calculated numerical fixtures cover each of the four
  supported families, at least two distinct regional intensities, and utilization
  boundaries; results agree within a documented floating-point tolerance.
- **SC-002**: Combined-plugin checks preserve all monetary values and resource
  counts while emitting at most one carbon total per resource.
- **SC-003**: Carbon decline, timeout, and cancellation checks preserve any
  successful monetary estimate and expose missing emissions clearly.
- **SC-004**: A clean-home transport check discovers the plugin and produces a
  carbon measurement consumed by the existing equivalency formatter.
- **SC-005**: All supported inputs work without credentials or network access at
  runtime, and every unsupported-data fixture declines explicitly.
