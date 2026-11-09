# Tasks: Stack-Aware Routing and a Prefix-Safe Web Frontend

Tests first in every phase. Web UI Fit is `Not applicable`: no web-parity
phase and no `web-ui` follow-up.

## Phase 1: Setup

- [X] T001 Baseline: `go test ./internal/webui/...` and the frontend harness pass before changes

## Phase 2: Routing (US1, US3)

- [X] T002 [P] Write `internal/webui/stacks_test.go`: two stacks reach their own sessions; unknown stack 404 after auth, 401 before; `/s/dev` redirects; name validation table; duplicate and late registration; root unaffected; token bootstrap at stack root; Shutdown closes stack sessions
- [X] T003 Implement `internal/webui/stacks.go` and refactor `server.go` (`buildMux`, `dispatch`, parent helpers, bootstrap, request log, shutdown)
- [X] T004 Shell: add the meta placeholder and relative URLs to `static/index.html`; inject the prefix in `handleRoot`; update the pinned shell assertions in `server_test.go`

## Phase 3: Frontend (US2)

- [X] T005 [P] Write `frontendtest/base.test.mjs`: prefix applied to fetch, overview EventSource, session check; empty prefix unchanged; a source scan asserts no absolute `/api`, `/static` literal is passed to a network call outside `withBase`
- [X] T006 Add `base()`/`withBase()` to `static/app.js`; apply in `request()`, `OverviewStream`, the budget check
- [X] T007 Apply `withBase` in `views/logs.js` (stream, viewer import) and `views/plugins.js` (operation stream); update the `logs`/`plugins` harness fake apps and logs/plugins assertions

## Phase 4: Verify and document

- [ ] T008 Deferred to #2039: the real binary cannot reach `/s/<stack>/` until a flag registers stacks. Rendered by hand with Playwright against a harness server instead (see PR)
- [X] T009 Docs: `architecture/web-ui.md` (routing, cookie, prefix), `contracts` note in spec 627 http-api
- [X] T010 `make validate`, `make test`, `make test-race` (webui), `make lint`, `make docs-lint`
- [X] T011 Code review against the base commit; fix substantiated findings
