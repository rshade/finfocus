package tui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// maxOverviewResourcesPerPage is the pagination threshold.
const maxOverviewResourcesPerPage = viewmodel.ClusterPageSize

// Column width constants for the overview table.
// All columns except Resource have fixed widths; Resource absorbs extra terminal width.
const (
	// colWidthType is the preferred width for the Type column when space allows.
	colWidthType      = 20
	colWidthStatus    = 12
	colWidthActual    = 12
	colWidthProjected = 12
	colWidthDelta     = 12
	colWidthDrift     = 8
	colWidthRecs      = 9
	// colWidthWarn fits "drift+2", the longest compact cell for the derived
	// warnings. A wider column would take the spare Resource width at 120 columns.
	colWidthWarn = 7
	// fixedOverviewColumnsTotal is the sum of fixed columns excluding Resource and Type.
	fixedOverviewColumnsTotal = colWidthStatus + colWidthActual +
		colWidthProjected + colWidthDelta + colWidthDrift + colWidthRecs + colWidthWarn
	// overviewColumnCount is the number of columns in the overview table.
	overviewColumnCount = 9
	// minResourceColWidth is the minimum width for the Resource column.
	minResourceColWidth = 12
	// minTypeColWidth is the minimum width for the Type column.
	minTypeColWidth = 12
	// minCompactColWidth is the minimum width when the terminal is too narrow.
	minCompactColWidth = 1
)

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

// phaseNames is the ordered list of loading phases for the checklist display.
//
//nolint:gochecknoglobals // Package-level slice used across tui package for phase rendering.
var phaseNames = []string{
	"Loading stack state",
	"Detecting changes",
	"Merging resources",
	"Starting cost plugins",
	"Preparing cost engine",
	"Enriching resources",
}

// GetPhaseNames returns a defensive copy of the phaseNames slice.
// The returned copy is safe to modify without affecting the original phaseNames.
func GetPhaseNames() []string {
	names := make([]string, len(phaseNames))
	copy(names, phaseNames)
	return names
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
	allRows   []engine.OverviewRowResult // All loaded rows (source of truth)
	rows      []engine.OverviewRowResult // Filtered/sorted rows
	ctx       context.Context            // Context for trace ID
	stackName string                     // Stack name from data loading

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
	detectErrMsg     string        // short description of change-detection failure (empty = none)

	// Budget health fields (populated by BudgetDataReadyMsg from background goroutine).
	budgetResult *engine.BudgetResult // Budget data from plugins (nil until loaded)
	budgetErr    error                // Budget fetch error (nil on success)
	budgetLoaded bool                 // True after BudgetDataReadyMsg received

	// Cluster expansion state (spec 624).
	expanded       map[string]bool // parent URN → expanded
	displayToRows  []int           // display row index → index into m.rows
	expansionNotes []string        // `†` footnotes for the list view
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
	rowResults := computeRowResults(skeletonRows)

	pi := textinput.New()
	pi.EchoMode = textinput.EchoPassword
	pi.Placeholder = "passphrase"

	m := OverviewModel{
		state:           initialState,
		allRows:         rowResults,
		rows:            slices.Clone(rowResults),
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
		expanded:        map[string]bool{},
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

// resourceColumnWidth returns the computed width for the Resource column.
func (m *OverviewModel) resourceColumnWidth() int {
	resourceWidth, _ := m.variableColumnWidths()
	return resourceWidth
}

// typeColumnWidth returns the computed width for the Type column.
func (m *OverviewModel) typeColumnWidth() int {
	_, typeWidth := m.variableColumnWidths()
	return typeWidth
}

// variableColumnWidths calculates widths for Resource and Type using the
// available table width budget after fixed columns and cell padding.
func (m *OverviewModel) variableColumnWidths() (int, int) {
	available := m.width - borderPadding - tablePaddingForColumns(overviewColumnCount) - fixedOverviewColumnsTotal
	if available <= 0 {
		return minCompactColWidth, minCompactColWidth
	}
	if available == 1 {
		return minCompactColWidth, minCompactColWidth
	}

	if available < minResourceColWidth+minTypeColWidth {
		// Terminal too narrow for minimum target widths; split proportionally
		// while preserving at least one character per column.
		resourceWidth := (available + 1) / 2 //nolint:mnd // Splitting available width between 2 variable columns.
		typeWidth := available - resourceWidth
		if resourceWidth < minCompactColWidth {
			resourceWidth = minCompactColWidth
		}
		if typeWidth < minCompactColWidth {
			typeWidth = minCompactColWidth
		}
		return resourceWidth, typeWidth
	}

	resourceTarget, typeTarget := m.targetVariableColumnWidths()
	targetTotal := resourceTarget + typeTarget
	if targetTotal <= available {
		// Assign any remaining width to Resource to avoid ragged table width.
		return resourceTarget + (available - targetTotal), typeTarget
	}

	overflow := targetTotal - available
	resourceSlack := resourceTarget - minResourceColWidth
	typeSlack := typeTarget - minTypeColWidth
	totalSlack := resourceSlack + typeSlack
	if totalSlack <= 0 {
		return minResourceColWidth, minTypeColWidth
	}

	reduceResource := min((overflow*resourceSlack)/totalSlack, resourceSlack)
	reduceType := overflow - reduceResource
	if reduceType > typeSlack {
		extra := reduceType - typeSlack
		reduceType = typeSlack
		reduceResource += extra
	}
	if reduceResource > resourceSlack {
		extra := reduceResource - resourceSlack
		reduceResource = resourceSlack
		reduceType += extra
	}

	resourceWidth := resourceTarget - reduceResource
	typeWidth := typeTarget - reduceType
	if resourceWidth < minResourceColWidth {
		resourceWidth = minResourceColWidth
	}
	if typeWidth < minTypeColWidth {
		typeWidth = minTypeColWidth
	}
	return resourceWidth, typeWidth
}

// targetVariableColumnWidths computes desired widths for Resource and Type
// based on visible data.
func (m *OverviewModel) targetVariableColumnWidths() (int, int) {
	resourceTarget := minResourceColWidth
	typeTarget := minTypeColWidth

	resourceTarget = max(resourceTarget, utf8.RuneCountInString(columnTitleResource))
	typeTarget = max(typeTarget, utf8.RuneCountInString(columnTitleType))

	for _, entry := range m.displayEntries() {
		resourceTarget = max(resourceTarget, utf8.RuneCountInString(m.entryResourceName(entry)))
		typeTarget = max(typeTarget, utf8.RuneCountInString(entry.row.Type))
	}
	return resourceTarget, typeTarget
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

	// Handle passphrase input (when prompt is visible, intercept key events only).
	// Non-key messages (e.g. spinner ticks) are forwarded to both inputs so the
	// loading animation continues while the user types the passphrase.
	if m.showPassphraseInput {
		if _, isKey := msg.(tea.KeyPressMsg); isKey {
			return m.handlePassphraseInput(msg)
		}
		var passCmd tea.Cmd
		m.passphraseInput, passCmd = m.passphraseInput.Update(msg)
		if m.loadingState != nil {
			spinCmd := m.loadingState.Update(msg)
			return m, tea.Batch(passCmd, spinCmd)
		}
		return m, passCmd
	}

	// Handle phase message (initializing state)
	if phaseMsg, ok := msg.(OverviewPhaseMsg); ok {
		m.progressMsg = phaseMsg.Phase
		m.currentPhaseIndex = phaseMsg.Index
		return m, nil
	}

	// Handle data ready (initializing → loading transition)
	if dataMsg, ok := msg.(OverviewDataReadyMsg); ok {
		if m.state != ViewStateInitializing {
			return m, nil // Ignore stale message
		}
		// allRows and rows must not share backing arrays because
		// refreshTable sorts m.rows in-place.
		m.allRows = computeRowResults(dataMsg.Rows)
		m.rows = slices.Clone(m.allRows)
		m.totalCount = dataMsg.TotalCount
		m.stackName = dataMsg.StackName
		m.state = ViewStateLoading
		m.table = m.buildOverviewTable()
		return m, nil
	}

	// Handle budget data ready (from background fetch goroutine)
	if budgetMsg, ok := msg.(BudgetDataReadyMsg); ok {
		m.budgetResult = budgetMsg.Result
		m.budgetErr = budgetMsg.Error
		m.budgetLoaded = true
		return m, nil
	}

	// Handle init error
	if errMsg, ok := msg.(OverviewInitErrorMsg); ok {
		if m.state != ViewStateInitializing {
			return m, nil // Ignore stale message
		}
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

	if _, ok := msg.(OverviewPreviewTickMsg); ok {
		if m.isPreviewLoading {
			// Compute elapsed from previewLoadStart, ignoring the tick's own elapsed.
			m.previewElapsed = time.Since(m.previewLoadStart)
			return m, tickPreviewCmd()
		}
		return m, nil
	}

	if changesMsg, ok := msg.(OverviewChangesReadyMsg); ok {
		m.applyPreviewChanges(changesMsg)
		return m, nil
	}

	// Handle state-only activation (sent after OverviewDataReadyMsg when no preview ran).
	if setStateMsg, ok := msg.(OverviewSetStateOnlyMsg); ok {
		m.isStateOnly = true
		m.previewCmd = setStateMsg.PreviewCmd
		m.detectErrMsg = setStateMsg.DetectErrMsg
		m.rebuildTable() // Rebuild to show "Projected*" header.
		return m, nil
	}

	// Handle cluster expansion results (sent after enrichment completes).
	if expMsg, ok := msg.(OverviewExpansionReadyMsg); ok {
		m.allRows = computeRowResults(expMsg.Rows)
		m.expansionNotes = expMsg.Notes
		m.applyFilter(m.textInput.Value())
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
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
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
		m.allRows[msg.Index] = engine.ComputeOverviewRowResult(msg.Row)
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
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
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
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyEnter:
			if m.passphraseChan != nil {
				select {
				case m.passphraseChan <- m.passphraseInput.Value():
				default:
				}
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
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	return m.handleListKeypress(keyMsg)
}

func (m OverviewModel) handleListKeypress(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case keyQuit, keyCtrlC:
		m.state = ViewStateQuitting
		return m, tea.Quit
	case keyEnter:
		m.selected = m.displayIndex(m.table.Cursor())
		if m.selected >= 0 && m.selected < len(m.rows) {
			m.state = ViewStateDetail
		}
		return m, nil
	case keyE:
		m.toggleExpansion(m.table.Cursor())
		return m, nil
	case keyRight:
		m.setExpansion(m.table.Cursor(), true)
		return m, nil
	case keyLeft:
		m.setExpansion(m.table.Cursor(), false)
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
		// Set isPreviewLoading synchronously so rapid double-presses cannot
		// pass the guard while the first preview is still starting.
		if m.isStateOnly && !m.isPreviewLoading && !m.previewLoaded && m.previewCmd != nil {
			m.isPreviewLoading = true
			m.previewLoadStart = time.Now()
			return m, tea.Batch(m.previewCmd, tickPreviewCmd())
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

// displayIndex converts a table cursor position to an index into m.rows,
// honoring the expansion display order built by buildOverviewTable.
func (m OverviewModel) displayIndex(cursor int) int {
	if cursor >= 0 && cursor < len(m.displayToRows) {
		return m.displayToRows[cursor]
	}
	return m.absoluteIndex(cursor)
}

// expandableRow returns the rows index at a display cursor when that row is a
// cluster row with children, or -1 otherwise.
func (m OverviewModel) expandableRow(cursor int) int {
	idx := m.displayIndex(cursor)
	if idx < 0 || idx >= len(m.rows) || len(m.rows[idx].ChildURNs) == 0 {
		return -1
	}
	return idx
}

// toggleExpansion flips the expanded state of the cluster row at cursor.
func (m *OverviewModel) toggleExpansion(cursor int) {
	idx := m.expandableRow(cursor)
	if idx < 0 {
		return
	}
	urn := m.rows[idx].URN
	if m.expanded == nil {
		m.expanded = map[string]bool{}
	}
	m.expanded[urn] = !m.expanded[urn]
	m.rebuildTable()
}

// setExpansion expands (true) or collapses (false) the cluster row at cursor.
func (m *OverviewModel) setExpansion(cursor int, expand bool) {
	idx := m.expandableRow(cursor)
	if idx < 0 {
		return
	}
	if m.expanded == nil {
		m.expanded = map[string]bool{}
	}
	urn := m.rows[idx].URN
	if m.expanded[urn] != expand {
		m.expanded[urn] = expand
		m.rebuildTable()
	}
}

func (m OverviewModel) handleDetailUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
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
	viewmodel.SortOverviewRows(m.rows, m.sortBy)
	m.rebuildTable()
}

// rebuildTable reconstructs the table with current rows and pagination.
func (m *OverviewModel) rebuildTable() {
	m.table = m.buildOverviewTable()
}

// formatOverviewWarnCell renders the TUI Warn cell. The comma-separated list
// is kept when it fits in the column. A longer list shows the first name and
// +N for the rest, so Resource keeps its width on a 120-column terminal.
func formatOverviewWarnCell(warnings []engine.OverviewWarning) string {
	full := engine.FormatOverviewWarnings(warnings)
	if utf8.RuneCountInString(full) <= colWidthWarn {
		return full
	}
	return string(warnings[0]) + "+" + strconv.Itoa(len(warnings)-1)
}

// overviewDisplayEntry pairs a row to render with its index in m.rows and
// whether it is an expansion child (rendered indented).
type overviewDisplayEntry struct {
	row     engine.OverviewRowResult
	rowsIdx int
	child   bool
}

// displayEntries computes the rows to show in the table: the current page's
// pagination units in sorted order, each followed by its children when
// expanded. Children are hidden while their parent is collapsed. When the
// filter box has text the list is rendered flat, preserving the filter's
// substring semantics.
func (m *OverviewModel) displayEntries() []overviewDisplayEntry {
	flattened, state := viewmodel.FlattenClusterRows(m.rows, m.expanded, m.textInput.Value() != "", m.currentPage)
	m.paginationEnabled = state.Enabled
	m.totalPages = state.TotalPages
	m.currentPage = state.Page

	entries := make([]overviewDisplayEntry, len(flattened))
	for i, e := range flattened {
		entries[i] = overviewDisplayEntry{row: e.Row, rowsIdx: e.RowsIdx, child: e.Child}
	}
	return entries
}

// expansionDisplayName renders a compact name for an expansion child row:
// live allocation rows show their namespace (or "(idle)"); projected rows
// use the regular URN display name.
func expansionDisplayName(row engine.OverviewRowResult) string {
	if row.LiveChildName != "" {
		return row.LiveChildName
	}
	return row.DisplayName
}

// entryResourceName is the untruncated Resource cell for a display entry:
// the ▸/▾ marker on cluster rows, the ↳ indent on children, else the
// regular URN display name.
func (m *OverviewModel) entryResourceName(entry overviewDisplayEntry) string {
	switch {
	case entry.child:
		return "  ↳ " + expansionDisplayName(entry.row)
	case len(entry.row.ChildURNs) > 0:
		if m.expanded[entry.row.URN] {
			return "▾ " + expansionDisplayName(entry.row)
		}
		return "▸ " + expansionDisplayName(entry.row)
	default:
		return entry.row.DisplayName
	}
}

// overviewTableRow renders one display entry as a table row, applying the
// expansion marker (▸/▾) to cluster rows and the ↳ indent to child rows.
func (m *OverviewModel) overviewTableRow(entry overviewDisplayEntry, resourceWidth int) table.Row {
	row := entry.row
	return table.Row{
		engine.TruncateOverviewResource(m.entryResourceName(entry), resourceWidth),
		row.Type,
		row.StatusDisplay,
		row.ActualDisplay,
		row.ProjectedDisplay,
		row.DeltaDisplay,
		row.DriftDisplay,
		row.RecsDisplay,
		formatOverviewWarnCell(row.Warnings),
	}
}

// buildOverviewTable creates a new table model with current configuration.
func (m *OverviewModel) buildOverviewTable() table.Model {
	projectedHeader := "Projected"
	if m.isStateOnly && !m.previewLoaded {
		projectedHeader = "Projected*"
	}
	resourceWidth := m.resourceColumnWidth()
	typeWidth := m.typeColumnWidth()
	columns := []table.Column{
		{Title: columnTitleResource, Width: resourceWidth},
		{Title: columnTitleType, Width: typeWidth},
		{Title: "Status", Width: colWidthStatus},
		{Title: "Actual", Width: colWidthActual},
		{Title: projectedHeader, Width: colWidthProjected},
		{Title: columnTitleDelta, Width: colWidthDelta},
		{Title: "Drift%", Width: colWidthDrift},
		{Title: "Recs", Width: colWidthRecs},
		{Title: "Warn", Width: colWidthWarn},
	}

	entries := m.displayEntries()
	rows := make([]table.Row, len(entries))
	m.displayToRows = make([]int, len(entries))

	for i, entry := range entries {
		m.displayToRows[i] = entry.rowsIdx
		rows[i] = m.overviewTableRow(entry, resourceWidth)
	}

	availableHeight := max(m.height-summaryHeight-1, minHeight)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(availableHeight),
		table.WithWidth(tableWidthFromColumns(columns)),
	)

	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	t.SetStyles(s)

	return t
}

// applyPreviewChanges applies preview statuses, property diffs, and projected
// properties to the source rows, then recomputes deltas and row results.
// Safe: Bubble Tea Update() is single-threaded; no concurrent reads on allRows.
func (m *OverviewModel) applyPreviewChanges(changesMsg OverviewChangesReadyMsg) {
	m.isPreviewLoading = false
	m.previewLoaded = true
	m.isStateOnly = false
	sources := make([]engine.OverviewRow, len(m.allRows))
	for i := range m.allRows {
		sources[i] = m.allRows[i].Source
	}
	engine.ApplyChangesToRows(sources, changesMsg.StatusByURN)
	engine.ApplyPropertyDiffsToRows(sources, changesMsg.PropertyDiffsByURN)
	engine.ApplyProjectedPropertiesToRows(sources, changesMsg.ProjectedPropsByURN)
	engine.PopulateComputedDeltas(sources, time.Now().Day())
	m.allRows = computeRowResults(sources)
	m.applyFilter(m.textInput.Value())
}

// computeRowResults converts enriched overview rows into pre-computed,
// display-ready row results. Rows must already carry ComputedDelta
// (populated by the enrichment bridge or PopulateComputedDeltas).
func computeRowResults(rows []engine.OverviewRow) []engine.OverviewRowResult {
	results := make([]engine.OverviewRowResult, len(rows))
	for i := range rows {
		results[i] = engine.ComputeOverviewRowResult(rows[i])
	}
	return results
}

// applyFilter filters rows based on text input. It always calls refreshTable
// and enablePaginationIfNeeded to keep pagination state consistent.
func (m *OverviewModel) applyFilter(filterText string) {
	// FilterOverviewRows copies when filterText is empty; refreshTable sorts
	// m.rows in-place and must not reorder the source m.allRows.
	m.rows = viewmodel.FilterOverviewRows(m.allRows, filterText)

	m.enablePaginationIfNeeded()
	m.refreshTable()
}

// overviewRowSortCost returns the primary cost for sorting.
func overviewRowSortCost(row engine.OverviewRowResult) float64 {
	return viewmodel.OverviewRowSortCost(row)
}

// overviewRowSortDelta returns the pre-computed delta for sorting, keeping
// display and sort order consistent.
func overviewRowSortDelta(row engine.OverviewRowResult) float64 {
	return viewmodel.OverviewRowSortDelta(row)
}

// enablePaginationIfNeeded checks if pagination should be enabled and clamps
// the current page to valid bounds.
func (m *OverviewModel) enablePaginationIfNeeded() {
	state := viewmodel.ClusterPagination(m.rows, m.textInput.Value() != "", m.currentPage)
	m.paginationEnabled = state.Enabled
	m.totalPages = state.TotalPages
	m.currentPage = state.Page
}

// getVisibleRows returns the pagination units on the current page.
func (m *OverviewModel) getVisibleRows() []engine.OverviewRowResult {
	entries, _ := viewmodel.FlattenClusterRows(m.rows, m.expanded, m.textInput.Value() != "", m.currentPage)
	rows := make([]engine.OverviewRowResult, 0, len(entries))
	for _, e := range entries {
		if e.Unit {
			rows = append(rows, e.Row)
		}
	}
	return rows
}

// renderPaginationFooter displays page info at the bottom.
func (m *OverviewModel) renderPaginationFooter() string {
	if !m.paginationEnabled {
		return ""
	}

	return fmt.Sprintf("Page %d/%d | Use PgUp/PgDn to navigate", m.currentPage, m.totalPages)
}

// tickPreviewCmd returns a command that fires OverviewPreviewTickMsg after 1 second.
// The model computes the actual elapsed from m.previewLoadStart in the handler.
func tickPreviewCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return OverviewPreviewTickMsg{}
	})
}
