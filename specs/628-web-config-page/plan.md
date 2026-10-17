# Implementation Plan: Web Config Page

**Branch**: `628-web-config-page` | **Date**: 2026-10-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/628-web-config-page/spec.md`

## Summary

Add a Config page to `finfocus --web`. It shows each known setting, the tier
that supplied the effective value, and the value stored in each tier. Edits
target the project or global `config.hujson` by patching that file in place.
Secrets are set or unset in the browser. `config get`, `config list`, and
`config set` share the same masking and the same file writer.

## Technical Context

**Language/Version**: Go 1.27.1
**Primary Dependencies**: `github.com/tailscale/hujson` (already required),
existing `net/http` web UI, vanilla JS SPA in `internal/webui/static`
**Storage**: The two existing configuration files. No new database.
**Testing**: testify in `internal/config` and `internal/webui`; Node tests in
`internal/webui/frontendtest`; Playwright case in `test/e2e/web_browser_test.go`
**Target Platform**: Linux, macOS, and Windows, same as the CLI
**Project Type**: Go CLI plus the existing loopback SPA
**Performance Goals**: One file read and one patch per save. No plugin calls.
**Constraints**: No secret value in a response, log, or audit record. No path
in the write request. Comments and unknown keys survive. Environment values
are not written. Loopback, token, and Origin rules from spec 627 stay.
**Scale/Scope**: One page, the known `config set` keys, and the read-only
alert, routing, and analyzer sections.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Core edits its own configuration files.
  No provider pricing and no cloud API.
- [x] **Test-Driven Development**: Patch, masking, HTTP, and page tests are
  written with the behavior they lock. The page is not a Bubble Tea view, so
  TUI goldens do not apply. The browser test reads the rendered control and
  the file on disk.
- [x] **Cross-Platform Compatibility**: `os.CreateTemp`, `Rename`, and the
  existing lock file. No OS-specific path logic beyond `filepath`.
- [x] **Documentation Integrity**: Architecture, reference, and guide pages
  for the web UI, the configuration reference, spec 627, and `CLAUDE.md`.
- [x] **Protocol Stability**: No finfocus-spec or plugin protocol change.
- [x] **Implementation Completeness**: View, preview, save, conflict,
  validation, masking, and the command-line writer ship together. Live
  reload, alert editors, and plugin install stay out of scope with no stubs.
- [x] **Persistence Model**: The files already exist and are optional.
  Commands still run with no config file. A failed or stale save leaves the
  previous file in place.
- [x] **Quality Gates**: `make validate`, `make test`, `make test-race`,
  `make lint`, and `make test-e2e-web` from this worktree.
- [x] **Multi-Repo Coordination**: None. #1752 stays a separate redaction bug.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/628-web-config-page/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/http.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/config/credential.go
internal/config/descriptor.go
internal/config/patch.go
internal/config/view.go
internal/config/lock.go
internal/engine/flatten.go          # isCredentialKey delegates
internal/cli/config_set.go
internal/cli/web.go
internal/webui/handlers_config.go
internal/webui/session.go
internal/webui/server.go
internal/webui/static/views/config.js
internal/webui/static/app.js
internal/webui/static/index.html
internal/webui/frontendtest/config.test.mjs
docs/src/content/docs/architecture/web-ui.md
docs/src/content/docs/reference/web-ui.md
docs/src/content/docs/guides/web-ui.md
specs/627-web-ui/spec.md
```

## Phase 0 and Phase 1

Research is in [research.md](research.md). Entities are in
[data-model.md](data-model.md). The HTTP contract is in
[contracts/http.md](contracts/http.md). Usage is in
[quickstart.md](quickstart.md).

## Post-design Constitution Check

The design still passes. The writer reads and patches file bytes, so
environment values folded by `New()` cannot be persisted. `webui` reaches
configuration only through the session callbacks. Spec 627 is amended instead
of contradicted.
