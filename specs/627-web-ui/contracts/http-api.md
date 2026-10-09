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
  The overview payload shape was settled in T014a (below): web rows are
  `engine.OverviewRowResult` and totals are `engine.OverviewTotals`, both now
  carrying JSON tags in `internal/engine/overview_result.go`. The CLI's
  `engine.RenderOverviewAsJSON` envelope (`OverviewJSONOutput`:
  metadata/resources/summary/budgets/errors, with `resources` marshaled from
  each row's `Source OverviewRow`) is unchanged and remains the parity
  reference: web rows carry the same values as the CLI's `resources` entries
  (each row's `source` field is byte-identical to the CLI `resources` entry),
  plus the server-computed display fields.

### Settled overview row shape (T014a)

`engine.OverviewRowResult` serializes with these JSON names (omitempty as
tagged): `urn`, `type`, `displayName`, `status` (string via
`ResourceStatus.MarshalJSON`), `parentUrn`, `childUrns`, `expansionSource`,
`liveChildName`, `actualMtd`, `projected`, `delta`, `driftPct`,
`driftWarning`, `resourceDisplay`, `statusDisplay`, `actualDisplay`,
`projectedDisplay`, `deltaDisplay`, `driftDisplay`, `recsDisplay`,
`activeRecs`, `dismissedRecs`, `hasError`, `propertyDiffs`,
`recommendations`, `warnings`, `actualCost`, `projectedCost`, `costDrift`,
`error`, `source` (the full `OverviewRow` schema).
`baselineProjectedCost` is internal to delta math and is never serialized
(`json:"-"`, matching `OverviewRow`).

`engine.OverviewTotals` serializes as: `totalActual`, `totalProjected`,
`totalDelta`, `totalSavings`, `currency`, `errors`, `mixedCurrencies`. The
CLI `summary` object keeps its existing names (`totalActualMTD`,
`projectedMonthly`, `projectedDelta`, `potentialSavings`, `currency`) — same
values, CLI-specific names, unchanged for back-compat.

Overview responses also include `totalActualDisplay`, `totalProjectedDisplay`,
`totalDeltaDisplay`, and `totalSavingsDisplay`, formatted by the shared engine
helpers for direct browser rendering.

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
| GET | `/api/overview/budget` | — | `{ budgets: BudgetHealthResult[], display: BudgetDisplay }` — the shared CLI health schema, without notification destinations |
| GET | `/api/overview/resource?urn=...` | — | `{ row: OverviewRowResult, display: OverviewDetailDisplay, budgets: BudgetHealthResult[], budgetDisplay: BudgetDisplay, activeRecommendations: [{ recommendation, savingsDisplay }] }` — property diffs and breakdowns are in the row; active recommendations and savings use the shared TUI detail selector |
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
| `budget` | `{ budgets: BudgetHealthResult[], display: BudgetDisplay }` | Budget footer data ready; notification destinations omitted |
| `error` | `{ message: string, phase?: number }` | Recoverable phase/plugin error (partial results continue) |
| `expansion` | `{ rows: OverviewRowResult[], notes: string[] }` | Cluster expansion finished after enrichment; replaces the row set (mirrors the TUI's `OverviewExpansionReadyMsg`) |
| `ready` | `{ totals: OverviewTotals }` | Load complete |
| `passphrase_required` | `{ stack: string }` | Encrypted stack — SPA shows masked prompt. A rejected passphrase emits `error` and then `passphrase_required` again |
| `snapshot` | `{ phases, rows, totals, budget, progress, ready, passphraseRequired, stack, preview, errors }` | State so far, including safe phase failures; sent first on every connect and when unlock handoff starts |
| `preview` | `{ status, elapsedMs, rows? }` | On-demand preview progress/result |

## Cost actual

| Method | Path | Body / Query | Response |
|--------|------|--------------|----------|
| POST | `/api/cost/actual/query` | `CostQuery` (`groupBy`: `""`, `resource`, `type`, `provider`, `daily`, `monthly`; optional `tag`: `key=value`). The group-by selector is a web-only addition beyond the TUI (FR-008a) | `{ results: CostResult[] \| CrossProviderAggregation[], summary, trends, carbon }` — `daily`/`monthly` via `engine.CreateCrossProviderAggregation`, `resource`/`type`/`provider` via `engine.GroupResults`, tag via the CLI's shared tag-filter parser, carbon via unified `engine` aggregation. An unsupported `groupBy` or malformed `tag` → 400 |
| GET | `/api/cost/actual/resource?id=...` | Optional `type`, `groupBy`, `tag` preserve the clicked row identity and grouping context | `{ result: CostResult, costDisplay, breakdown: [{name, costDisplay}], sustainability, recommendations: [{recommendation, savingsDisplay}] } |

## Recommendations

| Method | Path | Body / Query | Response |
|--------|------|--------------|----------|
| POST | `/api/recommendations/query` | `RecommendationQuery` (`includeDismissed` is a web-only addition beyond the TUI, FR-009a) | `{ items: Recommendation[], summary: RecommendationSummary, itemKeys, itemSavings, itemActions, savingsDisplay, actions, scoring?, errors, page, totalPages }` |
| GET | `/api/recommendations/item?id=...` | Optional server-provided `key` disambiguates IDless recommendations | `{ item: Recommendation, savingsDisplay, actionDisplay, scoreDisplay }`, including all scorer signals |

Dismissal mutations are **not** exposed (the web UI is read-only here; see
spec Assumptions). `includeDismissed` flows through
`eng.GetRecommendationsForResourcesWithDismissed`.

### Settled actual-cost and recommendation display fields (T029-T032)

Both queries accept a one-based `page` (default 1), clamp it to the available
pages, and return at most 250 domain items and display rows. Actual-cost sort
names for resource rows are `cost`, `name`, `type`, `delta`; recommendation sort names are
`savings`, `resource`, `action` (`name` and `type` are accepted aliases).
Substring filtering and sorting use the same viewmodel functions as the TUI.
Time aggregates support `cost` (total descending, period ascending for ties)
and `name` (period ascending). The browser labels `name` as Period and disables
Type and Delta for daily/monthly groups, which have neither field; requesting
either unsupported sort for a time grouping returns 400. Sorting the final
aggregates precedes pagination. Empty sources or filters return 200 with empty
arrays and zero totals for every grouping.

Actual-cost `results` is the domain array returned by the shared CLI fetch for
resource groupings, or `CrossProviderAggregation[]` for daily/monthly. The
legacy `date` grouping remains accepted for CLI compatibility and is never
offered in the browser selector. Additive `rows` contain `id`, `type`,
`costDisplay`, `currency`, `trend`, `trendPoints`, and `canDetail`.
`trendPoints` contains server-computed SVG coordinates derived from the shared
`history.TableTrends` output. Time aggregate rows do not offer resource detail.
Resource groups with equal sizes can share a synthetic ID, so the browser sends
both `id` and `type`, together with `groupBy` and `tag`, for stateless detail.

The actual-cost summary preserves `engine.AggregateResults` fields, including
its full filtered `resources` array, and adds `summary.carbon`. The same
`engine.AggregateSustainability` value is also exposed as top-level `carbon`.
`totalDisplay` uses the shared actual-cost total; `carbonDisplay`, `trends`,
`totalTrend`, `page`, and `totalPages` are additive display fields. Partial
failures appear in `errors` with resource/plugin identity and safe operation
labels; successful results continue to render.

Recommendation `summary` preserves the CLI JSON/NDJSON schema exactly:
`total_count`, `total_savings`, `currency`, `count_by_action_type`, and
`savings_by_action_type`. `itemSavings` and ordered `actions` carry formatted
savings, and `itemKeys` carry stable interaction identities.
Savings displays use the shared TUI dollar prefix, two decimal places and
currency code; an empty code defaults to USD. The CLI summary schema is unchanged.
An IDless recommendation key derives from its resource, action, description, source and
category; it never changes the domain recommendation ID. The detail request
sends that server-provided key so two actions for the same resource remain
independent. The domain item includes the full `scores` object.

`errors` preserves recommendation plugin identity while replacing raw failure
text with a safe operation label. A session still loading its engine answers
`503` / `not_ready`; these views retry on the overview ready/snapshot events.
All endpoints remain authenticated and use the shared redacted JSON writer.

### Shared detail and summary projections

The original domain objects and CLI JSON schemas are unchanged. The additive
presentation fields below are built in `internal/viewmodel` and consumed by both
the TUI and browser; JavaScript renders their financial strings verbatim.

- Overview totals include `unavailableReason` when `mixedCurrencies` is true.
  All four aggregate display strings are then empty and their numeric transport
  slots are zero placeholders, never the engine's partial accumulation. Resource
  rows remain available. REST queries and SSE `snapshot`/`ready` use this same
  guard; the browser shows the unavailable reason instead of aggregate money.
- Overview detail `display` contains ordered `actualBreakdown` and
  `projectedBreakdown` rows (`name`, `costDisplay`) and ordered `impact`/`drift`
  fields (`name`, `value`). They preserve the TUI's two-decimal overview money,
  current/baseline and after-change amounts, extrapolation, delta and percentage.
- Budget `display` contains `healthDisplay`, `amountsDisplay`, `footerDisplay`
  and `details`. Each detail has `name`, `healthDisplay`, `currentSpendDisplay`,
  `limitDisplay`, `forecastedDisplay`, `utilizationDisplay` and triggered
  `thresholds` (`text`, `critical`). Footer percentages use zero decimals;
  detail utilization uses one. Disabled budgets do not contribute to footer
  amounts; mixed-currency budgets show aggregate health only. These projections
  contain no notification destinations or raw budget protos.
- Actual-cost `summaryDisplay` adds `resourceCount`, `recommendationCount`,
  provider `name`/`costDisplay`/`shareDisplay`, `totalDisplay` and
  `carbonEquivalency`. It preserves the existing TUI rule of using positive
  `TotalCost`, otherwise `Monthly`, without changing canonical `summary` values.
  Time rows add `providersDisplay` using the TUI's sorted provider subtotals.
- Actual detail adds `provider`, `periodDisplay`, `monthlyDisplay`,
  `hourlyDisplay`, `deltaDisplay`, `notesDisplay`, ordered `breakdown`,
  `sustainabilityDisplay` and linked `recommendations`. Actual breakdowns retain
  four decimals; sustainability retains two. Errors take precedence over Notes,
  and recommendation savings ordering and reasoning match the TUI.
- Recommendations retain raw action enums for identity and filtering.
  `itemActions`, each action subtotal's `actionDisplay`, and detail
  `actionDisplay` use the common human-readable action label. Unknown enums
  remain unchanged. `scoreDisplay` has ordered `name`/`value` fields with two
  decimal places, `-` for missing signals, and scorer annotations.

## Estimate

| Method | Path | Body / Query | Response |
|--------|------|--------------|----------|
| GET | `/api/estimate/resources` | — | `{ resources: ResourceDescriptor[] }`, real source descriptors with current projected properties |
| GET | `/api/estimate/baseline?urn=...` | optional `pricingMode` | `{ resource, result: EstimateResult, pricingModes, pricingMode, display }` |
| POST | `/api/estimate/recalculate` | `{ urn, overrides: map[string]string, pricingMode? }` | `EstimateResult` fields plus `{ pricingMode, display }` |

`display` contains server-formatted `baseline`, `modified`, `change`, `arrow`,
and `properties: [{ key, originalValue, currentValue, delta }]`. The browser
renders these strings directly; cost arithmetic and formatting are shared with
the estimate TUI. Properties and property deltas follow the shared secret and
credential redaction rules before reaching any browser.

The resource list comes from the session's underlying CLI plan/state source,
never synthetic cluster display rows. Auto-detected sessions include real created
resources from the initial preview, using the canonical ingestion descriptor
mapping before cluster expansion. Matching resource IDs also take the initial
preview descriptor, including changed references and provider/region metadata.
Actual-cost and recommendation queries retain
the immutable deployed snapshot, including a valid empty first-deploy state.
Later on-demand previews retain the existing shared row membership.
Current preview properties override the
source descriptor by URN. A missing source/engine returns `503`; an unknown URN
returns `404`. Recalculation requires at least one override and passes the
original map unchanged as `engine.EstimateRequest.PropertyOverrides`.

`pricingModes` comes from `eng.DiscoverPricingSpec`: each entry carries an
opaque `id`, server-rendered `label`, `rate`, and `detailsDisplay` (tiers,
assumptions and usage hints), plus the full `details` metadata.
An empty selector retains automatic provider behavior. Explicit selectors are
validated in the shared engine against the resource's discovered provider and
billing-mode pair; unknown selectors return `400` / `invalid_pricing_mode`.
The pinned plugin protocol has no mode field: each selectable mode identifies
its supplying provider. Both baseline and modified costs use that provider's
`EstimateCost` RPC. Only an unimplemented RPC falls back to that same provider's
projected-cost RPC, bypassing the projected cache whose keys omit provider
identity. Advisory price options never replace the primary estimate.

`eng.EstimateBaseline` shares this RPC/fallback path while preserving the public
`EstimateCost` requirement for nonempty overrides. The TUI uses the same
provider-aware request and baseline helper; the plain CLI can select that
provider with its existing `--adapter` option. Provider failures return a safe
`502` / `estimate_unavailable` error; the browser retains the editor with an
inline error and does not claim recalculation succeeded. Requests inherit both
browser cancellation and session cancellation, and session shutdown joins them
before plugins close.

## Errors

All endpoints return `{ error: string, code: string }` with appropriate
HTTP status. Plugin failures mirror TUI behavior: partial results plus
inline error markers rather than hard 500s (spec Edge Cases).
