# Implementation Plan: Integration Tests for --filter Flag

**Branch**: `020-cost-filter-tests` | **Date**: Tue Dec 16 2025 | **Spec**: [spec.md](../spec.md)
**Input**: Feature specification from `/specs/020-cost-filter-tests/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implement comprehensive integration tests for the `--filter` flag across `cost projected` and `cost actual` commands. Tests will validate filtering by resource type, provider, and tags using actual exported Pulumi plan fixtures with 10-20 resources. The implementation will extend the existing integration test infrastructure in `test/integration/cli/` to ensure the filter functionality works correctly across all output formats and handles edge cases gracefully.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25.7 (from AGENTS.md and go.mod)
**Primary Dependencies**: Existing CLI infrastructure (`internal/cli/`, `internal/engine/`), test helpers (`test/integration/helpers/`)
**Storage**: N/A (CLI tool processing JSON files)
**Testing**: Go testing framework with testify assertions (existing pattern in test/integration/cli/)
**Target Platform**: Cross-platform (Linux, macOS, Windows - constitution requirement)
**Project Type**: CLI tool (single binary)
**Performance Goals**: Test suite execution time not to increase by more than 10 seconds (SC-003)
**Constraints**: Filter operations should not degrade significantly compared to unfiltered queries (NFR-001); case-sensitive filtering
**Scale/Scope**: 10-20 resources per test plan fixture

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic testing existing CLI/engine functionality - no plugin changes required
- [x] **Test-Driven Development**: Tests are the primary deliverable, implementing missing integration test coverage for existing functionality (80% minimum coverage targeted)
- [x] **Cross-Platform Compatibility**: CLI tool runs on Linux, macOS, Windows - existing cross-platform support maintained
- [x] **Documentation as Code**: Test implementation includes documentation through code comments and test naming (no separate docs needed for internal tests)
- [x] **Protocol Stability**: No protocol changes - testing existing functionality
- [x] **Quality Gates**: Implementation will follow `make lint` and `make test` requirements (SC-002, linting protocol)
- [x] **Multi-Repo Coordination**: No cross-repo dependencies - all work within finfocus-core

**Violations Requiring Justification**: (Fill only if any principle is violated)

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
# [REMOVE IF UNUSED] Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# [REMOVE IF UNUSED] Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# [REMOVE IF UNUSED] Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure: feature modules, UI flows, platform tests]
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation                  | Why Needed         | Simpler Alternative Rejected Because |
| -------------------------- | ------------------ | ------------------------------------ |
| [e.g., 4th project]        | [current need]     | [why 3 projects insufficient]        |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient]  |
