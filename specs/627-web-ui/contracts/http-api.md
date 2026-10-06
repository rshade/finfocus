# HTTP API Contract: Web UI

**Date**: 2026-10-06 | **Feature**: specs/627-web-ui
**Base**: `http://127.0.0.1:<port>` (default port kernel-assigned; the root-local `--port` flag, valid only with `--web`, pins it)

## Authentication & transport rules (FR-016)

- Startup prints `http://127.0.0.1:<port>/?token=<128-bit-hex>`.
- `GET /?token=...` validates the token (constant-time compare), sets
  `Set-Cookie: finfocus_session_<port>=<token>; HttpOnly; SameSite=Strict; Path=/`
  and answers `303` to `/` so the token leaves the address bar and browser
  history. The token is valid until the server stops (not single-use), so a
  second tab or reload with the printed URL works.
- The cookie name carries the port because cookies are not isolated by port:
  a second `finfocus --web` on another port would otherwise overwrite this
  session's cookie.
- Every other request must carry the cookie; missing/invalid → `401`.
- Every request must have `Host` matching `127.0.0.1:<port>` or
  `localhost:<port>`; otherwise → `403` (DNS-rebinding defense).
- `SameSite=Strict` does not separate two localhost ports (same site), so
  every `POST` also requires `Origin` equal to the server's own origin and
  `Content-Type: application/json`; otherwise → `403` / `415`. `GET`
  requests that carry an `Origin` header must match it too.
- Responses carry `Cache-Control: no-store`, `Referrer-Policy: no-referrer`,
  and a `Content-Security-Policy` of `default-src 'self'` (no inline script,
  no third-party origins; the SPA loads nothing from a CDN).
- Server binds to `127.0.0.1` only and has no option to bind elsewhere
  (FR-016a). No TLS (localhost); the cookie carries no `Secure` attribute
  for that reason.
- The token and request bodies (passphrase, estimate overrides) are never
  logged; request logging records method, path without query, and status.
- Payloads are redacted by the shared rules named in FR-018 before they are
  serialized: no Pulumi secret values, no credential-like property names, no
  budget notification destinations.
- JSON field names: types that already carry JSON tags (`CostResult`,
  `Recommendation`, `EstimateResult`, `CrossProviderAggregation`) keep them.
  `engine.OverviewRowResult` and `engine.OverviewTotals` have no JSON tags
  (`internal/engine/overview_result.go`), and the existing overview JSON is
  produced by `engine.RenderOverviewAsJSON` from the full `OverviewRow`
  schema. The overview payload shape is therefore settled in T014a before
  handlers are written (see tasks.md).

## Static assets

| Method | Path | Response |
|--------|------|----------|
| GET | `/` (with valid token) | `index.html` + session cookie |
| GET | `/static/*` | Embedded assets (cookie-authenticated) |

## Overview

| Method | Path | Body / Query | Response |
|--------|------|--------------|----------|
| GET | `/api/overview/stream` | — | SSE stream (see events below) |
| POST | `/api/overview/query` | `OverviewQuery` | `{ rows: OverviewRowResult[], totals: OverviewTotals, page, totalPages }` — filtered/sorted/flattened by `internal/viewmodel` |
| POST | `/api/overview/cluster/toggle` | `{ urn: string, expanded: string[] }` (the client's current expanded set) | Updated flattened page plus the new expanded set (same shape as query). The server keeps no expansion state, so two tabs never interfere |
| POST | `/api/overview/preview` | — | Triggers live `pulumi preview` (TUI `p`-key equivalent); at most one runs at a time (a second call attaches to the running one, `202`); progress streamed on the SSE channel; final message carries updated rows |
| GET | `/api/overview/budget` | — | `BudgetResult` |
| GET | `/api/overview/resource?urn=...` | — | Detail payload: `OverviewRowResult` + property diffs + breakdowns + per-budget status |
| POST | `/api/passphrase` | `{ passphrase: string }` | `202` on accept; hands the value to the loading pipeline, which passes it to the `pulumi` subprocess environment only (never `os.Setenv`, as `internal/pulumi/pulumi.go` does for the TUI); errors surface on SSE |

### SSE events (`/api/overview/stream`)

On connect (including a reconnect) the stream first sends a `snapshot` event
with the phases, rows, and totals gathered so far, so a reload or second tab
recovers the current state without restarting the load.

| event | data | Meaning |
|-------|------|---------|
| `phase` | `{ phase: 1..6, name: string, status: "active"\|"done"\|"error" }` | Loading checklist progress (mirrors TUI phase names) |
| `row` | `OverviewRowResult` | One enriched row (progressive enrichment) |
| `progress` | `{ loaded: n, total: m }` | Aggregate progress |
| `budget` | `BudgetResult` | Budget footer data ready |
| `error` | `{ message: string, phase?: number }` | Recoverable phase/plugin error (partial results continue) |
| `expansion` | `{ rows: OverviewRowResult[], notes: string[] }` | Cluster expansion finished after enrichment; replaces the row set (mirrors the TUI's `OverviewExpansionReadyMsg`) |
| `ready` | `{ totals: OverviewTotals }` | Load complete |
| `passphrase_required` | `{ stack: string }` | Encrypted stack — SPA shows masked prompt. A rejected passphrase emits `error` and then `passphrase_required` again |
| `snapshot` | `{ phases, rows, totals?, budget? }` | State so far, sent first on every connect |
| `preview` | `{ status, elapsedMs, rows? }` | On-demand preview progress/result |

## Cost actual

| Method | Path | Body / Query | Response |
|--------|------|--------------|----------|
| POST | `/api/cost/actual/query` | `CostQuery` (`groupBy`: `""`, `resource`, `type`, `provider`, `daily`, `monthly`; optional `tag`: `key=value`). The group-by selector is a web-only addition beyond the TUI (FR-008a) | `{ results: CostResult[] \| CrossProviderAggregation[], summary, trends, carbon }` — `daily`/`monthly` via `engine.CreateCrossProviderAggregation`, `resource`/`type`/`provider` via `engine.GroupResults`, tag via the CLI's shared tag-filter parser, carbon via unified `engine` aggregation. An unsupported `groupBy` or malformed `tag` → 400 |
| GET | `/api/cost/actual/resource?id=...` | — | Detail: breakdown, sustainability, linked recommendations |

## Recommendations

| Method | Path | Body / Query | Response |
|--------|------|--------------|----------|
| POST | `/api/recommendations/query` | `RecommendationQuery` (`includeDismissed` is a web-only addition beyond the TUI, FR-009a) | `{ items: Recommendation[], summary: { count, savings, byActionType } }` |
| GET | `/api/recommendations/item?id=...` | — | Detail incl. scorer signals |

Dismissal mutations are **not** exposed (the web UI is read-only here; see
spec Assumptions). `includeDismissed` flows through
`eng.GetRecommendationsForResourcesWithDismissed`.

## Estimate

| Method | Path | Body / Query | Response |
|--------|------|--------------|----------|
| GET | `/api/estimate/baseline?urn=...` | — | `{ resource: ResourceDescriptor, result: EstimateResult, pricingModes }` (modes via `eng.DiscoverPricingSpec`) |
| POST | `/api/estimate/recalculate` | `{ urn, overrides: map[string]string, pricingMode? }` | `EstimateResult` — direct `eng.EstimateCost` call, same shape as the TUI callback |

## Errors

All endpoints return `{ error: string, code: string }` with appropriate
HTTP status. Plugin failures mirror TUI behavior: partial results plus
inline error markers rather than hard 500s (spec Edge Cases).
