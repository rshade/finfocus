# Implementation Plan: Tag-Based Budget Filtering

**Branch**: `222-budget-tag-filter` | **Date**: 2026-02-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/222-budget-tag-filter/spec.md`

## Summary

Extend the `BudgetFilterOptions` struct to support tag-based filtering via a `Tags map[string]string` field. This enables filtering budgets by any metadata key (e.g., `namespace`, `cluster`, `environment`) using the existing `--filter "tag:key=value"` CLI syntax. The feature reuses existing patterns from resource filtering (`matchesProperties`) and budget filtering (`matchesBudgetFilter` which already supports tags via protobuf).

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: github.com/spf13/cobra (CLI), github.com/rshade/finfocus-spec (proto), path (glob matching)
**Storage**: N/A (filtering in-memory)
**Testing**: go test with testify (assert/require), table-driven tests
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single project (CLI tool)
**Performance Goals**: 3 seconds for 10,000 budgets (per SC-001)
**Constraints**: 100% backward compatibility, 90%+ test coverage
**Scale/Scope**: Up to 10,000 budgets per query

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in core, not a plugin. Filtering happens after plugin data retrieval.
- [x] **Test-Driven Development**: Tests planned for all new functions (90%+ coverage target)
- [x] **Cross-Platform Compatibility**: Pure Go code, no platform-specific dependencies
- [x] **Documentation Integrity**: CLAUDE.md will be updated with new filter pattern
- [x] **Protocol Stability**: No protocol changes required (uses existing Budget.Metadata)
- [x] **Implementation Completeness**: Full implementation planned, no stubs
- [x] **Quality Gates**: make lint and make test will pass
- [x] **Multi-Repo Coordination**: No cross-repo changes needed (uses existing proto)

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/222-budget-tag-filter/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no API changes)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── engine/
│   ├── budget.go           # MODIFY: Extend BudgetFilterOptions, add matchesBudgetTags
│   └── budget_test.go      # MODIFY: Add tag filtering tests
├── cli/
│   ├── cost_budget.go      # REVIEW: Budget rendering (no changes needed)
│   ├── cost_actual.go      # REVIEW: parseTagFilter exists, wire to budget filtering
│   └── filters.go          # NEW: Centralized filter parsing for budget commands
└── ...

test/
├── unit/
│   └── engine/
│       └── budget_filter_test.go  # NEW: Comprehensive tag filter tests
└── integration/
    └── budget_tag_filter_test.go  # NEW: End-to-end CLI tests
```

**Structure Decision**: Modify existing files (`budget.go`, `budget_test.go`) for engine changes. CLI filter parsing will be extracted to a dedicated file for reuse across commands.

## Complexity Tracking

> No constitution violations. Feature is a straightforward extension of existing patterns.
