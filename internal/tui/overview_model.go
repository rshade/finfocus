package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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

// PhaseNames is the ordered list of loading phases for the checklist display.
//
//nolint:gochecknoglobals // Package-level slice used across tui package for phase rendering.
var PhaseNames = []string{
	"Loading stack state",
	"Detecting changes",
	"Merging resources",
	"Starting cost plugins",
	"Preparing cost engine",
	"Enriching resources",
}

// OverviewPhaseMsg reports which phase of data loading is active.
// It is sent by the background goroutine to update the initializing spinner text.
type OverviewPhaseMsg struct {
	Phase string // human-readable label (kept for logging/compat)
	Index int    // 0-based index into PhaseNames
}

// OverviewPassphraseRequiredMsg signals that the stack is encrypted
// and PULUMI_CONFIG_PASSPHRASE must be collected from the user.
type OverviewPassphraseRequiredMsg struct{}

// OverviewDataReadyMsg signals that initial data loading is complete and the
// model should transition from ViewStateInitializing to ViewStateLoading.
type OverviewDataReadyMsg struct {
	Rows       []engine.OverviewRow
	TotalCount int
	StackName  string
}

// OverviewInitErrorMsg signals that initial data loading failed.
// The TUI transitions to ViewStateError and exits.
type OverviewInitErrorMsg struct {
	Err error
}

// OverviewModel is the Bubble Tea model for the interactive overview dashboard.
//
//nolint:recvcheck // Bubble Tea requires value receivers for Init/Update/View interface methods.
type OverviewModel struct {
	// View state
	state     ViewState
	allRows   []engine.OverviewRow // All loaded rows (source of truth)
	rows      []engine.OverviewRow // Filtered/sorted rows
	ctx       context.Context      // Context for trace ID
	stackName string               // Stack name from data loading

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

	// Phase checklist tracking
	currentPhaseIndex int

	// Passphrase prompt (inline TUI input when stack is encrypted)
	showPassphraseInput bool
	passphraseInput     textinput.Model
	passphraseChan      chan<- string

	// Error state
	err error

	// State-only / on-demand preview fields (Issue 3).
	isStateOnly      bool          // true when no preview has been loaded yet
	isPreviewLoading bool          // true while pulumi preview is running in background
	previewLoadStart time.Time     // when preview started (for elapsed display)
	previewLoaded    bool          // true after OverviewChangesReadyMsg received
	previewCmd       tea.Cmd       // command that starts background preview (injected at construction)
	previewElapsed   time.Duration // elapsed time since preview started
}

// NewOverviewModel creates a new interactive overview model.
// When skeletonRows is nil, the model starts in ViewStateInitializing
// (before data is available). When non-nil, it starts in ViewStateLoading
// (enrichment phase), preserving existing behavior.
//
// passphraseChan is an optional channel used to deliver a PULUMI_CONFIG_PASSPHRASE
// when the stack uses passphrase encryption. Pass nil if no passphrase check is needed.
//
// previewCmd is an optional Bubble Tea command that, when invoked, runs
// pulumi preview in the background and sends OverviewChangesReadyMsg.
// Pass nil when preview has already been run before TUI launch or in tests.
// When non-nil and isStateOnly is true, the user can press 'p' to trigger it.
func NewOverviewModel(
	ctx context.Context,
	skeletonRows []engine.OverviewRow,
	totalCount int,
	passphraseChan chan<- string,
	previewCmd tea.Cmd,
) (OverviewModel, tea.Cmd) {
	initialState := ViewStateLoading
	if skeletonRows == nil {
		initialState = ViewStateInitializing
		skeletonRows = []engine.OverviewRow{}
	}

	pi := textinput.New()
	pi.EchoMode = textinput.EchoPassword
	pi.Placeholder = "passphrase"

	m := OverviewModel{
		state:           initialState,
		allRows:         skeletonRows,
		rows:            skeletonRows,
		ctx:             ctx,
		totalCount:      totalCount,
		loadedCount:     0,
		width:           defaultWidth,
		height:          defaultHeight,
		sortBy:          SortByCost,
		textInput:       newTextInput(),
		currentPage:     1,
		passphraseInput: pi,
		passphraseChan:  passphraseChan,
		previewCmd:      previewCmd,
	}

	// Initialize table with skeleton data
	m.table = m.buildOverviewTable()

	// Initialize loading spinner
	m.loadingState = NewLoadingState()
	return m, m.loadingState.Init()
}

// Err returns any error that caused the TUI to exit (e.g., init failure).
func (m OverviewModel) Err() error {
	return m.err
}

// Init initializes the model (Bubble Tea interface).
func (m OverviewModel) Init() tea.Cmd {
	if m.loadingState != nil {
		return m.loadingState.Init()
	}
	return NewLoadingState().Init()
}

// Update handles messages and updates the model state (Bubble Tea interface).
//
//nolint:funlen,gocognit // Bubble Tea Update dispatches across all message types and view states; extraction would harm readability.
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

	// Handle passphrase required
	if _, ok := msg.(OverviewPassphraseRequiredMsg); ok {
		m.showPassphraseInput = true
		m.passphraseInput.Focus()
		return m, textinput.Blink
	}

	// Handle passphrase input (when prompt is visible, capture all key events)
	if m.showPassphraseInput {
		return m.handlePassphraseInput(msg)
	}

	// Handle phase message (initializing state)
	if phaseMsg, ok := msg.(OverviewPhaseMsg); ok {
		m.progressMsg = phaseMsg.Phase
		m.currentPhaseIndex = phaseMsg.Index
		return m, nil
	}

	// Handle data ready (initializing → loading transition)
	if dataMsg, ok := msg.(OverviewDataReadyMsg); ok {
		// Defensive copy: allRows and rows must not share backing arrays
		// because refreshTable sorts m.rows in-place.
		m.allRows = make([]engine.OverviewRow, len(dataMsg.Rows))
		copy(m.allRows, dataMsg.Rows)
		m.rows = make([]engine.OverviewRow, len(dataMsg.Rows))
		copy(m.rows, dataMsg.Rows)
		m.totalCount = dataMsg.TotalCount
		m.stackName = dataMsg.StackName
		m.state = ViewStateLoading
		m.table = m.buildOverviewTable()
		return m, nil
	}

	// Handle init error
	if errMsg, ok := msg.(OverviewInitErrorMsg); ok {
		m.state = ViewStateError
		m.err = errMsg.Err
		return m, tea.Quit
	}

	// Handle progress update
	if progressMsg, ok := msg.(OverviewLoadingProgressMsg); ok {
		return m.handleLoadingProgress(progressMsg)
	}

	// Handle all resources loaded
	if _, ok := msg.(OverviewAllResourcesLoadedMsg); ok {
		return m.handleAllResourcesLoaded()
	}

	// Handle on-demand preview messages (Issue 3: state-first phased loading).
	if _, ok := msg.(OverviewPreviewStartedMsg); ok {
		m.isPreviewLoading = true
		m.previewLoadStart = time.Now()
		return m, tickPreviewCmd()
	}

	if _, ok := msg.(OverviewPreviewTickMsg); ok {
		if m.isPreviewLoading {
			// Compute elapsed from previewLoadStart, ignoring the tick's own elapsed.
			m.previewElapsed = time.Since(m.previewLoadStart)
			return m, tickPreviewCmd()
		}
		return m, nil
	}

	if changesMsg, ok := msg.(OverviewChangesReadyMsg); ok {
		m.isPreviewLoading = false
		m.previewLoaded = true
		m.isStateOnly = false
		// Safe: Bubble Tea Update() is single-threaded; no concurrent reads on allRows.
		engine.ApplyChangesToRows(m.allRows, changesMsg.StatusByURN)
		m.applyFilter(m.textInput.Value())
		return m, nil
	}

	// Handle state-only activation (sent after OverviewDataReadyMsg when no preview ran).
	if setStateMsg, ok := msg.(OverviewSetStateOnlyMsg); ok {
		m.isStateOnly = true
		m.previewCmd = setStateMsg.PreviewCmd
		m.rebuildTable() // Rebuild to show "Projected*" header.
		return m, nil
	}

	// Handle filter input
	if m.showFilter {
		return m.handleFilterInput(msg)
	}

	// Handle state-specific updates
	switch m.state {
	case ViewStateInitializing:
		// Handle quit keys during initialization
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case keyQuit, keyCtrlC:
				m.state = ViewStateQuitting
				return m, tea.Quit
			}
		}
		// Forward spinner ticks to keep the spinner animated
		if m.loadingState != nil {
			return m, m.loadingState.Update(msg)
		}
		return m, nil
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

	// Apply initial sort and filter (applyFilter calls refreshTable internally)
	m.applyFilter(m.textInput.Value())
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

// handlePassphraseInput handles key events when the passphrase prompt is visible.
// On Enter: sends the passphrase to passphraseChan and hides the prompt.
// On Esc or Ctrl+C: quits the TUI (goroutine unblocks via context cancellation).
// Other keys are forwarded to the text input for character entry.
func (m OverviewModel) handlePassphraseInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case keyEnter:
			if m.passphraseChan != nil {
				m.passphraseChan <- m.passphraseInput.Value()
			}
			m.passphraseInput.SetValue("")
			m.showPassphraseInput = false
			return m, nil
		case keyCtrlC, keyEsc:
			m.state = ViewStateQuitting
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.passphraseInput, cmd = m.passphraseInput.Update(msg)
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
	case keyP:
		// Load pending changes on demand when in state-only mode.
		if !m.isPreviewLoading && !m.previewLoaded && m.previewCmd != nil {
			return m, tea.Batch(
				func() tea.Msg { return OverviewPreviewStartedMsg{} },
				m.previewCmd,
			)
		}
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
	projectedHeader := "Projected"
	if m.isStateOnly && !m.previewLoaded {
		projectedHeader = "Projected*"
	}
	columns := []table.Column{
		{Title: "Resource", Width: 30},      //nolint:mnd // Column width.
		{Title: "Type", Width: 20},          //nolint:mnd // Column width.
		{Title: "Status", Width: 10},        //nolint:mnd // Column width.
		{Title: "Actual", Width: 12},        //nolint:mnd // Column width.
		{Title: projectedHeader, Width: 12}, //nolint:mnd // Column width.
		{Title: "Delta", Width: 12},         //nolint:mnd // Column width.
		{Title: "Drift%", Width: 8},         //nolint:mnd // Column width.
		{Title: "Recs", Width: 4},           //nolint:mnd // Column width.
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
		// Copy to avoid aliasing; refreshTable sorts m.rows in-place
		// and must not reorder the source m.allRows.
		m.rows = make([]engine.OverviewRow, len(m.allRows))
		copy(m.rows, m.allRows)
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

// enablePaginationIfNeeded checks if pagination should be enabled and clamps
// the current page to valid bounds.
func (m *OverviewModel) enablePaginationIfNeeded() {
	if len(m.rows) > maxOverviewResourcesPerPage {
		m.paginationEnabled = true
		m.totalPages = (len(m.rows) + maxOverviewResourcesPerPage - 1) / maxOverviewResourcesPerPage
		if m.currentPage > m.totalPages {
			m.currentPage = m.totalPages
		}
		if m.currentPage < 1 {
			m.currentPage = 1
		}
	} else {
		m.paginationEnabled = false
		m.currentPage = 1
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

// tickPreviewCmd returns a command that fires OverviewPreviewTickMsg after 1 second.
// The model computes the actual elapsed from m.previewLoadStart in the handler.
func tickPreviewCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return OverviewPreviewTickMsg{}
	})
}
