# Implementation Plan: Spot Recommendations from Plugins

**Branch**: `2013-spot-recommendations` | **Date**: 2026-11-08 |
**Spec**: [spec.md](spec.md)

## Summary

Map the two recommendation spot fields from finfocus-spec v0.7.11 onto
`engine.Recommendation`, build one recommendation label from the shared spot
label pieces, and render it on every recommendations surface. Carry the fields
through the dismissal snapshot and the scorer request, and version the
recommendations cache key because its stored JSON changes.

## Technical Context

- Go 1.27.3, finfocus-spec v0.7.11 (already pinned; no `go.mod` change).
- Storage: recommendations cache (Bolt, JSON of `RecommendationsResult`),
  `dismissed.json` (`config.DismissalStore`).
- Testing: testify; TUI golden files; node `frontendtest` harnesses.

## Constitution Check

- Plugin domain knowledge stays in the plugin: core reads the category and a
  plugin-supplied score through the existing `proto.PricingCategoryLabel` and
  `proto.SpotRisk`, and adds no spot, provider, or action-type table. PASS.
- Principle VII: see `Web UI Fit` in `spec.md`; the web reads strings built by
  the same Go method as the CLI. PASS.
- No TODOs or stubs; tests first. PASS.

## Design Decisions

- **Label without savings.** `SpotDisplay` for cost results ends with a
  percent-vs-on-demand segment. A recommendation states its saving as an
  amount in its own column, so its label stops after the risk.
  `SpotRiskDisplay` is the shared prefix; `SpotDisplay` appends to it.
- **Separate CLI column, shown only when needed.** It follows the existing
  `STATUS` and score columns, so output without a spot row is byte-identical.
- **Cache suffix `/recs-v1`.** Dismissed ids are already hashed into the key,
  which answers the issue's dismissal question. The stored format is what
  changes; the repository rule for stored cache JSON (`/pricing-v1`) applies.
- **Scorer.** The fields join the scorer request and the allowlist names. The
  score cache key is a hash of the sent message, so it changes by itself.
- **Scorer enum by name.** `scoring.toProto` maps every enum back from its
  name through `enumValue` and the generated `_value` map; the pricing
  category follows that same pattern rather than adding a reverse helper.
- **Recorder mock unchanged.** Its responses are random; a fixed spot entry
  would change other tests' expectations. Tests use an in-process fake
  client, as the existing recommendation tests do.

## Source Code

```text
internal/proto/adapter.go                 Recommendation fields, mapping
internal/engine/spot.go                   SpotRiskDisplay, Recommendation.SpotDisplay
internal/engine/types.go                  Recommendation fields
internal/engine/engine.go                 convert, snapshot, cache key suffix
internal/engine/overview_dismissal.go     snapshot -> recommendation
internal/config/dismissed.go              LastKnownRecommendation fields
internal/config/scoring.go                allowlist names
internal/scoring/convert.go               toProto, applyAllowlist
internal/cli/cost_recommendations.go      table column, JSON, snapshot
internal/tui/recommendations_model.go     list suffix, detail line
internal/tui/overview_view.go             overview detail line
internal/viewmodel/overview_detail.go     SpotDisplay on detail recommendation
internal/viewmodel/recommendation_query.go ItemSpot
internal/webui/handlers_recommendations.go item spotDisplay
internal/webui/static/views/recommendations.js
internal/webui/static/views/overview.js
docs/src/content/docs/guides/user-guide.md
```

## Complexity Tracking

None.

## Verification

2026-11-08, `make build` binary, isolated `FINFOCUS_HOME`, and a throwaway
plugin (not committed) built against finfocus-spec v0.7.11 that returns, per
resource, a DYNAMIC recommendation with risk 0.12, a DYNAMIC one with no risk,
and a RIGHTSIZE one that wrongly carries risk 0.5:

- `cost recommendations --verbose`: `PRICING` column with
  `SPOT · risk 0.12` and `SPOT · risk unknown`; the RIGHTSIZE rows are blank.
- `--output json` / `ndjson`: `pricing_category: "DYNAMIC"`, and
  `spot_interruption_risk: 0.12` only on the known-risk rows; the RIGHTSIZE
  risk is dropped.
- `dismiss <id>` then a default re-query omits it; `--include-dismissed` lists
  it as `Dismissed`. The CLI writes no last-known snapshot (see spec US2).

The web render check is recorded in `tasks.md` T013.
