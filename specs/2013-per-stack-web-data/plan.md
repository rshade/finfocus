# Implementation Plan: Per-Stack-Safe Web Data Path

**Branch**: `2013-per-stack-web-data` | **Date**: 2026-11-08 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2013-per-stack-web-data/spec.md`

## Summary

Make the pieces `finfocus --web` builds per run safe to build once per stack in
one process (issue #2038, prerequisite for #2039). Four changes, all inside
`internal/cli` and `internal/pulumi`:

1. A process-wide `sharedStores` value opens the cost cache and resource
   history once, is injected into each stack's engine and overview pipeline, and
   closes once. The pipeline stops opening history itself when one is injected.
2. ESC becomes a per-stack source derived from the root source's options plus
   that stack's own `Pulumi.<stack>.yaml` environment list. The raw `--stack`
   flag value is no longer consulted on the web path.
3. The passphrase channel, submit callback, and prompt wiring are owned by one
   `webStack` runtime, so each stack has its own channel.
4. `pulumidetect.ListStacks` and `ValidateStacks` give #2039 a real stack list.

`runWebSession` is refactored to build exactly one `webStack` through the same
constructors #2039 will call per stack. Single-stack behavior is unchanged.

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`)
**Primary Dependencies**: Cobra, ax-go, bbolt (`cache`, `history` stores),
`internal/esc` (ESC SDK source), `internal/pulumi`, testify
**Storage**: Existing Bolt files only (`cache.db`, `history.db`); no new store
**Testing**: testify; `internal/cli` unit tests with stubbed Pulumi runner and
temp `FINFOCUS_HOME`; `go test -race -shuffle=on -count=3` on touched packages
**Target Platform**: Linux, macOS, Windows (no OS-specific code added)
**Project Type**: single Go module (CLI + embedded web server)
**Performance Goals**: no regression; the change removes a 0.5 s and 1 s lock
wait per extra stack
**Constraints**: single-stack behavior byte-identical; no secret logged; no
`webui` import of `cli`; lint config untouched
**Scale/Scope**: ~5 files changed in `internal/cli`, 1 in `internal/pulumi`,
new tests; N stacks per process (N small, tens at most)

## Constitution Check

- [x] **Plugin-First Architecture**: orchestration only; no provider logic.
- [x] **Test-Driven Development**: tasks put failing tests first (two engines
  one process, per-stack ESC, passphrase routing, ListStacks); no TUI change.
- [x] **Cross-Platform Compatibility**: pure Go; paths via `filepath`.
- [x] **Documentation Integrity**: `docs/.../architecture/web-ui.md` gets a
  short "per-stack runtime" note; CLAUDE.md gets the new invariants.
- [x] **Protocol Stability**: no proto or plugin-protocol change.
- [x] **Implementation Completeness**: no stubs; #2039 consumes real code.
- [x] **Persistence Model**: no new store; sharing one handle keeps stores
  optional and corruption-recovering (same open path).
- [x] **Web Parity**: spec records `Not applicable` (prerequisite refactor, no
  CLI feature, no endpoint). Exempt from hook, render check, follow-up issue.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint`,
  `make test-race` on touched packages.
- [x] **Multi-Repo Coordination**: none; finfocus-spec untouched.

**Violations Requiring Justification**: none.

Post-design re-check: still passes. The plan adds one package-private type per
concern and no global variable (lint forbids `init` and globals).

## Project Structure

### Documentation (this feature)

```text
specs/2013-per-stack-web-data/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/internal-api.md
├── checklists/requirements.md
└── tasks.md            # /speckit.tasks
```

### Source Code (repository root)

```text
internal/cli/
├── web_stores.go          # NEW sharedStores: open once, inject, close once
├── web_stack.go           # NEW webStack runtime (per-stack channel, ESC, session)
├── web.go                 # runWebSession builds one webStack
├── web_data.go            # newEngine/reload use injected stores; per-stack ESC
├── common_execution.go    # engine builder that accepts injected stores
├── esc_source.go          # per-stack source from root options
├── overview_pipeline.go   # config.history injection at 3 history sites
└── *_test.go              # new tests listed in tasks
internal/pulumi/
└── pulumi.go              # ListStacks, ValidateStacks; GetCurrentStack reuses
```

**Structure Decision**: keep everything in the existing `internal/cli` and
`internal/pulumi` packages. `internal/webui` needs no change: `Session` already
holds one `SubmitPassphrase` callback and one prompt state per instance, so
per-stack routing is a wiring matter on the `cli` side.
