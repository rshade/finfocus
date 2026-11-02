# Feature Specification: Spot Exposure Totals

**Feature Branch**: `1837-spot-exposure-totals`
**Created**: 2026-11-02
**Status**: Implemented
**Input**: User description: "Show how much of a total is spot and how risky
it is: spot cost, spot share of spend, and cost-weighted interruption risk,
in `cost projected` totals and the overview (GitHub issue #1837, part of epic
367). Builds on the structured `PricingCategory` and `SpotInterruptionRisk`
fields from #1836."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See the spot exposure of a total (Priority: P1)

A person reads the `cost projected` summary or the overview summary and sees,
next to the total, how much of it is spot, what share that is, and the
cost-weighted interruption risk: "this adds $300/mo of spot at about 0.18
weighted risk" instead of one blended number.

**Why this priority**: This is the feature. A total that mixes on-demand and
spot hides interruption exposure.

**Independent Test**: Point core at a plugin that answers DYNAMIC with risk
0.2 and 0.1 for two resources and STANDARD for a third. The summary shows the
spot cost, its share of the total, and the weighted risk; JSON carries the
same figures.

**Acceptance Scenarios**:

1. **Given** results with DYNAMIC rows, **When** `cost projected` or the
   overview renders its summary, **Then** a `Spot exposure` line shows the
   spot cost, share, and weighted risk, and JSON carries `summary.spot`.
2. **Given** no DYNAMIC row, **When** the same output is rendered, **Then**
   nothing is added: text and JSON are byte-identical to before.
3. **Given** a DYNAMIC row whose risk is 0.0 (unknown), **When** the exposure
   is computed, **Then** the row counts toward the spot cost and share but not
   toward the weighted risk, and the line states how many rows were left out.
4. **Given** every DYNAMIC row has an unknown risk, **When** it is rendered,
   **Then** the weighted risk reads `unknown` and JSON omits `weightedRisk`;
   it is never shown as 0.
5. **Given** rows in different currencies, **When** the exposure is computed,
   **Then** no figures are summed and the exposure is absent.

### User Story 2 - See the spot change of a cost diff (Priority: P2)

A reviewer of a change runs `cost projected` on a plan (before and after) and
sees how the spot cost changes, with replacements counted once.

**Acceptance Scenarios**:

1. **Given** a diff that creates a spot resource, **When** it is rendered,
   **Then** a `Spot change` line shows the delta with the before and after
   spot cost.
2. **Given** a replacement (collapsed to one `update` entry), **When** the
   change is computed, **Then** the resource counts once on each side.
3. **Given** a diff with no spot resource on either side, **Then** nothing is
   added.

## Requirements *(mandatory)*

- **FR-001**: One function, `engine.ComputeSpotExposure`, computes the
  exposure. Every surface calls it (through `SpotExposureFromCosts` for cost
  results and from the overview totals). No renderer formats an exposure
  number itself.
- **FR-002**: Spot cost is the sum of the cost of rows whose category is
  DYNAMIC. Share is spot cost divided by the total of all rows given to the
  function, absent when that total is not positive.
- **FR-003**: Weighted risk is `sum(cost * risk) / sum(cost)` over DYNAMIC
  rows with a known risk (finite, greater than 0 and at most 1). It is absent
  when no such row carries cost.
- **FR-004**: Rows with unknown risk are counted in `spotResources` and in
  `riskUnknownResources`, and the display text names how many were excluded.
- **FR-005**: Basis. Projected figures use the monthly cost. The overview uses
  `ProjectedCost.MonthlyCost` over the rows `TotalProjected` sums (error rows
  and live allocation children are skipped), never `MTDCost`. The `cost
  projected` summary uses `CostResult.Monthly` of the results the existing
  total sums; rows that failed to price are skipped.
- **FR-006**: Currency. Rows with different non-empty currencies are never
  summed: the exposure is absent. (The overview already refuses to render its
  table and JSON in that case; `cost projected` does not check currency, so
  this is where the exposure differs from its total.)
- **FR-007**: Diff. `ComputeSpotExposureChange` sums each side once over the
  diff entries, which are already one per resource after
  `collapseReplacements`: before from each entry's `Before`, after from
  `AfterCosts()`, the basis `TotalAfter` uses. The delta is after minus before.
  The stack-wide total (unchanged state resources) is not included.
- **FR-008**: Informational only. The exposure does not feed budgets, exit
  codes, thresholds, or CI gates.
- **FR-009**: Output without any spot row is unchanged.

## Key Entities

See `contracts/exposure.md`.

## Web UI Fit *(mandatory)*

- **Disposition**: Addition to an existing view (Overview totals bar). No new
  page, endpoint, or query parameter.
- **Web data**: The totals payload (`/api/overview/query`,
  `/api/overview/resource`, and the SSE snapshot) embeds
  `engine.OverviewTotals`, which gains `spot`. The figures and the `display`
  text are built by `engine.ComputeSpotExposure`, the function the CLI calls.
  The browser formats nothing; `overview.js` renders `totals.spot.display`
  verbatim as a `Spot exposure` item beside the existing totals.
- **Layout fit**: One more `totals-item` in the existing totals bar, after
  Potential savings; it wraps like the others. Absent when `spot` is absent.
- **Not applicable**: `cost projected` has no web page; the web Estimate and
  Forecast pages show no cross-resource total this feature extends. The
  overview TUI renders no totals line, so it has nothing to extend.
- **Render evidence**: see the verification notes in `plan.md`.
- **Completion**: A UX addition to an existing view; no follow-up `web-ui`
  issue is needed.

## Assumptions

- The mapping boundary (`proto.SpotRisk`, #1836) already guarantees a finite
  risk in [0, 1], non-zero only for DYNAMIC. The aggregation still treats any
  other value as unknown rather than trusting it.
- Risk 0.0 and unset are indistinguishable (proto3), so 0.0 on a DYNAMIC row
  is "unknown".

## Out of Scope

- Using exposure to gate CI or budgets (open question in the epic).
- Savings versus on-demand (#1838) and spot recommendations (#1839).
- Spot exposure of actual-cost output (`GetActualCostResponse` has no
  category), of the `cost estimate` comparison, and of the stack-wide total in
  a diff.

## Success Criteria *(mandatory)*

- **SC-001**: The exposure appears in the `cost projected` table and JSON
  (plain and diff), the overview table and JSON, and the web totals bar.
- **SC-002**: With no DYNAMIC row, text and JSON are unchanged.
- **SC-003**: Table-driven tests cover mixed, all-spot, no-spot,
  all-unknown-risk, some-unknown-risk, zero total, error rows, mixed
  currency, and the replacement case.
- **SC-004**: `make lint`, `make test`, and `go vet ./...` pass.
