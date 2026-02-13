# Feature Specification: TUI Detail View Recommendations

**Feature Branch**: `510-tui-detail-recommendations`
**Created**: 2026-02-11
**Status**: Draft
**Input**: User description: "feat(tui): display recommendations in resource detail view for cost projected/actual"
**GitHub Issue**: #575

## Clarifications

### Session 2026-02-12

- Q: How should multiple recommendations be ordered within the RECOMMENDATIONS section? → A: Highest estimated savings first (descending), with reasoning/caveats displayed beneath each recommendation when available (e.g., Graviton migration compatibility warnings).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Recommendations in Projected Cost Detail (Priority: P1)

A user runs `cost projected` in interactive mode, selects a resource row by pressing
Enter, and sees a detail view. When recommendations exist for that resource, they
appear as a dedicated section showing the action type, description, and estimated
savings for each recommendation. The user gets a complete cost picture without
switching to a separate recommendations command.

**Why this priority**: This is the core value proposition. Users currently must run
a separate `cost recommendations` command to see optimization suggestions. Embedding
them in the detail view they already use eliminates context switching and surfaces
actionable insights at the point of decision.

**Independent Test**: Can be fully tested by running `cost projected` with a plan
that produces resources with recommendations, pressing Enter on a resource, and
verifying the recommendations section renders correctly.

**Acceptance Scenarios**:

1. **Given** a resource with one or more recommendations, **When** the user presses
   Enter on that resource row in the projected cost TUI, **Then** the detail view
   displays a "RECOMMENDATIONS" section listing each recommendation with its action
   type (e.g., RIGHTSIZE) and description, ordered by highest estimated savings first.
2. **Given** a recommendation with estimated savings greater than zero, **When** it
   is rendered in the detail view, **Then** the savings amount and currency are
   displayed alongside the recommendation (e.g., "$15.00 USD/mo savings").
3. **Given** a recommendation with zero or no estimated savings, **When** it is
   rendered in the detail view, **Then** only the action type and description are
   shown without a savings indicator.
4. **Given** a recommendation with reasoning/caveats (e.g., "Ensure application
   compatibility with ARM64 architecture"), **When** it is rendered in the detail
   view, **Then** the reasoning lines are displayed beneath the recommendation
   description as indented warnings.

---

### User Story 2 - View Recommendations in Actual Cost Detail (Priority: P1)

A user runs `cost actual` in interactive mode, selects a resource row, and sees the
detail view. When recommendations exist, they appear in the same format and position
as in the projected cost detail view.

**Why this priority**: Actual cost users have the same need for inline recommendations.
The implementation is symmetric with projected cost, and parity between both views
is essential for a consistent user experience.

**Independent Test**: Can be fully tested by running `cost actual` with resources
that have recommendations, pressing Enter on a resource, and verifying the
recommendations section appears.

**Acceptance Scenarios**:

1. **Given** a resource with recommendations in the actual cost view, **When** the
   user opens the detail view, **Then** recommendations appear in the same format
   and section position as in the projected cost detail view.

---

### User Story 3 - Graceful Absence of Recommendations (Priority: P2)

When no recommendations exist for a resource (either because the plugin doesn't
support recommendations or none apply to this resource), the detail view renders
normally without any empty or placeholder recommendations section.

**Why this priority**: Avoiding visual clutter for resources without recommendations
ensures the detail view remains clean and only shows relevant information.

**Independent Test**: Can be fully tested by viewing a resource with no recommendations
and verifying no "RECOMMENDATIONS" header appears in the detail view.

**Acceptance Scenarios**:

1. **Given** a resource with no recommendations, **When** the user opens the detail
   view, **Then** no "RECOMMENDATIONS" section header or content appears.
2. **Given** a resource with an empty recommendations list, **When** the user opens
   the detail view, **Then** the detail view renders all other sections normally
   without a gap or empty section.

---

### User Story 4 - Machine-Readable Output Includes Recommendations (Priority: P3)

When users run `cost projected` or `cost actual` with JSON or NDJSON output format,
the output includes the recommendations field for each resource that has them. This
enables downstream tooling and scripts to consume recommendation data alongside costs.

**Why this priority**: Programmatic consumers benefit from the same data enrichment
without requiring a separate recommendations API call. This is a natural consequence
of populating the existing data field.

**Independent Test**: Can be tested by running cost commands with `--output json` and
verifying the JSON output includes the `recommendations` array for resources that
have recommendations.

**Acceptance Scenarios**:

1. **Given** a resource with recommendations and `--output json` specified, **When**
   the command completes, **Then** the JSON output includes a `recommendations` array
   with action type, description, and savings for each recommendation.
2. **Given** a resource with no recommendations and `--output json` specified, **When**
   the command completes, **Then** the JSON output omits the `recommendations` field
   (not present, not an empty array).

---

### Edge Cases

- What happens when the recommendation fetch fails (network error, plugin crash)?
  The cost display proceeds normally; the failure is logged but not shown to the user.
- What happens when a recommendation has no currency specified? A sensible default
  currency (USD) is used for display purposes.
- What happens when a resource has many recommendations (e.g., 10+)? All are displayed
  in a scrollable detail view without truncation.
- What happens when the recommendation action type is unrecognized? It is displayed
  as-is in the bracket notation (e.g., "[UNKNOWN_TYPE]").
- What happens when a recommendation has no reasoning entries? Only the action type,
  description, and savings are shown; no empty warnings section appears.
- What happens when multiple recommendations have identical savings? They appear in
  plugin-returned order within that savings tier.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The resource detail view in both projected and actual cost TUI MUST
  display a "RECOMMENDATIONS" section when recommendations exist for the selected
  resource.
- **FR-002**: Each recommendation MUST display its action type label (e.g., RIGHTSIZE,
  TERMINATE, DELETE_UNUSED) and human-readable description. When the recommendation
  includes reasoning entries (from the proto `Reasoning` field), they MUST be displayed
  as indented lines beneath the description to surface warnings and caveats (e.g.,
  "Ensure application compatibility with ARM64 before migrating to Graviton").
- **FR-003**: When a recommendation has estimated savings greater than zero, the
  display MUST include the savings amount and currency (e.g., "$15.00 USD/mo savings").
- **FR-004**: When a recommendation has zero or no estimated savings, the display
  MUST show only the action type and description without a savings indicator.
- **FR-005**: The "RECOMMENDATIONS" section MUST NOT appear in the detail view when
  no recommendations exist for the resource.
- **FR-006**: Recommendation data retrieval failures MUST NOT prevent or delay cost
  display; failures are handled silently from the user's perspective.
- **FR-007**: JSON and NDJSON output formats MUST include recommendation data in the
  output when recommendations are present for a resource.
- **FR-008**: The "RECOMMENDATIONS" section MUST appear after the sustainability
  section and before the notes section in the detail view layout.
- **FR-009**: Recommendations within the section MUST be sorted by estimated savings
  in descending order (highest savings first). Recommendations with zero or missing
  savings appear last, in plugin-returned order.
- **FR-010**: The internal `Recommendation` struct MUST be extended with a `Reasoning`
  field (`[]string`) mapped from the proto `Recommendation.Reasoning` repeated field,
  to carry warnings and caveats from plugins through to the TUI rendering layer.

### Key Entities

- **Recommendation**: An optimization suggestion for a cloud resource, characterized
  by an action type (what to do), a description (human-readable explanation),
  optional estimated savings (monetary value and currency), and optional reasoning
  entries (warnings, caveats, or prerequisites for safe implementation).
- **CostResult**: The per-resource cost calculation that already contains a
  recommendations field, which this feature populates during the cost command
  execution flow.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view all available recommendations for any resource directly
  from the cost detail view without running a separate command, reducing the number
  of commands needed from 2 to 1 for a complete cost + recommendation picture.
- **SC-002**: Cost display remains fully functional and unblocked when recommendation
  data is unavailable, with zero user-visible errors from recommendation failures.
- **SC-003**: Each displayed recommendation provides actionable information: what
  action to take, why, and how much it could save (when savings data is available).
- **SC-004**: Both projected and actual cost views present recommendations in an
  identical format and position, providing a consistent user experience across
  cost commands.

## Assumptions

- Recommendations are fetched using the same plugin connections already established
  for cost calculation, requiring no additional plugin configuration.
- The existing recommendation merge logic correctly matches recommendations to
  resources by resource ID.
- Plugins that do not support recommendations return empty results rather than
  errors, which is the existing behavior.
- Currency defaults to "USD" when a recommendation does not specify a currency, which
  is the standard default for the project.
- The detail view is scrollable, so there is no practical limit on the number of
  recommendations that can be displayed.

## Scope Boundaries

### In Scope

- Displaying recommendations in the interactive TUI detail view for both cost
  projected and cost actual commands.
- Including recommendations in JSON/NDJSON output.
- Best-effort recommendation fetching with graceful degradation.

### Out of Scope

- New CLI flags for enabling/disabling recommendation display.
- Recommendation filtering within the detail view (users use `cost recommendations`
  for filtered views).
- Recommendation dismissal from the detail view (handled by existing dismissal
  workflow).
- Changes to the recommendations standalone command or its TUI.
- Performance optimization of recommendation fetching (existing caching is reused).

## Dependencies

- The existing recommendation field on cost results must remain in the data model.
- The existing recommendation merge and fetch engine functions must remain available
  and functional.
- Plugin protocol must continue to support recommendation responses.
