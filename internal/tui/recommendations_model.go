package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rshade/finfocus/internal/engine"
	listview "github.com/rshade/finfocus/internal/tui/list"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// RecommendationSortField represents the field to sort recommendations by.
type RecommendationSortField = viewmodel.RecommendationSortField

const (
	// SortBySavings sorts by estimated savings (descending).
	SortBySavings = viewmodel.SortBySavings
	// SortByResourceID sorts by resource ID (ascending).
	SortByResourceID = viewmodel.SortByResourceID
	// SortByActionType sorts by action type (ascending).
	SortByActionType = viewmodel.SortByActionType
)

const (
	// numRecommendationSortFields is the number of available sort fields.
	numRecommendationSortFields = 3

	// topRecommendationsLimit is the maximum number of recommendations to show in summary.
	topRecommendationsLimit = 5

	// recSummaryHeight is the height reserved for the summary section.
	recSummaryHeight = 10

	// Table column widths for recommendations.
	recColWidthResource    = 35
	recColWidthAction      = 15
	recColWidthSavings     = 12
	recColWidthDescription = 30

	// recDescTruncateLen is the maximum description length in table rows.
	recDescTruncateLen = 27

	// recColGap is the space between table columns.
	recColGap = 2

	// recMinDescWidth is the narrowest description column a spot list shrinks to.
	recMinDescWidth = 10

	// recSpotListGaps counts the gaps before Description in a list with a Pricing column.
	recSpotListGaps = 4

	// defaultCurrency is used when no currency is specified.
	defaultCurrency = "USD"
)

// getCurrencySymbol returns the symbol for a currency code, or the code itself if unknown.
func getCurrencySymbol(currency string) string {
	return viewmodel.CurrencySymbol(currency)
}

// RecommendationsSummary contains aggregated statistics for recommendations display.
type RecommendationsSummary struct {
	// TotalCount is the total number of recommendations.
	TotalCount int

	// TotalSavings is the savings that can all be realized (engine.AchievableSavings).
	TotalSavings float64

	// Currency is the currency for savings (typically "USD").
	Currency string

	// CountByAction maps action type to count of recommendations.
	CountByAction map[string]int

	// SavingsByAction maps action type to total savings.
	SavingsByAction map[string]float64

	// TopRecommendations contains the top 5 recommendations sorted by savings.
	TopRecommendations []engine.Recommendation
}

// NewRecommendationsSummary creates a summary from a list of recommendations.
// It calculates aggregate statistics and extracts the top 5 by savings.
func NewRecommendationsSummary(recs []engine.Recommendation) *RecommendationsSummary {
	summary := &RecommendationsSummary{
		TotalCount:      len(recs),
		CountByAction:   make(map[string]int),
		SavingsByAction: make(map[string]float64),
	}

	if len(recs) == 0 {
		return summary
	}

	// Calculate aggregates.
	for _, rec := range recs {
		summary.CountByAction[rec.Type]++
		summary.SavingsByAction[rec.Type] += rec.EstimatedSavings

		// Set currency from first recommendation with a currency.
		if summary.Currency == "" && rec.Currency != "" {
			summary.Currency = rec.Currency
		}
	}

	summary.TotalSavings = engine.AchievableSavings(recs)

	// Sort by savings descending and take top 5.
	sorted := make([]engine.Recommendation, len(recs))
	copy(sorted, recs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].EstimatedSavings > sorted[j].EstimatedSavings
	})

	if len(sorted) > topRecommendationsLimit {
		summary.TopRecommendations = sorted[:topRecommendationsLimit]
	} else {
		summary.TopRecommendations = sorted
	}

	return summary
}

// recRowLayout sizes the list columns. A list with a spot recommendation gets a
// Pricing column before Description, and Description shrinks so the spot label
// stays inside the terminal width. A list without one keeps the fixed layout.
type recRowLayout struct {
	pricing     int
	description int
}

func newRecRowLayout(recs []engine.Recommendation, width int) recRowLayout {
	pricing := 0
	for _, rec := range recs {
		pricing = max(pricing, lipgloss.Width(rec.SpotDisplay()))
	}
	if pricing == 0 {
		return recRowLayout{description: recColWidthDescription}
	}
	fixed := recColWidthResource + recColWidthAction + recColWidthSavings + pricing + recSpotListGaps*recColGap
	return recRowLayout{
		pricing:     pricing,
		description: min(recColWidthDescription, max(recMinDescWidth, width-fixed)),
	}
}

// truncateRecDescription keeps the list's original rule: anything longer than
// the column less the ellipsis is cut and ends in "...".
func truncateRecDescription(description string, width int) string {
	if keep := width - len("..."); len(description) > keep {
		return description[:keep] + "..."
	}
	return description
}

// renderRecommendation formats one unselected recommendation with the layout
// of a list that holds only that recommendation.
func renderRecommendation(rec engine.Recommendation) string {
	return renderRecommendationRow(rec, false, newRecRowLayout([]engine.Recommendation{rec}, defaultWidth))
}

func renderRecommendationRow(rec engine.Recommendation, selected bool, layout recRowLayout) string {
	// Format resource ID (truncate if too long)
	resourceID := rec.ResourceID
	if len(resourceID) > recColWidthResource {
		resourceID = resourceID[:recColWidthResource-3] + "..."
	}

	actionType := FormatActionType(rec.Type)
	if len(actionType) > recColWidthAction {
		actionType = actionType[:recColWidthAction-3] + "..."
	}

	// Format savings with correct currency symbol
	currency := rec.Currency
	if currency == "" {
		currency = defaultCurrency
	}
	savings := fmt.Sprintf("%s%.2f", getCurrencySymbol(currency), rec.EstimatedSavings)

	row := fmt.Sprintf("%-*s  %-*s  %*s  ",
		recColWidthResource, resourceID,
		recColWidthAction, actionType,
		recColWidthSavings, savings,
	)
	if layout.pricing > 0 {
		spot := rec.SpotDisplay()
		row += spot + strings.Repeat(" ", layout.pricing-lipgloss.Width(spot)) + "  "
	}
	row += fmt.Sprintf("%-*s", layout.description, truncateRecDescription(rec.Description, layout.description))
	row += scoreSuffix(rec.Scores)

	// Apply selection styling
	if selected {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Render(row)
	}

	return row
}

// scoreSuffix renders the compact score cell appended to a list row. It is empty for
// recommendations the scorer did not rate, so unscored output is unchanged.
func scoreSuffix(scores *engine.RecommendationScores) string {
	if scores == nil {
		return ""
	}
	suffix := "  risk " + formatScore(scores.Risk)
	if scores.NeedsReview {
		suffix += "  REVIEW"
	}
	if scores.DuplicateGroupID != "" {
		suffix += "  " + scores.DuplicateGroupID
	}
	return suffix
}

// formatScore formats a 0-to-1 signal, or "-" when the scorer did not compute it.
func formatScore(v *float64) string { return viewmodel.FormatScore(v) }

// Messages for RecommendationsViewModel.
type recommendationsLoadingMsg struct {
	recommendations []engine.Recommendation
	err             error
}

// RecommendationsViewModel is the Bubble Tea model for interactive recommendations display.
type RecommendationsViewModel struct {
	// View state
	state              ViewState
	allRecommendations []engine.Recommendation // Source of truth
	recommendations    []engine.Recommendation // Filtered/sorted for display

	// Interactive components
	virtualList *listview.VirtualListModel[engine.Recommendation]
	textInput   textinput.Model

	// Display configuration
	width      int
	height     int
	sortBy     RecommendationSortField
	showFilter bool
	verbose    bool

	// Loading state
	loading  *LoadingState
	fetchCmd tea.Cmd

	// Aggregated data
	summary *RecommendationsSummary
	profile string

	// rowLayout is the column layout of the current list.
	rowLayout recRowLayout

	// pluginErrors lists plugins whose recommendations query failed.
	pluginErrors []engine.RecommendationError

	// Error handling
	err error
}

// NewRecommendationsViewModel creates a new model with the given recommendations.
func NewRecommendationsViewModel(recs []engine.Recommendation) *RecommendationsViewModel {
	return NewRecommendationsViewModelWithProfile(recs, "")
}

// NewRecommendationsViewModelWithProfile creates a model that names the
// workload hint used for this result. An empty profile is shown as default.
func NewRecommendationsViewModelWithProfile(
	recs []engine.Recommendation,
	profile string,
) *RecommendationsViewModel {
	m := &RecommendationsViewModel{
		state:              ViewStateList,
		allRecommendations: recs,
		recommendations:    recs,
		textInput:          newRecTextInput(),
		summary:            NewRecommendationsSummary(recs),
		profile:            profile,
		width:              defaultWidth,
		height:             defaultHeight,
	}
	m.applySort()
	m.rebuildList()
	return m
}

// RecommendationFetcher is a context-aware function that fetches recommendations.
// The fetcher should check ctx.Done() to support cancellation.
type RecommendationFetcher func(ctx context.Context) ([]engine.Recommendation, error)

// NewRecommendationsViewModelWithLoading creates a model that starts in loading state.
// The fetcher receives a context that can be used for cancellation.
func NewRecommendationsViewModelWithLoading(
	ctx context.Context,
	fetcher RecommendationFetcher,
) *RecommendationsViewModel {
	m := &RecommendationsViewModel{
		state:     ViewStateLoading,
		loading:   NewLoadingState(),
		textInput: newRecTextInput(),
		summary:   &RecommendationsSummary{Currency: defaultCurrency}, // Initialize with empty summary
		width:     defaultWidth,
		height:    defaultHeight,
		fetchCmd: func() tea.Msg {
			recs, err := fetcher(ctx)
			return recommendationsLoadingMsg{recommendations: recs, err: err}
		},
	}
	return m
}

// WithPluginErrors records plugins whose query failed, so the list shows them
// instead of reading as a complete result.
func (m *RecommendationsViewModel) WithPluginErrors(errs []engine.RecommendationError) *RecommendationsViewModel {
	m.pluginErrors = errs
	return m
}

// newRecTextInput creates a new text input for filtering recommendations.
func newRecTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Filter recommendations..."
	ti.CharLimit = filterInputCharLimit
	ti.SetWidth(filterInputWidth)
	return ti
}

// SetVerbose sets whether to show all recommendations (verbose mode).
func (m *RecommendationsViewModel) SetVerbose(verbose bool) {
	m.verbose = verbose
}

// Init initializes the model.
func (m *RecommendationsViewModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.state == ViewStateLoading {
		cmds = append(cmds, m.loading.Init(), m.fetchCmd)
	} else if m.showFilter {
		cmds = append(cmds, textinput.Blink)
	}
	return tea.Batch(cmds...)
}

// Update handles messages and updates the model state.
func (m *RecommendationsViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window resizing
	if winMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = winMsg.Width
		m.height = winMsg.Height
		m.rebuildList()
	}

	// Handle loading complete
	if loadMsg, ok := msg.(recommendationsLoadingMsg); ok {
		return m.handleLoadingComplete(loadMsg)
	}

	// Handle filter input
	if m.showFilter {
		return m.handleFilterInput(msg)
	}

	// Handle state-specific updates
	switch m.state {
	case ViewStateInitializing:
		// RecommendationsViewModel does not use the Initializing state; included for exhaustiveness.
		return m, nil
	case ViewStateLoading:
		return m.handleLoadingUpdate(msg)
	case ViewStateList:
		return m.handleListUpdate(msg)
	case ViewStateDetail:
		return m.handleDetailUpdate(msg)
	case ViewStateQuitting, ViewStateError:
		return m.handleQuitUpdate(msg)
	default:
		return m, nil
	}
}

func (m *RecommendationsViewModel) handleLoadingComplete(
	msg recommendationsLoadingMsg,
) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = ViewStateError
		return m, tea.Quit
	}
	m.allRecommendations = msg.recommendations
	m.recommendations = msg.recommendations
	m.summary = NewRecommendationsSummary(msg.recommendations)
	m.state = ViewStateList
	m.applySort()
	m.rebuildList()
	return m, nil
}

func (m *RecommendationsViewModel) handleFilterInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
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

func (m *RecommendationsViewModel) handleLoadingUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.loading.Update(msg)
}

func (m *RecommendationsViewModel) handleListUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyQuit, keyCtrlC:
			m.state = ViewStateQuitting
			return m, tea.Quit
		case keyEnter:
			if len(m.recommendations) > 0 {
				m.state = ViewStateDetail
			}
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

	// Forward navigation to virtual list
	if m.virtualList != nil {
		updatedModel, cmd := m.virtualList.Update(msg)
		if vl, ok := updatedModel.(*listview.VirtualListModel[engine.Recommendation]); ok {
			m.virtualList = vl
		}
		return m, cmd
	}

	return m, nil
}

func (m *RecommendationsViewModel) handleDetailUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyQuit, keyCtrlC:
			m.state = ViewStateQuitting
			return m, tea.Quit
		case keyEsc:
			m.state = ViewStateList
			return m, nil
		}
	}
	return m, nil
}

func (m *RecommendationsViewModel) handleQuitUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyQuit, keyCtrlC:
			m.state = ViewStateQuitting
			return m, tea.Quit
		}
	}
	return m, nil
}

// applyFilter filters recommendations based on the text input value.
func (m *RecommendationsViewModel) applyFilter() {
	m.recommendations = viewmodel.FilterRecommendations(m.allRecommendations, m.textInput.Value())
	m.summary = NewRecommendationsSummary(m.recommendations)
	m.applySort()
	m.rebuildList()
}

// cycleSort cycles through the available sort fields.
func (m *RecommendationsViewModel) cycleSort() {
	m.sortBy = (m.sortBy + 1) % numRecommendationSortFields
	m.applySort()
	m.rebuildList()
}

// applySort sorts recommendations based on the current sort field.
func (m *RecommendationsViewModel) applySort() {
	viewmodel.SortRecommendations(m.recommendations, m.sortBy)
}

// rebuildList rebuilds the virtual list model with current recommendations.
func (m *RecommendationsViewModel) rebuildList() {
	availableHeight := max(m.height-recSummaryHeight-1, minHeight)
	layout := newRecRowLayout(m.recommendations, m.width)
	m.rowLayout = layout
	m.virtualList = listview.NewVirtualListModel(
		m.recommendations,
		availableHeight,
		m.width,
		func(rec engine.Recommendation, selected bool) string {
			return renderRecommendationRow(rec, selected, layout)
		},
	)
}

// View renders the current view.
func (m *RecommendationsViewModel) View() tea.View {
	switch m.state {
	case ViewStateInitializing:
		// RecommendationsViewModel does not use the Initializing state; included for exhaustiveness.
		return tea.NewView("")
	case ViewStateQuitting:
		return tea.NewView("")
	case ViewStateError:
		return tea.NewView(fmt.Sprintf("Error: %v\n", m.err))
	case ViewStateLoading:
		return tea.NewView(RenderLoading(m.loading))
	case ViewStateDetail:
		if m.virtualList != nil {
			selected := m.virtualList.Selected()
			if selected >= 0 && selected < len(m.recommendations) {
				return tea.NewView(RenderRecommendationDetail(m.recommendations[selected], m.width))
			}
		}
		return tea.NewView(msgSelectedOutOfBounds)
	case ViewStateList:
		return tea.NewView(m.renderListView())
	default:
		return tea.NewView("")
	}
}

func (m *RecommendationsViewModel) renderListView() string {
	summary := "Profile: " + displayedRecommendationProfile(m.profile) + "\n" +
		RenderRecommendationsSummaryTUI(m.summary, m.width) + renderRecommendationPluginErrors(m.pluginErrors)

	// Render the virtual list
	var listView string
	if m.virtualList != nil {
		// Add table header before virtual list
		header := fmt.Sprintf("%-*s  %-*s  %*s  ",
			recColWidthResource, "Resource",
			recColWidthAction, "Action",
			recColWidthSavings, "Savings",
		)
		if m.rowLayout.pricing > 0 {
			header += fmt.Sprintf("%-*s  ", m.rowLayout.pricing, "Pricing")
		}
		header += fmt.Sprintf("%-*s", m.rowLayout.description, "Description")
		headerStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			BorderBottom(true).
			Bold(true)
		listView = headerStyle.Render(header) + "\n" + m.virtualList.View().Content
	}

	helpText := "\n[/] Filter  [s] Sort  [↑↓/jk] Navigate  [Enter] Details  [q] Quit"

	if m.showFilter {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			summary,
			listView,
			"\nFilter: "+m.textInput.View(),
			helpText,
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, summary, listView, helpText)
}

// NewRecommendationsTable is deprecated: Use VirtualListModel for better performance.
// This function is kept for backward compatibility with tests.
// It creates a simple list representation instead of a table model.
func NewRecommendationsTable(recs []engine.Recommendation, height int) string {
	var result strings.Builder
	for i, rec := range recs {
		if i >= height {
			break
		}
		currency := rec.Currency
		if currency == "" {
			currency = defaultCurrency
		}
		savings := fmt.Sprintf("%s%.2f", getCurrencySymbol(currency), rec.EstimatedSavings)
		desc := rec.Description
		if len(desc) > recDescTruncateLen {
			desc = desc[:recDescTruncateLen] + "..."
		}
		fmt.Fprintf(&result, "%s | %s | %s | %s\n",
			rec.ResourceID, rec.Type, savings, desc)
	}
	return result.String()
}

func displayedRecommendationProfile(profile string) string {
	if profile == "" {
		return "default"
	}
	return profile
}

// RenderRecommendationsSummaryTUI renders the recommendations summary for TUI display.
func RenderRecommendationsSummaryTUI(summary *RecommendationsSummary, _ int) string {
	if summary == nil {
		return "No recommendations available."
	}

	currency := summary.Currency
	if currency == "" {
		currency = defaultCurrency
	}

	var sb strings.Builder
	_, _ = sb.WriteString("RECOMMENDATIONS SUMMARY\n")
	_, _ = sb.WriteString("=======================\n")
	fmt.Fprintf(&sb, "Total: %d recommendations\n", summary.TotalCount)
	fmt.Fprintf(&sb, "Potential Savings: %s%.2f\n", getCurrencySymbol(currency), summary.TotalSavings)

	if len(summary.CountByAction) > 0 {
		_, _ = sb.WriteString("\nBy Action Type:\n")

		// Plugins spell one action differently (RIGHTSIZE or its full enum name), so
		// lines are merged by display label.
		counts := make(map[string]int, len(summary.CountByAction))
		savingsByLabel := make(map[string]float64, len(summary.CountByAction))
		for actionType, count := range summary.CountByAction {
			label := FormatActionType(actionType)
			counts[label] += count
			savingsByLabel[label] += summary.SavingsByAction[actionType]
		}
		labels := make([]string, 0, len(counts))
		for label := range counts {
			labels = append(labels, label)
		}
		sort.Strings(labels)

		currencySymbol := getCurrencySymbol(currency)
		for _, label := range labels {
			fmt.Fprintf(&sb, "  %s: %d (%s%.2f)\n", label, counts[label], currencySymbol, savingsByLabel[label])
		}
	}

	return sb.String()
}

// RenderRecommendationDetail renders a detailed view of a single recommendation.
func RenderRecommendationDetail(rec engine.Recommendation, width int) string {
	_ = width // Reserved for future width-aware rendering

	currency := rec.Currency
	if currency == "" {
		currency = defaultCurrency
	}

	var sb strings.Builder
	_, _ = sb.WriteString("RECOMMENDATION DETAIL\n")
	_, _ = sb.WriteString("=====================\n\n")
	fmt.Fprintf(&sb, "Resource:    %s\n", rec.ResourceID)
	fmt.Fprintf(&sb, "Action Type: %s\n", FormatActionType(rec.Type))
	fmt.Fprintf(&sb, "Savings:     %s%.2f %s\n",
		getCurrencySymbol(currency), rec.EstimatedSavings, currency)
	if spot := rec.SpotDisplay(); spot != "" {
		fmt.Fprintf(&sb, "Pricing:     %s\n", spot)
	}
	fmt.Fprintf(&sb, "Description: %s\n", rec.Description)
	writeScoreDetail(&sb, rec.Scores)
	_, _ = sb.WriteString("\n[Esc] Back to list  [q] Quit")

	return sb.String()
}

// writeScoreDetail appends the scorer's ratings to a recommendation detail view.
func writeScoreDetail(sb *strings.Builder, scores *engine.RecommendationScores) {
	if scores == nil {
		return
	}
	_, _ = sb.WriteString("\nScores (ranking signals, not approval to act)\n")
	for _, field := range viewmodel.ScoreDisplay(scores) {
		if field.Name == "Needs review" {
			_, _ = sb.WriteString("Needs review\n")
			continue
		}
		fmt.Fprintf(sb, "%-17s%s\n", field.Name+":", field.Value)
	}
}

// renderRecommendationPluginErrors lists failed plugins below the summary, or
// returns "" when every plugin answered.
func renderRecommendationPluginErrors(errs []engine.RecommendationError) string {
	if len(errs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n" + CriticalStyle.Render("PLUGIN ERRORS") + "\n")
	for _, e := range errs {
		fmt.Fprintf(&sb, "  %s: %s\n", e.PluginName, e.Error)
	}
	return sb.String()
}
