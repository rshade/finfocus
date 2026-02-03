# Research: Tag-Based Budget Filtering

**Feature**: 222-budget-tag-filter
**Date**: 2026-02-02

## Research Questions

### 1. Existing Tag Filtering Patterns

**Question**: How does the codebase currently handle tag-based filtering?

**Findings**:

1. **Resource Filtering** (`internal/engine/engine.go:1849`):
   - `matchesProperties()` handles `tag:key` prefix by stripping it and searching Properties map
   - Uses substring matching for values

2. **Budget Filtering** (`internal/engine/budget.go:117-127`):
   - `matchesBudgetFilter()` already implements tag filtering for `pbc.BudgetFilter`
   - Uses exact match: `metaVal != val` (line 124)
   - Tags are stored with `tag:` prefix in metadata key

3. **CLI Parsing** (`internal/cli/cost_actual.go:363`):
   - `parseTagFilter()` parses `tag:key=value` syntax
   - Returns `map[string]string` for tags

**Decision**: Reuse the existing `parseTagFilter()` pattern for CLI parsing and adapt `matchesBudgetFilter()` logic for the Go-native `BudgetFilterOptions` struct.

**Rationale**: Consistency with existing patterns reduces cognitive load and ensures predictable behavior.

**Alternatives Considered**:

- Creating entirely new filter parsing logic - rejected for inconsistency
- Using regex instead of glob patterns - rejected per spec (glob is sufficient)

---

### 2. Glob Pattern Matching Implementation

**Question**: What's the best approach for glob pattern matching in Go?

**Findings**:

1. **Standard Library**: `path.Match()` supports basic glob patterns:
   - `*` matches any sequence of characters
   - `?` matches single character
   - `[abc]` matches character class
   - Pattern must match entire string

2. **Limitations**:
   - `path.Match()` is designed for file paths
   - No support for `**` recursive matching (not needed)

3. **Alternative**: `filepath.Match()` - identical behavior but platform-specific

**Decision**: Use `path.Match()` for glob pattern matching.

**Rationale**: Standard library, no external dependencies, sufficient for the `*` wildcard requirement.

**Alternatives Considered**:

- `github.com/gobwas/glob` - more powerful but adds dependency
- Custom implementation - unnecessary complexity
- `regexp` - too powerful, harder syntax for users

---

### 3. BudgetFilterOptions vs pbc.BudgetFilter

**Question**: Should we extend `BudgetFilterOptions` or use `pbc.BudgetFilter` directly?

**Findings**:

1. **Current State**:
   - `BudgetFilterOptions` (Go struct): Only has `Providers []string`
   - `pbc.BudgetFilter` (protobuf): Has Providers, Regions, ResourceTypes, Tags
   - `FilterBudgets()` uses `pbc.BudgetFilter`
   - `Engine.GetBudgets()` uses `BudgetFilterOptions`

2. **Gap Analysis**:
   - `GetBudgets()` only passes `filter.Providers` to `FilterBudgetsByProvider()`
   - The full `FilterBudgets()` function with tag support is not used by the engine

**Decision**: Extend `BudgetFilterOptions` with `Tags map[string]string` and update `GetBudgets()` to use the full `FilterBudgets()` function with a converted `pbc.BudgetFilter`.

**Rationale**: Provides Go-native API while leveraging existing protobuf-based filtering logic.

**Alternatives Considered**:

- Replace `BudgetFilterOptions` with `pbc.BudgetFilter` - breaks Go API design patterns
- Duplicate filtering logic in `BudgetFilterOptions` - DRY violation

---

### 4. CLI Filter Parsing Unification

**Question**: How should we parse `--filter` flags for budget commands?

**Findings**:

1. **Current State**:
   - `cost projected` has `--filter` flag for resource filtering
   - `cost actual` has `--filter` flag (different implementation)
   - No budget-specific filter parsing exists

2. **Filter Syntax**:
   - Provider: `provider=kubecost`
   - Tag: `tag:namespace=production`
   - Multiple filters: Multiple `--filter` flags (AND logic for tags)

**Decision**: Create `parseBudgetFilters()` function that builds `BudgetFilterOptions` from `[]string` filter flags.

**Rationale**: Centralized parsing enables consistent behavior across commands.

**Alternatives Considered**:

- Per-command parsing - leads to inconsistency
- Using protobuf BudgetFilter directly from CLI - awkward API

---

### 5. Error Handling for Malformed Filters

**Question**: How should malformed filter syntax be handled?

**Findings**:

1. **Spec Requirement** (FR-010): CLI MUST validate filter syntax and exit with descriptive error
2. **Current Patterns**:
   - `parseTagFilter()` silently ignores malformed input
   - Resource filtering returns `true` for invalid filters (includes all)

**Decision**: Implement explicit validation in `parseBudgetFilters()`:

- Missing `=` in `tag:key` → error
- Empty key after `tag:` → error
- Unknown filter type → warning (for forward compatibility)

**Rationale**: Fail-fast prevents user confusion from unexpected results.

**Alternatives Considered**:

- Silent ignore (current behavior) - confusing when filter doesn't work
- Strict rejection of unknown types - breaks forward compatibility

---

## Implementation Strategy

Based on research findings:

1. **Engine Layer** (`internal/engine/budget.go`):
   - Add `Tags map[string]string` to `BudgetFilterOptions`
   - Add `matchesBudgetTags()` with `path.Match()` for glob support
   - Update `GetBudgets()` to convert `BudgetFilterOptions` → `pbc.BudgetFilter` and call `FilterBudgets()`

2. **CLI Layer** (`internal/cli/`):
   - Add `parseBudgetFilters()` function
   - Add `validateBudgetFilter()` for syntax validation
   - Wire `--filter` flag to budget commands (if applicable)

3. **Testing**:
   - Unit tests for `matchesBudgetTags()` with exact/glob patterns
   - Unit tests for `parseBudgetFilters()` with valid/invalid inputs
   - Integration tests for CLI end-to-end filtering

## Key Code References

| File | Function | Line | Purpose |
|------|----------|------|---------|
| `internal/engine/budget.go` | `BudgetFilterOptions` | 27-30 | Struct to extend |
| `internal/engine/budget.go` | `matchesBudgetFilter` | 93-130 | Tag matching logic to adapt |
| `internal/engine/budget.go` | `FilterBudgetsByProvider` | 270-298 | Current filtering (provider-only) |
| `internal/engine/budget.go` | `GetBudgets` | 316-384 | Entry point to modify |
| `internal/cli/cost_actual.go` | `parseTagFilter` | 363-376 | Tag parsing pattern to reuse |
| `internal/engine/engine.go` | `matchesProperties` | 1849-1888 | Resource tag filtering reference |
