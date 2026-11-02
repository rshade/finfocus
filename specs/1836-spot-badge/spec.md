# Feature Specification: Spot Badge

**Feature Branch**: `1836-spot-badge`
**Created**: 2026-11-02
**Status**: Implemented
**Input**: User description: "Show spot (interruptible) pricing and its
interruption risk on every cost surface (GitHub issue #1836, part of epic
367). `CostResult` has no pricing-category or spot-risk field, and only
`cost estimate` reads them, as free-text notes. Carry both as data fields and
render one `SPOT` tag everywhere."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See that a price is spot (Priority: P1)

A person runs `cost projected`, `cost estimate`, or the overview, and a
resource a plugin priced as spot is visibly different from an on-demand one:
it carries a `SPOT` tag with the interruption risk next to it.

**Why this priority**: This is the whole feature. A spot price read as an
on-demand price hides an interruption risk the person has not accepted.

**Independent Test**: Point core at a plugin that answers
`pricing_category = DYNAMIC` with `spot_interruption_risk_score = 0.12` for
one resource and a standard price for another. The first row shows
`SPOT · risk 0.12`; the second row looks exactly as it did before.

**Acceptance Scenarios**:

1. **Given** a projected result with category DYNAMIC and risk 0.12, **When**
   it is rendered by `cost projected`, `cost estimate`, the overview table,
   the overview TUI, or the web dashboard, **Then** `SPOT · risk 0.12`
   appears with that row.
2. **Given** category DYNAMIC and risk 0.0, **When** it is rendered, **Then**
   the tag reads `SPOT · risk unknown`, never `risk 0.00`.
3. **Given** category STANDARD, COMMITTED, OTHER, or UNSPECIFIED, **When** it
   is rendered, **Then** no tag appears and the row is byte-identical to the
   output before this feature.
4. **Given** `NO_COLOR`, `--plain`, or JSON output, **When** a spot row is
   rendered, **Then** the tag text (or the structured fields) is still there.

---

### User Story 2 - Machines read the same data (Priority: P1)

A script or agent reading JSON, NDJSON, or MCP output gets the category and
the risk as data fields, not as text it has to parse.

**Independent Test**: `cost projected --output json` on a DYNAMIC result
contains `pricingCategory: "DYNAMIC"` and `spotInterruptionRisk: 0.12`. A
result with no category contains neither key.

**Acceptance Scenarios**:

1. **Given** a DYNAMIC result, **When** rendered as JSON or NDJSON, **Then**
   `pricingCategory` and `spotInterruptionRisk` are present.
2. **Given** an UNSPECIFIED result, **When** rendered as JSON, **Then** both
   keys are absent (`omitempty`), so existing consumers see no new keys.
3. **Given** a `cost estimate` result, **When** rendered, **Then** the old
   `Pricing:` and `Spot risk:` note strings are gone and the structured
   fields carry the same information.

---

### User Story 3 - A cached result keeps the tag (Priority: P2)

A second run that is served from the projected cache still shows `SPOT`.

**Independent Test**: Run `cost projected` twice against the same plugin. The
second run (cache hit) shows the same tag and the same risk as the first, even
if the plugin would now answer differently.

**Acceptance Scenarios**:

1. **Given** a stored projected result with category DYNAMIC, **When** it is
   read back, **Then** category and risk are unchanged.
2. **Given** a cache entry written before this feature, **When** a lookup
   runs, **Then** it misses (new key suffix), so a spot resource is never
   shown as on-demand from stale data.

### Edge Cases

- Risk is negative or NaN: not a valid spec value. A non-positive score
  renders as `risk unknown`.
- Category DYNAMIC on a row with an error: the error row shows no tag, as it
  shows no price.
- Cluster child rows and other synthetic rows have no category: no tag.
- A plugin that sends a risk score with a non-DYNAMIC category (a spec
  violation): the score is stored but no tag renders.
- The overview table is narrow: the TUI list shows the short tag `SPOT`
  after the amount and widens the Projected column only while a spot row is
  present; the full label is in the detail view.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `CostResult` MUST carry `PricingCategory` (string-typed enum,
  empty when unspecified) and `SpotInterruptionRisk` (float64), both
  `omitempty` JSON data fields. See `contracts/fields.md`.
- **FR-002**: The projected path (`mapProjectedProto`,
  `mapProtoCostResultToEngine`) and the estimate path
  (`estimateResponseToCostResult`) MUST populate them from
  `pricing_category` and `spot_interruption_risk_score`.
- **FR-003**: `engine.SpotDisplay(category, risk)` MUST be the only function
  that builds the label. It returns `SPOT · risk 0.12`, `SPOT · risk unknown`
  (risk <= 0), or `""` for any category other than DYNAMIC.
- **FR-004**: `OverviewRow.ProjectedCost` MUST carry the same two fields, and
  `OverviewRowResult.SpotDisplay` MUST hold the label every overview renderer
  reads (the same discipline as `CalculateRowDelta()`).
- **FR-005**: Every surface MUST render the label from that one value:
  `cost projected` table (Notes column), `cost estimate` table, overview
  table, overview TUI (list tag and detail line), the `cost` TUI list and
  detail, the web Overview (cell and detail panel), and the web Estimate page.
- **FR-006**: Color MUST NOT be the only signal. The TUI and web add a cyan
  accent to text that is already present; `NO_COLOR` and `--plain` render the
  text; JSON carries the fields.
- **FR-007**: The `Pricing:` and `Spot risk:` note strings in
  `estimateResponseToCostResult` MUST be removed.
- **FR-008**: The projected cache key MUST gain a `/pricing-v1` suffix so
  entries written without the fields are not read.
- **FR-009**: Core MUST hold no spot knowledge beyond reading the generic
  spec fields: no instance-family tables, no provider aliases, no on-demand
  equivalent, no savings, no totals.
- **FR-010**: Rows that are not DYNAMIC MUST render unchanged, including TUI
  column widths and existing golden files.

### Key Entities

- **PricingCategory**: `STANDARD`, `COMMITTED`, `DYNAMIC`, `OTHER`, or empty.
  Only DYNAMIC has meaning to core.
- **Spot label**: the display string from `SpotDisplay`.

## Web UI Fit *(mandatory)*

- **Disposition**: Addition to existing views (Overview list and detail,
  Estimate comparison). No new page, endpoint, or query parameter.
- **Web data**: The Overview row already serializes `OverviewRowResult`, so
  the new `spotDisplay` string rides on the existing payloads
  (`/api/overview/query`, `/api/overview/resource`, the SSE snapshot). The
  Estimate payload's `display` gains `baselineSpot` and `modifiedSpot`. The
  label is built in Go by `engine.SpotDisplay`, the same function the CLI and
  TUI call. The browser formats nothing.
- **Layout fit**: In the Overview table the tag is a pill after the amount in
  the Projected cell; in the detail panel it is a `Pricing` row after
  `Projected`; on the Estimate page it follows the `Baseline` or `Modified`
  amount. The pill uses text plus a cyan border and tint
  (`--spot`, `--spot-bg`, `--spot-border`) in light and dark.
- **Additions**: None beyond the pill style.
- **Render evidence**: 2026-11-02, `finfocus --web --no-browser` against
  `examples/plans/aws-simple-plan.json` with a plugin answering DYNAMIC and
  risk 0.12. Overview showed `$21.90` plus a `SPOT · risk 0.12` pill (computed
  color `rgb(0, 82, 94)` on `rgb(216, 244, 248)`, above 7:1); the detail
  panel listed `Pricing: SPOT · risk 0.12`; the Estimate page showed the pill
  after Baseline and Modified. Rows without a category had no pill.
- **Completion**: This is a UX addition to existing views, so no follow-up
  `web-ui` issue is needed: the web surface ships with the feature.

## Assumptions

- Plugins follow the spec rule that a non-zero risk score appears only with
  category DYNAMIC (`costsource.proto`).
- `GetProjectedCostResponse` and `EstimateCostResponse` already carry both
  fields in finfocus-spec v0.7.10; no spec change is needed.

## Out of Scope

- Savings versus on-demand (issue #1838; needs a spec field).
- Spot exposure totals and weighted risk (issue #1837).
- Spot recommendations (#1839) and a spot what-if mode (#1841).
- Actual-cost results: `GetActualCostResponse` has no category field.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A DYNAMIC result shows `SPOT` on all five CLI/TUI/web surfaces.
- **SC-002**: Output for non-DYNAMIC results is byte-identical to before,
  proven by the existing goldens passing unchanged.
- **SC-003**: A cache hit returns the same category and risk that were stored.
- **SC-004**: `make lint`, `make test`, and `go vet ./...` pass.
