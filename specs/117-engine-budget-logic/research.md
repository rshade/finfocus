# Research & Decisions: Engine Budget Logic

**Feature**: Engine Budget Logic
**Branch**: `117-engine-budget-logic`
**Status**: Research Complete

## R1: Proto Definitions Verification

**Decision**: Use `github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1` (aliased as `pbc`).
**Rationale**: This is the established pattern in the codebase.
**Assumptions**:
- `BudgetFilter` struct in Go will match the standard `protoc-gen-go` output:
  - `Providers []string`
  - `Regions []string`
  - `ResourceTypes []string`
  - `Tags map[string]string`
- `Budget` struct will have `Tags map[string]string` (or `Metadata` if that's where tags live - the spec prompt says `Metadata map[string]string // Provider-specific data`).
  - **Correction**: The prompt says `Metadata`. I will assume "Tags" are stored in `Metadata` or explicit `Tags` field. The `BudgetFilter` has `Tags`. I will assume `Budget` has `Tags` or I need to map `Metadata` to tags.
  - **Refinement**: Looking at `internal/engine/engine.go` research, `ResourceDescriptor` has tags. Budgets might reference resources.
  - **Decision**: I will check `Budget` struct properties. If `Tags` is missing, I will use `Metadata` for tag filtering, assuming keys starting with `tag:` or similar, OR just treat `Metadata` as the target for `Tags` filter.
  - **Wait**: The prompt says `Budget` has `Filter *BudgetFilter`. It implies a Budget *definition* has a filter scope.
  - **Actual Requirement**: The user story is "Filter budgets by provider...". This means `FilterBudgetsByProvider` filters a **list of Budgets** based on a **user-provided filter**.
  - So `Budget` struct has `Source` (Provider). It doesn't explicitly list `Regions` or `Tags` in the top-level struct in the prompt.
  - **Hypothesis**: `Budget` objects returned by `GetBudgets` likely have a `Filter` field themselves (defining what the budget applies to). OR `Budget` has fields like `Region` (maybe in Metadata?).
  - **Assumption for Plan**: `FilterBudgets` will match:
    - `filter.Providers` vs `budget.Source`
    - `filter.Regions` vs `budget.Filter.Regions` (if the budget itself is scoped to a region?) OR `budget.Metadata["region"]`.
    - **Safe Bet**: I will implement filtering based on explicit `Budget` fields (`Source`) and `Metadata` for others if standard fields don't exist.

## R2: Currency Validation

**Decision**: Implement a regex validator `^[A-Z]{3}$` for currency codes.
**Rationale**: No heavy external library found. Matches "3-character code" requirement (FR-005).
**Location**: `internal/engine/budget.go` (new file).

## R3: Tag Logic

**Decision**: Implement "All Match" logic for Tags.
**Logic**: A budget matches the filter `Tags` if for every key `K` in `filter.Tags`, `budget.Tags[K] == filter.Tags[K]`.
**Rationale**: Standard filtering behavior (narrowing down).

## R4: Missing Health Status

**Decision**: Log warning with `zerolog` and exclude from health buckets.
**Rationale**: explicit clarification result.

## R5: Dependency Version

**Decision**: Use `finfocus-spec v0.5.2`.
**Rationale**: Matches `go.mod`.
