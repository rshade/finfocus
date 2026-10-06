# Data Model: Web UI

**Date**: 2026-10-06 | **Feature**: specs/627-web-ui

The web UI introduces no new domain entities. All payloads are JSON
serializations of existing engine types (FR-011a). Only one new runtime
entity exists — the web session — and it is in-memory only.

## WebSession (new, in-memory, `internal/webui`)

| Field | Type | Notes |
|-------|------|-------|
| Token | string | 128-bit `crypto/rand` hex, generated at startup (FR-016) |
| Source flags | struct | The root-local flags given with `--web` (`--pulumi-json`, `--pulumi-state`, `--stack`, `--from`, `--to`, `--adapter`, `--filter`) — captured at launch (FR-012). Group-by and include-dismissed are per-request query fields, not session state |
| WorkingDir | string | cwd used for source auto-detection (`detectPulumiProject` searches from `.`, as `finfocus overview` does); `--project-dir` does not change it |
| Engine | *engine.Engine | The shared engine instance (cache/history/dismissal store wired exactly as the CLI does) |
| Overview pipeline state | channels/callbacks | Drives SSE enrichment events; keeps the latest phases, rows, and totals for the `snapshot` event |
| Preview in flight | single-flight guard | At most one on-demand `pulumi preview` at a time |
| Estimate baseline | *engine.EstimateResult + engine.ResourceDescriptor | For what-if recalculation |

Lifecycle: created on `finfocus --web` launch → destroyed on interrupt
(FR-015); a closed browser tab does not end it. Delete-safe: nothing persists; no store corruption possible.

## Existing entities exposed over HTTP (unchanged meaning)

| Entity | Go type | Used by view |
|--------|---------|--------------|
| Overview row | `engine.OverviewRowResult` (`internal/engine/overview_result.go:14`; no JSON tags, see contract) | Overview list/detail |
| Overview totals/result | `engine.OverviewResult` (`overview_result.go:83`) | Overview summary/footer |
| Budget status | `engine.BudgetResult` (`internal/engine/budget.go:41`) | Overview footer + detail |
| Cost result | `engine.CostResult` (`internal/engine/types.go:483`) | Cost actual list/detail |
| Time aggregation | `engine.CrossProviderAggregation` (`types.go:618`) | Cost actual daily/monthly |
| Recommendation | `engine.Recommendation` (`types.go:147`) | Recommendations list/detail |
| Estimate | `engine.EstimateRequest`/`EstimateResult` (`types.go:893,842`) | Estimate view |
| Dismissal record | `config.DismissalRecord` (`internal/config/dismissed.go:47`) | Read for include-dismissed; no writes from web (read-only) |
| Trend sparkline data | `[]float64` via `internal/history` | Cost actual trend column (rendered as inline SVG server-side or numeric series — decided at tasks; computation stays in Go) |

## New viewmodel types (`internal/viewmodel`)

Thin query/parameter types only — they carry user interaction state, never
computed numbers:

- `OverviewQuery{ Filter string; Sort SortField; Page int; Expanded []string }`
- `CostQuery{ Filter string; Sort SortField; GroupBy engine.GroupBy; Tag string }` —
  `GroupBy` is one of `""`, `resource`, `type`, `provider`, `daily`, `monthly`
  (the web-only selector, FR-008a; the shared validator also accepts the
  deprecated `date`, which the UI does not offer); `Tag` is a `key=value`
  filter run through the same parser as the CLI's `tag:key=value`
- `RecommendationQuery{ Filter string; Sort RecommendationSortField; IncludeDismissed bool }` —
  `IncludeDismissed` is the web-only toggle (FR-009a)
- `SortField` / `RecommendationSortField` — extracted verbatim from
  `internal/tui/cost_model.go:67-78` and `recommendations_model.go:18-27`.

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
