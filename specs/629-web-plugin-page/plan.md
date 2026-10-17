# Implementation Plan: Web Plugin Page

**Branch**: `629-web-plugin-page` | **Date**: 2026-10-17 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/629-web-plugin-page/spec.md`

## Summary

Add a Plugins page to `finfocus --web`. It lists the built-in catalog merged
with what is installed, and it installs, updates, removes, and validates
catalog plugins with an explicit confirmation and typed progress. Plugin
settings use the config page's writer. On the next plugin launch,
`pluginhost.NewClient` injects those settings into that plugin's environment
only. A digest of the delivered values is part of the plugin cache key.
jev, Prometheus, Kubernetes, and the recorder advertise the settings they
already read through `pluginsdk.WithSettings` in finfocus-spec v0.7.6.

## Technical Context

**Language/Version**: Go 1.27.1
**Primary Dependencies**: finfocus-spec v0.7.6 `pluginsdk.WithSettings`,
existing `net/http` web UI, vanilla JS SPA in `internal/webui/static`,
existing registry installer and GitHub client
**Storage**: Existing plugin directory and the two `config.hujson` files.
Delivered-setting memory lives in the plugin client for one process. No new
database.
**Testing**: testify in `internal/registry`, `internal/pluginhost`,
`internal/config`, `internal/engine`, `internal/webui`, and the nested plugin
modules; Node tests in `internal/webui/frontendtest`; Playwright case in
`test/e2e`
**Target Platform**: Linux, macOS, and Windows, same as the CLI
**Project Type**: Go CLI plus the existing loopback SPA
**Performance Goals**: Catalog render does not download archives. Install
progress is streamed. Settings validation is local. One plugin restart at
most during `NewClient`.
**Constraints**: Catalog names only on the web path. No skipped checksum.
Secrets never reach the browser, logs, errors, or another plugin. CLI
install text is unchanged. `webui` does not import `cli`. Loopback, token,
and Origin rules from spec 627 stay.
**Scale/Scope**: One page, the built-in catalog (about seven entries), and
the settings those plugins plus the recorder already read.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Core does not learn provider pricing or
  a plugin's setting names except by reading the descriptor the plugin
  returns. jev, Prometheus, Kubernetes, and recorder declare their own
  settings.
- [x] **Test-Driven Development**: Registry policy, env precedence, cache
  digest, HTTP, and page tests are written with the behavior they lock. The
  page is not a Bubble Tea view. The browser test reads the dialog, the
  progress text, and the recorder's received value.
- [x] **Cross-Platform Compatibility**: Install paths stay on `filepath`.
  The installer lock already has Unix and Windows implementations. No new
  OS-specific code.
- [x] **Documentation Integrity**: Plugin system guide, CLI reference, three
  web UI guides, spec 627, spec 628, and `CLAUDE.md`.
- [x] **Protocol Stability**: No proto change. finfocus-spec v0.7.6 is
  already pinned, and `internal/pluginupgrade` already hops to v0.7.6.
- [x] **Implementation Completeness**: Catalog, confirmation, progress,
  trust rules, validate, both setting forms, delivery, and the built-in
  descriptors ship together. Custom registries, signatures, init, upgrade,
  conformance, certify, and MCP tools stay out of scope with no stubs.
- [x] **Persistence Model**: Plugin files and `config.hujson` already exist
  and are optional. No new store. A failed install does not leave a
  half-installed version. Commands still run with an empty plugin directory.
- [x] **Quality Gates**: `make validate`, `make test`, `make test-race`,
  `make lint`, `make test-frontend`, `make test-e2e-web`, and
  `make check-plugin-boundaries` from this worktree.
- [x] **Multi-Repo Coordination**: The settings descriptor shipped in
  finfocus-spec v0.7.6 (spec issue 635). This change consumes it. Nested
  modules stay on that same pin.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/629-web-plugin-page/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/http.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/registry/installer.go
internal/registry/progress.go
internal/registry/github.go
internal/pluginhost/process.go
internal/pluginhost/host.go
internal/pluginhost/settings_env.go
internal/engine/engine.go
internal/config/plugins.go
internal/cli/web.go
internal/cli/plugin_install.go
internal/webui/session.go
internal/webui/server.go
internal/webui/sse.go
internal/webui/handlers_plugins.go
internal/webui/static/views/plugins.js
internal/webui/static/app.js
internal/webui/static/index.html
internal/webui/frontendtest/plugins.test.mjs
plugins/jev/
plugins/prometheus/
plugins/kubernetes/
plugins/recorder/plugin.go
docs/src/content/docs/guides/plugin-system.md
docs/src/content/docs/reference/cli-commands.md
docs/src/content/docs/architecture/web-ui.md
docs/src/content/docs/reference/web-ui.md
docs/src/content/docs/guides/web-ui.md
specs/627-web-ui/spec.md
specs/628-web-config-page/spec.md
```

## Phase 0 and Phase 1

Research is in [research.md](research.md). Entities are in
[data-model.md](data-model.md). The HTTP contract is in
[contracts/http.md](contracts/http.md). Usage is in
[quickstart.md](quickstart.md).

## Post-design Constitution Check

The design still passes. Settings writes go through `config.PatchFile` via
the config callbacks, so comments and environment values cannot be folded in
by `Config.Save`. `webui` reaches install and launch only through session
callbacks. Delivery memory is process-local. Specs 627 and 628 are amended
instead of contradicted. The v0.7.6 hop already exists, so this feature does
not add a protocol hop.
