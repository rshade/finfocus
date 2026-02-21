package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/rshade/finfocus/internal/engine"
)

// stateOnlyFootnote returns the footnote appended to the list view when costs
// are projected from current state without a fresh pulumi preview.
// It derives the hours-per-month value from engine.HoursPerMonth to stay in
// sync with the canonical constant used for all cost projections.
func stateOnlyFootnote() string {
	return fmt.Sprintf("* projected at current state (%dh/mo)", engine.HoursPerMonth)
}

// View renders the current view (Bubble Tea interface).
func (m OverviewModel) View() string {
	switch m.state {
	case ViewStateQuitting:
		return ""
	case ViewStateError:
		return fmt.Sprintf("Error: %v\n", m.err)
	case ViewStateInitializing:
		return m.renderInitializingView()
	case ViewStateLoading:
		return m.renderLoadingView()
	case ViewStateDetail:
		return m.renderDetailView()
	case ViewStateList:
		return m.renderListView()
	default:
		return ""
	}
}

// renderInitializingView renders the banner and phase checklist during the
// initializing state (before data is available). When the passphrase prompt
// is active it replaces the checklist with an inline password input.
func (m OverviewModel) renderInitializingView() string {
	banner := RenderBanner(m.width)

	// Passphrase prompt replaces checklist when active.
	if m.showPassphraseInput {
		prompt := lipgloss.JoinVertical(lipgloss.Left,
			WarningStyle.Render("This stack uses passphrase encryption."),
			"",
			LabelStyle.Render("Enter PULUMI_CONFIG_PASSPHRASE: ")+m.passphraseInput.View(),
			SubtleStyle.Render("(hidden — press Enter to continue, Esc to cancel)"),
		)
		bannerW := lipgloss.Width(banner)
		constrainedPrompt := lipgloss.NewStyle().Width(bannerW).Render(prompt)
		return lipgloss.JoinVertical(lipgloss.Center, banner, "", constrainedPrompt, "")
	}

	// Phase checklist
	spinnerView := ""
	if m.loadingState != nil {
		spinnerView = m.loadingState.spinner.View()
	}
	lines := make([]string, 0, len(phaseNames))
	for i, name := range phaseNames {
		switch {
		case i < m.currentPhaseIndex:
			lines = append(lines, OKStyle.Render("  ✓ "+name))
		case i == m.currentPhaseIndex:
			lines = append(lines, InfoStyle.Render("  "+spinnerView+" "+name+"..."))
		default:
			lines = append(lines, SubtleStyle.Render("  · "+name))
		}
	}

	checklist := lipgloss.NewStyle().
		Width(m.width-borderPadding).
		Padding(1, 2). //nolint:mnd // View padding.
		Render(strings.Join(lines, "\n"))
	return lipgloss.JoinVertical(lipgloss.Center, banner, "", checklist, "")
}

// renderLoadingView renders the loading spinner with progress banner.
func (m OverviewModel) renderLoadingView() string {
	banner := m.renderProgressBanner()
	spinner := "Loading..."

	return lipgloss.JoinVertical(lipgloss.Left, banner, spinner)
}

// renderProgressBanner displays loading progress at the top of the screen.
func (m OverviewModel) renderProgressBanner() string {
	if m.progressMsg == "" {
		return ""
	}

	return InfoStyle.
		Width(m.width-borderPadding).
		Padding(0, 1).
		Render(m.progressMsg)
}

// renderListView renders the main table view with optional filter input.
func (m OverviewModel) renderListView() string {
	var sections []string

	// Progress banner (shown during progressive loading)
	if m.loadedCount < m.totalCount && m.progressMsg != "" {
		sections = append(sections, m.renderProgressBanner())
	}

	// Table
	sections = append(sections, m.table.View())

	// Pagination footer
	if m.paginationEnabled {
		sections = append(sections, m.renderPaginationFooter())
	}

	// Status bar with sort/filter indicators
	statusBar := m.renderStatusBar()
	sections = append(sections, statusBar)

	// Footer footnote for state-only mode.
	if m.isStateOnly && !m.previewLoaded {
		footnote := SubtleStyle.Render("* projected at current state (730h/mo)")
		sections = append(sections, footnote)
	}

	// Filter input (if active)
	if m.showFilter {
		filterView := LabelStyle.Render("Filter: ") + m.textInput.View()
		sections = append(sections, filterView)
	}

	// Footer footnote for state-only mode (after filter so filter appears above footnote).
	if m.isStateOnly && !m.previewLoaded {
		footnote := SubtleStyle.Render(stateOnlyFootnote())
		sections = append(sections, footnote)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderStatusBar displays current sort field, filter status, and preview hints.
func (m OverviewModel) renderStatusBar() string {
	sortLabel := m.getSortLabel()
	filterStatus := ""
	stackPrefix := ""

	if m.stackName != "" {
		stackPrefix = fmt.Sprintf("Stack: %s | ", m.stackName)
	}

	if m.textInput.Value() != "" {
		filterStatus = fmt.Sprintf(" | Filtered: %d/%d", len(m.rows), len(m.allRows))
	}

	var status string
	switch {
	case m.isPreviewLoading:
		// Show elapsed timer while preview is loading.
		elapsed := formatPreviewElapsed(m.previewElapsed)
		status = fmt.Sprintf(
			"%sLoading pending changes (%s)%s | Sort: %s | Press 's' to cycle, '/' to filter, 'q' to quit",
			stackPrefix, elapsed, filterStatus, sortLabel,
		)
	case m.isStateOnly && !m.previewLoaded:
		// Offer 'p' hint when in state-only mode and preview not loaded.
		detectFailed := ""
		if m.detectErrMsg != "" {
			detectFailed = " (change detection failed)"
		}
		status = fmt.Sprintf(
			"%sSort: %s%s | [p] load pending changes%s | Press 's' to cycle, '/' to filter, 'q' to quit",
			stackPrefix, sortLabel, filterStatus, detectFailed,
		)
	default:
		status = fmt.Sprintf(
			"%sSort: %s%s | Press 's' to cycle, '/' to filter, 'q' to quit",
			stackPrefix, sortLabel, filterStatus,
		)
	}
	return SubtleStyle.Render(status)
}

// formatPreviewElapsed formats a duration as M:SS for the elapsed preview timer.
func formatPreviewElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	secs := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", secs/60, secs%60) //nolint:mnd // Time formatting.
}

// getSortLabel returns the human-readable label for the current sort field.
func (m OverviewModel) getSortLabel() string {
	switch m.sortBy {
	case SortByCost:
		return "Cost"
	case SortByName:
		return "Name"
	case SortByType:
		return "Type"
	case SortByDelta:
		return "Delta"
	default:
		return "Unknown"
	}
}

// renderDetailView renders the detail view for a selected resource.
func (m OverviewModel) renderDetailView() string {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return msgSelectedOutOfBounds
	}

	row := m.rows[m.selected]
	var content strings.Builder

	// Header and metadata
	content.WriteString(HeaderStyle.Render("RESOURCE DETAIL"))
	content.WriteString("\n\n")
	content.WriteString(LabelStyle.Render("URN:    "))
	content.WriteString(ValueStyle.Render(row.URN))
	content.WriteString("\n")
	content.WriteString(LabelStyle.Render("Type:   "))
	content.WriteString(ValueStyle.Render(row.Type))
	content.WriteString("\n")
	content.WriteString(LabelStyle.Render("Status: "))
	content.WriteString(ValueStyle.Render(row.Status.String()))
	content.WriteString("\n\n")

	// Cost sections
	renderDetailActualCost(&content, row)
	renderDetailProjectedCost(&content, row)
	renderDetailCostDrift(&content, row)
	renderDetailRecommendations(&content, row)
	renderDetailError(&content, row)

	content.WriteString(SubtleStyle.Render("\nPress ESC to return"))

	return BoxStyle.Width(m.width - borderPadding).Render(content.String())
}

// renderDetailActualCost writes actual cost details to the builder.
func renderDetailActualCost(content *strings.Builder, row engine.OverviewRow) {
	if row.ActualCost == nil {
		return
	}
	content.WriteString(HeaderStyle.Render("ACTUAL COST (MTD)"))
	content.WriteString("\n")
	content.WriteString(LabelStyle.Render("  Total: "))
	content.WriteString(ValueStyle.Render(engine.FormatOverviewCurrency(row.ActualCost.MTDCost)))
	content.WriteString("\n")
	renderBreakdown(content, row.ActualCost.Breakdown)
	content.WriteString("\n")
}

// renderDetailProjectedCost writes projected cost details to the builder.
func renderDetailProjectedCost(content *strings.Builder, row engine.OverviewRow) {
	if row.ProjectedCost == nil {
		return
	}
	content.WriteString(HeaderStyle.Render("PROJECTED COST (Monthly)"))
	content.WriteString("\n")
	content.WriteString(LabelStyle.Render("  Total: "))
	content.WriteString(ValueStyle.Render(engine.FormatOverviewCurrency(row.ProjectedCost.MonthlyCost)))
	content.WriteString("\n")
	renderBreakdown(content, row.ProjectedCost.Breakdown)
	content.WriteString("\n")
}

// renderDetailCostDrift writes cost drift details to the builder.
func renderDetailCostDrift(content *strings.Builder, row engine.OverviewRow) {
	if row.CostDrift == nil {
		return
	}
	content.WriteString(HeaderStyle.Render("COST DRIFT"))
	content.WriteString("\n")
	content.WriteString(LabelStyle.Render("  Extrapolated Monthly: "))
	content.WriteString(ValueStyle.Render(engine.FormatOverviewCurrency(row.CostDrift.ExtrapolatedMonthly)))
	content.WriteString("\n")
	content.WriteString(LabelStyle.Render("  Projected: "))
	content.WriteString(ValueStyle.Render(engine.FormatOverviewCurrency(row.CostDrift.Projected)))
	content.WriteString("\n")
	content.WriteString(LabelStyle.Render("  Delta: "))
	content.WriteString(ValueStyle.Render(engine.FormatOverviewCurrency(row.CostDrift.Delta)))
	content.WriteString("\n")
	content.WriteString(LabelStyle.Render("  Drift: "))
	driftStyle := ValueStyle
	if row.CostDrift.IsWarning {
		driftStyle = WarningStyle
	}
	content.WriteString(driftStyle.Render(fmt.Sprintf("%.1f%%", row.CostDrift.PercentDrift)))
	content.WriteString("\n\n")
}

// renderDetailRecommendations writes active (non-dismissed) recommendations to
// the builder. Dismissed and snoozed recommendations are excluded from the
// detail view — they are only reflected in the count badge.
func renderDetailRecommendations(content *strings.Builder, row engine.OverviewRow) {
	// Collect active recs only.
	var active []engine.Recommendation
	for _, rec := range row.Recommendations {
		if rec.Status != engine.RecommendationStatusDismissed &&
			rec.Status != engine.RecommendationStatusSnoozed {
			active = append(active, rec)
		}
	}
	if len(active) == 0 {
		return
	}
	content.WriteString(HeaderStyle.Render("RECOMMENDATIONS"))
	content.WriteString("\n")
	for i, rec := range active {
		fmt.Fprintf(content, "  %d. %s\n", i+1, rec.Description)
		content.WriteString(LabelStyle.Render("     Savings: "))
		content.WriteString(ValueStyle.Render(
			engine.FormatOverviewCurrency(rec.EstimatedSavings) + "\n",
		))
	}
	content.WriteString("\n")
}

// renderDetailError writes error details to the builder.
func renderDetailError(content *strings.Builder, row engine.OverviewRow) {
	if row.Error == nil {
		return
	}
	content.WriteString(HeaderStyle.Render("ERROR"))
	content.WriteString("\n")
	content.WriteString(CriticalStyle.Render(fmt.Sprintf("  Type: %s\n", row.Error.ErrorType.String())))
	content.WriteString(CriticalStyle.Render(fmt.Sprintf("  Message: %s\n", row.Error.Message)))
	content.WriteString("\n")
}

// renderBreakdown writes a cost breakdown map to the builder.
// Keys are sorted for deterministic output.
func renderBreakdown(content *strings.Builder, breakdown map[string]float64) {
	if len(breakdown) == 0 {
		return
	}
	content.WriteString(LabelStyle.Render("  Breakdown:\n"))
	keys := make([]string, 0, len(breakdown))
	for k := range breakdown {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, category := range keys {
		fmt.Fprintf(content, "    %s: %s\n", category, engine.FormatOverviewCurrency(breakdown[category]))
	}
}
