# Implementation Plan: Recommendation History

**Branch**: `1729-recommendation-history` | **Date**: 2026-10-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/1729-recommendation-history/spec.md`

## Summary

`cost recommendations history` currently reads only the local dismissal file.
finfocus-spec v0.7.7, which this repository already pins, adds an optional
`GetRecommendationHistory` RPC. The adapter will expose that call, the engine
will merge plugin events with the local record, and the command will open
installed plugins without a Pulumi plan. An optional actor is stored on the
existing lifecycle event so older dismissal files keep loading. A history
read does not write the file.

## Technical Context

**Language/Version**: Go 1.27.2
**Primary Dependencies**: finfocus-spec v0.7.7 (already pinned), Cobra, gRPC
**Storage**: Existing `dismissed.json` (optional). History reads do not write it.
**Testing**: testify, `go test` on `internal/config`, `internal/proto`, `internal/engine`, and `internal/cli`
**Target Platform**: Linux, macOS, and Windows CLI
**Project Type**: single CLI and plugin host
**Performance Goals**: One history RPC per connected plugin. No plan parse and no pricing calls.
**Constraints**: No new protocol, no provider-specific logic, no dismissal-file migration, no web page.
**Scale/Scope**: Adapter, engine merge, history command, command reference, and tests for the acceptance cases.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Core orchestrates an existing optional RPC. It does not interpret provider history.
- [x] **Test-Driven Development**: Adapter, engine, config, and CLI tests cover the acceptance cases before the behavior is considered done. No TUI view changes.
- [x] **Cross-Platform Compatibility**: File and gRPC code uses the standard library and the existing plugin host. No Unix-only calls.
- [x] **Documentation Integrity**: Command help and `docs/src/content/docs/reference/cli-commands.md` change with the command. `CLAUDE.md` records the merge rules.
- [x] **Protocol Stability**: The RPC shipped in finfocus-spec v0.7.7. Core consumes that release and does not edit protos.
- [x] **Implementation Completeness**: Full merge, fallback, and output behavior. No stubs or TODO comments.
- [x] **Persistence Model**: `dismissed.json` stays optional. The command works with an empty store. The read does not require or modify prior plugin state.
- [x] **Quality Gates**: `make validate`, `make test`, and `make lint`, plus race tests for the changed packages.
- [x] **Multi-Repo Coordination**: finfocus-spec #640 is merged and released as v0.7.7. This issue is the core half (#1816).

**Violations Requiring Justification**: None.

Post-design re-check: the same gates hold. The actor field is optional JSON,
plugin failures stay on the logging path, and cancellation is the only new
failure that replaces a successful listing.

## Project Structure

### Documentation (this feature)

```text
specs/1729-recommendation-history/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── history-command.md
│   └── adapter.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/config/dismissed.go          # optional Actor on LifecycleEvent
internal/proto/adapter.go             # GetRecommendationHistory on the client adapter
internal/engine/recommendation_history.go
internal/cli/cost_recommendations_history.go
docs/src/content/docs/reference/cli-commands.md
```

**Structure Decision**: Single Go module. Tests sit next to the packages they
exercise. Integration mocks that implement `proto.CostSourceClient` gain the
new method so they still compile; their default answer is unsupported history.

## Complexity Tracking

None.
