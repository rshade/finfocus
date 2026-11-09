# Feature Specification: Spot Recommendations from Plugins

**Feature Branch**: `2013-spot-recommendations`
**Created**: 2026-11-08
**Status**: Implemented
**Input**: User description: "Spot recommendations from plugins (GitHub
issue #1839, part of epic #367). A plugin recommends moving an on-demand resource to
spot, with savings and interruption risk, through the existing recommendations
pipeline. Core renders it with the same `SPOT` tag and `risk unknown` rule as
the spot badge (#1836). The spec fields `Recommendation.pricing_category` and
`Recommendation.spot_interruption_risk_score` ship in finfocus-spec v0.7.11."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See a spot recommendation with its risk (Priority: P1)

A person runs `cost recommendations`, opens the recommendations TUI, opens a
resource in the overview, or opens the web Recommendations page. A plugin's
recommendation to move a resource to spot shows its savings and the label
`SPOT · risk 0.12` (or `SPOT · risk unknown`).

**Why this priority**: This is the feature. Without the tag, a move to spot
reads like any other modify recommendation and the interruption risk is lost.

**Independent Test**: Point core at a plugin whose recommendation has
`pricing_category` DYNAMIC, `spot_interruption_risk_score` 0.12, and estimated
savings 62. Every surface shows the savings and `SPOT · risk 0.12`; JSON carries
`pricing_category: "DYNAMIC"` and `spot_interruption_risk: 0.12`.

**Acceptance Scenarios**:

1. **Given** a DYNAMIC recommendation with risk 0.12, **When** it is rendered
   in the CLI table, TUI list and detail, overview detail (TUI and web), and
   web Recommendations table and detail, **Then** each shows
   `SPOT · risk 0.12` next to the existing savings.
2. **Given** a DYNAMIC recommendation with risk 0 (unset), **When** it is
   rendered, **Then** the label reads `SPOT · risk unknown`, and JSON omits
   the risk.
3. **Given** a recommendation that is not DYNAMIC but carries a risk score,
   **When** it is mapped, **Then** the risk is dropped and no label appears.
4. **Given** a risk outside 0-1 or not finite, **When** it is mapped, **Then**
   it is dropped and the label reads `SPOT · risk unknown`.
5. **Given** recommendations with no DYNAMIC entry, **When** they are rendered,
   **Then** output is unchanged from before this feature (no new column, no
   new JSON keys).

### User Story 2 - Dismiss a spot recommendation (Priority: P2)

A person dismisses a spot recommendation and later lists recommendations with
`--include-dismissed`. The dismissed entry still shows as spot.

**Why this priority**: Dismissal must keep working for the new kind of
recommendation, and the dismissed row should not lose its meaning.

**Independent Test**: Dismiss a DYNAMIC recommendation; re-query. The default
query omits it; `--include-dismissed` lists it as `Dismissed`. When the plugin
returns it with `include_dismissed`, it keeps `SPOT · risk 0.12`.

**Acceptance Scenarios**:

1. **Given** a dismissed spot recommendation, **When** recommendations are
   queried again, **Then** it is absent from the default result.
2. **Given** a dismissal record that carries a last-known snapshot of a spot
   recommendation, **When** it is rebuilt (`--include-dismissed`, overview),
   **Then** the rebuilt recommendation keeps its spot label.

The CLI `dismiss` and `snooze` commands do not write a last-known snapshot
today for any recommendation (`DismissRequest.Recommendation` is never set),
so a dismissal the plugin no longer returns lists as `no details available`.
Capturing the snapshot needs a plan load and a recommendation lookup by an id
plugins may re-mint; that is a separate change (see Out of Scope).

### User Story 3 - Scorer sees the pricing target (Priority: P3)

With scoring enabled, the scorer receives the recommendation's pricing
category and risk, so it can rank spot moves. Ranking is the scorer's only
effect.

**Acceptance Scenarios**:

1. **Given** scoring is enabled and the allowlist is empty, **When** a spot
   recommendation is scored, **Then** the request carries `pricing_category`
   and `spot_interruption_risk_score`.
2. **Given** a `field_allowlist` that does not name them, **When** it is
   scored, **Then** both fields are cleared.

### Edge Cases

- A dismissal file written before this feature has no spot fields; it loads
  and the entry shows no label.
- A recommendations cache entry written before this feature lacks the new
  fields; it must not be read (see FR-007).
- Core never reorders recommendations by risk. Sort orders are unchanged.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Core reads `Recommendation.pricing_category` with
  `proto.PricingCategoryLabel` and `spot_interruption_risk_score` with
  `proto.SpotRisk`, the same mapping the projected path uses. Core holds no
  spot knowledge beyond the category DYNAMIC: no action type, provider, or
  resource-type table.
- **FR-002**: `engine.Recommendation` gains `PricingCategory` and
  `SpotInterruptionRisk` (JSON `pricingCategory`, `spotInterruptionRisk`,
  both omitted when empty or zero).
- **FR-003**: One label. `engine.SpotRiskDisplay(category, risk)` builds
  `SPOT · risk 0.12` / `SPOT · risk unknown` / `SPOT · risk <0.01`, and
  `engine.SpotDisplay` is built on it, so the tag and the unknown rule are
  shared. A recommendation's label is `Recommendation.SpotDisplay()`, which
  has no savings segment: the recommendation's own savings column already
  states the saving as an amount. Renderers show that string and format
  nothing.
- **FR-004**: CLI. The table adds a `PRICING` column only when a displayed
  row is spot. JSON and NDJSON items add `pricing_category` and
  `spot_interruption_risk` (omitted when empty or zero).
- **FR-005**: TUI. The list row appends the label after the description; the
  detail view adds a `Pricing:` line. The overview detail adds a `Pricing:`
  line under the recommendation's savings. Recommendations linked into a cost
  detail (`viewmodel.LinkedCostRecommendations`, TUI cost view and web Cost
  detail) append the label after their savings.
- **FR-006**: Dismissal. `Recommendation.ID` stays the dismissal id. The local
  snapshot (`config.LastKnownRecommendation`) records `pricing_category` and
  `spot_interruption_risk` (omitted when empty), and every snapshot-to-
  recommendation conversion restores them through
  `engine.RecommendationFromSnapshot`, which also sets the recommendation id
  on overview rows (it was empty before).
- **FR-007**: Cache. The dismissed ids already feed the recommendations cache
  key, so dismissal needs no change there. The cached `RecommendationsResult`
  JSON gains fields, so the key gains the format suffix `/recs-v1` (after
  `/include-dismissed`, before the profile suffix). A cache entry from an
  earlier release would otherwise show a spot recommendation untagged until
  it expires.
- **FR-008**: Scorer. `scoring.toProto` sends both fields; `field_allowlist`
  accepts `pricing_category` and `spot_interruption_risk_score` and clears
  them when not named. Both are a pricing enum and a probability, never an
  identifier, so identifier pseudonymization does not apply. The score cache
  key hashes the sent message, so no
  separate key change is needed. Risk only annotates: no core sort or filter
  reads it.
- **FR-009**: Color is never the only signal; `NO_COLOR`, plain, JSON, and
  NDJSON carry the text.

### Key Entities

- **Recommendation** (`engine`): adds `PricingCategory`, `SpotInterruptionRisk`.
- **LastKnownRecommendation** (`config`): adds `PricingCategory`,
  `SpotInterruptionRisk`.
- **RecommendationPage** (`viewmodel`): adds `ItemSpot []string`.
- **OverviewDetailRecommendation** (`viewmodel`): adds `SpotDisplay`.
- **CostRecommendation** (`viewmodel`): adds `SpotDisplay`.

## Web UI Fit *(mandatory)*

- **Disposition**: Addition to existing views (Recommendations table and
  detail dialog, Overview detail panel). No new page, endpoint, or query
  parameter.
- **API hooks**: `POST /api/recommendations/query` returns `itemSpot` built
  by `viewmodel.BuildRecommendationPage`, which calls
  `engine.Recommendation.SpotDisplay`, the method the CLI table, TUI, and
  overview call. `GET /api/recommendations/item` adds `spotDisplay` from the
  same method. `GET /api/overview/resource` carries `spotDisplay` on each
  `activeRecommendations` entry (`viewmodel.OverviewDetailRecommendations`),
  and the Cost detail carries it on each linked recommendation
  (`viewmodel.LinkedCostRecommendations`).
- **Layout fit**: The existing `spot-badge` (`static/spot.js`) renders in the
  Action cell of the Recommendations table, as a `Pricing` line in the
  recommendation detail dialog, and after the savings in the Overview detail
  list and the Cost detail's linked recommendations. The browser formats nothing.
- **Completion**: UX-only `web-ui` follow-up #2054 covers layout and copy
  review of the badge on the Recommendations, Overview, and Cost views.

## Assumptions

- The spot logic (which resource is eligible, the savings, the risk) belongs
  to the plugin, aws-ce for AWS. aws-public prices on demand only.
- `action_type` stays MODIFY or MIGRATE as the plugin chooses; the spec has
  no spot action type on purpose.

## Out of Scope

- The plugin side (`finfocus-plugin-aws-ce` emitting the recommendation).
- Ranking by risk in core (decided 2026-11-02: risk only annotates).
- Spot savings percent on recommendations: the savings column already states
  the amount.
- Writing a last-known snapshot from `cost recommendations dismiss` and
  `snooze`, which affects every recommendation type.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A DYNAMIC recommendation shows savings and the spot label in
  `cost recommendations` (table, JSON, NDJSON), the recommendations TUI (list
  and detail), the overview detail (TUI and web), the cost detail's linked
  recommendations (TUI and web), and the web Recommendations page.
- **SC-002**: Risk 0 reads `risk unknown`; a non-DYNAMIC risk never appears.
- **SC-003**: A dismissed spot recommendation is absent from a re-query and is
  listed as `Dismissed` under `--include-dismissed`; a snapshot that carries
  the spot fields rebuilds with its label.
- **SC-004**: `rg` finds no spot, provider, or action-type table added to
  core; tests cover mapping, label, cache key, snapshot round trip, scorer
  allowlist, and each renderer.
- **SC-005**: `make lint`, `make test`, and `go vet ./...` pass.
