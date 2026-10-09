# Data Model: Web UI

**Date**: 2026-10-06 | **Feature**: specs/627-web-ui

The web UI introduces no new domain entities. All payloads are JSON
serializations of existing engine types (FR-011a). Only one new runtime
entity exists — the web session — and it is in-memory only.

## Session (new, in-memory, `internal/webui`)

| Field | Type | Notes |
|-------|------|-------|
| Server token | string | Owned by `webui.Server`, separate from Session; 128-bit `crypto/rand` hex generated at startup (FR-016) |
| CLI source flags | struct | The root-local flags given with `--web` (`--pulumi-json`, `--pulumi-state`, `--stack`, `--from`, `--to`, `--adapter`, `--filter`) — captured at launch (FR-012). Group-by and include-dismissed are per-request query fields, not session state |
| CLI working directory | string | cwd used for source auto-detection (`detectPulumiProject` searches from `.`, as `finfocus overview` does); `--project-dir` does not change it |
| Engine | *engine.Engine | The shared engine instance (cache/history/dismissal store wired exactly as the CLI does) |
| Overview pipeline state | channels/callbacks | Drives SSE enrichment events; keeps the latest phases, rows, and totals for the `snapshot` event |
| Preview in flight | single-flight guard | At most one on-demand `pulumi preview` at a time |
| SessionOptions | callbacks + DayOfMonth | CLI supplies EstimateResources, ActualCosts, Recommendations, Trends, Preview and SubmitPassphrase; Session stores neither a passphrase nor a mutable estimate baseline |

Lifecycle: created on `finfocus --web` launch → destroyed on interrupt
(FR-015); a closed browser tab does not end it. Session state is discarded; existing optional CLI cache/history stores retain their normal behavior.

## Existing entities exposed over HTTP (unchanged meaning)

| Entity | Go type | Used by view |
|--------|---------|--------------|
| Overview row | `engine.OverviewRowResult` (settled JSON tags in `internal/engine/overview_result.go`; `source` carries the full shared CLI `engine.OverviewRow` schema) | Overview list/detail |
| Overview totals/result | `engine.OverviewTotals` within the shared `engine.OverviewResult` | Overview summary/footer |
| Budget status | `[]engine.BudgetHealthResult`, computed from `engine.BudgetResult` | Overview footer + detail; notification destinations excluded |
| Cost result | `engine.CostResult` (`internal/engine/types.go:483`) | Cost actual list/detail |
| Time aggregation | `engine.CrossProviderAggregation` (`types.go:618`) | Cost actual daily/monthly |
| Recommendation | `engine.Recommendation` (`types.go:147`) | Recommendations list/detail |
| Estimate | `engine.EstimateRequest`/`EstimateResult` (`types.go:893,842`) | Estimate view |
| Dismissal record | `config.DismissalRecord` (`internal/config/dismissed.go:47`) | Read for include-dismissed; no writes from web (read-only) |
| Trend sparkline data | `[]float64` via `internal/history` | Cost actual trend column; shared Go helpers supply trend text and SVG points |

## Query and display types

Transport query structs live in `internal/webui`; shared filtering, sorting and
display structs live in `internal/viewmodel`. Display payloads carry engine
results and Go-formatted values, with no independent browser cost calculation:

- `OverviewQuery{ Filter string; Sort string; Page int; Expanded []string }`
- `CostQuery{ Filter string; Sort string; GroupBy string; Tag string; Page int }` —
  `GroupBy` is one of `""`, `resource`, `type`, `provider`, `daily`, `monthly`
  (the web-only selector, FR-008a; the shared validator also accepts the
  deprecated `date`, which the UI does not offer); `Tag` is a `key=value`
  filter run through the same parser as the CLI's `tag:key=value`
- `RecommendationQuery{ Filter string; Sort string; IncludeDismissed bool; Page int }` —
  `IncludeDismissed` is the web-only toggle (FR-009a)
- `SortField` / `RecommendationSortField` — shared enums in
  `internal/viewmodel`, consumed by TUI and transport handlers.

## Shared presentation projections

`CostSummaryDisplay`, `BudgetDisplay`, `OverviewDetailDisplay`, ordered
`DisplayField` values and `CostBreakdown` rows preserve existing TUI presentation
rules in Go. They are additive to the canonical resource, recommendation and
summary JSON objects. Both UIs consume the same money, percentage, scorer,
sustainability and provider-subtotal projections. Budget projections include
only safe amounts, status and triggered-threshold text.

`OverviewTotalsPayload` uses the engine's `MixedCurrencies` validity flag. When
set, monetary display strings are empty, numeric transport totals are zero
placeholders and `unavailableReason` explains why totals cannot be shown. Rows
remain available through REST and the initial/ready SSE state.

The CLI session owns separate cloned deployed and initial-preview descriptors.
Initial preview creations join estimate choices through canonical ingestion;
synthetic cluster children never enter these source descriptors. Actual-cost
and recommendation queries retain the deployed snapshot, including empty state.

## Estimate transport

`GET /api/estimate/resources` returns redacted `engine.ResourceDescriptor` entries.
`GET /api/estimate/baseline?urn=...&pricingMode=...` returns `resource`, `result`,
`pricingModes`, `pricingMode`, and shared `display`. Each discovered mode includes
an opaque `id`, label, rate, typed pricing details and formatted details.

`POST /api/estimate/recalculate` accepts `urn`, exact string-valued `overrides`,
and optional `pricingMode`. It returns the embedded `engine.EstimateResult` with
`display` and `pricingMode`. Mode IDs select the actual discovered plugin and
billing mode through the shared engine; unknown IDs are rejected. Estimates are
computed per request, without modifying the session's source descriptors.

## Validation rules

- All query inputs are validated server-side against the same rules the TUI
  applies (unknown sort field → 400; invalid group-by → 400).
- Passphrase submission (FR-014) is a single POST held in memory only for
  the stack-load call and passed to the `pulumi` subprocess environment
  only; never stored, never logged, never echoed.
- Every payload goes through the shared redaction rules (FR-018) before it
  is serialized; the estimate view's resource properties use the same rule
  as `engine.BuildAttributes` / `redactedProperties`.
- Estimate property overrides are passed through to
  `engine.EstimateRequest.PropertyOverrides` unchanged — validation and
  recalculation are engine-owned.

## State transitions

- **Overview loading**: `initializing → loading (phase 1..6) → enriching
  (per-row) → ready | error` — mirrors `ViewState` in
  `internal/tui/cost_model.go:48-64`; streamed as SSE events.
- **Cluster expansion**: `collapsed ↔ expanded` per cluster URN. The client
  holds the expanded set (as the TUI model holds `expanded`) and sends it with
  each query; the server recomputes flattened rows via the extracted
  `viewmodel` function and keeps no expansion state, so tabs stay independent.
- **Estimate**: `baseline → editing → recalculating → showing delta` —
  mirrors `estimate_model.go` states.
