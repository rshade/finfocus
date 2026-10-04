# Implementation Plan: Pass include_dismissed From cost recommendations

**Branch**: `622-include-dismissed` | **Date**: 2026-10-04 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/622-include-dismissed/spec.md`

## Summary

`--include-dismissed` calls a new engine method that sets `IncludeDismissed` on the plugin request
and appends `/include-dismissed` to the recommendations cache key. Excluded IDs are still loaded
and sent. The default method, the overview, and the analyzer are unchanged. The adapter copies the
bool onto `pbc.GetRecommendationsRequest`. The module requires the finfocus-spec commit that adds
field 8 (`specs/599-include-dismissed`).

## Technical Context

**Language/Version**: Go 1.27.1

**Primary Dependencies**: finfocus-spec (field `include_dismissed`), Cobra, existing engine cache

**Storage**: N/A (cache key suffix only; no new store)

**Testing**: testify tests in `internal/engine` and `internal/proto`

**Target Platform**: The finfocus CLI on Linux, macOS, and Windows

**Project Type**: CLI orchestrator

**Performance Goals**: No extra plugin call on a repeated request of the same kind

**Constraints**: Do not change `GetRecommendationsForResources`'s signature. Do not drop excluded
IDs when the flag is set. Default cache keys stay byte-compatible with today's keys.

**Scale/Scope**: One engine method, one adapter field, one CLI branch, docs for the existing flag

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Core only forwards a protocol field. No provider pricing.
- [x] **Test-Driven Development**: Engine and adapter tests are written before the wiring. No TUI
  change, so no golden file.
- [x] **Cross-Platform Compatibility**: No OS-specific code.
- [x] **Documentation Integrity**: Command reference, flag help, ROADMAP, and CLAUDE.md update with
  the code.
- [x] **Protocol Stability**: The field is additive in finfocus-spec. Core consumes the published
  getter and does not fork the proto.
- [x] **Implementation Completeness**: The flag, both fetch paths, and the cache key are finished.
  No stubs.
- [x] **Persistence Model**: No new store. The cache remains optional and deletable.
- [x] **Quality Gates**: `make test` and `make lint` before the PR.
- [x] **Multi-Repo Coordination**: Depends on finfocus-spec `599-include-dismissed`. No hop until
  `pluginsdk.SpecVersion` moves past v0.7.4.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/622-include-dismissed/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/engine/engine.go
internal/engine/recommendations_include_dismissed_test.go
internal/proto/adapter.go
internal/proto/adapter_recommendations_test.go
internal/cli/cost_recommendations.go
docs/src/content/docs/reference/cli-commands.md
docs/src/content/docs/architecture/roadmap.md
ROADMAP.md
CLAUDE.md
go.mod
go.sum
```

## Complexity Tracking

No constitution violations.
