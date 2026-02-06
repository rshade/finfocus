# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

**Language/Version**: Markdown, Go 1.25.7 (for code verification)
**Primary Dependencies**: Jekyll (for docs site), GitHub Pages
**Storage**: Git repository (docs folder)
**Testing**: `make docs-lint` (markdown-link-check, markdownlint)
**Target Platform**: Web (GitHub Pages) / GitHub Repo View
**Project Type**: Documentation
**Performance Goals**: N/A
**Constraints**: Must match implementation in `internal/` exactly.
**Scale/Scope**: ~5 new files, ~5 updated files.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Documentation supports the plugin architecture (documenting the Analyzer and Plugin commands).
- [x] **Test-Driven Development**: Documentation will be linted (`make docs-lint`).
- [x] **Cross-Platform Compatibility**: Documentation covers cross-platform env vars.
- [x] **Documentation as Code**: This IS documentation as code.
- [x] **Protocol Stability**: Documenting the existing protocol.
- [x] **Quality Gates**: CI checks include doc linting.
- [x] **Multi-Repo Coordination**: Documenting cross-repo dependencies (plugins).

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/010-sync-docs-codebase/
├── plan.md              # This file
├── research.md          # Findings from codebase analysis
├── data-model.md        # Documentation Information Architecture
├── quickstart.md        # Draft content for Analyzer Setup
├── contracts/           # API contracts (N/A for docs, using for doc templates)
└── tasks.md             # Implementation tasks
```

### Source Code (repository root)

```text
docs/
├── architecture/
│   └── analyzer.md          # NEW: Analyzer architecture
├── getting-started/
│   └── analyzer-setup.md    # NEW: Analyzer setup guide
├── reference/
│   ├── cli-commands.md      # UPDATE: Add plugin/analyzer commands
│   ├── config-reference.md  # NEW: Config schema
│   ├── error-codes.md       # NEW: Error reference
│   └── environment-variables.md # NEW: Env var reference
├── guides/
│   ├── user-guide.md        # UPDATE: Add analyzer/cross-provider usage
│   └── developer-guide.md   # UPDATE: Add analyzer dev guide
└── README.md                # UPDATE: Sync features and links
```

**Structure Decision**: Standard Hugo/Jekyll docs structure used in repo.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | | |
