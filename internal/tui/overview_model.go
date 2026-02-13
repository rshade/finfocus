package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rshade/finfocus/internal/engine"
)

// maxOverviewResourcesPerPage is the pagination threshold.
const maxOverviewResourcesPerPage = 250

// OverviewResourceLoadedMsg is sent when a single resource's data is enriched.
type OverviewResourceLoadedMsg struct {
	Index int
	Row   engine.OverviewRow
}

// OverviewLoadingProgressMsg is sent periodically during loading.
type OverviewLoadingProgressMsg struct {
	Loaded int
	Total  int
}

// OverviewAllResourcesLoadedMsg is sent when all resources are enriched.
type OverviewAllResourcesLoadedMsg struct{}

// OverviewModel is the Bubble Tea model for the interactive overview dashboard.
//
//nolint:recvcheck // Bubble Tea requires value receivers for Init/Update/View interface methods.
type OverviewModel struct {
	// View state
	state   ViewState
	allRows []engine.OverviewRow // All loaded rows (source of truth)
	rows    []engine.OverviewRow // Filtered/sorted rows
	ctx     context.Context      // Context for trace ID

	// Interactive components
	table     table.Model
	textInput textinput.Model
	selected  int

	// Display configuration
	width      int
	height     int
	sortBy     SortField
	showFilter bool

	// Loading state
	loadedCount int
	totalCount  int
	progressMsg string

	// Pagination
	paginationEnabled bool
	currentPage       int
	totalPages        int

	// Loading spinner
	loadingState *LoadingState

	// Error state
	err error
}

// NewOverviewModel creates a new interactive overview model.
func NewOverviewModel(
	ctx context.Context,
	skeletonRows []engine.OverviewRow,
	totalCount int,
) (OverviewModel, tea.Cmd) {
	m := OverviewModel{
		state:       ViewStateLoading,
		allRows:     skeletonRows,
		rows:        skeletonRows,
		ctx:         ctx,
		totalCount:  totalCount,
		loadedCount: 0,
		width:       defaultWidth,
		height:      defaultHeight,
		sortBy:      SortByCost,
		textInput:   newTextInput(),
		currentPage: 1,
	}

	// Initialize table with skeleton data
	m.table = m.buildOverviewTable()

	// Initialize loading spinner
	m.loadingState = NewLoadingState()
	return m, m.loadingState.Init()
}

// Init initializes the model (Bubble Tea interface).
func (m OverviewModel) Init() tea.Cmd {
	if m.loadingState != nil {
		return m.loadingState.Init()
	}
	return NewLoadingState().Init()
}

// Update handles messages and updates the model state (Bubble Tea interface).
func (m OverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window resizing
	if winMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = winMsg.Width
		m.height = winMsg.Height
		m.rebuildTable()
		return m, nil
	}

	// Handle resource loaded
	if loadedMsg, ok := msg.(OverviewResourceLoadedMsg); ok {
		return m.handleResourceLoaded(loadedMsg)
	}

	// Handle progress update
	if progressMsg, ok := msg.(OverviewLoadingProgressMsg); ok {
		return m.handleLoadingProgress(progressMsg)
	}

	// Handle all resources loaded
	if _, ok := msg.(OverviewAllResourcesLoadedMsg); ok {
		return m.handleAllResourcesLoaded()
	}

	// Handle filter input
	if m.showFilter {
		return m.handleFilterInput(msg)
	}

	// Handle state-specific updates
	switch m.state {
	case ViewStateLoading:
		return m, nil
	case ViewStateList:
		return m.handleListUpdate(msg)
	case ViewStateDetail:
		return m.handleDetailUpdate(msg)
	case ViewStateQuitting, ViewStateError:
		return m, nil
	default:
		return m, nil
	}
}

func (m OverviewModel) handleResourceLoaded(msg OverviewResourceLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Index >= 0 && msg.Index < len(m.allRows) {
		m.allRows[msg.Index] = msg.Row
		m.loadedCount++

		// Update filtered/sorted view (applyFilter calls refreshTable)
		m.applyFilter(m.textInput.Value())
	}
	return m, nil
}

func (m OverviewModel) handleLoadingProgress(msg OverviewLoadingProgressMsg) (tea.Model, tea.Cmd) {
	percent := 0
	if msg.Total > 0 {
		percent = (msg.Loaded * 100) / msg.Total //nolint:mnd // Percentage calculation.
	}
	m.progressMsg = fmt.Sprintf("Loading: %d/%d resources (%d%%)", msg.Loaded, msg.Total, percent)
	return m, nil
}

func (m OverviewModel) handleAllResourcesLoaded() (tea.Model, tea.Cmd) {
	m.state = ViewStateList
	m.loadedCount = m.totalCount
	m.progressMsg = ""

	// Apply initial sort and filter
	m.applyFilter(m.textInput.Value())
	m.refreshTable()
	m.enablePaginationIfNeeded()

	return m, nil
}

func (m OverviewModel) handleFilterInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case keyEnter, keyEsc:
			m.showFilter = false
			m.textInput.Blur()
			m.applyFilter(m.textInput.Value())
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m OverviewModel) handleListUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	return m.handleListKeypress(keyMsg)
}

func (m OverviewModel) handleListKeypress(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case keyQuit, keyCtrlC:
		m.state = ViewStateQuitting
		return m, tea.Quit
	case keyEnter:
		m.selected = m.absoluteIndex(m.table.Cursor())
		if m.selected >= 0 && m.selected < len(m.rows) {
			m.state = ViewStateDetail
		}
		return m, nil
	case keySlash:
		m.showFilter = true
		m.textInput.Focus()
		return m, textinput.Blink
	case keyS:
		m.cycleSort()
		return m, nil
	case keyEsc:
		if m.textInput.Value() != "" {
			m.textInput.SetValue("")
			m.applyFilter("")
		}
		return m, nil
	case "pgup":
		if m.paginationEnabled && m.currentPage > 1 {
			m.currentPage--
			m.rebuildTable()
		}
		return m, nil
	case "pgdown":
		if m.paginationEnabled && m.currentPage < m.totalPages {
			m.currentPage++
			m.rebuildTable()
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(keyMsg)
		return m, cmd
	}
}

// absoluteIndex converts a page-relative table cursor to an absolute row index.
func (m OverviewModel) absoluteIndex(cursor int) int {
	if m.paginationEnabled {
		return (m.currentPage-1)*maxOverviewResourcesPerPage + cursor
	}
	return cursor
}

func (m OverviewModel) handleDetailUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case keyQuit, keyCtrlC:
			m.state = ViewStateQuitting
			return m, tea.Quit
		case keyEsc:
			m.state = ViewStateList
			m.table.Focus()
			return m, nil
		}
	}
	return m, nil
}

// cycleSortField advances to the next sort field.
func (m *OverviewModel) cycleSort() {
	m.sortBy = (m.sortBy + 1) % numSortFields
	m.refreshTable()
}

// refreshTable re-sorts and rebuilds the table.
func (m *OverviewModel) refreshTable() {
	// Sort rows
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

	m.rebuildTable()
}

// rebuildTable reconstructs the table with current rows and pagination.
func (m *OverviewModel) rebuildTable() {
	m.table = m.buildOverviewTable()
}

// buildOverviewTable creates a new table model with current configuration.
func (m *OverviewModel) buildOverviewTable() table.Model {
	columns := []table.Column{
		{Title: "Resource", Width: 30},  //nolint:mnd // Column width.
		{Title: "Type", Width: 20},      //nolint:mnd // Column width.
		{Title: "Status", Width: 10},    //nolint:mnd // Column width.
		{Title: "Actual", Width: 12},    //nolint:mnd // Column width.
		{Title: "Projected", Width: 12}, //nolint:mnd // Column width.
		{Title: "Delta", Width: 12},     //nolint:mnd // Column width.
		{Title: "Drift%", Width: 8},     //nolint:mnd // Column width.
		{Title: "Recs", Width: 4},       //nolint:mnd // Column width.
	}

	visibleRows := m.getVisibleRows()
	rows := make([]table.Row, len(visibleRows))

	for i, overviewRow := range visibleRows {
		resourceName := truncateResourceName(overviewRow.URN)
		statusStr := overviewRow.Status.String()

		actualStr := "-"
		if overviewRow.ActualCost != nil {
			actualStr = fmt.Sprintf("$%.2f", overviewRow.ActualCost.MTDCost)
		}

		projectedStr := "-"
		if overviewRow.ProjectedCost != nil {
			projectedStr = fmt.Sprintf("$%.2f", overviewRow.ProjectedCost.MonthlyCost)
		}

		deltaStr := "-"
		if overviewRow.CostDrift != nil {
			deltaStr = fmt.Sprintf("$%.2f", overviewRow.CostDrift.Delta)
		}

		driftPctStr := "-"
		if overviewRow.CostDrift != nil {
			driftPctStr = fmt.Sprintf("%.1f%%", overviewRow.CostDrift.PercentDrift)
		}

		recsStr := "-"
		if len(overviewRow.Recommendations) > 0 {
			recsStr = strconv.Itoa(len(overviewRow.Recommendations))
		}

		rows[i] = table.Row{
			resourceName,
			overviewRow.Type,
			statusStr,
			actualStr,
			projectedStr,
			deltaStr,
			driftPctStr,
			recsStr,
		}
	}

	availableHeight := m.height - summaryHeight - 1
	if availableHeight < minHeight {
		availableHeight = minHeight
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(availableHeight),
	)

	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	t.SetStyles(s)

	return t
}

// truncateResourceName shortens a URN for display.
func truncateResourceName(urn string) string {
	const maxLen = 30
	if urn == "" {
		return urn
	}
	if len(urn) <= maxLen {
		return urn
	}
	// Extract resource name from URN (last component).
	// strings.Split always returns at least one element so no length check needed.
	parts := strings.Split(urn, "::")
	name := parts[len(parts)-1]
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}

// applyFilter filters rows based on text input. It always calls refreshTable
// and enablePaginationIfNeeded to keep pagination state consistent.
func (m *OverviewModel) applyFilter(filterText string) {
	if filterText == "" {
		m.rows = m.allRows
	} else {
		query := strings.ToLower(filterText)
		filtered := []engine.OverviewRow{}

		for _, row := range m.allRows {
			if strings.Contains(strings.ToLower(row.URN), query) ||
				strings.Contains(strings.ToLower(row.Type), query) {
				filtered = append(filtered, row)
			}
		}

		m.rows = filtered
	}

	m.enablePaginationIfNeeded()
	m.refreshTable()
}

// getCost returns the primary cost for sorting.
func (m *OverviewModel) getCost(row engine.OverviewRow) float64 {
	if row.ProjectedCost != nil {
		return row.ProjectedCost.MonthlyCost
	}
	if row.ActualCost != nil {
		return row.ActualCost.MTDCost
	}
	return 0.0
}

// getDelta returns the drift delta for sorting.
func (m *OverviewModel) getDelta(row engine.OverviewRow) float64 {
	if row.CostDrift != nil {
		return row.CostDrift.Delta
	}
	return 0.0
}

// enablePaginationIfNeeded checks if pagination should be enabled.
func (m *OverviewModel) enablePaginationIfNeeded() {
	if len(m.rows) > maxOverviewResourcesPerPage {
		m.paginationEnabled = true
		m.totalPages = (len(m.rows) + maxOverviewResourcesPerPage - 1) / maxOverviewResourcesPerPage
		m.currentPage = 1
	} else {
		m.paginationEnabled = false
	}
}

// getVisibleRows returns the rows for the current page.
func (m *OverviewModel) getVisibleRows() []engine.OverviewRow {
	if !m.paginationEnabled {
		return m.rows
	}

	start := (m.currentPage - 1) * maxOverviewResourcesPerPage
	end := start + maxOverviewResourcesPerPage
	if end > len(m.rows) {
		end = len(m.rows)
	}

	if start >= len(m.rows) {
		return []engine.OverviewRow{}
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

// AllRows returns all loaded rows (for external access).
func (m *OverviewModel) AllRows() []engine.OverviewRow {
	return m.allRows
}
