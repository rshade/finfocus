# Implementation Plan: Per-Request Credentials for Pooled Plugins

**Branch**: `2023-per-request-credentials` | **Date**: 2026-11-25 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/2023-per-request-credentials/spec.md`

## Summary

Extend the multi-stack plugin pool so stacks that differ only in declared
credentials share one process of a plugin that advertises
`supports_per_request_credentials`. The process starts without those ESC
values. Each holder's connection attaches its own set to every call.

## Technical Context

**Language/Version**: Go 1.27.4
**Primary Dependencies**: finfocus-spec v0.7.12 (`pluginsdk` credentials), gRPC
**Storage**: none
**Testing**: testify, in-memory gRPC servers, `esctest`
**Target Platform**: Linux, macOS, Windows
**Project Type**: single
**Constraints**: no new module (`otelgrpc` and the
OpenTelemetry SDK become direct requirements), no wire change, no `os.Setenv`
**Scale/Scope**: `internal/pluginhost`, `internal/esc`, `internal/config`,
`internal/proto`, docs

## Constitution Check

- [x] **Plugin-First**: orchestration only. Core knows no credential name.
- [x] **TDD**: tests precede each change. No TUI change.
- [x] **Cross-Platform**: no platform-specific code.
- [x] **Documentation**: ESC guide, configuration reference, `CLAUDE.md`.
- [x] **Protocol Stability**: no protocol change.
- [x] **Completeness**: no stubs.
- [x] **Persistence**: no new state.
- [x] **Web Parity**: Not applicable, with the reason in the spec.
- [x] **Quality Gates**: `make validate`, `test`, `test-race`, `lint`.
- [x] **Multi-Repo**: finfocus-spec half is released (v0.7.12).

## Project Structure

```text
internal/config/esc.go              # credentials field, validation, copies
internal/esc/source.go              # options field, accessor, copy
internal/pluginhost/credentials.go  # plan, per-holder connection, masking
internal/pluginhost/settings_env.go # implied allowlist, withheld resolution
internal/pluginhost/host.go         # launch and restart by support
internal/pluginhost/pool.go         # channel key, holder digest and connection
internal/pluginhost/grpc.go         # one plugin dial, masked error in log and span
internal/pluginhost/process.go      # no re-dial after exit
internal/proto/adapter.go           # NewCostSourceClient parameter
```

## Risks

- **Shared blast radius**: two opt-ins, and `channel` in the key (research D4).
- **PR #2287** also edits `host.go` and `pool_test.go` (install identity on
  `Client`). The edits are separate; expect a small rebase.
