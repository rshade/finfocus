# Research: Web UI (`finfocus --web`)

**Date**: 2026-10-06 | **Feature**: specs/627-web-ui

## Decision 1: Shared logic layer — new `internal/viewmodel` package

- **Decision**: Extract all presentation-neutral view logic (filter, sort,
  cluster display flattening/pagination, carbon aggregation) into a new
  `internal/viewmodel` package operating on existing `engine` types. Both
  the bubbletea models and the web server call these functions.
- **Rationale**: FR-011a/b forbid parallel logic. Research confirmed the
  TUI models currently own this logic as methods
  (`internal/tui/overview_model.go:920,713`, `internal/tui/cost_model.go:326,351`,
  `internal/tui/recommendations_model.go:453,482`); the functions are pure
  loops over engine types and trivially extractable. Engine stays focused
  on data production; viewmodel shapes data for display.
- **Alternatives considered**: Put functions in `internal/engine` — rejected
  because filter/sort/display-flatten are presentation concerns, not cost
  computation; engine already has a clean data-production boundary. Duplicate
  in JS — rejected (explicitly forbidden by FR-011b).

## Decision 2: Unify duplicated orchestration into `internal/cli` shared helpers

- **Decision**: The overview pipeline (6-phase orchestration, dismissal
  delta, cluster expansion, budget fetch) is currently duplicated between
  the plain path (`internal/cli/overview.go:273`) and the TUI goroutine
  (`overview.go:1257`). Extract a single `OverviewPipeline` helper in
  `internal/cli` that yields phase/progress/row events via callbacks; plain,
  TUI, and web each subscribe with their own renderer.
- **Rationale**: The pipeline *steps* are already shared engine primitives;
  only orchestration is forked. A callback/event-based pipeline serves all
  three frontends without the web server reaching into cli internals.
- **Alternatives considered**: Have web call the plain path and poll — loses
  progressive loading (FR-007). Have web shell out to the CLI — fragile,
  wasteful, still a parallel path.

## Decision 3: Carbon/sustainability aggregation unification

- **Decision**: Move `aggregateCarbonFromResults`
  (`internal/tui/cost_view.go:517`) into `internal/engine` (next to
  `renderSustainabilitySummary` at `internal/engine/project.go:322`) as one
  exported aggregation function; TUI and web both consume it.
- **Rationale**: This is the one place where cost math is genuinely
  computed twice today; adding a third (web) would violate the user's
  data-integrity constraint.

## Decision 4: Transport — REST JSON + Server-Sent Events, stdlib `net/http`

- **Decision**: `net/http` server (stdlib, no framework). Endpoints return
  JSON serializations of existing engine/viewmodel types. Progressive
  loading (phases, per-row enrichment) streams over SSE
  (`text/event-stream`). Estimate edits and view queries are ordinary
  POST/GET requests.
- **Rationale**: SSE is one-directional server→client, exactly matching the
  enrichment flow; implementable with stdlib only (Flusher). WebSocket adds
  a dependency and bidirectional machinery nothing needs. Polling loses the
  progressive UX the spec requires (FR-007).
- **Alternatives considered**: WebSocket (gorilla/websocket or nhooyr) —
  rejected, unnecessary dependency. gRPC-web — rejected, heavy tooling for
  a localhost SPA. Long-polling — rejected, worse UX than SSE.

## Decision 5: Frontend — no-build vanilla JS SPA, embedded with `go:embed`

- **Decision**: The SPA is hand-written ES modules (no transpile/bundle
  step) under `internal/webui/static/`, embedded via `go:embed` (precedent:
  `internal/registry/embed.go:10`). `make build` stays pure-Go.
- **Rationale**: The repo is Go-first with strict cross-platform CI; a node
  toolchain in the CLI build path would break `go build` purity and add a
  release-time asset pipeline. Modern browsers natively support ES modules,
  fetch, and EventSource — everything the four views need. Accessibility
  (FR-017) is easier to verify with explicit semantic HTML than framework
  output.
- **Alternatives considered**: Vite + React/Svelte — rejected for build
  complexity and dependency supply-chain surface. WASM/Go-in-browser —
  rejected, large binary downloads and immature toolchain for this use case.
  HTMX — considered; vanilla fetch+SSE is equivalent weight without a
  vendored dependency.

## Decision 6: Session token + host validation

- **Decision**: Server generates 128-bit `crypto/rand` hex token at startup.
  Initial GET to `/` with `?token=` validates (constant-time) and sets an
  `HttpOnly; SameSite=Strict` cookie named per port, then redirects to `/`;
  the token stays valid for the server's lifetime so a second tab works.
  All API/SSE requests authenticate via the cookie (EventSource cannot set
  headers, query-token on SSE would leak into logs). All requests
  additionally require a `Host` header matching `127.0.0.1`/`localhost` to
  defeat DNS rebinding, and every `POST` requires a same-origin `Origin` and
  `Content-Type: application/json`, because `SameSite=Strict` does not
  separate two localhost ports. Per FR-016.
- **Rationale**: Jupyter-style bearer-in-URL alone leaks tokens into
  browser history and server logs on every SSE reconnect; cookie
  bootstrapped from one-time URL token avoids that. Host check is two lines
  and closes the rebinding hole the clarify session raised.
- **Relation to MCP**: `finfocus mcp-server --transport=http` (ax-go
  `mcp`) binds loopback by default and needs `--allow-non-loopback` for
  anything else. `--web` follows the same fail-closed stance but offers no
  non-loopback option at all (FR-016a). It does not reuse the MCP server and
  adds no tool to the MCP allow-list golden
  (`internal/cli/testdata/mcp/tools.golden`); Decision 9 records why the
  root-local `--web` flag leaves that golden unchanged.
- **Alternatives considered**: Token query param on every request —
  rejected (log/history leakage). mTLS/localhost certs — rejected, absurd
  complexity for local tool.

## Decision 7: Port selection and browser opening

- **Decision**: Default port `0` (kernel-assigned ephemeral); actual port
  printed in the URL. `--port` flag pins a port and fails clearly if taken
  (FR-003). Browser opened via `runtime.GOOS` switch
  (`xdg-open`/`open`/`rundll32`); `--no-browser` suppresses (FR-002). Both
  are root-local flags valid only with `--web` (Decision 9).
- **Rationale**: Auto port removes the most common launch failure
  (collision) while still allowing pinning. A tiny platform switch avoids a
  new dependency (`pkg/browser`) for ~20 lines of code.

## Decision 8: Testing strategy for web layer

- **Decision**: Go unit tests for viewmodel extraction (table-driven,
  testify), `httptest`-based handler/SSE tests in `internal/webui`, and
  parity tests asserting web JSON == TUI/plain output for shared fixtures
  (SC-002). Browser-level validation in `test/e2e/` (a separate Go module
  with its own `go.mod`, build tag `e2e`, run from that directory with
  `FINFOCUS_BINARY` set, as the Makefile's `test-e2e-kind` target does) for
  the three core journeys, runnable but not in the default `make test`. The browser
  driver is Playwright (Decision 11), confined to the `test/e2e` module.
- **Rationale**: Constitution mandates TDD and 80%/95% coverage; handler
  tests via `httptest` need no browser. Playwright covers the DOM
  accessibility/keyboard requirements (FR-017) that Go tests cannot.

## Decision 9: `--web` is a root-local flag on the main program

- **Decision** (user, 2026-10-06): `finfocus --web` is a flag on the root
  `finfocus` command, run from the Pulumi project directory. It is defined
  with `cmd.Flags()` on the root (root-local), the same way `--mcp` is
  (`internal/cli/root.go`, `mcpFlag` in `internal/cli/mcp.go`), never with
  `PersistentFlags()`. The root `RunE` gains a `--web` branch beside the
  existing `--mcp` branch. With no source flags it reuses the overview
  auto-detect path: `detectPulumiProject` (Pulumi binary, project found by
  walking up from the working directory, current stack),
  `exportStateFromProject`, and `loadOverviewFromAutoDetect`
  (`internal/cli/common_execution.go`, `internal/cli/overview.go`). The root
  command already delegates to the overview dashboard inside a Pulumi project
  (`runRootDefault`), so `finfocus --web` is the browser form of bare
  `finfocus`.
- **Verified facts** (grep of this repo, 2026-10-06):
  - Root persistent flags today: `--debug`, `--skip-version-check`,
    `--cache-ttl`, `--project-dir`. Root-local: `--mcp`.
  - `--stack` is persistent on `cost` (`newCostCmd`) and local on `overview`
    (`-s`). `--pulumi-json`, `--pulumi-state`, `--from`, `--to`, `--adapter`,
    `--filter` are local flags on `overview`, `cost actual`, and others. None of
    them is defined on the root, so root-local definitions cannot collide.
    Cobra only merges a parent's persistent flags into a child, so root-local
    flags are invisible to every subcommand.
  - `--project-dir` only steers config resolution and `runRootDefault`'s
    is-this-a-project check. `detectPulumiProject` always searches from `.`,
    so `--web` leaves `--project-dir` with that meaning and does not use it
    for source detection (FR-012).
- **Root-local flags this feature adds** (all meaningful only with `--web`;
  rejected with a clear error otherwise, because a root-local flag is
  otherwise parsed silently): `--web`, `--port`, `--no-browser`,
  `--pulumi-json`, `--pulumi-state`, `--stack`, `--from`, `--to`, `--adapter`,
  `--filter`. No short forms, to avoid colliding with ax-go's mounted flags.
  `--web` and `--mcp` are mutually exclusive.
- **Terraform** (verified): `--terraform-state` exists only on `cost
  projected`, `cost actual` (needs `--from`), and `cost forecast`.
  `overview`, `cost recommendations`, and `cost estimate` have no Terraform
  input, and `detectPulumiProject` is Pulumi-only; there is no Terraform
  auto-detect anywhere. So `--web` supports Pulumi only and Terraform is out
  of scope (FR-012a). Partial Terraform support (the actual-cost view alone via
  `--terraform-state` and `--from`) was considered and rejected: the landing
  view would be missing, and it would add a flag the other three views could
  not honor.
- **MCP golden** (verified from code): `applyMCPExclusions` marks the root
  with the node-only `mcp.Exclude` (`mcpExcludedCommands`, path `nil`), and
  `internal/cli/testdata/mcp/tools.golden` lists tool names only (21 lines of
  `finfocus-<command>` names). A root flag is not a subcommand and the root
  is already excluded, so `--web` adds no tool and the golden is unchanged.
  Two guards are still needed: `lifecycle.hostsMCP` is true during a
  dispatched call, so the root's `runRootMCP` branch (which returns
  `errRootNotATool`) must stay ahead of the `--web` branch, and `--web` must
  not start a server inside a tool call. The change is covered by tests that
  the golden passes unchanged and that `--web` with `--mcp` is rejected.
- **Alternatives considered**:
  - *`--web` on each command* (`cost actual --web`, `overview --web`, ...):
    rejected. Several commands would each need the flag, the port and browser
    flags, and the server wiring. The browser session spans four views, so
    attaching it to one command would make the other three unreachable by
    construction.
  - *`finfocus web` subcommand*: rejected by the user's decision to attach to
    the main program. It would also be a new MCP expose-or-exclude decision
    and a golden change, which the root flag avoids.
  - *Persistent root flag*: rejected. It would be inherited by every
    subcommand, so a redefinition (for example `--stack` on `cost`, or
    `--plain` on `cost history view`) would make Cobra reject the command
    (CLAUDE.md, flag-redefinition trap), and `--web` would appear on
    commands that cannot honor it.
- **Rationale**: One entry point matching the user's workflow ("run in the
  same directory as the program"), no new subcommand, no MCP golden churn, and
  precedent in the existing `--mcp` root-local flag.

## Decision 10: Group-by selector and include-dismissed toggle are web-only

- **Decision** (user, 2026-10-06): Keep the in-UI group-by selector (actual-cost
  view) and the include-dismissed toggle (recommendations view) as web-only
  enhancements that re-fetch through the shared code path. The earlier "TUI
  parity" wording is removed: the TUI fixes `groupBy` when the model is built
  (`internal/tui/cost_model.go`) and has no include-dismissed handling
  (grep of `internal/tui/` finds none); both exist only as CLI flags.
- **Dimensions the CLI supports on `cost actual --group-by`** (verified in
  `internal/cli/cost_actual.go` and `internal/engine/types.go`): `resource`,
  `type`, `provider`, `date` (deprecated alias of `daily`), `daily`,
  `monthly`, and `tag:key=value` (`parseTagFilter` clears the group-by and
  filters by tag instead). `--filter` also accepts `type=...` and `tag:k=v`.
- **Per dimension, the web UI offers**:

  | Dimension | Web UI | Reason |
  | --- | --- | --- |
  | none | Offered | The CLI default |
  | `resource` | Offered | Per-resource table is the base view |
  | `type` | Offered | Meaningful in a table; same engine grouping |
  | `provider` | Offered | Meaningful in a table; same engine grouping |
  | `daily` | Offered | Time series through `CreateCrossProviderAggregation` |
  | `monthly` | Offered | Same |
  | `date` | Not offered | Deprecated alias of `daily`; a duplicate choice |
  | `tag:key=value` | Offered as a filter input | It filters, not groups; the shared parser validates it |

  No offered dimension is unsuitable for a web view. The server accepts the
  values the shared validator accepts (so `date` still works if sent) and
  returns 400 for the rest.
- **Alternatives considered**: Launch-time grouping only (strict TUI parity):
  rejected by the user; re-fetching is cheap and the selector is what makes
  the view explorable. Client-side regrouping: rejected, FR-011b forbids it.

## Decision 11: E2E browser driver is Playwright via playwright-go

- **Decision** (user, 2026-10-06): Browser e2e tests use Playwright through
  `github.com/mxschmitt/playwright-go`, added to
  `test/e2e/go.mod` only. The root `go.mod` must not change (checked by
  `go mod tidy -diff`). CI already verifies the Go version stays in step
  between the root and `test/e2e/go.mod` (`.github/workflows/ci.yml`).
- **Browser binaries**: playwright-go downloads its driver and the browsers on
  demand. Install explicitly with
  `go run github.com/mxschmitt/playwright-go/cmd/playwright install --with-deps chromium`
  from `test/e2e` (pin the same module version as `go.mod`). The tests are
  behind their own build tag, `e2e_web`, and a Makefile target `test-e2e-web`
  runs the install step then the tests. CI needs a dedicated job (like
  `e2e-kind`) that runs that target. Locally, a test skips with a clear
  message when the binary under test or the browser is missing, as
  `e2e_black_box_test.go` already skips when the binary is absent.
- **Note on the kind precedent**: `test-e2e-kind` does not skip when its tools
  are absent; it is isolated by the `e2e_kind` build tag and a dedicated CI
  job. The web e2e follows the same isolation (tag plus job) and adds a skip
  for local runs.
- **Alternatives considered**: Node Playwright via `npx`: arguably stronger for
  accessibility checks (`@axe-core/playwright`) and tracks upstream faster,
  but adds a Node toolchain and a `package.json` to a Go-only repo. Not
  chosen; the concern is recorded here for review. Chromedp: Chrome DevTools
  only, no cross-browser, weaker accessibility tooling.

## Browser module maintenance update

The nested test module pins `github.com/mxschmitt/playwright-go v0.6201.1`.
The [upstream releases](https://github.com/mxschmitt/playwright-go/releases)
record the module-path migration at v0.6100. The former v0.6000.0 installer
requested Playwright 1.60.0 driver archives that returned HTTP 404 from all
three configured official CDN mirrors during validation. The maintained
version's module-resolved installer successfully installs Chromium.
`make test-e2e-web` runs that exact resolved version from `test/e2e`; it uses
neither an independent CLI pin nor `@latest`, and adds no root dependency.
