# Implementation Plan: Plugin Init Recorder Fixtures

**Branch**: `001-plugin-init-recorder` | **Date**: 2026-01-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-plugin-init-recorder/spec.md`

## Summary

Deliver a recorder-compatible plugin initialization flow that fetches canonical fixtures remotely (with version pinning and offline fallback), runs core commands through the recorder plugin, and writes recorded request fixtures into the generated plugin project.

## Technical Context

**Language/Version**: Go 1.25.6  
**Primary Dependencies**: finfocus-spec SDK, Cobra CLI, zerolog, gRPC, pluginsdk  
**Storage**: Local filesystem for fixture downloads and recorded request output  
**Testing**: go test, testify/assert + require, make test, make lint  
**Target Platform**: Cross-platform CLI (Linux, macOS, Windows)  
**Project Type**: Single CLI project  
**Performance Goals**: Initialization recording completes within 5 minutes for typical plans  
**Constraints**: Support offline mode, avoid embedded fixtures, handle network failures gracefully  
**Scale/Scope**: Single-user CLI workflows; plans with hundreds of resources

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Orchestration only; recorder plugin handles request capture.
- [x] **Test-Driven Development**: Tests planned before implementation with coverage targets.
- [x] **Cross-Platform Compatibility**: CLI flow remains OS-agnostic.
- [x] **Documentation Synchronization**: README/docs updates planned with implementation.
- [x] **Protocol Stability**: No breaking protocol changes introduced.
- [x] **Implementation Completeness**: No stubs or TODOs; full workflows implemented.
- [x] **Quality Gates**: Plan includes lint/test validation.
- [x] **Multi-Repo Coordination**: Spec dependency tracked in requirements.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/001-plugin-init-recorder/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/finfocus/
internal/cli/
internal/registry/
plugins/recorder/
test/fixtures/
test/integration/
```

**Structure Decision**: Single CLI repository with recorder plugin and CLI orchestration under existing `internal/cli` and `plugins/recorder` packages.

## Complexity Tracking

No constitution violations requiring mitigation.

## Phase 0: Outline & Research

- Confirm plugin init flow extension points and fixture download patterns in the CLI.
- Validate recorder plugin configuration and request recording behavior.
- Decide on fixture sourcing strategy and offline fallback behavior.

**Output**: `research.md`

## Phase 1: Design & Contracts

- Define data model for fixture sources, recording sessions, and recorded requests.
- Draft API contracts representing fixture retrieval and plugin initialization workflows.
- Capture a quickstart implementation outline and integration checkpoints.
- Update agent context via `.specify/scripts/bash/update-agent-context.sh opencode`.

**Outputs**: `data-model.md`, `contracts/*`, `quickstart.md`

## Constitution Check (Post-Design)

- [x] Plugin-first orchestration remains intact.
- [x] TDD and quality gates remain enforceable.
- [x] Cross-platform behavior preserved.
- [x] Documentation updates still required.
- [x] Protocol stability maintained.
- [x] Implementation completeness maintained.
- [x] Multi-repo dependencies tracked.

## Phase 2: Planning

- Translate requirements into executable tasks (tests first, then implementation).
- Identify affected packages and update docs/lint/test plans.
- Produce `tasks.md` via `/speckit.tasks`.
