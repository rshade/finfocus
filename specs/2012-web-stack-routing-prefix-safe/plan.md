# Implementation Plan: Stack-Aware Routing and a Prefix-Safe Web Frontend

**Branch**: `2012-web-stack-routing-prefix-safe` | **Spec**: [spec.md](spec.md)

## Summary

Add `Server.AddStack(name, StackOptions)`. Each stack is a lightweight child
`*Server` that owns a `Session`, a route mux, and its own plugin operations,
and shares the parent's token, port, access rules, logger, and log source. The
parent's handler chain stays the single outer layer; a dispatch step after
authentication and hardening strips `/s/<stack>` and calls the child mux. On
the frontend, `withBase` is added to `app.js` and applied at every network
site.

## Technical Context

- Go (`internal/webui`), hand-written ES modules, no build step.
- Tests: Go `testing` + testify; node `--test` harnesses in `frontendtest/`.
- No new dependency.

## Constitution Check

- **I/VI**: no TODOs or stubs; stack registration is a complete API.
- **II (tests first)**: Go router tests and frontend tests precede changes.
- **VII (web parity)**: `Not applicable`, recorded in the spec.
- Core stays provider-agnostic; nothing here touches pricing.

## Design

1. **Child servers.** `newStackServer(parent, name, opts)` builds a `Server`
   with `parent`, `prefix`, its own session, `ops`, and a mux from the shared
   `buildMux` (also used by the root). `Port` (and so the cookie name) reads
   through `parent`; the token, logger, and access rules are immutable after
   `New` and are copied. The child has no `httpServer` and never listens.
2. **Dispatch.** In `handler()` the chain is unchanged except that the final
   handler is `s.dispatch(mux)`, which matches `/s/<name>/…`, strips the
   prefix, and calls the child mux, or answers 404 / 308.
3. **Auth bootstrap.** `authenticate` and `handleRoot` use `rootPathOf`,
   which accepts `/` and `/s/<registered>/`. The 303 goes back to that path.
4. **Request log.** The logs-route exemption tests the path with the stack
   prefix removed.
5. **Shell.** `index.html` carries `<meta name="finfocus-base" content="">`
   and relative asset URLs; `handleRoot` fills the content from the validated
   prefix.
6. **Frontend.** `base()` reads the meta once; `withBase(path)` prefixes a
   root-relative path. `request()`, both `EventSource` constructors, the
   session check, and the viewer import use it. `api` callers keep their
   `/api/...` paths, so the existing assertions hold.
7. **Shutdown.** Closes each child's session and operations.

## Risks and Mitigations

- `app.js` is linked with no imports by the harness and views are linked to a
  fake app that exports only `el`; `withBase` lives in `app.js` and the two
  views that need it (`logs.js`, `plugins.js`) import it. The two harness
  fakes gain the export.
- `Server.Handle` assumes one mux: it keeps targeting the root mux; documented.
- The shell's static URLs relative to `/s/dev/` require the trailing slash;
  FR-005 guarantees it.

## Project Structure

```text
internal/webui/stacks.go        # AddStack, dispatch, name validation
internal/webui/stacks_test.go
internal/webui/server.go        # buildMux, chain, bootstrap, shutdown
internal/webui/static/app.js    # base(), withBase()
internal/webui/static/index.html
internal/webui/static/views/{logs,plugins}.js
internal/webui/frontendtest/    # base.test.mjs + fake-app updates
test/e2e/web_prefix_test.go
```
