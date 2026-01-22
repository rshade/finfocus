package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rshade/finfocus/internal/engine"
)

// Terminal and layout constants.
const (
	defaultWidth  = 80
	defaultHeight = 20
	minHeight     = 5
	summaryHeight = 8
)

// Filter input constants.
const (
	filterInputWidth     = 30  // Width of the filter text input field.
	filterInputCharLimit = 156 // Maximum characters allowed in filter input.
)

// Keyboard constants.
const (
	keyEsc   = "esc"
	keyEnter = "enter"
	keyQuit  = "q"
	keyCtrlC = "ctrl+c"
	keySlash = "/"
	keyS     = "s"
)

// ViewState represents the current state of the TUI view.
type ViewState int

const (
	// ViewStateLoading indicates data is being fetched.
	ViewStateLoading ViewState = iota
	// ViewStateList shows the resource table.
	ViewStateList
	// ViewStateDetail shows details for a single resource.
	ViewStateDetail
	// ViewStateQuitting indicates the application is exiting.
	ViewStateQuitting
	// ViewStateError indicates a fatal error occurred.
	ViewStateError
)

// SortField represents the field to sort the resource table by.
type SortField int

const (
	// SortByCost sorts by monthly/total cost.
	SortByCost SortField = iota
	// SortByName sorts by resource ID.
	SortByName
	// SortByType sorts by resource type.
	SortByType
	// SortByDelta sorts by cost delta.
	SortByDelta
)

const (
	// numSortFields is the number of available sort fields.
	numSortFields = 4
)

// Messages.
type loadingCompleteMsg struct {
	results []engine.CostResult
	err     error
}

// CostViewModel is the Bubble Tea model for interactive cost display.
type CostViewModel struct {
	// View state
	state      ViewState
	allResults []engine.CostResult // All loaded results (source of truth)
	results    []engine.CostResult // Currently visible (filtered/sorted)

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
	loading  *LoadingState
	fetchCmd tea.Cmd

	// Actual Cost specific
	groupBy      engine.GroupBy
	aggregations []engine.CrossProviderAggregation
	isActual     bool

	// Error handling
	err error
}

func newTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Filter resources..."
	ti.CharLimit = filterInputCharLimit
	ti.Width = filterInputWidth
	return ti
}

// NewCostViewModel creates a new model with the given results.
func NewCostViewModel(results []engine.CostResult) *CostViewModel {
	m := &CostViewModel{
		state:      ViewStateList,
		allResults: results,
		results:    results,
		table:      NewResultTable(results, defaultHeight),
		textInput:  newTextInput(),
	}
	m.applySort() // Apply default sort
	return m
}

// NewCostViewModelWithLoading creates a model that starts in loading state.
func NewCostViewModelWithLoading(fetcher func() ([]engine.CostResult, error)) *CostViewModel {
	m := &CostViewModel{
		state:     ViewStateLoading,
		loading:   NewLoadingState(),
		textInput: newTextInput(),
		fetchCmd: func() tea.Msg {
			res, err := fetcher()
			return loadingCompleteMsg{results: res, err: err}
		},
	}
	return m
}

// NewCostViewModelFromActual creates a new model for actual costs.
func NewCostViewModelFromActual(results []engine.CostResult, groupBy engine.GroupBy) *CostViewModel {
	m := &CostViewModel{
		state:      ViewStateList,
		allResults: results,
		results:    results,
		groupBy:    groupBy,
		isActual:   true,
		textInput:  newTextInput(),
	}

	if groupBy.IsTimeBasedGrouping() {
		aggs, err := engine.CreateCrossProviderAggregation(results, groupBy)
		if err != nil {
			// Fall back to non-aggregated view on error.
			m.table = NewActualCostTable(results, defaultHeight)
			return m
		}
		m.aggregations = aggs
		m.table = NewAggregationTable(aggs, defaultHeight)
	} else {
		m.table = NewActualCostTable(results, defaultHeight)
	}
	return m
}

// Init initializes the model.
func (m *CostViewModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.state == ViewStateLoading {
		// Start spinner and fetch
		cmds = append(cmds, m.loading.Init(), m.fetchCmd)
	} else if m.showFilter {
		// Only include blink command when filter is visible (avoids unnecessary command batching).
		cmds = append(cmds, textinput.Blink)
	}
	return tea.Batch(cmds...)
}

// Update handles messages and updates the model state.
func (m *CostViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window resizing
	if winMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = winMsg.Width
		m.height = winMsg.Height
		m.rebuildTable()
	}

	// Handle loading complete
	if loadMsg, ok := msg.(loadingCompleteMsg); ok {
		return m.handleLoadingComplete(loadMsg)
	}

	// Handle filter input
	if m.showFilter {
		return m.handleFilterInput(msg)
	}

	// Handle state-specific updates
	switch m.state {
	case ViewStateLoading:
		return m.handleLoadingUpdate(msg)
	case ViewStateList:
		return m.handleListUpdate(msg)
	case ViewStateDetail, ViewStateQuitting, ViewStateError:
		return m.handleGenericUpdate(msg)
	default:
		return m, nil
	}
}

func (m *CostViewModel) handleLoadingComplete(msg loadingCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = ViewStateError
		return m, tea.Quit
	}
	m.allResults = msg.results
	m.results = msg.results
	m.state = ViewStateList
	m.applySort()
	m.rebuildTable()
	return m, nil
}

func (m *CostViewModel) handleFilterInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case keyEnter, keyEsc:
			m.showFilter = false
			m.textInput.Blur()
			m.applyFilter()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *CostViewModel) handleLoadingUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.loading.Update(msg)
}

func (m *CostViewModel) handleListUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case keyQuit, keyCtrlC:
			m.state = ViewStateQuitting
			return m, tea.Quit
		case keyEnter:
			if m.isActual && m.groupBy.IsTimeBasedGrouping() {
				return m, nil
			}
			m.selected = m.table.Cursor()
			m.state = ViewStateDetail
			return m, nil
		case keySlash:
			m.showFilter = true
			m.textInput.Focus()
			return m, nil
		case keyS:
			m.cycleSort()
			return m, nil
		case keyEsc:
			if m.textInput.Value() != "" {
				m.textInput.SetValue("")
				m.applyFilter()
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *CostViewModel) handleGenericUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case keyQuit, keyCtrlC:
			m.state = ViewStateQuitting
			return m, tea.Quit
		case keyEsc:
			if m.state == ViewStateDetail {
				m.state = ViewStateList
				m.table.Focus()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *CostViewModel) applyFilter() {
	val := m.textInput.Value()
	if val == "" {
		m.results = m.allResults
	} else {
		var filtered []engine.CostResult
		query := strings.ToLower(val)
		for _, r := range m.allResults {
			if strings.Contains(strings.ToLower(r.ResourceType), query) ||
				strings.Contains(strings.ToLower(r.ResourceID), query) {
				filtered = append(filtered, r)
			}
		}
		m.results = filtered
	}
	m.applySort()
	m.rebuildTable()
}

func (m *CostViewModel) cycleSort() {
	m.sortBy = (m.sortBy + 1) % numSortFields
	m.applySort()
	m.rebuildTable()
}

func (m *CostViewModel) applySort() {
	if m.isActual && m.groupBy.IsTimeBasedGrouping() {
		return
	}

	sort.Slice(m.results, func(i, j int) bool {
		a, b := m.results[i], m.results[j]
		switch m.sortBy {
		case SortByCost:
			costA, costB := a.Monthly, b.Monthly
			if m.isActual {
				costA, costB = a.TotalCost, b.TotalCost
			}
			return costA > costB
		case SortByName:
			return a.ResourceID < b.ResourceID
		case SortByType:
			return a.ResourceType < b.ResourceType
		case SortByDelta:
			return a.Delta > b.Delta
		default:
			return false
		}
	})
}

func (m *CostViewModel) rebuildTable() {
	availableHeight := m.height - summaryHeight - 1
	if availableHeight < minHeight {
		availableHeight = minHeight
	}

	switch {
	case m.isActual && m.groupBy.IsTimeBasedGrouping():
		m.table = NewAggregationTable(m.aggregations, availableHeight)
	case m.isActual:
		m.table = NewActualCostTable(m.results, availableHeight)
	default:
		m.table = NewResultTable(m.results, availableHeight)
	}
}

// View renders the current view.
func (m *CostViewModel) View() string {
	switch m.state {
	case ViewStateQuitting:
		return ""
	case ViewStateError:
		return fmt.Sprintf("Error: %v\n", m.err)
	case ViewStateLoading:
		return RenderLoading(m.loading)
	case ViewStateDetail:
		if m.selected >= 0 && m.selected < len(m.results) {
			return RenderDetailView(m.results[m.selected], m.width)
		}
		return "Error: selected index out of bounds"
	case ViewStateList:
		return m.renderListView()
	default:
		return ""
	}
}

func (m *CostViewModel) renderListView() string {
	summary := RenderCostSummary(m.results, m.width)
	tableView := m.table.View()

	if m.showFilter {
		return lipgloss.JoinVertical(lipgloss.Left, summary, tableView, "\nFilter: "+m.textInput.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, summary, tableView)
}
