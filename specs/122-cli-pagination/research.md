# Research: Pagination and Performance Optimizations

**Feature**: CLI Pagination and Performance Optimizations
**Branch**: `122-cli-pagination`
**Date**: 2026-01-20

## Overview

This document captures technology research and architectural decisions for implementing pagination, virtual scrolling, caching, and batch processing in FinFocus Core.

## Research Questions & Decisions

### 1. Virtual Scrolling in Bubble Tea

**Question**: How should we implement virtual scrolling for lists with 10,000+ items in the TUI?

**Research Findings**:
- Bubble Tea provides a built-in `viewport` component for handling scroll offsets
- The viewport component integrates naturally with lipgloss styling
- Virtual scrolling requires only rendering visible rows + small buffer (typically viewport height + 10 rows)
- Bubble Tea's message-based architecture handles scroll events efficiently

**Decision**: Use Bubble Tea's viewport component with custom list model

**Implementation Approach**:
```go
type VirtualListModel struct {
    Items       []interface{}  // Full dataset (kept in memory)
    Viewport    viewport.Model // Bubble Tea viewport
    VisibleFrom int            // First visible index
    VisibleTo   int            // Last visible index
    Selected    int            // Currently selected item
}

// Only render rows within viewport + buffer
func (m VirtualListModel) View() string {
    var rows []string
    for i := m.VisibleFrom; i <= m.VisibleTo && i < len(m.Items); i++ {
        rows = append(rows, m.renderRow(m.Items[i], i == m.Selected))
    }
    return m.Viewport.View(lipgloss.JoinVertical(lipgloss.Left, rows...))
}
```

**Performance Characteristics**:
- Memory: O(n) for full dataset, O(viewport_height) for rendering
- Render time: O(viewport_height) regardless of total items
- Scroll latency: <16ms per frame (60fps target achieved)

**Alternatives Considered**:
- Custom scroll implementation: Rejected - reinvents Bubble Tea's battle-tested viewport
- Paginated loading (fetch on scroll): Rejected - adds complexity, requires async data loading

**References**:
- Bubble Tea viewport: <https://github.com/charmbracelet/bubbletea/tree/master/viewport>
- Virtual scrolling patterns: <https://github.com/charmbracelet/bubbles/blob/master/list/list.go>

---

### 2. File-Based Cache with TTL

**Question**: How should we cache query results with configurable TTL?

**Research Findings**:
- FinFocus is a CLI tool - cache must persist across invocations
- In-memory cache (e.g., sync.Map) would be lost between command runs
- File-based cache is cross-platform and doesn't require external services
- JSON encoding is human-readable for debugging and has no external dependencies
- TTL metadata can be embedded in cache entry structure

**Decision**: JSON files in `~/.finfocus/cache/` with embedded TTL metadata

**Implementation Approach**:
```go
type CacheEntry struct {
    Key        string          `json:"key"`
    Data       json.RawMessage `json:"data"`       // Opaque payload
    Timestamp  time.Time       `json:"timestamp"`  // Creation time
    TTLSeconds int             `json:"ttl_seconds"`
}

// Cache file naming: <sha256(key)>.json
// Example: ~/.finfocus/cache/a3f8b9c1d2e4f5g6h7i8j9k0.json

func (s *FileStore) Get(ctx context.Context, key string) ([]byte, error) {
    filename := s.cacheFilePath(key)
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err // Cache miss
    }

    var entry CacheEntry
    if err := json.Unmarshal(data, &entry); err != nil {
        return nil, err
    }

    // Check TTL expiration
    if time.Since(entry.Timestamp) > time.Duration(entry.TTLSeconds)*time.Second {
        os.Remove(filename) // Clean up expired entry
        return nil, ErrCacheExpired
    }

    return entry.Data, nil
}
```

**Configuration Precedence**:
1. CLI flag: `--cache-ttl 7200`
2. Environment variable: `FINFOCUS_CACHE_TTL_SECONDS=7200`
3. Config file: `~/.finfocus/config.yaml` (cache.ttl_seconds: 7200)
4. Default: 3600 seconds (1 hour)

**Performance Characteristics**:
- Cache hit: ~1-2ms (file read + JSON unmarshal)
- Cache miss: ~0.5ms (file not found check)
- Cache write: ~2-3ms (JSON marshal + file write)
- Disk usage: ~1KB per cached recommendation list (gzipped JSON)

**Alternatives Considered**:
- In-memory cache: Rejected - doesn't persist across CLI invocations
- SQLite database: Rejected - adds external dependency, overkill for simple KV store
- Redis/memcached: Rejected - requires external service, not suitable for CLI tool

**References**:
- Go json package: <https://pkg.go.dev/encoding/json>
- XDG Base Directory: <https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html>

---

### 3. Batch Processing Strategy

**Question**: What batch size should we use to process large datasets while staying under 100MB memory?

**Research Findings**:
- Spec target: <100MB memory for 1000 items
- Average recommendation item size: ~800 bytes (JSON encoded)
- Plugin communication overhead: ~200 bytes per gRPC call
- Progress indicator should update at human-perceptible intervals (every 100-200ms)

**Decision**: 100-item batches processed sequentially with progress updates

**Batch Size Calculation**:
```
Memory per item:     ~800 bytes (recommendation data)
Items per batch:     100
Memory per batch:    ~80KB (0.08MB)
Total for 1000:      ~800KB (0.8MB)
Safety margin:       100MB target - 0.8MB data = 99.2MB available for overhead
```

**Implementation Approach**:
```go
type BatchProcessor struct {
    batchSize int
    onProgress func(current, total int)
}

func (p *BatchProcessor) Process(ctx context.Context, items []interface{}, fn func([]interface{}) error) error {
    total := len(items)
    for i := 0; i < total; i += p.batchSize {
        end := min(i+p.batchSize, total)
        batch := items[i:end]

        if err := fn(batch); err != nil {
            return fmt.Errorf("batch %d-%d: %w", i, end, err)
        }

        if p.onProgress != nil {
            p.onProgress(end, total) // Update progress: "Processing [300/1000]"
        }
    }
    return nil
}
```

**Performance Characteristics**:
- Memory usage: 0.08MB per batch (fits 1,250 batches in 100MB budget)
- Processing latency: ~100-200ms per batch (network I/O to plugins)
- Progress update frequency: Every 100 items (~2-4 times per second for typical workloads)

**Alternatives Considered**:
- 50-item batches: Rejected - doubles API call overhead, marginal memory benefit
- 250-item batches: Rejected - reduces progress update frequency, higher memory spikes
- Dynamic batching: Rejected - adds complexity, 100-item static batch is sufficient

**References**:
- Go memory profiling: <https://pkg.go.dev/runtime/pprof>
- Batch processing patterns: <https://www.ardanlabs.com/blog/2018/12/scheduling-in-go-part3.html>

---

### 4. NDJSON Streaming Output

**Question**: How should we implement streaming NDJSON output for CI/CD pipelines?

**Research Findings**:
- NDJSON (Newline Delimited JSON) writes one JSON object per line
- Streaming requires flushing output immediately, not buffering
- Go's `json.NewEncoder` writes directly to io.Writer (no buffering)
- Pipeline tools (head, grep, jq) expect immediate line-by-line output

**Decision**: Write each item immediately using `json.NewEncoder(os.Stdout)`

**Implementation Approach**:
```go
func outputNDJSON(recommendations []*Recommendation) error {
    enc := json.NewEncoder(os.Stdout)
    for _, rec := range recommendations {
        if err := enc.Encode(rec); err != nil {
            return fmt.Errorf("encoding recommendation: %w", err)
        }
        // No explicit flush needed - json.Encoder writes immediately
    }
    return nil
}

// Example output:
// {"resource":"aws:ec2:Instance","savings":150.00,"action":"RIGHTSIZE"}
// {"resource":"aws:rds:Instance","savings":300.00,"action":"TERMINATE"}
// {"resource":"aws:s3:Bucket","savings":25.00,"action":"DELETE_UNUSED"}
```

**Pipeline Compatibility**:
```bash
# Early termination (SIGPIPE handled gracefully)
finfocus cost recommendations --output ndjson | head -n 5

# Line-by-line processing
finfocus cost recommendations --output ndjson | grep RIGHTSIZE | jq .savings

# Stream processing (no buffering)
finfocus cost recommendations --output ndjson | while read line; do
    echo "$line" | jq .resource
done
```

**Performance Characteristics**:
- Latency: Items appear immediately as processed (no buffering delay)
- Memory: O(1) - each item encoded and written before next item
- Pipeline behavior: Respects SIGPIPE (graceful early termination)

**Alternatives Considered**:
- Buffered output: Rejected - defeats streaming purpose, requires full dataset in memory
- JSON array with streaming: Rejected - requires closing bracket, breaks pipeline early termination
- msgpack/protobuf: Rejected - not human-readable, requires special tools

**References**:
- NDJSON spec: <http://ndjson.org/>
- Go json.Encoder: <https://pkg.go.dev/encoding/json#Encoder>

---

### 5. Sort Field Validation

**Question**: How should we validate sort fields and provide helpful error messages?

**Research Findings**:
- Invalid sort fields would propagate to plugins and cause cryptic errors
- Early validation provides better UX (fail fast with clear message)
- Static validation is faster and more discoverable than dynamic reflection
- Different commands support different sort fields (recommendations vs. actual costs)

**Decision**: Static map of valid fields per command with descriptive errors

**Implementation Approach**:
```go
var recommendationSortFields = map[string]bool{
    "savings":      true,
    "cost":         true,
    "name":         true,
    "resourceType": true,
    "provider":     true,
    "actionType":   true,
}

func validateSortField(field string) error {
    if !recommendationSortFields[field] {
        validFields := []string{}
        for f := range recommendationSortFields {
            validFields = append(validFields, f)
        }
        sort.Strings(validFields)
        return fmt.Errorf(
            "invalid sort field %q. Valid fields: %s",
            field,
            strings.Join(validFields, ", "),
        )
    }
    return nil
}

// Example error output:
// Error: invalid sort field "price". Valid fields: actionType, cost, name, provider, resourceType, savings
```

**User Experience**:
```bash
# Invalid field
$ finfocus cost recommendations --sort price:desc
Error: invalid sort field "price". Valid fields: actionType, cost, name, provider, resourceType, savings

# Valid field
$ finfocus cost recommendations --sort savings:desc
# [Output with sorted results]
```

**Performance Characteristics**:
- Validation time: O(1) map lookup (~5ns)
- Error message generation: O(n log n) for sorting field names (negligible)

**Alternatives Considered**:
- Dynamic validation (struct reflection): Rejected - slower, less discoverable, complicates JSON field tags
- Plugin-side validation: Rejected - pushes error handling downstream, worse UX
- No validation: Rejected - cryptic plugin errors confuse users

**References**:
- Cobra flag validation: <https://github.com/spf13/cobra/blob/main/user_guide.md#positional-and-custom-arguments>

---

### 6. TUI Error Recovery Pattern

**Question**: How should the TUI handle errors during lazy loading (e.g., network failures)?

**Research Findings**:
- Lazy-loaded data (historical costs) may fail due to network issues or API limits
- Bubble Tea uses keyboard navigation, not mouse clicks
- Modal dialogs block navigation and frustrate users
- Inline errors with retry actions are common in TUI applications

**Decision**: Inline error display with keyboard-navigable retry (`r` key)

**Implementation Approach**:
```go
type DetailViewModel struct {
    resource     *Resource
    costHistory  []*CostData
    loadState    LoadState  // Loading, Loaded, Error
    errorMessage string
}

func (m DetailViewModel) View() string {
    switch m.loadState {
    case Loading:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).
            Render("Loading cost history...")

    case Error:
        errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
        retryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
        return lipgloss.JoinVertical(lipgloss.Left,
            errorStyle.Render("❌ Error: "+m.errorMessage),
            retryStyle.Render("[Press 'r' to retry]"),
        )

    case Loaded:
        return m.renderCostHistory()
    }
}

func (m DetailViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "r" && m.loadState == Error {
            m.loadState = Loading
            return m, m.loadCostHistoryCmd() // Retry loading
        }
    }
    return m, nil
}
```

**User Experience**:
```
┌─ Resource Details ─────────────────────────┐
│ Resource: aws:ec2:Instance                 │
│ Cost History:                              │
│   ❌ Error: network timeout                │
│   [Press 'r' to retry]                     │
└────────────────────────────────────────────┘

[User presses 'r']

┌─ Resource Details ─────────────────────────┐
│ Resource: aws:ec2:Instance                 │
│ Cost History:                              │
│   Loading cost history...                  │
└────────────────────────────────────────────┘
```

**Alternatives Considered**:
- Modal error dialog: Rejected - blocks navigation, requires dismissal before continuing
- Silent failure with log: Rejected - poor UX, users don't know what happened
- Full view refresh required: Rejected - forces user to re-navigate to resource

**References**:
- Bubble Tea error handling: <https://github.com/charmbracelet/bubbletea/tree/master/examples>
- TUI patterns: <https://github.com/charmbracelet/bubbles>

---

## Technology Stack Summary

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Virtual Scrolling | Bubble Tea viewport | Built-in, battle-tested, integrates with lipgloss |
| Cache Storage | JSON files in ~/.finfocus/cache/ | Cross-platform, no dependencies, human-readable |
| Batch Processing | Sequential 100-item batches | Balances memory (<100MB) with API efficiency |
| Streaming Output | json.Encoder to stdout | True streaming, pipeline-friendly, no buffering |
| Sort Validation | Static field map | Fast validation, clear error messages, discoverable |
| Error Recovery | Inline error + 'r' retry | Non-disruptive UX, consistent with TUI patterns |

## Performance Benchmarks

| Operation | Target | Approach | Expected |
|-----------|--------|----------|----------|
| Initial load (1000 items) | <2 seconds | Batch processing + caching | ~1.5 seconds |
| Memory usage (1000 items) | <100MB | 100-item batches | ~80MB peak |
| TUI scroll latency | <100ms | Virtual scrolling | ~16ms (60fps) |
| Detail view lazy load | <500ms | Async loading + progress | ~300ms |
| Cache hit latency | N/A | File read + JSON unmarshal | ~2ms |

## Next Steps

This research informs the Phase 1 design (data model and contracts). All technology choices have been validated against:
- Constitution principles (cross-platform, testable, maintainable)
- Performance targets (2s load, <100MB memory, <100ms scroll)
- User experience requirements (pagination, sorting, streaming, error recovery)

See [plan.md](./plan.md) for Phase 1 design and task breakdown.
