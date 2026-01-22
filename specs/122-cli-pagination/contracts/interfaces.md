# API Contracts: Internal Go Interfaces

**Feature**: CLI Pagination and Performance Optimizations
**Branch**: `122-cli-pagination`
**Date**: 2026-01-20

## Overview

This document defines the Go interfaces for pagination, caching, batch processing, and virtual scrolling. These are **internal contracts** (not gRPC), used for component communication within FinFocus Core.

---

## 1. Sorter Interface

**Package**: `internal/cli/pagination`

Provides generic sorting functionality for slices based on field names.

```go
package pagination

// Sorter handles sorting of items by field name and direction
type Sorter interface {
    // Sort applies sorting to a slice based on field name and direction
    // Returns error if field is invalid or items cannot be sorted
    Sort(items []interface{}, field string, desc bool) error

    // ValidateField checks if a sort field is valid for this item type
    // Returns error with list of valid fields if invalid
    ValidateField(field string) error

    // ValidFields returns all valid sort field names
    ValidFields() []string
}

// Example implementation for recommendations
type RecommendationSorter struct {
    validFields map[string]bool
}

func NewRecommendationSorter() *RecommendationSorter {
    return &RecommendationSorter{
        validFields: map[string]bool{
            "savings":      true,
            "cost":         true,
            "name":         true,
            "resourceType": true,
            "provider":     true,
            "actionType":   true,
        },
    }
}

func (s *RecommendationSorter) ValidateField(field string) error {
    if !s.validFields[field] {
        return fmt.Errorf(
            "invalid sort field %q. Valid fields: %s",
            field,
            strings.Join(s.ValidFields(), ", "),
        )
    }
    return nil
}

func (s *RecommendationSorter) ValidFields() []string {
    fields := make([]string, 0, len(s.validFields))
    for f := range s.validFields {
        fields = append(fields, f)
    }
    sort.Strings(fields)
    return fields
}

func (s *RecommendationSorter) Sort(items []interface{}, field string, desc bool) error {
    if err := s.ValidateField(field); err != nil {
        return err
    }

    recommendations := make([]*Recommendation, len(items))
    for i, item := range items {
        rec, ok := item.(*Recommendation)
        if !ok {
            return fmt.Errorf("item %d is not a Recommendation", i)
        }
        recommendations[i] = rec
    }

    sort.Slice(recommendations, func(i, j int) bool {
        var less bool
        switch field {
        case "savings":
            less = recommendations[i].Savings < recommendations[j].Savings
        case "cost":
            less = recommendations[i].MonthlyCost < recommendations[j].MonthlyCost
        case "name":
            less = recommendations[i].ResourceName < recommendations[j].ResourceName
        // ... other fields
        }
        if desc {
            return !less
        }
        return less
    })

    // Copy back to interface slice
    for i, rec := range recommendations {
        items[i] = rec
    }

    return nil
}
```

**Usage Example**:

```go
sorter := pagination.NewRecommendationSorter()

// Validate sort field from CLI flag
if err := sorter.ValidateField("savings"); err != nil {
    return err // "invalid sort field..."
}

// Sort items
items := []interface{}{rec1, rec2, rec3}
if err := sorter.Sort(items, "savings", true); err != nil {
    return err
}
```

---

## 2. CacheStore Interface

**Package**: `internal/engine/cache`

Provides persistent caching with TTL expiration.

```go
package cache

import (
    "context"
    "time"
)

// CacheStore manages persistent cached data with TTL
type CacheStore interface {
    // Get retrieves cached data by key
    // Returns nil and no error if key not found or expired
    // Returns error only for I/O or corruption issues
    Get(ctx context.Context, key string) ([]byte, error)

    // Set stores data with TTL
    // If ttl is 0, uses configured default TTL
    // Returns error if write fails
    Set(ctx context.Context, key string, data []byte, ttl time.Duration) error

    // Clear removes all expired entries
    // Returns error if cleanup fails
    Clear(ctx context.Context) error

    // Delete removes a specific key
    // Returns no error if key doesn't exist (idempotent)
    Delete(ctx context.Context, key string) error

    // Size returns current cache size in bytes
    Size(ctx context.Context) (int64, error)
}

// FileStore implements CacheStore using JSON files
type FileStore struct {
    directory  string
    defaultTTL time.Duration
}

func NewFileStore(directory string, defaultTTL time.Duration) (*FileStore, error) {
    if err := os.MkdirAll(directory, 0755); err != nil {
        return nil, fmt.Errorf("creating cache directory: %w", err)
    }
    return &FileStore{
        directory:  directory,
        defaultTTL: defaultTTL,
    }, nil
}

func (s *FileStore) Get(ctx context.Context, key string) ([]byte, error) {
    filename := s.cacheFilePath(key)

    data, err := os.ReadFile(filename)
    if os.IsNotExist(err) {
        return nil, nil // Cache miss, not an error
    }
    if err != nil {
        return nil, fmt.Errorf("reading cache file: %w", err)
    }

    var entry CacheEntry
    if err := json.Unmarshal(data, &entry); err != nil {
        return nil, fmt.Errorf("unmarshaling cache entry: %w", err)
    }

    if entry.IsExpired() {
        os.Remove(filename) // Clean up expired entry
        return nil, nil     // Expired, treat as cache miss
    }

    return entry.Data, nil
}

func (s *FileStore) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
    if ttl == 0 {
        ttl = s.defaultTTL
    }

    entry := CacheEntry{
        Key:        key,
        Data:       json.RawMessage(data),
        Timestamp:  time.Now(),
        TTLSeconds: int(ttl.Seconds()),
    }

    encoded, err := json.Marshal(entry)
    if err != nil {
        return fmt.Errorf("marshaling cache entry: %w", err)
    }

    filename := s.cacheFilePath(key)
    if err := os.WriteFile(filename, encoded, 0644); err != nil {
        return fmt.Errorf("writing cache file: %w", err)
    }

    return nil
}

func (s *FileStore) cacheFilePath(key string) string {
    hash := sha256.Sum256([]byte(key))
    filename := hex.EncodeToString(hash[:]) + ".json"
    return filepath.Join(s.directory, filename)
}
```

**Usage Example**:

```go
store, err := cache.NewFileStore("~/.finfocus/cache", 1*time.Hour)
if err != nil {
    return err
}

// Generate cache key from query parameters
key := fmt.Sprintf("recommendations:%s:%s", provider, region)

// Try cache first
if data, err := store.Get(ctx, key); err == nil && data != nil {
    // Cache hit
    var results []Recommendation
    json.Unmarshal(data, &results)
    return results, nil
}

// Cache miss - fetch data
results := fetchRecommendations(ctx, provider, region)

// Store in cache
data, _ := json.Marshal(results)
store.Set(ctx, key, data, 0) // Use default TTL

return results, nil
```

---

## 3. BatchProcessor Interface

**Package**: `internal/engine/batch`

Handles batch processing of large datasets with progress tracking.

```go
package batch

import "context"

// BatchProcessor processes items in fixed-size batches
type BatchProcessor interface {
    // Process executes fn on items in batches of batchSize
    // fn receives each batch and returns error if batch fails
    // Processing stops on first error
    Process(ctx context.Context, items []interface{}, batchSize int, fn func([]interface{}) error) error

    // WithProgress attaches a progress callback
    // Callback is invoked after each batch with (current, total) counts
    WithProgress(callback func(current, total int)) BatchProcessor
}

// DefaultBatchProcessor implements BatchProcessor
type DefaultBatchProcessor struct {
    onProgress func(current, total int)
}

func NewBatchProcessor() *DefaultBatchProcessor {
    return &DefaultBatchProcessor{}
}

func (p *DefaultBatchProcessor) WithProgress(callback func(current, total int)) BatchProcessor {
    p.onProgress = callback
    return p
}

func (p *DefaultBatchProcessor) Process(ctx context.Context, items []interface{}, batchSize int, fn func([]interface{}) error) error {
    total := len(items)

    for i := 0; i < total; i += batchSize {
        // Check for cancellation
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        // Calculate batch boundaries
        end := min(i+batchSize, total)
        batch := items[i:end]

        // Process batch
        if err := fn(batch); err != nil {
            return fmt.Errorf("batch %d-%d: %w", i, end, err)
        }

        // Update progress
        if p.onProgress != nil {
            p.onProgress(end, total)
        }
    }

    return nil
}
```

**Usage Example**:

```go
processor := batch.NewBatchProcessor().WithProgress(func(current, total int) {
    fmt.Printf("\rProcessing resources... [%d/%d]", current, total)
})

err := processor.Process(ctx, resources, 100, func(batch []interface{}) error {
    // Process this batch of 100 resources
    for _, res := range batch {
        if err := processResource(res); err != nil {
            return err
        }
    }
    return nil
})

if err != nil {
    return fmt.Errorf("batch processing failed: %w", err)
}
```

---

## 4. VirtualList Interface

**Package**: `internal/tui/list`

Implements virtual scrolling for large TUI lists using Bubble Tea.

```go
package list

import tea "github.com/charmbracelet/bubbletea"

// VirtualList renders only visible items from a large dataset
type VirtualList interface {
    // Update handles Bubble Tea messages (keyboard, window size)
    Update(msg tea.Msg) (tea.Model, tea.Cmd)

    // View renders only visible rows within the viewport
    View() string

    // SetItems replaces the full dataset
    SetItems(items []interface{})

    // SelectedItem returns the currently selected item
    SelectedItem() interface{}

    // SelectedIndex returns the index of the selected item
    SelectedIndex() int
}

// Model implements VirtualList
type Model struct {
    items       []interface{}
    viewport    viewport.Model
    visibleFrom int
    visibleTo   int
    selected    int
    height      int
    width       int
    renderFunc  func(item interface{}, selected bool) string
}

func New(width, height int, renderFunc func(interface{}, bool) string) Model {
    vp := viewport.New(width, height)
    return Model{
        items:      []interface{}{},
        viewport:   vp,
        height:     height,
        width:      width,
        renderFunc: renderFunc,
    }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up", "k":
            if m.selected > 0 {
                m.selected--
                m.updateVisibleRange()
            }
        case "down", "j":
            if m.selected < len(m.items)-1 {
                m.selected++
                m.updateVisibleRange()
            }
        case "home":
            m.selected = 0
            m.updateVisibleRange()
        case "end":
            m.selected = len(m.items) - 1
            m.updateVisibleRange()
        }
    case tea.WindowSizeMsg:
        m.height = msg.Height
        m.width = msg.Width
        m.viewport.Width = msg.Width
        m.viewport.Height = msg.Height
        m.updateVisibleRange()
    }

    return m, nil
}

func (m Model) View() string {
    if len(m.items) == 0 {
        return "No items"
    }

    var rows []string
    for i := m.visibleFrom; i <= m.visibleTo && i < len(m.items); i++ {
        row := m.renderFunc(m.items[i], i == m.selected)
        rows = append(rows, row)
    }

    content := lipgloss.JoinVertical(lipgloss.Left, rows...)
    m.viewport.SetContent(content)
    return m.viewport.View()
}

func (m *Model) updateVisibleRange() {
    // Ensure selected item is within viewport
    if m.selected < m.visibleFrom {
        m.visibleFrom = m.selected
    }
    if m.selected >= m.visibleTo {
        m.visibleFrom = m.selected - m.height + 1
    }

    m.visibleFrom = max(0, m.visibleFrom)
    m.visibleTo = min(len(m.items)-1, m.visibleFrom+m.height-1)
}
```

**Usage Example**:

```go
// Custom render function
renderFunc := func(item interface{}, selected bool) string {
    rec := item.(*Recommendation)
    style := lipgloss.NewStyle()
    if selected {
        style = style.Background(lipgloss.Color("6"))
    }
    return style.Render(fmt.Sprintf("%s: $%.2f", rec.ResourceName, rec.Savings))
}

// Create virtual list
listModel := list.New(80, 20, renderFunc)
listModel.SetItems(recommendations) // Can be 10,000+ items

// Use in Bubble Tea program
p := tea.NewProgram(listModel)
if err := p.Start(); err != nil {
    return err
}
```

---

## Interface Dependencies

```
┌──────────────┐
│ CLI Handler  │
└──────┬───────┘
       │ uses
       ▼
┌──────────────┐
│ Sorter       │ (validates and sorts items)
└──────────────┘

┌──────────────┐
│ Engine       │
└──────┬───────┘
       │ uses
       ▼
┌──────────────┐       ┌──────────────────┐
│ CacheStore   │───────│ BatchProcessor   │
└──────────────┘       └──────────────────┘

┌──────────────┐
│ TUI App      │
└──────┬───────┘
       │ uses
       ▼
┌──────────────┐
│ VirtualList  │
└──────────────┘
```

---

## Testing Strategy

Each interface has corresponding test suite:

### Sorter Tests
- Valid field validation
- Invalid field error messages
- Ascending/descending sort correctness
- Type safety (non-recommendation items)

### CacheStore Tests
- Cache hit/miss behavior
- TTL expiration
- File I/O errors
- Concurrent access (if needed)

### BatchProcessor Tests
- Batch boundary calculation
- Progress callback invocation
- Error handling (stop on first error)
- Context cancellation

### VirtualList Tests
- Scroll behavior (up/down/pgup/pgdown)
- Visible range calculation
- Window resize handling
- Empty list rendering

---

## Next Steps

These interfaces will be implemented during the task execution phase (`/speckit.implement`). All interfaces follow:
- Constitution Principle VI (no TODOs/stubs)
- Go best practices (context-aware, error handling)
- Testability (clear contracts, mockable)

See [plan.md](../plan.md) for Phase 2 task breakdown.
