# Quickstart: Web UI Validation Guide

**Date**: 2026-10-06 | **Feature**: specs/627-web-ui

Runnable scenarios proving the feature works end-to-end. Contract details:
[contracts/http-api.md](contracts/http-api.md); payload shapes:
[data-model.md](data-model.md).

## Prerequisites

- Built binary: `make build`
- Fixture plans: `testdata/simple-plan.json` and `test/e2e/fixtures/`
- Test plugins as used by existing e2e suites (see `test/README.md`)
- `--web` is a root-local flag on `finfocus`, run from the Pulumi project
  directory. The override flags below are also root-local and only valid with
  `--web`: `--pulumi-json`, `--pulumi-state`, `--stack`, `--from`, `--to`,
  `--adapter`, `--filter`, plus `--port` and `--no-browser`. They take the same
  values as on `finfocus overview`. Terraform is not supported (spec FR-012a).
- For Scenario 4 only: browsers installed through the playwright-go install
  step (`make test-e2e-web` does it)

## Scenario 1 — Launch and overview parity (US1, SC-001/SC-002)

```sh
bin/finfocus --web --pulumi-json testdata/simple-plan.json --no-browser --port 8484
```

Expected:

- Stdout prints `http://127.0.0.1:8484/?token=<hex>` and keeps serving.
- `curl -b cookiejar -c cookiejar "http://127.0.0.1:8484/?token=<hex>"`
  returns the SPA HTML and sets the session cookie.
- `curl -b cookiejar -N http://127.0.0.1:8484/api/overview/stream` emits
  `phase`, `row`, `progress`, and finally `ready` SSE events.
- `curl -b cookiejar -X POST http://127.0.0.1:8484/api/overview/query -d '{}'`
  returns rows whose resource set, totals, and per-row values equal
  `bin/finfocus overview --pulumi-json testdata/simple-plan.json --output json`
  (parity check — diff the two JSON outputs).
- Unauthenticated: `curl http://127.0.0.1:8484/api/overview/query` → `401`;
  wrong Host header → `403`; a `POST` with a foreign `Origin` → `403`; the
  `?token=` request answers `303` to `/` and a second request with the same
  token still works.
- No secret leaks: a fixture with a Pulumi secret input and a credential-like
  property name produces no secret value or property name in any response.
- Ctrl+C shuts the server down cleanly (FR-015).

### Scenario 1b — Auto-detect and flag rules (US1, FR-001, FR-012)

From a Pulumi project directory (a scratch project on a local backend, or the
demo project):

```sh
cd <pulumi-project-dir> && <repo-root>/bin/finfocus --web --no-browser
```

- With no source flags the session loads the same project, stack, and state
  `finfocus overview` loads from that directory; the printed URL works as in
  Scenario 1.
- `bin/finfocus --port 8484` (no `--web`) → error naming `--web`; the same for
  each of `--no-browser`, `--pulumi-json`, `--pulumi-state`, `--stack`,
  `--from`, `--to`, `--adapter`, `--filter`.
- `bin/finfocus --web --mcp` → error (mutually exclusive).
- `bin/finfocus cost actual --web` and `bin/finfocus overview --web` → unknown
  flag (root-local, not inherited). `bin/finfocus --web --terraform-state x`
  → unknown flag.
- `cd` into a directory with no Pulumi project and run `bin/finfocus --web` →
  the same error `finfocus overview` gives, no server started.
- `go test ./internal/cli/... -run MCP` and `test/integration/mcp_server_test.go`
  pass with `internal/cli/testdata/mcp/tools.golden` unchanged (the root is
  already excluded from MCP; `--web` adds no tool).

## Scenario 2 — Filter, sort, drill-down, clusters (US1/US2, SC-003/SC-004)

With a ≥250-row fixture stack (or generated fixture):

- POST `/api/overview/query` with `{"filter":"aws"}` returns only matching
  rows; `{"sort":"cost"}` matches the TUI cost sort order; pagination caps
  pages at 250 rows.
- POST `/api/overview/cluster/toggle` with a cluster URN expands/collapses
  children identically to the TUI cluster expand key (`e`) (compare against golden-file
  fixtures from `internal/tui`).
- Timing: filter/sort responses return in <1s for a 1,000-resource fixture.

## Scenario 3 — Cost actual, recommendations, estimate (US2/US3)

- POST `/api/cost/actual/query` with each `groupBy` the web UI offers
  (`resource`, `type`, `provider`, `daily`, `monthly`, and `""`) returns the
  result `finfocus cost actual --group-by <value>` gives; `{"tag":"env=prod"}`
  equals `--group-by tag:env=prod`; an unsupported `groupBy` returns `400`.
  The selector is a web-only addition beyond the TUI.
- POST `/api/recommendations/query` with `{"includeDismissed":true}` returns
  the same items as `finfocus cost recommendations --include-dismissed`
  (a web-only toggle beyond the TUI).
- POST `/api/estimate/recalculate` with a property override returns deltas
  equal to `finfocus cost estimate` non-interactive output for the same
  override — the numbers must be identical because both call
  `eng.EstimateCost` (SC-002a).

## Scenario 4 — Browser journeys (Playwright via playwright-go, `e2e_web` tag)

```sh
make test-e2e-web
```

The target builds the binary, installs the chromium browser through the
playwright-go install step (run from `test/e2e`), then runs:

```sh
cd test/e2e && FINFOCUS_BINARY="$(git rev-parse --show-toplevel)/bin/finfocus" \
  go test -tags e2e_web -run TestWebUI -v ./...
```

Without the browser or the binary the tests skip with a message. CI runs the
same target in a dedicated job. The root `go.mod` is unchanged; playwright-go
appears only in `test/e2e/go.mod`.

Covers, in a real browser:

1. Overview loads progressively and shows the table (US1).
2. Keyboard-only navigation: tab to filter, sort, open detail, expand a
   cluster; focus always visible (FR-017).
3. Actual-cost view: change group-by, observe the re-fetch (web-only
   addition); recommendations view: toggle include-dismissed (web-only
   addition).
4. Estimate view: edit a property, commit, observe recalculated delta
   without reload (US3).
5. Encrypted-stack fixture: masked passphrase prompt appears, unlock
   proceeds (FR-014).

## Scenario 5 — TUI regression

`UPDATE_GOLDEN=1` must NOT be needed: after the viewmodel extraction,
`go test ./internal/tui/...` golden files pass unchanged, proving the TUI
renders identically on the shared logic.

## Expected overall outcome

All five scenarios pass; web/TUI/CLI numbers are identical on every fixture
tested (SC-002/SC-002a); no new third-party dependencies appear in the
root `go.mod` (`go mod tidy -diff` clean).
