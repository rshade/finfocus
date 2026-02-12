# Quickstart Guide: Overview Command

**Feature**: Unified Cost Overview Dashboard  
**Date**: 2026-02-11  
**Target Audience**: Developers implementing this feature

---

## Overview

The Overview Command provides a unified, interactive dashboard combining actual costs, projected costs, and recommendations for every resource in a Pulumi stack. This guide walks you through the implementation from start to finish.

---

## Prerequisites

1. **Familiarize with Existing Codebase**:
   - `internal/cli/cost_actual.go` - Actual cost command
   - `internal/cli/cost_projected.go` - Projected cost command
   - `internal/cli/cost_recommendations.go` - Recommendations command
   - `internal/tui/cost_model.go` - Interactive TUI pattern
   - `internal/engine/types.go` - Core data types
   - `internal/ingest/state.go` - Pulumi state ingestion
   - `internal/ingest/pulumi_plan.go` - Pulumi preview ingestion

2. **Read Planning Documents**:
   - [research.md](research.md) - Technical decisions and patterns
   - [data-model.md](data-model.md) - Data structures and validation
   - [contracts/](contracts/) - API contracts and interfaces

3. **Development Environment**:
   - Go 1.22+
   - Access to a Pulumi project for testing
   - Cloud provider credentials configured (AWS, Azure, or GCP)

---

## Implementation Phases

### Phase 1: Core Data Structures

**Goal**: Define types for unified overview data.

**Files to Create**:
- `internal/engine/overview_types.go`

**Steps**:

1. **Define `OverviewRow`** (see [data-model.md](data-model.md#1-overviewrow)):
   ```go
   type OverviewRow struct {
       URN              string
       Type             string
       ResourceID       string
       Status           ResourceStatus
       ActualCost       *ActualCostData
       ProjectedCost    *ProjectedCostData
       Recommendations  []Recommendation
       CostDrift        *CostDriftData
       Error            *OverviewRowError
   }
   ```

2. **Define `ResourceStatus` enum**:
   ```go
   type ResourceStatus int
   const (
       StatusActive ResourceStatus = iota
       StatusCreating
       StatusUpdating
       StatusDeleting
       StatusReplacing
   )
   ```

3. **Define `ActualCostData`, `ProjectedCostData`, `CostDriftData`** (reuse existing `Recommendation` type).

4. **Add `Validate()` methods** for each type (see data-model.md for validation rules).

**Test Files**:
- `internal/engine/overview_types_test.go` - Unit tests for validation

**Verification**: Run `go test ./internal/engine/overview_types_test.go -v`

---

### Phase 2: Merge Logic

**Goal**: Merge Pulumi state and preview into unified overview rows.

**Files to Create**:
- `internal/engine/overview_merge.go`

**Steps**:

1. **Implement `MergeResourcesForOverview`** (see [engine-interface.md](contracts/engine-interface.md#1-mergeresourcesforoverview)):
   ```go
   func MergeResourcesForOverview(
       ctx context.Context,
       state *ingest.StackExport,
       plan *ingest.PulumiPlan,
   ) ([]OverviewRow, error)
   ```

2. **Logic**:
   - Iterate through `state.Deployment.Resources` (preserves order)
   - For each resource, check if `plan.Steps` contains matching URN
   - Map operation (`op`) to `ResourceStatus`
   - Return skeleton rows (costs nil, only URN/Type/Status populated)

3. **Add helper**: `MapOperationToStatus(op string) ResourceStatus`

**Test Files**:
- `internal/engine/overview_merge_test.go` - Table-driven tests

**Test Cases**:
- Empty state and plan
- State with no changes (all StatusActive)
- Mixed operations (create, update, delete, replace)
- New resources in plan (not in state)

**Verification**: Run `go test ./internal/engine/overview_merge_test.go -v`

---

### Phase 3: Cost Drift Calculation

**Goal**: Detect cost drift when actual spending differs from projections.

**Files to Create**:
- `internal/engine/overview_drift.go`

**Steps**:

1. **Implement `CalculateCostDrift`** (see [engine-interface.md](contracts/engine-interface.md#3-calculatecostdrift)):
   ```go
   func CalculateCostDrift(
       actualMTD, projected float64,
       dayOfMonth, daysInMonth int,
   ) (*CostDriftData, error)
   ```

2. **Formula**:
   - `extrapolated = actualMTD * (daysInMonth / dayOfMonth)`
   - `delta = extrapolated - projected`
   - `percentDrift = (delta / projected) * 100`
   - Return `CostDriftData` if `abs(percentDrift) > 10.0`

3. **Edge Cases**:
   - Day 1-2: Return error "insufficient data"
   - Projected = 0 (deleted resource): Return nil
   - Actual = 0 (new resource): Return nil

**Test Files**:
- `internal/engine/overview_drift_test.go` - Edge case tests

**Verification**: Run `go test ./internal/engine/overview_drift_test.go -v`

---

### Phase 4: Data Enrichment

**Goal**: Fetch actual costs, projected costs, and recommendations for each resource.

**Files to Create**:
- `internal/engine/overview_enrich.go`

**Steps**:

1. **Implement `EnrichOverviewRow`** (see [engine-interface.md](contracts/engine-interface.md#2-enrichoverviewrow)):
   ```go
   func EnrichOverviewRow(
       ctx context.Context,
       row *OverviewRow,
       plugins map[string]pluginhost.PluginClient,
       dateRange DateRange,
   ) (*OverviewRow, error)
   ```

2. **Logic**:
   - Call `plugin.GetActualCost()` if resource exists in state
   - Call `plugin.GetProjectedCost()` if resource has pending changes
   - Call `plugin.GetRecommendations()`
   - Calculate `CostDrift` if both actual and projected exist
   - On error, populate `row.Error` (don't fail entire operation)

3. **Implement `EnrichOverviewRows`** for concurrent enrichment (see [engine-interface.md](contracts/engine-interface.md#concurrency-strategy)):
   - Launch goroutines (limit to 10 concurrent)
   - Use `sync.WaitGroup` to track completion
   - Send updates via channel for progressive loading

**Test Files**:
- `internal/engine/overview_enrich_test.go` - Unit tests with mock plugins

**Verification**: Run `go test ./internal/engine/overview_enrich_test.go -v`

---

### Phase 5: CLI Command

**Goal**: Create the `finfocus overview` command.

**Files to Create**:
- `internal/cli/overview.go`

**Steps**:

1. **Implement `NewOverviewCmd`** (see [cli-interface.md](contracts/cli-interface.md#function-signature)):
   ```go
   func NewOverviewCmd() *cobra.Command
   ```

2. **Add Flags** (see [cli-interface.md](contracts/cli-interface.md#command-flags)):
   - `--pulumi-json`
   - `--pulumi-state`
   - `--from`, `--to`
   - `--adapter`, `--output`, `--filter`
   - `--plain`, `--yes`, `--no-pagination`

3. **Implement `executeOverview`**:
   - Load Pulumi state and preview
   - Detect pending changes (optimization per FR-008)
   - Display pre-flight confirmation (unless `--yes`)
   - Merge resources
   - Detect TTY: launch TUI or render plain table
   - Open plugins
   - Enrich rows (concurrent)
   - Render output

4. **Add to root command**: In `internal/cli/root.go`, add `NewOverviewCmd()` to `cmd.AddCommand(...)`.

**Test Files**:
- `internal/cli/overview_test.go` - Unit tests for flag parsing, validation

**Verification**: Run `go test ./internal/cli/overview_test.go -v`

---

### Phase 6: Interactive TUI

**Goal**: Build the interactive Bubble Tea dashboard.

**Files to Create**:
- `internal/tui/overview_model.go`
- `internal/tui/overview_view.go`
- `internal/tui/overview_detail.go`

**Steps**:

1. **Implement `OverviewModel`** (see [tui-interface.md](contracts/tui-interface.md#overview-model)):
   ```go
   type OverviewModel struct {
       state      ViewState
       allRows    []OverviewRow
       rows       []OverviewRow
       table      table.Model
       textInput  textinput.Model
       detailView *DetailViewModel
       // ... other fields
   }
   ```

2. **Implement `NewOverviewModel`**:
   - Initialize table columns
   - Set initial state to `ViewStateLoading`
   - Return command to start loading

3. **Implement `Update`** (see [tui-interface.md](contracts/tui-interface.md#2-update-bubble-tea-interface)):
   - Handle `resourceLoadedMsg`: Update row in `allRows`
   - Handle `loadingProgressMsg`: Update progress banner
   - Handle `allResourcesLoadedMsg`: Hide banner, transition to `ViewStateList`
   - Handle `tea.KeyMsg`: Keyboard navigation (see [tui-interface.md](contracts/tui-interface.md#4-keyboard-shortcuts))

4. **Implement `View`** (see [tui-interface.md](contracts/tui-interface.md#3-view-bubble-tea-interface)):
   - Render progress banner (if loading)
   - Render table (if list state)
   - Render detail view (if detail state)

5. **Implement Sorting, Filtering, Pagination** (see [tui-interface.md](contracts/tui-interface.md#5-sorting)).

**Test Files**:
- `internal/tui/overview_model_test.go` - State transition tests

**Verification**: Run `go test ./internal/tui/overview_model_test.go -v`

---

### Phase 7: Output Rendering

**Goal**: Render JSON, NDJSON, and plain table formats.

**Files to Create**:
- `internal/engine/overview_render.go`

**Steps**:

1. **Implement `RenderOverviewAsJSON`** (see [output-format.md](contracts/output-format.md#json-format---output-json)):
   ```go
   func RenderOverviewAsJSON(
       rows []OverviewRow,
       metadata StackContext,
   ) (string, error)
   ```

2. **Implement `RenderOverviewAsNDJSON`** (see [output-format.md](contracts/output-format.md#ndjson-format---output-ndjson)):
   - One row per line (no metadata wrapper)

3. **Implement `RenderOverviewAsTable`** (see [output-format.md](contracts/output-format.md#table-format-plain-mode)):
   - ASCII table with fixed-width columns
   - Summary footer

**Test Files**:
- `internal/engine/overview_render_test.go` - Golden file tests

**Verification**: Run `go test ./internal/engine/overview_render_test.go -v`

---

### Phase 8: Integration Tests

**Goal**: End-to-end testing with fixture data.

**Files to Create**:
- `internal/cli/overview_integration_test.go`

**Steps**:

1. **Create Test Fixtures** (see [research.md](research.md#9-testing-strategy)):
   - `testdata/overview/state-no-changes.json`
   - `testdata/overview/state-mixed-changes.json`
   - `testdata/overview/plan-no-changes.json`
   - `testdata/overview/plan-mixed-changes.json`

2. **Write Integration Tests**:
   - Full command execution with fixture files
   - Verify exit codes
   - Verify output formats (JSON, table)
   - Test error scenarios (missing files, invalid dates)

**Verification**: Run `go test ./internal/cli/overview_integration_test.go -v`

---

### Phase 9: Documentation

**Goal**: Update user-facing documentation.

**Files to Update**:
- `README.md` - Add overview command example
- `docs/commands/overview.md` - Full command reference

**Steps**:

1. **Add to README.md**:
   ```markdown
   ### Unified Cost Overview
   
   View actual costs, projected costs, and recommendations in a single dashboard:
   
   ```bash
   finfocus overview
   ```
   
   Interactive TUI with progressive loading, filtering, and detail views.
   ```

2. **Create `docs/commands/overview.md`**:
   - Command usage
   - Flag descriptions
   - Examples
   - Screenshots (if applicable)

---

## Testing Checklist

- [ ] Unit tests for all core functions (80%+ coverage)
- [ ] Table-driven tests for edge cases
- [ ] Integration tests with fixture data (95%+ coverage for critical paths)
- [ ] Golden file tests for output formats
- [ ] Benchmark tests for performance
- [ ] Manual testing with real Pulumi stack

---

## Performance Verification

Run benchmarks to verify performance targets:

```bash
go test -bench=. -benchmem ./internal/engine/
```

**Targets** (from research.md):
- `BenchmarkMergeResourcesForOverview`: <5ms for 100 resources
- `BenchmarkCalculateDrift`: <10ms for 1000 calculations
- `BenchmarkRenderTable`: <50ms for 250 resources

---

## Constitution Compliance

Verify compliance with [.specify/memory/constitution.md](../../.specify/memory/constitution.md):

- [ ] **Plugin-First Architecture**: Not applicable (CLI command, not plugin)
- [ ] **Test-Driven Development**: Tests written before implementation (80%+ coverage)
- [ ] **Cross-Platform Compatibility**: Go code compiles on Linux, macOS, Windows
- [ ] **Documentation Integrity**: README and docs/ updated concurrently
- [ ] **Protocol Stability**: Not applicable (no protocol changes)
- [ ] **Implementation Completeness**: No stubs, no TODOs, full implementation
- [ ] **Quality Gates**: `make lint` and `make test` pass before claiming complete

---

## Debugging Tips

### TUI Not Rendering

1. Check TTY detection: `term.IsTerminal(os.Stdout.Fd())`
2. Verify Bubble Tea initialization: `tea.NewProgram(model).Run()`
3. Use `--debug` flag to enable logging to file

### Costs Not Loading

1. Check plugin authentication (cloud credentials)
2. Verify plugin versions are compatible (`finfocus plugin list`)
3. Check audit logs in `~/.finfocus/logs/`

### Performance Issues

1. Enable profiling: `go test -cpuprofile=cpu.prof -memprofile=mem.prof`
2. Analyze with `go tool pprof cpu.prof`
3. Check concurrent goroutine limit (default 10)

---

## References

- **Research**: [research.md](research.md)
- **Data Model**: [data-model.md](data-model.md)
- **Contracts**: [contracts/](contracts/)
- **Feature Spec**: [spec.md](spec.md)
- **Constitution**: [.specify/memory/constitution.md](../../.specify/memory/constitution.md)

---

## Next Steps

After completing implementation:

1. Run full test suite: `make test`
2. Run linter: `make lint`
3. Manual testing with real Pulumi stack
4. Update CHANGELOG.md
5. Create PR with summary of changes

---

**Quickstart Version**: v1.0.0  
**Last Updated**: 2026-02-11
