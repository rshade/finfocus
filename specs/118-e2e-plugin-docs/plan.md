# Implementation Plan: Update documentation for E2E testing and plugin ecosystem

**Branch**: `118-e2e-plugin-docs` | **Date**: 2026-01-19 | **Spec**: [specs/118-e2e-plugin-docs/spec.md](specs/118-e2e-plugin-docs/spec.md)
**Input**: Feature specification from `/specs/118-e2e-plugin-docs/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

This feature updates the project documentation to accurately reflect the new E2E testing capabilities, plugin ecosystem architecture, and troubleshooting procedures. It involves creating comprehensive guides for testing setup, explaining the relationship between Core and Plugins (Public vs. CostExplorer), and detailing cost calculation workflows. The documentation will be organized by category within the `docs/` directory (`testing/`, `architecture/`, `guides/`) and will include Mermaid architecture diagrams and a plugin compatibility matrix.

## Technical Context

**Language/Version**: Markdown (GFM), Mermaid (for diagrams)
**Primary Dependencies**: Jekyll (for site generation), mermaid.js (for rendering diagrams)
**Storage**: Git repository (docs folder)
**Testing**: `markdownlint-cli2` for linting, manual verification of `make test-e2e` steps
**Target Platform**: GitHub Pages / Documentation Site
**Project Type**: Documentation
**Performance Goals**: N/A
**Constraints**: Links must be relative for local browsing but compatible with the hosted site structure.
**Scale/Scope**: ~5-10 new/updated markdown files.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: N/A (Documentation update, but accurately describes this architecture).
- [x] **Test-Driven Development**: N/A (Documentation update, but instructions enable TDD for users).
- [x] **Cross-Platform Compatibility**: Documentation covers cross-platform testing (Linux, macOS, Windows).
- [x] **Documentation Synchronization**: This feature *is* the synchronization of documentation with recent implementation changes.
- [x] **Protocol Stability**: N/A
- [x] **Implementation Completeness**: Documentation will be complete and cover all specified areas.
- [x] **Quality Gates**: Will pass markdown linting (`markdownlint-cli2`).
- [x] **Multi-Repo Coordination**: Describes the multi-repo ecosystem (Core, Spec, Plugin).

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/118-e2e-plugin-docs/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
docs/
├── architecture/
│   ├── ecosystem.md          # NEW: Plugin ecosystem architecture and relationships
│   └── diagrams/             # (Optional) Assets if not pure Mermaid
├── testing/
│   ├── e2e-guide.md          # NEW: Comprehensive E2E testing guide
│   └── test-scenarios.md     # NEW: Detailed test scenarios (if split from guide)
├── guides/
│   ├── troubleshooting.md    # NEW: Common issues and solutions
│   └── cost-calculation.md   # NEW: Explanation of cost workflows (Projected vs Actual)
├── reference/
│   └── plugin-compatibility.md # NEW: Plugin feature matrix
├── index.md                  # UPDATE: Link to new sections
└── README.md                 # UPDATE: Core README with quick links

plugins/
└── aws-public/
    └── README.md             # UPDATE: Clarify role as fallback plugin (if in mono-repo or simulated)
```

**Structure Decision**: Adopting the `docs/category/file.md` structure as clarified in the specification to improve organization and scalability.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | | |