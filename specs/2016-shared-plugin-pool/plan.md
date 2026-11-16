# Implementation Plan: Shared Plugin Processes Across Web Stacks

**Branch**: `2016-shared-plugin-pool` | **Date**: 2026-11-15 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2016-shared-plugin-pool/spec.md`

## Summary

`finfocus --web --stack a,b,...` launches a full plugin set for every
activated stack. Add a refcounted `pluginhost.Pool`, created once by
`runWebStacks` and carried on the context every stack derives from. The
registry's per-plugin open asks the pool instead of calling
`pluginhost.NewClient` when the context carries one. The pool keys a process
by binary path, the exact launch environment, and the stack's whole ESC offer
for that plugin; it owns each process's launch context, so no stack, pipeline,
or request can kill a process another stack holds. Each holder receives its
own `Client` copy whose `Close` releases one hold. Single-stack and CLI paths
never see a pool and are unchanged.

## Technical Context

**Language/Version**: Go 1.27.4
**Primary Dependencies**: gRPC (`google.golang.org/grpc`), finfocus-spec
`pluginsdk` v0.7.12, zerolog, testify
**Storage**: N/A (in-memory pool only)
**Testing**: `go test` with testify; bufconn gRPC fake launcher in
`internal/pluginhost`; `test/integration/web_sigterm_orphan_test.go` with the
router-like fixture
**Target Platform**: Linux, macOS, Windows (process groups already abstracted
by `configureProcessGroup` / `killProcessGroup`)
**Project Type**: CLI with embedded web dashboard
**Performance Goals**: One process per (plugin binary, environment) instead
of one per activated stack
**Constraints**: No credential or ESC value may reach another stack's plugin;
single-stack and CLI launch paths byte-for-byte unchanged in behavior
**Scale/Scope**: Typically 2-10 stacks and 1-8 plugins per dashboard

## Constitution Check

- [x] **Plugin-First Architecture**: Orchestration only; no provider logic.
- [x] **Test-Driven Development**: Pool unit tests are written first against
  a fake launcher; integration test extends the SIGTERM orphan check.
- [x] **Cross-Platform Compatibility**: Uses existing launcher and process
  group helpers; no new OS calls.
- [x] **Documentation Integrity**: `docs/.../architecture/web-ui.md` and
  `CLAUDE.md` document the sharing rule.
- [x] **Protocol Stability**: No protocol change.
- [x] **Implementation Completeness**: No stubs; crash and failure paths are
  implemented, not deferred.
- [x] **Persistence Model**: No persistent state.
- [x] **Web Parity**: Web UI Fit is `Not applicable` with a reason: no new
  surface, same dashboard with fewer processes.
- [x] **Quality Gates**: `make validate`, `make test`, `make test-race`,
  `make lint`, `make test-e2e-web`.
- [x] **Multi-Repo Coordination**: None; core only.

## Project Structure

### Documentation (this feature)

```text
specs/2016-shared-plugin-pool/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── internal-api.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/pluginhost/
├── host.go          # NewClient split: resolve launch inputs, then launch
├── pool.go          # NEW: Pool, WithPool, PoolFromContext, key, holds
└── pool_test.go     # NEW: unit tests with a counting fake launcher
internal/registry/
└── registry.go      # openPlugin uses the context's pool when present
internal/cli/
├── web_multistack.go          # runWebStacks creates, attaches, closes the pool
└── web_multistack_pool_test.go # NEW: two stacks share one launch
test/integration/
└── web_sigterm_orphan_test.go # multi-stack variant
docs/src/content/docs/architecture/web-ui.md
CLAUDE.md
```

**Structure Decision**: The pool lives in `internal/pluginhost` next to
`NewClient`, because the key depends on unexported launch-input resolution
(`resolveInjectedFrom`). The registry is the single caller that decides between
pool and direct launch. The pool reaches every plugin open of a stack (pipeline
open, `webData.reloadPlugins`, `resolveScorerClient`) through the context,
the same way the stack's ESC source already does.

## Complexity Tracking

None.
