# Overview Command API Contracts

**Feature**: Unified Cost Overview Dashboard  
**Date**: 2026-02-11

---

## Overview

This directory contains API contracts for the Overview Command feature. Since this is a CLI command (not a REST/GraphQL API), these contracts define the internal Go interfaces, function signatures, and data exchange formats between components.

---

## Contract Documents

1. **[cli-interface.md](cli-interface.md)** - CLI command interface and flags
2. **[engine-interface.md](engine-interface.md)** - Core engine functions for data merging and enrichment
3. **[tui-interface.md](tui-interface.md)** - Interactive TUI component interface
4. **[output-format.md](output-format.md)** - JSON/NDJSON/Table output schemas

---

## Contract Versioning

All contracts follow semantic versioning aligned with the FinFocus core version:
- **Breaking changes**: Major version bump
- **New optional features**: Minor version bump
- **Bug fixes/clarifications**: Patch version bump

**Initial Version**: v1.0.0

---

## Validation Rules

All contracts MUST:
1. Be testable with unit tests
2. Have clear input/output specifications
3. Define error conditions and error types
4. Specify performance expectations (where applicable)
5. Be documented with godoc comments in implementation

---

## References

- **Data Model**: `../data-model.md`
- **Research**: `../research.md`
- **Constitution**: `/.specify/memory/constitution.md`
