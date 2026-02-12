# TUI Interface Contract

**Component**: Interactive Overview TUI  
**Version**: v1.0.0  
**Date**: 2026-02-11

---

## Overview Model

```go
// OverviewModel is the Bubble Tea model for the interactive overview dashboard.
//
// State machine:
//   ViewStateLoading → ViewStateList (default) ⇄ ViewStateDetail
//                   ↓
//   ViewStateError or ViewStateQuitting
//
// The model manages:
//   - Progressive data loading with updates
//   - Keyboard navigation and filtering
//   - Detail view transitions
//   - Progress banner display
type OverviewModel struct {
    // View state
    state      ViewState       // Current view (loading, list, detail, error)
    allRows    []OverviewRow   // All loaded rows (source of truth)
    rows       []OverviewRow   // Filtered/sorted rows
    ctx        context.Context // Context for trace ID
    
    // Interactive components
    table      table.Model       // Bubble Tea table
    textInput  textinput.Model   // Filter input
    detailView *DetailViewModel  // Detail view (nil if not active)
    
    // Display configuration
    width      int
    height     int
    sortBy     SortField        // Current sort field
    showFilter bool             // Filter input visible?
    
    // Loading state
    loadedCount int              // Resources loaded so far
    totalCount  int              // Total resources to load
    progressMsg string           // "Loading: 45/100 resources"
    
    // Pagination
    paginationEnabled bool
    currentPage       int
    totalPages        int
    
    // Error state
    err        error             // Fatal error (if state = ViewStateError)
}
```

---

## Core Functions

### 1. NewOverviewModel

Creates and initializes the TUI model.

```go
// NewOverviewModel creates a new interactive overview model.
//
// Parameters:
//   - ctx: Context for logging and tracing
//   - skeletonRows: Initial skeleton rows (URN, Type, Status populated)
//   - totalCount: Total resources to load
//
// Returns:
//   - OverviewModel: Initialized model in ViewStateLoading
//   - tea.Cmd: Command to start loading data
func NewOverviewModel(
    ctx context.Context,
    skeletonRows []OverviewRow,
    totalCount int,
) (OverviewModel, tea.Cmd)
```

---

### 2. Update (Bubble Tea Interface)

Handles messages and updates model state.

```go
// Update is the Bubble Tea update function.
//
// Handles:
//   - resourceLoadedMsg: Single resource data arrived
//   - loadingProgressMsg: Progress update (X/Y loaded)
//   - allResourcesLoadedMsg: All data fetched, hide progress banner
//   - tea.KeyMsg: Keyboard input (navigation, filter, sort, quit)
//   - tea.WindowSizeMsg: Terminal resize
//
// Returns:
//   - tea.Model: Updated model
//   - tea.Cmd: Command to execute (or nil)
func (m OverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
```

**Key Messages**:

```go
// resourceLoadedMsg is sent when a single resource's cost data is fetched.
type resourceLoadedMsg struct {
    URN  string
    Row  OverviewRow
}

// loadingProgressMsg is sent periodically during loading.
type loadingProgressMsg struct {
    Loaded int
    Total  int
}

// allResourcesLoadedMsg is sent when all resources are enriched.
type allResourcesLoadedMsg struct{}
```

---

### 3. View (Bubble Tea Interface)

Renders the current view.

```go
// View is the Bubble Tea view function.
//
// Renders:
//   - ViewStateLoading: Spinner + progress banner
//   - ViewStateList: Table + optional filter input + optional progress banner
//   - ViewStateDetail: Detail view for selected resource
//   - ViewStateError: Error message
//   - ViewStateQuitting: Empty (terminal cleared)
//
// Returns:
//   - string: Rendered view (ANSI formatted)
func (m OverviewModel) View() string
```

---

### 4. Keyboard Shortcuts

| Key | Action | State |
|-----|--------|-------|
| `↑` / `k` | Move cursor up | List |
| `↓` / `j` | Move cursor down | List |
| `Enter` | Open detail view for selected resource | List |
| `/` | Enter filter mode | List |
| `Esc` | Exit filter mode or detail view | Filter, Detail |
| `s` | Cycle sort field (Cost → Name → Type → Delta → Cost) | List |
| `q` / `Ctrl+C` | Quit | All |
| `PgUp` / `PgDn` | Navigate pages (if pagination enabled) | List |

**Implementation**:

```go
func (m OverviewModel) handleKeyPress(key string) (tea.Model, tea.Cmd) {
    switch key {
    case "up", "k":
        m.table.MoveUp(1)
    case "down", "j":
        m.table.MoveDown(1)
    case "enter":
        return m.openDetailView()
    case "/":
        m.showFilter = true
        m.textInput.Focus()
    case "esc":
        if m.showFilter {
            m.showFilter = false
            m.textInput.Blur()
        } else if m.state == ViewStateDetail {
            m.state = ViewStateList
            m.detailView = nil
        }
    case "s":
        m.cycleSortField()
        m.refreshTable()
    case "q", "ctrl+c":
        m.state = ViewStateQuitting
        return m, tea.Quit
    }
    return m, nil
}
```

---

### 5. Sorting

```go
// cycleSortField advances to the next sort field.
func (m *OverviewModel) cycleSortField() {
    m.sortBy = (m.sortBy + 1) % numSortFields
}

// refreshTable re-sorts and re-renders the table.
func (m *OverviewModel) refreshTable() {
    switch m.sortBy {
    case SortByCost:
        sort.Slice(m.rows, func(i, j int) bool {
            return m.getCost(m.rows[i]) > m.getCost(m.rows[j])
        })
    case SortByName:
        sort.Slice(m.rows, func(i, j int) bool {
            return m.rows[i].URN < m.rows[j].URN
        })
    case SortByType:
        sort.Slice(m.rows, func(i, j int) bool {
            return m.rows[i].Type < m.rows[j].Type
        })
    case SortByDelta:
        sort.Slice(m.rows, func(i, j int) bool {
            return m.getDelta(m.rows[i]) > m.getDelta(m.rows[j])
        })
    }
    m.updateTableRows()
}

// getCost returns the primary cost for sorting (projected if available, else actual).
func (m *OverviewModel) getCost(row OverviewRow) float64 {
    if row.ProjectedCost != nil {
        return row.ProjectedCost.MonthlyCost
    }
    if row.ActualCost != nil {
        return row.ActualCost.MTDCost
    }
    return 0.0
}
```

---

### 6. Filtering

```go
// applyFilter filters rows based on text input.
func (m *OverviewModel) applyFilter(filterText string) {
    if filterText == "" {
        m.rows = m.allRows
        return
    }
    
    filtered := []OverviewRow{}
    for _, row := range m.allRows {
        if strings.Contains(strings.ToLower(row.URN), strings.ToLower(filterText)) ||
           strings.Contains(strings.ToLower(row.Type), strings.ToLower(filterText)) {
            filtered = append(filtered, row)
        }
    }
    m.rows = filtered
}
```

---

## Detail View

```go
// DetailViewModel displays details for a single resource.
type DetailViewModel struct {
    row      OverviewRow
    width    int
    height   int
}

// renderDetailView creates the detail view string.
func (m *OverviewModel) renderDetailView() string {
    if m.detailView == nil {
        return ""
    }
    
    row := m.detailView.row
    var b strings.Builder
    
    // Header
    b.WriteString(lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("39")).
        Render("Resource Detail"))
    b.WriteString("\n\n")
    
    // Metadata
    b.WriteString(fmt.Sprintf("URN:    %s\n", row.URN))
    b.WriteString(fmt.Sprintf("Type:   %s\n", row.Type))
    b.WriteString(fmt.Sprintf("Status: %s\n", formatStatus(row.Status)))
    b.WriteString("\n")
    
    // Actual Cost
    if row.ActualCost != nil {
        b.WriteString("Actual Cost (MTD)\n")
        b.WriteString(fmt.Sprintf("  Total: %s %s\n", row.ActualCost.Currency, formatCurrency(row.ActualCost.MTDCost)))
        if len(row.ActualCost.Breakdown) > 0 {
            b.WriteString("  Breakdown:\n")
            for category, cost := range row.ActualCost.Breakdown {
                b.WriteString(fmt.Sprintf("    %s: %s\n", category, formatCurrency(cost)))
            }
        }
        b.WriteString("\n")
    }
    
    // Projected Cost
    if row.ProjectedCost != nil {
        b.WriteString("Projected Cost (Monthly)\n")
        b.WriteString(fmt.Sprintf("  Total: %s %s\n", row.ProjectedCost.Currency, formatCurrency(row.ProjectedCost.MonthlyCost)))
        if len(row.ProjectedCost.Breakdown) > 0 {
            b.WriteString("  Breakdown:\n")
            for category, cost := range row.ProjectedCost.Breakdown {
                b.WriteString(fmt.Sprintf("    %s: %s\n", category, formatCurrency(cost)))
            }
        }
        b.WriteString("\n")
    }
    
    // Recommendations
    if len(row.Recommendations) > 0 {
        b.WriteString("Recommendations\n")
        for i, rec := range row.Recommendations {
            b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, rec.Description))
            b.WriteString(fmt.Sprintf("     Savings: %s %s\n", rec.Currency, formatCurrency(rec.EstimatedSavings)))
        }
        b.WriteString("\n")
    }
    
    // Footer
    b.WriteString("\nPress ESC to return")
    
    return b.String()
}
```

---

## Progress Banner

```go
// renderProgressBanner displays loading progress at the top of the screen.
func (m *OverviewModel) renderProgressBanner() string {
    if m.state != ViewStateLoading && m.loadedCount >= m.totalCount {
        return "" // Hide banner when fully loaded
    }
    
    percent := 0
    if m.totalCount > 0 {
        percent = (m.loadedCount * 100) / m.totalCount
    }
    
    msg := fmt.Sprintf("Loading: %d/%d resources (%d%%)", m.loadedCount, m.totalCount, percent)
    
    return lipgloss.NewStyle().
        Background(lipgloss.Color("39")).
        Foreground(lipgloss.Color("0")).
        Bold(true).
        Padding(0, 1).
        Width(m.width).
        Render(msg)
}
```

---

## Pagination

```go
// enablePaginationIfNeeded checks if pagination should be enabled.
func (m *OverviewModel) enablePaginationIfNeeded() {
    if len(m.allRows) > maxResourcesPerPage {
        m.paginationEnabled = true
        m.totalPages = (len(m.allRows) + maxResourcesPerPage - 1) / maxResourcesPerPage
        m.currentPage = 1
    }
}

// getVisibleRows returns the rows for the current page.
func (m *OverviewModel) getVisibleRows() []OverviewRow {
    if !m.paginationEnabled {
        return m.rows
    }
    
    start := (m.currentPage - 1) * maxResourcesPerPage
    end := start + maxResourcesPerPage
    if end > len(m.rows) {
        end = len(m.rows)
    }
    
    return m.rows[start:end]
}

// renderPaginationFooter displays page info at the bottom.
func (m *OverviewModel) renderPaginationFooter() string {
    if !m.paginationEnabled {
        return ""
    }
    
    return fmt.Sprintf("Page %d/%d | Use PgUp/PgDn to navigate", m.currentPage, m.totalPages)
}
```

---

## Testing Requirements

### Unit Tests

- State transitions (loading → list → detail)
- Keyboard input handling
- Sorting logic (all fields)
- Filtering logic (URN and Type matching)
- Pagination logic (page boundaries)

### Snapshot Tests

- Rendered views (golden file comparison)
- Progress banner display
- Detail view layout

**Test Coverage Target**: 80% (UI layer, not critical path)

---

## References

- **Existing TUI**: `internal/tui/cost_model.go`, `internal/tui/recommendations_model.go`
- **Bubble Tea Docs**: https://github.com/charmbracelet/bubbletea
- **Data Model**: `../data-model.md`

---

**Contract Version**: v1.0.0  
**Last Updated**: 2026-02-11
