# Research: Unified Cost Overview Dashboard

**Feature**: Overview Command  
**Date**: 2026-02-11  
**Purpose**: Resolve all technical unknowns from Technical Context and establish design decisions for the implementation plan.

---

## Executive Summary

The Overview Command feature requires merging data from three existing pipelines (actual costs, projected costs, and recommendations) into a unified interactive dashboard. This research establishes architectural patterns, performance strategies, and UI/UX decisions to deliver a responsive, production-ready experience for stacks of all sizes.

---

## 1. Data Merging Strategy

### Decision: Resource-Centric Merge with URN as Key

**Rationale**:
- Pulumi URN is the canonical identifier across state and preview
- Each resource may have multiple data facets: actual cost, projected cost, recommendations
- Merge at the resource level allows partial data (e.g., only actual costs available)

**Implementation Pattern**:
```go
type OverviewRow struct {
    URN              string
    Type             string
    Status           ResourceStatus  // active, creating, updating, deleting, replacing
    ActualCost       *ActualCostData // nil if not available
    ProjectedCost    *ProjectedCostData // nil if no pending changes
    Recommendations  []Recommendation
    CostDrift        *CostDriftData // nil if projected unavailable
}

type ResourceStatus int
const (
    StatusActive ResourceStatus = iota
    StatusCreating
    StatusUpdating
    StatusDeleting
    StatusReplacing
)
```

**Alternatives Considered**:
- **Three separate tables**: Rejected because it violates the core requirement of unified view
- **Plugin-level merge**: Rejected because URN mapping happens in the ingest layer, not plugins

**Data Sources**:
1. **Pulumi State** (`pulumi stack export`) → actual resources
2. **Pulumi Preview** (`pulumi preview --json`) → pending changes
3. **Plugin APIs** → actual costs, projected costs, recommendations

---

## 2. Progressive Loading Architecture

### Decision: Streaming Model with Bubble Tea Message Bus

**Rationale**:
- Bubble Tea (existing TUI framework) supports async message passing
- Each resource can update independently without blocking
- Matches existing `tui.CostViewModel` pattern

**Implementation Pattern**:
```go
// Messages for progressive updates
type resourceLoadedMsg struct {
    URN    string
    Data   OverviewRow
}

type loadingProgressMsg struct {
    Loaded int
    Total  int
}

type allResourcesLoadedMsg struct{}

// Model Update cycle
func (m OverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case resourceLoadedMsg:
        m.rows[msg.URN] = msg.Data
        m.loadedCount++
        return m, nil
    case loadingProgressMsg:
        m.progress = msg
        return m, nil
    }
}
```

**Concurrency Strategy**:
- Launch goroutines for each resource's cost queries
- Use `sync.WaitGroup` to track completion
- Send updates via channels → Bubble Tea Cmd
- Reuse existing `engine.CalculateProjectedCostsWithContext` concurrency model

**Alternatives Considered**:
- **Batch loading (wait for all)**: Rejected per FR-002 (immediate UI render requirement)
- **Server-Sent Events (SSE)**: Over-engineered for local CLI tool
- **Worker pool pattern**: Already implemented in engine layer, no need to duplicate

**Performance Targets**:
- Initial render: <500ms (FR-002)
- Per-resource update: <50ms UI refresh
- Concurrent queries: 10 resources in parallel (existing engine limit)

---

## 3. Cost Drift Detection

### Decision: 10% Threshold with Extrapolation to Full Month

**Rationale**:
- FR-004 mandates 10% threshold
- Must compare apples-to-apples (monthly costs)
- Actual costs are MTD (month-to-date), projected are monthly

**Calculation Formula**:
```go
func CalculateCostDrift(actual, projected float64, dayOfMonth int) *CostDriftData {
    daysInMonth := 30 // simplified, use time.Now().AddDate(0,1,-1).Day() in production
    extrapolatedMonthly := actual * (float64(daysInMonth) / float64(dayOfMonth))
    
    delta := extrapolatedMonthly - projected
    percentDrift := (delta / projected) * 100
    
    if math.Abs(percentDrift) > 10.0 {
        return &CostDriftData{
            ExtrapolatedMonthly: extrapolatedMonthly,
            Projected:          projected,
            PercentDrift:       percentDrift,
            IsWarning:          true,
        }
    }
    return nil
}
```

**Edge Cases**:
- Day 1 of month: Extrapolation unreliable, show warning "Insufficient data (day 1)"
- Deleted resources: Actual cost exists, projected is $0.00 → don't calculate drift
- New resources: No actual cost, only projected → don't calculate drift

**Alternatives Considered**:
- **Daily rate comparison**: Rejected because cloud billing varies daily (weekday/weekend)
- **Historical trend analysis**: Out of scope for v1, noted in ROADMAP.md

---

## 4. Interactive UI Framework

### Decision: Extend Existing `tui.CostViewModel` Pattern

**Rationale**:
- Existing `cost recommendations` command has proven TUI implementation
- Bubble Tea table component supports sorting, filtering, navigation
- Reuse `tui/table.go`, `tui/styles.go` for consistency

**Key Components**:
1. **Main Table View**:
   - Columns: Resource ID, Type, Status, Actual (MTD), Projected (Monthly), Delta, Drift%, Recs
   - Sort by: Cost, Name, Type, Delta (cycle with 's' key)
   - Filter: Type '/' to enter filter mode
   
2. **Detail View** (Enter key):
   - Resource metadata
   - Cost breakdown (compute, storage, network)
   - List of recommendations with savings
   - Press Escape to return

3. **Progress Banner**:
   - Top of screen: "Loading: 45/100 resources (45%)"
   - Disappears when all data loaded
   - Uses existing `tui.ProgressBar` component

**Pagination Strategy** (FR-009):
```go
const maxResourcesPerPage = 250

if len(rows) > maxResourcesPerPage {
    // Enable pagination mode
    model.paginationEnabled = true
    model.currentPage = 1
    model.totalPages = (len(rows) + maxResourcesPerPage - 1) / maxResourcesPerPage
}
```

**Alternatives Considered**:
- **Custom TUI framework (tcell)**: Rejected, too much reinvention
- **Web UI (localhost server)**: Rejected, CLI tool should stay CLI
- **Split pane layout**: Rejected, complicates mobile terminal UX

---

## 5. Non-Interactive (Plain) Mode

### Decision: ASCII Table with Summary Footer

**Rationale**:
- TTY detection already exists in `cli.isTerminal()`
- Reuse existing table rendering from `engine.RenderAsTable()`
- CI/CD pipelines need parseable output

**Output Format**:
```
RESOURCE                          TYPE              STATUS    ACTUAL(MTD)  PROJECTED   DELTA     DRIFT%   RECS
aws:ec2/instance:Instance-web-1   aws:ec2/instance  active    $42.50       $100.00    +$57.50   +15%     3
aws:s3/bucket:Bucket-logs         aws:s3/bucket     active    $5.20        $12.00     +$6.80    +8%      1
aws:rds/instance:Database-main    aws:rds/instance  creating  -            $200.00    +$200.00  N/A      0

SUMMARY
-------
Total Actual (MTD):        $47.70
Projected Monthly:         $312.00
Projected Delta:           +$264.30
Potential Savings (Recs):  $150.00
```

**Flag Behavior**:
- `--plain`: Force non-interactive mode even if TTY detected
- Auto-detect: Use `term.IsTerminal(os.Stdout.Fd())`

**Alternatives Considered**:
- **JSON output only**: Rejected, not human-readable for quick checks
- **Markdown table**: Rejected, ASCII is more universal
- **CSV format**: Could add as `--output csv` in future

---

## 6. Pre-Flight Confirmation & Optimization

### Decision: Detect Changes Early, Skip Unnecessary Queries

**Rationale**:
- FR-008 requires optimization when no changes pending
- Projected cost queries are expensive (API rate limits, cost)
- Pre-flight confirmation (FR-005) prevents accidental large queries

**Detection Logic**:
```go
func DetectPendingChanges(ctx context.Context, planPath string) (bool, int, error) {
    plan, err := ingest.LoadPulumiPlanWithContext(ctx, planPath)
    if err != nil {
        return false, 0, err
    }
    
    changeOps := []string{"create", "update", "replace", "delete"}
    changesCount := 0
    for _, step := range plan.Steps {
        if slices.Contains(changeOps, step.Op) {
            changesCount++
        }
    }
    
    return changesCount > 0, changesCount, nil
}
```

**Pre-Flight Prompt**:
```
Stack: my-stack-dev
Resources: 150 total
Pending changes: 10 (5 create, 3 update, 2 delete)
Plugins: aws-plugin, azure-plugin

This will query costs for 150 resources across 2 plugins.
Estimated API calls: ~300 (actual + projected + recommendations)

Continue? [Y/n]: _
```

**Bypass Flag**: `--yes` or `-y` to skip confirmation

**Alternatives Considered**:
- **Always query everything**: Rejected, wasteful and slow
- **Per-resource confirmation**: Rejected, too granular
- **Cost estimate for API calls**: Out of scope, cloud providers don't charge for cost APIs

---

## 7. Authentication Strategy

### Decision: Delegate to Cloud Provider SDKs (Status Quo)

**Rationale**:
- FR-010 mandates standard SDK credential chains
- Existing plugins already handle authentication
- No credential storage in finfocus

**Provider Patterns**:
1. **AWS**: `AWS_PROFILE` env var, `~/.aws/credentials`, IAM roles
2. **Azure**: `az login`, service principals, managed identities
3. **GCP**: Application Default Credentials (ADC), service accounts

**Error Handling**:
```go
// Plugin returns authentication error
if errors.Is(err, pluginhost.ErrAuthenticationFailed) {
    return fmt.Errorf("authentication failed for %s: ensure credentials are configured (see plugin docs)", pluginName)
}
```

**Alternatives Considered**:
- **OAuth flow in CLI**: Rejected, out of scope and violates FR-010
- **Credential proxy service**: Rejected, security risk
- **Credential config file**: Rejected, delegates to cloud SDKs per constitution

---

## 8. Resource Ordering

### Decision: Preserve Pulumi State File Order (FR-011)

**Rationale**:
- Pulumi state order reflects dependency graph
- Developers expect resources in logical order
- No need for custom ordering logic

**Implementation**:
```go
// ingest.StateToResourceDescriptors already preserves order
func MergeResourcesForOverview(state *ingest.StackExport, plan *ingest.PulumiPlan) []OverviewRow {
    rows := make([]OverviewRow, 0, len(state.Deployment.Resources))
    
    // Iterate state resources (preserves order)
    for _, resource := range state.Deployment.Resources {
        row := OverviewRow{
            URN:    resource.URN,
            Type:   resource.Type,
            Status: StatusActive, // default
        }
        
        // Check if resource has pending changes
        for _, step := range plan.Steps {
            if step.URN == resource.URN {
                row.Status = mapOperationToStatus(step.Op)
                break
            }
        }
        
        rows = append(rows, row)
    }
    
    // Add new resources from plan (not in state)
    // ... implementation
    
    return rows
}
```

**Alternatives Considered**:
- **Alphabetical sort**: Rejected, loses dependency context
- **Cost-descending sort**: Available as TUI sort option, not default
- **Custom dependency graph order**: Rejected, Pulumi already does this

---

## 9. Testing Strategy

### Decision: Multi-Layer Testing with Fixture Data

**Rationale**:
- Constitution mandates 80% coverage (95% for critical paths)
- TUI components need snapshot testing
- Integration tests validate end-to-end flow

**Test Layers**:

1. **Unit Tests** (80%+ coverage):
   - `overview_merge_test.go`: Resource merging logic
   - `overview_drift_test.go`: Cost drift calculation
   - `overview_model_test.go`: TUI model state transitions
   
2. **Integration Tests** (critical paths, 95%):
   - `overview_integration_test.go`: Full flow with fixture JSON files
   - Test cases:
     - Stack with no changes (optimization path)
     - Stack with all change types (create/update/delete/replace)
     - Partial API failures with retry logic
     - Large stack (300 resources) with pagination
   
3. **Snapshot Tests** (UI consistency):
   - Golden file testing for rendered tables
   - Compare actual output vs expected output
   - Use `testdata/overview/golden/*.txt`

**Test Fixtures**:
```
testdata/overview/
├── state-no-changes.json       # 10 resources, no pending updates
├── state-mixed-changes.json    # 50 resources, 10 updates
├── state-large-stack.json      # 300 resources, pagination test
├── plan-no-changes.json        # Empty steps array
├── plan-mixed-changes.json     # create, update, delete, replace
└── golden/
    ├── table-no-changes.txt    # Expected plain mode output
    └── table-with-changes.txt  # Expected plain mode output
```

**Alternatives Considered**:
- **End-to-end tests with real clouds**: Too slow, expensive, flaky
- **Mock plugin responses**: Already done in existing engine tests
- **Browser automation (for TUI)**: Rejected, TUI is terminal-based

---

## 10. Performance Benchmarks

### Decision: Establish Baseline with Benchmark Suite

**Rationale**:
- FR-002 mandates <500ms initial render
- Need to measure data loading, merging, rendering separately
- Identify bottlenecks early

**Benchmark Suite**:
```go
// benchmark_test.go
func BenchmarkMergeResources(b *testing.B) {
    // Benchmark: 100 resources state + plan merge
    // Target: <5ms
}

func BenchmarkCalculateDrift(b *testing.B) {
    // Benchmark: Drift calculation for 1000 resources
    // Target: <10ms
}

func BenchmarkRenderTable(b *testing.B) {
    // Benchmark: ASCII table render for 250 resources
    // Target: <50ms
}

func BenchmarkTUIUpdate(b *testing.B) {
    // Benchmark: Bubble Tea model update cycle
    // Target: <16ms (60fps)
}
```

**Profiling Strategy**:
- Use `go test -bench=. -cpuprofile=cpu.prof`
- Analyze with `go tool pprof` to identify hotspots
- Target: 90%+ time in plugin I/O, <10% in UI/merge logic

**Alternatives Considered**:
- **Load testing with real APIs**: Rejected, rate limits and cost
- **Memory profiling only**: Rejected, CPU is the bottleneck for UI

---

## 11. Error Handling & Retry Logic

### Decision: Per-Resource Errors with Interactive Retry

**Rationale**:
- Edge case requirement: partial API failures
- User must have control over retry behavior
- Don't block entire command on single resource failure

**Error States**:
```go
type OverviewRowError struct {
    URN          string
    ErrorType    ErrorType  // auth, network, rate_limit, unknown
    Message      string
    Retryable    bool
}

type ErrorType int
const (
    ErrorTypeAuth ErrorType = iota
    ErrorTypeNetwork
    ErrorTypeRateLimit
    ErrorTypeUnknown
)
```

**Interactive Prompt** (per spec edge case):
```
API call failed for aws:ec2/instance:Instance-web-1
Error: rate limit exceeded (429)
Retry? [y/n/skip]: _

y     → Retry immediately
n     → Show error indicator in table
skip  → Exclude resource from view
```

**Non-Interactive Behavior**:
- Default to 'n' (show error, don't retry)
- Log all errors to audit trail
- Display error summary at end (existing pattern)

**Alternatives Considered**:
- **Exponential backoff auto-retry**: Rejected, user must consent
- **Fail entire command**: Rejected, violates partial failure requirement
- **Silent skip**: Rejected, user must be informed

---

## 12. Data Model Summary

### Core Types

```go
// OverviewRow is the unified representation of a resource in the overview dashboard.
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

type ActualCostData struct {
    MTDCost      float64
    Currency     string
    Period       DateRange
    Breakdown    map[string]float64  // compute, storage, network
}

type ProjectedCostData struct {
    MonthlyCost  float64
    Currency     string
    Breakdown    map[string]float64
}

type CostDriftData struct {
    ExtrapolatedMonthly float64
    Projected           float64
    Delta               float64
    PercentDrift        float64
    IsWarning           bool
}

type ResourceStatus int
const (
    StatusActive ResourceStatus = iota
    StatusCreating
    StatusUpdating
    StatusDeleting
    StatusReplacing
)
```

---

## 13. Technology Stack

**Language**: Go 1.22+ (existing project requirement)  
**CLI Framework**: Cobra (existing, `github.com/spf13/cobra`)  
**TUI Framework**: Bubble Tea (existing, `github.com/charmbracelet/bubbletea`)  
**Table Rendering**: Bubble Tea Tables (existing, `github.com/charmbracelet/bubbles/table`)  
**Styling**: Lipgloss (existing, `github.com/charmbracelet/lipgloss`)  
**Concurrency**: Go goroutines + channels (existing engine pattern)  
**Testing**: Go testing + testify (existing)  
**State Management**: Pulumi state/preview JSON (existing ingest layer)

**No New Dependencies Required** ✅

---

## 14. Open Questions & Risks

### Risks Identified

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| API rate limits cause timeouts on large stacks | Medium | High | Pre-flight confirmation, backoff, retry logic |
| TUI performance degrades on >500 resources | Low | Medium | Pagination at 250, tested with benchmarks |
| Cost drift false positives on day 1-2 of month | High | Low | Display warning "Insufficient data (day X)" |
| Plugin authentication errors | Medium | Medium | Clear error messages with provider-specific docs links |

### Open Questions

1. **Q**: Should we cache actual costs to avoid repeated API calls?  
   **A**: Yes, reuse existing cache layer (`internal/engine/cache`), TTL from config.

2. **Q**: How to handle multi-currency stacks?  
   **A**: Out of scope for v1, display warning if multiple currencies detected.

3. **Q**: What if state file has >1000 resources?  
   **A**: Pagination handles up to 1000 (4 pages), beyond that suggest filtering.

---

## 15. Implementation Priorities

### Phase 1: Core Merge Logic (P0)
- Resource merging from state + plan
- Status detection (active/creating/updating/deleting/replacing)
- Cost drift calculation

### Phase 2: Interactive TUI (P0)
- Main table view with progressive loading
- Progress banner
- Sort and filter

### Phase 3: Detail View (P1)
- Resource detail modal
- Cost breakdown display
- Recommendations list

### Phase 4: Non-Interactive Mode (P1)
- ASCII table rendering
- Summary footer

### Phase 5: Error Handling (P1)
- Partial failure support
- Interactive retry prompts

---

## References

- **Feature Spec**: `specs/509-overview-command/spec.md`
- **Constitution**: `.specify/memory/constitution.md`
- **Existing TUI**: `internal/tui/cost_model.go`, `internal/tui/recommendations_model.go`
- **Existing Engine**: `internal/engine/types.go`, `internal/engine/projected.go`
- **Ingest Layer**: `internal/ingest/state.go`, `internal/ingest/pulumi_plan.go`

---

**Research Completed**: 2026-02-11  
**Next Step**: Generate `data-model.md` and `contracts/` for Phase 1 design
