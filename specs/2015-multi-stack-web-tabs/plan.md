# Implementation Plan: Multi-Stack Web Dashboard with a Tab Bar

**Branch**: `2015-multi-stack-web-tabs` | **Date**: 2026-11-09 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2015-multi-stack-web-tabs/spec.md`

## Summary

`--stack` becomes a list. One stack keeps today's path. Two or more build one
`webStack` per name (spec 2013), register each under `/s/<name>/` (spec 2012),
and start a stack's pipeline only when the browser activates its tab. The shell
gains a tab bar that picks the active stack, derives the request prefix from it,
and keeps one overview stream open. The server adds a stack list endpoint and a
per-stack activate endpoint on the root only.

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`); browser ES modules, no build step
**Primary Dependencies**: Cobra, `internal/webui`, `internal/pulumi`
(`ListStacks`, `ValidateStacks`), the existing `webStack` runtime
**Storage**: None new. The shared cache and history stores are unchanged.
**Testing**: testify; `node --test` frontend harness in
`internal/webui/frontendtest/`; `test/e2e` web tests; Playwright render
**Target Platform**: Linux, macOS, Windows (no platform-specific code)
**Project Type**: Single Go module with an embedded static frontend
**Performance Goals**: Dashboard start with N stacks starts exactly one stack
**Constraints**: CSP `default-src 'self'` (no inline script); the browser formats
nothing; no iframes; one SSE stream
**Scale/Scope**: Typically 2 to 5 stacks per project

## Constitution Check

- [x] **Plugin-First Architecture**: orchestration and UI only.
- [x] **Test-Driven Development**: tests precede each task; frontend harness
  and Go tests cover the new paths. No `internal/tui/` change.
- [x] **Cross-Platform Compatibility**: no OS-specific code.
- [x] **Documentation Integrity**: web-ui architecture page, CLI reference,
  and the web guide are in the tasks; schema golden regenerated.
- [x] **Protocol Stability**: no protocol change.
- [x] **Implementation Completeness**: no stubs.
- [x] **Persistence Model**: no new store.
- [x] **Web Parity**: the spec carries Web UI Fit (extends the shell). The
  contract names the Go methods. One UX-only `web-ui` follow-up is filed on
  completion.
- [x] **Quality Gates**: `make lint`, `make test`, race, `make test-e2e-web`.
- [x] **Multi-Repo Coordination**: none.

**Violations Requiring Justification**: none.

## Project Structure

### Documentation (this feature)

```text
specs/2015-multi-stack-web-tabs/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── http-api.md
└── tasks.md
```

### Source Code

```text
internal/cli/web.go                  # --stack list, validation, multi runner
internal/cli/web_stack.go            # idempotent start
internal/cli/web_multistack.go       # new: stack list normalization + runner
internal/webui/stacks.go             # StackOptions.Activate, state, exports, stacksMeta
internal/webui/server.go             # shell carries the stack list (handleRoot)
internal/webui/session.go            # Failed, PassphraseRequired, SetPipelineEnded
internal/webui/static/views/overview.js  # prompt reads the active stack's snapshot
internal/cli/web_test.go             # flag-type pins (also web_stack_parity_test.go)
internal/webui/handlers_stacks.go    # new: GET /api/stacks, POST activate
internal/webui/static/app.js         # tab bar, active-stack base, stream reset
internal/webui/static/index.html     # tablist container, stack label
internal/webui/static/styles.css     # tab bar styles
internal/webui/frontendtest/         # stacks.test.mjs
internal/cli/testdata/schema-root.golden.json
docs/src/content/docs/architecture/web-ui.md, docs guide + CLI reference
test/e2e/                            # two-stack web e2e
```

**Structure Decision**: extend existing files; the new Go files hold only the
stack-list logic so `web.go` stays readable.
