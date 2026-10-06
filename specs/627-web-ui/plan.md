# Implementation Plan: Web UI (Browser-Based SPA)

**Branch**: `627-web-ui` | **Date**: 2026-10-06 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/627-web-ui/spec.md`

## Summary

Add `finfocus --web`: a root-local flag on the main program, run from the
Pulumi project directory and auto-detecting project, stack, and state the way
`finfocus overview` does (research Decision 9). It starts a localhost-only web
server (stdlib `net/http`,
per-session token) that serves an embedded, no-build vanilla-JS SPA and
streams the same progressive data the TUI shows, over REST JSON + SSE. The
hard constraint (FR-011a/b) is that **no new cost logic may be written for
the web**: the plan therefore centers on extracting the filter/sort/
display/aggregation logic currently trapped in the bubbletea models into a
shared `internal/viewmodel` package, unifying the duplicated overview
orchestration into an event-driven `OverviewPipeline` in `internal/cli`,
and unifying the double-computed carbon aggregation into `internal/engine`.
TUI, plain CLI, and web then all render the same outputs of the same
functions.

## Technical Context

**Language/Version**: Go 1.27.1 (existing repo); frontend = ES2022+ vanilla JavaScript (no build step)
**Primary Dependencies**: stdlib `net/http`, `go:embed`, `crypto/rand` only — no new Go or npm dependencies in the root module; existing `internal/engine`, `internal/pluginhost`, `internal/config`, `internal/history`, `internal/greenops`
**Storage**: Existing optional stores only (dismissal store `internal/config/dismissed.go`, history, cache); the web session itself is in-memory
**Testing**: testify table-driven unit tests (viewmodel, engine), `httptest` handler/SSE tests, fixture-based parity tests (web JSON vs plain/TUI), security tests (token, Host, Origin, redaction), browser e2e (Playwright through playwright-go, confined to the `test/e2e` module, build tag `e2e_web`; research Decision 11) for the three core journeys
**Target Platform**: Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64); evergreen browsers
**Project Type**: single Go CLI project with embedded static SPA (`internal/webui/static/`)
**Performance Goals**: launch→populated dashboard <30s for ≤250 resources; filter/sort <1s at 1,000 resources (spec SC-001, SC-004)
**Launch surface**: `--web` is a root-local flag (`cmd.Flags()`, never persistent), like `--mcp`. Root-local companions, valid only with `--web` and rejected without it: `--port`, `--no-browser`, `--pulumi-json`, `--pulumi-state`, `--stack`, `--from`, `--to`, `--adapter`, `--filter`. `--project-dir` is the existing root persistent flag, reused unchanged. Pulumi only: `--terraform-state` is not accepted (spec FR-012a). `--web` and `--mcp` are mutually exclusive. The root is already excluded from MCP (`mcpExcludedCommands`), so `internal/cli/testdata/mcp/tools.golden` does not change
**Constraints**: loopback-only bind (no non-loopback option) + per-session token + Host and Origin validation + redaction parity with the CLI JSON output; no duplicated cost/filter/sort logic (FR-011a/b); TUI must remain fully functional; pure-Go `make build` (no node in the build path)
**Scale/Scope**: 4 views (overview, cost actual, recommendations, estimate); single-user local session; stacks up to ~1,000 resources

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Feature is orchestration/UI only; all cost data still flows from plugins through `internal/engine`. No provider logic added to core.
- [x] **Test-Driven Development**: Tasks are test-first (constitution NON-NEGOTIABLE). Extraction tasks land with unit tests before web handlers consume them; handlers get `httptest` tests; parity tests prove web == TUI numbers (SC-002a). Golden-file TUI tests must keep passing after extraction (TUI views unchanged in output).
- [x] **Cross-Platform Compatibility**: stdlib server + `go:embed` + `runtime.GOOS` browser-open switch; CI cross-platform build unchanged; no node toolchain in the build.
- [x] **Documentation Integrity**: Plan includes updates to README (new flag), `docs/src/content/docs/` (web UI page), and godoc for all new exported symbols (80%+ coverage gate).
- [x] **Protocol Stability**: No protobuf changes; finfocus-spec untouched.
- [x] **Implementation Completeness**: No stubs/TODOs permitted; every endpoint backed by real engine calls from day one.
- [x] **Read-only / Stateless Boundary (CONTEXT.md)**: Foreground server tied to one CLI invocation; no write access to infrastructure (the on-demand `pulumi preview` is the TUI's existing read-only action); no provider logic; no new store.
- [x] **Security**: Loopback only, token + Host + Origin checks, no secret or notification URL in any payload (FR-016, FR-016a, FR-018). Separate from the MCP server and its tool allow-list.
- [x] **Persistence Model**: No new persistent stores. Web session state is in-memory; existing optional stores reused as-is. Commands still work with all stores deleted.
- [x] **Quality Gates**: `make lint`, `make test`, `make validate`, govulncheck, docstring coverage all apply to new packages.
- [x] **Multi-Repo Coordination**: None required — core-only change, no spec or plugin repo impact.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/627-web-ui/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── http-api.md      # REST + SSE contract
└── tasks.md             # Phase 2 output (/speckit.tasks - NOT created here)
```

### Source Code (repository root)

```text
internal/
├── viewmodel/                 # NEW — presentation-neutral logic extracted from TUI models
│   ├── filter.go              # substring filters (overview, cost, recommendations)
│   ├── sort.go                # sort fields + comparators for all three views
│   ├── cluster_display.go     # tree flatten/pagination (from overview_model.displayEntries)
│   └── *_test.go
├── engine/
│   ├── sustainability.go      # NEW — unified carbon aggregation (absorbs tui/cost_view.go:517)
│   └── ... (existing; EstimateCost, EnrichOverviewRows, GetBudgets, etc. reused as-is)
├── cli/
│   ├── overview_pipeline.go   # NEW — event-driven overview orchestration shared by plain/TUI/web
│   ├── root.go                # `--web` and companion root-local flags; RunE branch after the --mcp branch
│   ├── web.go                 # NEW — `--web` wiring: flag validation, auto-detect reuse, server lifecycle, browser open
│   └── ... (existing commands refactored to call pipeline/viewmodel)
├── tui/
│   └── ... (models refactored to delegate filter/sort/display to internal/viewmodel;
│              carbon summary to engine; no visual change — golden files must pass)
└── webui/                     # NEW — HTTP layer (thin transport only, FR-011a)
    ├── server.go              # net/http server, token+host middleware, lifecycle
    ├── static.go              # go:embed static assets, cookie bootstrap
    ├── handlers_overview.go   # overview JSON + SSE enrichment stream
    ├── handlers_cost.go       # actual-cost endpoints (incl. group-by, trends, carbon)
    ├── handlers_recommendations.go
    ├── handlers_estimate.go   # edit/recalculate → eng.EstimateCost
    ├── static/                # vanilla ES-module SPA (no build step)
    │   ├── index.html
    │   ├── app.js             # router, SSE client, state
    │   ├── views/             # overview.js, cost.js, recommendations.js, estimate.js
    │   └── styles.css         # light/dark palettes, WCAG AA contrast, focus-visible
    └── *_test.go              # httptest handler/SSE/token tests

test/e2e/
└── web_browser_test.go        # e2e_web build tag (test/e2e module, playwright-go): 3 core journeys, keyboard nav, contrast

Makefile                           # new `test-e2e-web` target: install browsers, run the e2e_web tests
```

**Structure Decision**: Single-project layout. The web surface is a new
`internal/webui` package (thin transport) plus a new `internal/viewmodel`
package (shared presentation logic) plus one orchestration refactor in
`internal/cli`. Static SPA assets live inside `internal/webui/static/` and
are embedded — no separate frontend project, no build tooling.

## Complexity Tracking

No constitution violations — table intentionally empty.

## Browser E2E Driver

Playwright via `github.com/playwright-community/playwright-go`, added to
`test/e2e/go.mod` only; the root `go.mod` does not change and `go mod tidy
-diff` stays clean. Browser binaries are not vendored: the `test-e2e-web`
Makefile target runs the playwright-go install step (chromium) from `test/e2e`
before `go test -tags e2e_web`. CI gets a dedicated job (like `e2e-kind`) that
runs that target, and the tests skip locally with a clear message when the
`FINFOCUS_BINARY` or the browser is missing. Node Playwright via `npx` was
considered and not chosen (research Decision 11).

## Follow-ups

Usability testing of the three core journeys is a later follow-up and not a
gate for this feature (spec Follow-ups). Terraform support is out of scope
(spec FR-012a).
