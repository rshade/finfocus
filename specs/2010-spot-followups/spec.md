# Feature Specification: Spot Follow-ups

**Feature Branch**: `feat/spot-followups`
**Created**: 2026-11-02
**Status**: Implemented
**Input**: Three follow-ups left by the merged spot work (#1836 badge, #1837
exposure totals, #1838 savings), listed in `specs/1836-spot-badge/spec.md`
(Out of Scope) and found in #1836's review.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Analyzer diagnostics mark spot resources (Priority: P1)

A person running `pulumi preview` with the FinFocus analyzer sees the spot
label (`SPOT · risk 0.12 · -62% vs on-demand`) in a resource's cost
diagnostic when the plugin priced it DYNAMIC. Diagnostics for every other
resource are unchanged byte for byte.

**Acceptance**:

1. A DYNAMIC `CostResult` yields a diagnostic message that contains
   `CostResult.SpotDisplay()`, the one label builder.
2. An empty, STANDARD, COMMITTED, or OTHER category yields the same message as
   before this change.

### User Story 2 - `cost projected` keeps the SPOT tag on large costs (Priority: P2)

The Cost column of the projected TUI table was 15 characters wide, so
`$1234567.89 SPOT` lost its tag. The column now widens to the longest cell
when a cell exceeds 15 characters. Other columns keep their widths, and a
table with no long cell is unchanged.

### User Story 3 - One web spot badge helper (Priority: P3)

`spotBadge` lived in both `overview.js` and `estimate.js`. It now lives in
`static/spot.js` and both views import it. The rendered DOM is unchanged.

## Requirements

- **FR-001**: The analyzer adds no formatting or spot knowledge: it calls
  `CostResult.SpotDisplay()`. The category, risk, and on-demand fields already
  reach it, because `Server.Analyze` prices through
  `Engine.GetProjectedCost`, which maps plugin results with
  `mapProtoCostResultToEngine`.
- **FR-002**: The label follows the cost line, after a ` | ` separator and
  before sustainability metrics.
- **FR-003**: The Cost column width is `max(15, longest cell)`.
- **FR-004**: The web helper keeps the same element, class, and text.

## Web UI Fit *(mandatory)*

- **Disposition**: UX-only refactor of existing views. Same Go output, no new
  endpoint, no new payload field.
- **Web data**: Unchanged. `spotDisplay`, `baselineSpot`, and `modifiedSpot`
  are still built by `engine.SpotDisplay`.
- **Layout fit**: Unchanged; the badge DOM is identical.
- **Not applicable**: Stories 1 and 2 are analyzer and terminal surfaces with
  no web counterpart.
- **Completion**: No follow-up `web-ui` issue.

## Out of Scope

- Any new spot formatting. `engine.SpotDisplay` stays the only builder.
