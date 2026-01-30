# Implementation Plan: CLI Pagination and Performance Optimizations

**Branch**: `122-cli-pagination` | **Date**: 2026-01-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/122-cli-pagination/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

This feature adds pagination, sorting, and performance optimizations to the FinFocus CLI and TUI to support enterprise-scale deployments with hundreds or thousands of resources. The implementation includes:

- CLI pagination flags (`--limit`, `--page`, `--page-size`, `--offset`, `--sort`)
- TUI virtual scrolling for large lists (10,000+ items)
- Streaming NDJSON output for CI/CD integration
- Batch processing (100 items per batch) with progress indicators
- Query result caching (1-hour TTL, configurable)
- Lazy loading for TUI detail views with inline error recovery

Technical approach: Enhance existing CLI command handlers and TUI components with pagination/sorting logic, implement virtual list rendering using Bubble Tea's viewport, add caching layer to engine package, and create batch processing utilities.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**:
- github.com/spf13/cobra v1.10.2 (CLI framework)
- github.com/charmbracelet/bubbletea v0.27.2+ (TUI framework)
- github.com/charmbracelet/lipgloss v0.13.1+ (TUI styling)
- github.com/rs/zerolog v1.34.0 (structured logging)
- github.com/rshade/finfocus-spec v0.5.2+ (protocol definitions)

**Storage**:
- File-based query cache in `~/.finfocus/cache/` (JSON files with TTL metadata)
- Configuration in `~/.finfocus/config.yaml` (cache.ttl_seconds setting)

**Testing**: Go testing stdlib + testify v1.11.1 (unit tests with 80%+ coverage, integration tests in test/integration/, E2E tests in test/e2e/)

**Target Platform**: Cross-platform (Linux amd64/arm64, macOS amd64/arm64, Windows amd64)

**Project Type**: Single project (CLI tool with TUI components)

**Performance Goals**:
- Initial load of 1000+ item lists in <2 seconds
- Memory usage <100MB for 1000 items
- TUI scroll latency <100ms
- Detail view lazy load <500ms

**Constraints**:
- Batch processing: 100 items per batch
- Cache TTL: 3600 seconds (1 hour) default
- Virtual scrolling: render only visible rows + buffer
- Memory budget: <100MB for large datasets

**Scale/Scope**:
- Support 10,000+ recommendations in TUI mode
- Handle 1000+ resources in CLI output
- Stream unlimited NDJSON results without buffering

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is a core enhancement (CLI/TUI/engine), not a plugin. Orchestration logic only.
- [x] **Test-Driven Development**: Tests will be written before implementation (80% minimum coverage, 95% for critical paths)
- [x] **Cross-Platform Compatibility**: All Go code, no platform-specific dependencies. Works on Linux, macOS, Windows.
- [x] **Documentation Synchronization**: README.md and docs/ will be updated in same PR (new CLI flags, TUI features, config options)
- [x] **Protocol Stability**: No protocol buffer changes required (internal feature only)
- [x] **Implementation Completeness**: Full implementation required - no TODOs, stubs, or placeholders allowed per Principle VI
- [x] **Quality Gates**: Will run `make lint` and `make test` before completion (per constitution)
- [x] **Multi-Repo Coordination**: No cross-repo changes required (internal-only feature)

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/122-cli-pagination/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output - technology research
├── data-model.md        # Phase 1 output - data structures
├── quickstart.md        # Phase 1 output - usage guide
├── contracts/           # Phase 1 output - API contracts (internal Go interfaces)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created yet)
```

### Source Code (repository root)

```text
# Single project structure (existing FinFocus Core layout)
internal/
├── cli/                 # CLI command handlers
│   ├── cost_recommendations.go  # Add pagination flags
│   └── pagination/              # NEW: shared pagination logic
│       ├── flags.go             # Flag definitions and validation
│       ├── sorter.go            # Generic sorting utilities
│       └── metadata.go          # Pagination metadata structures
├── engine/              # Cost calculation engine
│   ├── cache/                   # NEW: query result caching
│   │   ├── store.go             # File-based cache with TTL
│   │   ├── key.go               # Cache key generation
│   │   └── ttl.go               # TTL configuration
│   └── batch/                   # NEW: batch processing
│       ├── processor.go         # Batch orchestration (100-item chunks)
│       └── progress.go          # Progress indicator integration
├── tui/                 # Terminal UI components
│   ├── list/                    # NEW: virtual scrolling list component
│   │   ├── viewport.go          # Virtual list with viewport
│   │   ├── model.go             # Bubble Tea model
│   │   └── render.go            # Efficient row rendering
│   └── detail/                  # Detail view with lazy loading
│       ├── loader.go            # Async data loading
│       └── error.go             # Inline error state with retry
└── config/              # Configuration management
    └── cache.go         # NEW: cache configuration

test/
├── unit/
│   ├── cli/pagination/          # Pagination logic tests
│   ├── engine/cache/            # Cache tests
│   ├── engine/batch/            # Batch processing tests
│   └── tui/list/                # Virtual list tests
├── integration/
│   ├── cli_pagination_test.go   # End-to-end CLI pagination
│   └── tui_virtual_scroll_test.go  # TUI scroll performance
└── fixtures/
    ├── large_dataset_1000.json  # 1000-item test data
    └── large_dataset_10000.json # 10,000-item test data
```

**Structure Decision**: Using existing single-project Go structure. New packages added under `internal/` for pagination utilities, caching, batch processing, and enhanced TUI components. Tests follow established pattern (unit/integration/fixtures).

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations - this section intentionally left empty.

## Phase 0: Research & Technology Choices

**Status**: ✅ Complete

### Research Questions Resolved

Based on the specification and constitution requirements, the following research areas have been investigated:

1. **Virtual Scrolling Implementation in Bubble Tea**
   - Decision: Use Bubble Tea's viewport component with custom list model
   - Rationale: Built-in support for scroll offsets, works well with lipgloss styling
   - Alternative considered: Custom scroll implementation (rejected - reinvents wheel)

2. **File-Based Cache with TTL**
   - Decision: JSON files in `~/.finfocus/cache/` with embedded TTL metadata
   - Rationale: Simple, cross-platform, no external dependencies, easy to debug
   - Alternative considered: In-memory cache (rejected - doesn't persist across invocations)

3. **Batch Processing Strategy**
   - Decision: 100-item batches processed sequentially with progress updates
   - Rationale: Balances memory usage (<100MB target) with API call efficiency
   - Alternative considered: 250-item batches (rejected - higher memory risk)

4. **NDJSON Streaming Output**
   - Decision: Write each item immediately using `json.NewEncoder(os.Stdout)`
   - Rationale: True streaming without buffering, pipeline-friendly
   - Alternative considered: Buffered output (rejected - defeats streaming purpose)

5. **Sort Field Validation**
   - Decision: Static map of valid fields per command with descriptive errors
   - Rationale: Early validation, clear user feedback, prevents plugin errors
   - Alternative considered: Dynamic validation (rejected - less discoverable)

6. **TUI Error Recovery Pattern**
   - Decision: Inline error display with keyboard-navigable retry (`r` key)
   - Rationale: Consistent with Bubble Tea patterns, non-disruptive UX
   - Alternative considered: Modal error dialog (rejected - blocks navigation)

See [research.md](./research.md) for detailed findings and benchmarks.

## Phase 1: Data Model & Contracts

**Status**: ✅ Complete

### Data Model

Key data structures for pagination, caching, and virtual scrolling:

```go
// Pagination request parameters (CLI flags)
type PaginationParams struct {
    Limit    int    // --limit (0 = unlimited)
    Offset   int    // --offset (skip N items)
    Page     int    // --page (1-indexed, mutually exclusive with Offset)
    PageSize int    // --page-size (default 20)
    SortBy   string // --sort (format: "field:asc|desc")
}

// Pagination metadata (response)
type PaginationMeta struct {
    Page       int `json:"page"`
    PageSize   int `json:"page_size"`
    TotalItems int `json:"total_items"`
    TotalPages int `json:"total_pages"`
}

// Cache entry with TTL
type CacheEntry struct {
    Key        string          `json:"key"`
    Data       json.RawMessage `json:"data"`
    Timestamp  time.Time       `json:"timestamp"`
    TTLSeconds int             `json:"ttl_seconds"`
}

// Virtual list state (TUI)
type VirtualListModel struct {
    Items       []interface{} // Full dataset
    Viewport    viewport.Model
    VisibleFrom int // First visible index
    VisibleTo   int // Last visible index
    Selected    int // Currently selected item
}

// Batch processing progress
type BatchProgress struct {
    Current    int
    Total      int
    StartTime  time.Time
    OnProgress func(current, total int) // Callback for CLI/TUI updates
}
```

See [data-model.md](./data-model.md) for complete entity definitions and relationships.

### API Contracts

**Internal Go Interfaces** (not gRPC - internal-only feature):

```go
// internal/cli/pagination/sorter.go
type Sorter interface {
    // Sort applies sorting to a slice based on field name and direction
    Sort(items []interface{}, field string, desc bool) error
    // ValidateField checks if a sort field is valid for this item type
    ValidateField(field string) error
}

// internal/engine/cache/store.go
type CacheStore interface {
    // Get retrieves cached data by key (returns nil if expired or not found)
    Get(ctx context.Context, key string) ([]byte, error)
    // Set stores data with TTL (uses configured default if ttl=0)
    Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
    // Clear removes all expired entries
    Clear(ctx context.Context) error
}

// internal/engine/batch/processor.go
type BatchProcessor interface {
    // Process executes fn on items in batches of batchSize
    Process(ctx context.Context, items []interface{}, batchSize int, fn func([]interface{}) error) error
    // WithProgress attaches progress callback
    WithProgress(callback func(current, total int)) BatchProcessor
}

// internal/tui/list/viewport.go
type VirtualList interface {
    // Update handles Bubble Tea messages (scroll, selection)
    Update(msg tea.Msg) (tea.Model, tea.Cmd)
    // View renders only visible rows
    View() string
    // SetItems replaces the full dataset
    SetItems(items []interface{})
}
```

See [contracts/](./contracts/) for complete interface definitions and usage examples.

### Quickstart Guide

**CLI Pagination**:

```bash
# Limit results
finfocus cost recommendations --limit 10

# Pagination with page number
finfocus cost recommendations --page 2 --page-size 20

# Offset-based pagination
finfocus cost recommendations --offset 40 --limit 20

# Sorting
finfocus cost recommendations --sort savings:desc

# Streaming output for CI/CD
finfocus cost recommendations --output ndjson | head -n 5
```

**Configuration**:

```yaml
# ~/.finfocus/config.yaml
cache:
  ttl_seconds: 3600  # 1 hour default
  enabled: true
  directory: ~/.finfocus/cache
```

**Environment Variables**:

```bash
export FINFOCUS_CACHE_TTL_SECONDS=7200  # Override config
export FINFOCUS_CACHE_ENABLED=false     # Disable caching
```

See [quickstart.md](./quickstart.md) for complete usage guide with examples.

## Phase 2: Task Breakdown

**Status**: ⏳ Pending (`/speckit.tasks` command)

Task breakdown will be generated by running `/speckit.tasks` after this plan is approved. Expected task categories:

1. **Pagination Infrastructure** (CLI flags, validation, metadata)
2. **Cache Implementation** (store, TTL, key generation)
3. **Batch Processing** (processor, progress tracking)
4. **TUI Virtual Scrolling** (viewport, model, rendering)
5. **TUI Lazy Loading** (detail view, error recovery)
6. **CLI Integration** (add flags to commands, wire up pagination)
7. **Testing** (unit tests 80%+, integration tests, fixtures)
8. **Documentation** (README, docs/, CLI help text)

## Constitution Re-Check (Post-Design)

All constitution principles remain satisfied after Phase 1 design:

- ✅ **Plugin-First Architecture**: Internal core feature, no plugin changes
- ✅ **Test-Driven Development**: Test plan covers 80%+ coverage with integration tests
- ✅ **Cross-Platform Compatibility**: Pure Go, file-based cache works on all platforms
- ✅ **Documentation Synchronization**: README and docs/ updates included in task list
- ✅ **Protocol Stability**: No protocol changes
- ✅ **Implementation Completeness**: All features fully specified, no stubs/TODOs planned
- ✅ **Quality Gates**: `make lint` and `make test` required before completion
- ✅ **Multi-Repo Coordination**: No cross-repo dependencies

## Next Steps

1. **Review this plan** for technical accuracy and completeness
2. **Run `/speckit.tasks`** to generate actionable task breakdown
3. **Run `/speckit.implement`** to execute task list with Constitution compliance

## References

- Feature Specification: [spec.md](./spec.md)
- Constitution: [.specify/memory/constitution.md](../../.specify/memory/constitution.md)
- Clarifications: See spec.md "Clarifications" section (Session 2026-01-20)
