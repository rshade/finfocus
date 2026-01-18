# Phase 0: Research & Design Decisions

**Feature**: v0.2.1 Developer Experience Improvements
**Date**: 2026-01-17

## 1. Concurrency Model for Plugin Listing

**Decision**: Use `golang.org/x/sync/errgroup` with a semaphore pattern (via `SetLimit`).

**Rationale**:
- **Structured Concurrency**: `errgroup` manages the lifecycle of multiple goroutines and propagates the first error encountered, simplifying error handling compared to raw `sync.WaitGroup`.
- **Resource Safety**: `SetLimit` allows us to bound concurrency (to `runtime.NumCPU()`) preventing resource exhaustion (file descriptors, thread context switching) on systems with many installed plugins.
- **Context Propagation**: Integrates natively with `context.Context` for cancellation (e.g., if one plugin hangs or user sends SIGINT).

**Alternatives Considered**:
- **Raw `go` routines + `sync.WaitGroup`**: Harder to handle error propagation and cancellation correctly.
- **Worker Pool (buffered channel)**: Valid, but `errgroup` is more idiomatic for "scatter-gather" tasks where we wait for all results.

## 2. Plugin Cleanup Strategy (`--clean`)

**Decision**: Implement "Swap and Sweep" logic.
1. Install the new version fully to a temporary/staging location or side-by-side directory.
2. Verify installation success.
3. If `--clean` is set, identify all *other* version directories for that plugin in the registry.
4. Execute `os.RemoveAll` on them.

**Rationale**:
- **Safety**: Only removing old versions *after* success ensures we don't leave the user with zero working versions if the upgrade fails.
- **Simplicity**: Deleting specific version directories is safer than manipulating a "current" symlink (though we likely use symlinks, cleaning the actual data directories is the goal).

**Windows Consideration**: Ensure no processes are holding locks on the old version binaries. Since plugins are subprocesses spawned on demand, they should not be running during an install command unless the user has a background process (unlikely for CLI).

## 3. Shared Filter Logic Location

**Decision**: `internal/cli/filters.go`

**Rationale**:
- **Proximity**: The logic couples CLI flags (`[]string`) to Engine calls. It belongs in the CLI layer as an adapter/helper, not in the core Engine (which shouldn't know about CLI flag parsing quirks) or a generic `pkg` (too specific to this app's resource model).

## 4. `GetPluginInfo` Integration

**Decision**:
- Add `GetPluginInfo` method to `pluginhost.Client`.
- Call it immediately during client initialization/handshake.
- Use a short timeout (5s default).
- Map gRPC errors: `Unimplemented` -> "Legacy".

**Rationale**:
- **Early Validation**: Checking compatibility at connection time prevents runtime errors later when asking the plugin to perform work.
- **UX**: Distinguishing "Legacy" (old but valid) from "Incompatible" (new but wrong protocol) is crucial for user trust.
