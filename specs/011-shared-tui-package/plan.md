# Implementation Plan: Create shared TUI package with Bubble Tea and Lip Gloss components

**Branch**: `011-shared-tui-package` | **Date**: Tue Dec 09 2025 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/011-shared-tui-package/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Create a shared internal TUI package at `internal/tui/` providing common Bubble Tea and Lip Gloss components, styles, and utilities for consistent CLI command interfaces across the FinFocus codebase.

## Technical Context

**Language/Version**: Go 1.25.7 (aligned with project go.mod)  
**Primary Dependencies**: github.com/charmbracelet/bubbletea v1.2.4, github.com/charmbracelet/bubbles v0.20.0, github.com/charmbracelet/lipgloss v1.0.0, golang.org/x/term v0.27.0, golang.org/x/text v0.21.0  
**Storage**: N/A (UI rendering package)  
**Testing**: Go testing framework with table-driven tests  
**Target Platform**: Cross-platform (Linux amd64/arm64, macOS amd64/arm64, Windows amd64)  
**Project Type**: Internal library package  
**Performance Goals**: <100ms TUI render time, <50ms component initialization  
**Constraints**: Terminal width >=40 chars, NO_COLOR environment support, CI compatibility  
**Scale/Scope**: 8+ files, 500+ LOC, used by 6+ CLI commands

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration/UI logic for the core CLI tool, not a data source plugin (compliant)
- [x] **Test-Driven Development**: Unit tests planned for all components with 80% minimum coverage (95% for critical paths) (compliant)
- [x] **Cross-Platform Compatibility**: Uses cross-platform Go dependencies, tested on Linux/macOS/Windows (compliant)
- [x] **Documentation as Code**: Code comments and usage examples included, developer-focused docs (compliant)
- [x] **Protocol Stability**: No protocol buffer changes required (compliant)
- [x] **Quality Gates**: All CI checks (lint, test, coverage, security) will pass (compliant)
- [x] **Multi-Repo Coordination**: No cross-repo dependencies (compliant)

**Violations Requiring Justification**: None - all principles satisfied

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

```text
internal/tui/
├── colors.go        # Color scheme constants and definitions
├── styles.go        # Lip Gloss style definitions for text, status, containers
├── detect.go        # TTY detection utilities with NO_COLOR/CI support
├── components.go    # Reusable UI components (status, priority, delta rendering)
├── progress.go      # Progress bar component with color coding
├── render.go        # Money and percentage formatting utilities
├── table.go         # Table configuration helpers (planned for future use)
├── spinner.go       # Loading spinner presets (planned for future use)
└── tui_test.go      # Comprehensive unit tests for all components
```

**Structure Decision**: Internal package structure following Go conventions. Located in `internal/` to prevent external imports while allowing use across CLI commands within the monorepo.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation                  | Why Needed         | Simpler Alternative Rejected Because |
| -------------------------- | ------------------ | ------------------------------------ |
| [e.g., 4th project]        | [current need]     | [why 3 projects insufficient]        |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient]  |
