# Feature Specification: Spot Savings vs On-Demand

**Feature Branch**: `1838-spot-savings`
**Created**: 2026-11-02
**Status**: Implemented
**Input**: User description: "Show 'Savings vs On-Demand' next to a spot price,
from an on-demand equivalent the plugin supplies (GitHub issue #1838, part of
epic 367). Builds on the `PricingCategory` and `SpotInterruptionRisk` fields
from #1836. The spec field `on_demand_equivalent_monthly_cost` ships in
finfocus-spec v0.7.10."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See how much cheaper spot is (Priority: P1)

A person reads a spot-priced resource in `cost projected`, `cost estimate`, or
the overview and sees the saving against on demand next to the price:
`SPOT · risk 0.12 · -62% vs on-demand`.

**Why this priority**: This is the feature. A spot price alone does not say
whether the interruption risk is worth taking.

**Independent Test**: Point core at a plugin that answers DYNAMIC with a
monthly cost of 38 and an on-demand equivalent of 100. Every surface shows
`-62% vs on-demand`; JSON carries both prices and `spotSavingsPercent: 62`.

**Acceptance Scenarios**:

1. **Given** a DYNAMIC result with an on-demand equivalent, **When** it is
   rendered anywhere, **Then** the spot label ends with `-62% vs on-demand`
   (`+15% vs on-demand` when spot costs more) and JSON carries
   `onDemandEquivalentMonthlyCost` and `spotSavingsPercent`.
2. **Given** a DYNAMIC result with no on-demand equivalent, **When** it is
   rendered, **Then** the label ends with `spot savings unavailable`, never
   `0%`, and JSON omits both fields.
3. **Given** a DYNAMIC result whose on-demand equivalent is 0, **When** it is
   rendered, **Then** nothing is divided, the label reads
   `spot savings unavailable`, and JSON keeps the plugin's 0 but omits
   `spotSavingsPercent`.
4. **Given** a result that is not DYNAMIC but carries the field, **When** it is
   mapped, **Then** the field is ignored: no label, no JSON.
5. **Given** a negative or non-finite on-demand equivalent, **When** it is
   mapped, **Then** it is dropped, so the label reads `spot savings
   unavailable`.

## Requirements *(mandatory)*

- **FR-001**: The on-demand equivalent always comes from the plugin. Core makes
  no second call, estimates nothing, and holds no spot knowledge beyond the
  category DYNAMIC.
- **FR-002**: Savings percent is `(onDemand - spot) / onDemand * 100`, computed
  once by `engine.SpotSavingsPercent`. Positive means spot is cheaper.
- **FR-003**: Presence, not value. The field is read through the generated
  pointer (`*float64`). An explicit 0 is present: it is kept on the result and
  produces no percentage.
- **FR-004**: The field counts only on DYNAMIC results. `proto.OnDemandEquivalent`
  drops it elsewhere and drops negative or non-finite values. Spot cost that is
  negative or non-finite also yields no percentage.
- **FR-005**: One label. `engine.SpotDisplay(category, risk, savingsPercent)`
  builds `SPOT · risk 0.12 · -62% vs on-demand`; every renderer shows that
  string and formats nothing (#1836 FR on a single label). The percentage is
  rounded to a whole number; a saving that rounds to zero reads `0% vs
  on-demand` (a real figure, unlike the unavailable marker).
- **FR-006**: Color is never the only signal: the text is in the label, so
  `NO_COLOR`, `--plain`, JSON, and NDJSON carry it.
- **FR-007**: JSON carries both prices and the percentage on `CostResult`
  (`onDemandEquivalentMonthlyCost`, `spotSavingsPercent`) and on the overview
  row's `projectedCost`. Output for a result with no spot row is unchanged.
- **FR-008**: Cache. The projected cache stores `[]CostResult` as JSON, so the
  new fields round-trip with no extra code. Nothing new is sent to a plugin,
  so the cache key gains no suffix. The new fields join the unreleased
  `/pricing-v1` suffix from #1836.
- **FR-009**: Informational only: no budget, exit code, or gate reads it. Spot
  exposure totals (#1837) are unchanged.

## Key Entities

See `contracts/fields.md`.

## Web UI Fit *(mandatory)*

- **Disposition**: Addition to existing views (Overview table and detail panel,
  Estimate comparison). No new page, endpoint, or query parameter.
- **Web data**: The overview row payload carries `spotDisplay`
  (`OverviewRowResult.SpotDisplay`) and the estimate display carries
  `baselineSpot` and `modifiedSpot` (`viewmodel.EstimateDisplay`). Both are
  built by `engine.SpotDisplay`, the function the CLI calls, so they now end in
  the savings text. The browser formats nothing: `overview.js` and
  `estimate.js` render the strings verbatim in the existing `spot-badge`.
- **Layout fit**: The badge is longer. In the overview table it stays on one
  line beside the amount and widens the Projected column; in the detail panel
  it sits on the `Pricing` row.
- **Not applicable**: The Cost page (actual cost has no pricing category), the
  Forecast page, and Recommendations show no spot label.
- **Render evidence**: see the verification notes in `plan.md`.
- **Completion**: A UX addition to existing views; no follow-up `web-ui` issue
  is needed.

## Assumptions

- The plugin states the on-demand equivalent on the same basis as its monthly
  cost (aws-ce does). Core does not rescale it.
- A DYNAMIC result without the field comes from an older plugin or a missing
  Price List row; the marker says so rather than guessing.

## Out of Scope

- A request-side way to ask a plugin for a pricing category (twin call),
  rejected in the epic.
- Savings in totals (#1837 reports exposure, not savings) and spot
  recommendations (#1839).
- Actual-cost output: `GetActualCostResponse` has no category.

## Success Criteria *(mandatory)*

- **SC-001**: The label shows the saving or the unavailable marker in
  `cost projected` (table, JSON, diff), `cost estimate` (table, JSON, TUI),
  the overview (table, JSON, TUI detail), and the web Overview and Estimate.
- **SC-002**: An on-demand equivalent of 0 never divides and never reads as 0%.
- **SC-003**: Table-driven tests cover present, absent, zero, negative,
  non-finite, non-DYNAMIC, and spot-above-on-demand cases, and the cache
  round trip.
- **SC-004**: `make lint`, `make test`, and `go vet ./...` pass.
