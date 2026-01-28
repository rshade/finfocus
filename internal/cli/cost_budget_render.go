package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
)

// Scoped budget rendering constants.
const (
	scopedBoxWidth         = 60
	scopedMinProgressBar   = 20
	scopedBoxTitlePadding  = 4   // Padding for title separator line
	maxPercentageForBarCap = 100 // Maximum percentage for progress bar display
	healthOKLabel          = "OK"
	healthWarningLabel     = "WARNING"
	healthCriticalLabel    = "CRITICAL"
	healthExceededLabel    = "EXCEEDED"
	healthUnspecified      = "UNSPECIFIED"
)

// BudgetScopeFilter defines which budget scopes to render.
type BudgetScopeFilter struct {
	// ShowGlobal displays the global budget section.
	ShowGlobal bool
	// ShowProvider displays the BY PROVIDER section.
	ShowProvider bool
	// ShowTag displays the BY TAG section.
	ShowTag bool
	// ShowType displays the BY TYPE section.
	ShowType bool
	// ProviderFilter limits provider display to specific providers.
	ProviderFilter []string
}

// NewBudgetScopeFilter creates a filter from a --budget-scope flag value.
// Empty string means show all scopes. Otherwise, accepts comma-separated values:
// - "global" - show global budget only
// - "provider" - show BY PROVIDER section
// - "provider=aws" - show only AWS provider budget
// - "tag" - show BY TAG section
// NewBudgetScopeFilter creates a BudgetScopeFilter from a comma-separated scope flag.
// The scopeFlag controls which sections of scoped budget output are enabled and may
// include: "global", "provider", "provider=<name>", "tag", and "type". An empty
// scopeFlag enables all sections. Multiple specifiers can be combined with commas.
// Provider names are stored lowercased when provided as "provider=<name>". If no
// valid specifiers are found the filter defaults to enabling all sections.
func NewBudgetScopeFilter(scopeFlag string) *BudgetScopeFilter {
	filter := &BudgetScopeFilter{}

	// Empty flag means show all scopes
	if scopeFlag == "" {
		filter.ShowGlobal = true
		filter.ShowProvider = true
		filter.ShowTag = true
		filter.ShowType = true
		return filter
	}

	// Parse comma-separated scope specifiers
	for part := range strings.SplitSeq(scopeFlag, ",") {
		part = strings.TrimSpace(strings.ToLower(part))

		switch {
		case part == "global":
			filter.ShowGlobal = true
		case strings.HasPrefix(part, "provider="):
			filter.ShowProvider = true
			provider := strings.TrimPrefix(part, "provider=")
			if provider != "" {
				filter.ProviderFilter = append(filter.ProviderFilter, provider)
			}
		case part == "provider":
			filter.ShowProvider = true
		case part == "tag":
			filter.ShowTag = true
		case part == "type":
			filter.ShowType = true
		}
	}

	// If no valid scope was specified, default to all
	if !filter.ShowGlobal && !filter.ShowProvider && !filter.ShowTag && !filter.ShowType {
		filter.ShowGlobal = true
		filter.ShowProvider = true
		filter.ShowTag = true
		filter.ShowType = true
	}

	return filter
}

// RenderScopedBudgetStatus renders the hierarchical scoped budget result to the writer.
// used. Any error returned by the underlying rendering routine is propagated.
func RenderScopedBudgetStatus(w io.Writer, result *engine.ScopedBudgetResult, filter *BudgetScopeFilter) error {
	if result == nil {
		return nil
	}

	if filter == nil {
		filter = NewBudgetScopeFilter("")
	}

	if isWriterTerminal(w) {
		return renderStyledScopedBudget(w, result, filter)
	}
	return renderPlainScopedBudget(w, result, filter)
}

// renderStyledScopedBudget renders a styled, boxed representation of a scoped budget result using Lip Gloss.
// It writes the overall health, optional GLOBAL/BY PROVIDER/BY TAG/BY TYPE sections, critical scope warnings,
// and general warnings to w according to the provided filter.
// w is the destination writer, result is the scoped budget result to render, and filter selects which sections to include.
// Returns any error encountered while writing the rendered box.
func renderStyledScopedBudget(w io.Writer, result *engine.ScopedBudgetResult, filter *BudgetScopeFilter) error {
	// Title style
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(boxTitleColor())

	// Section header style
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("33"))

	// Border style for sections
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(boxBorderColor()).
		Padding(0, 1).
		Width(scopedBoxWidth)

	var content strings.Builder

	// Main title
	content.WriteString(titleStyle.Render("BUDGET STATUS"))
	content.WriteString("\n")
	content.WriteString(strings.Repeat("═", scopedBoxWidth-scopedBoxTitlePadding))
	content.WriteString("\n\n")

	// Overall health summary
	content.WriteString(renderOverallHealthSummary(result))
	content.WriteString("\n")

	sectionsRendered := 0

	// Global section
	if filter.ShowGlobal && result.Global != nil {
		content.WriteString(sectionStyle.Render("GLOBAL"))
		content.WriteString("\n")
		content.WriteString(renderScopedStatusLine(result.Global))
		content.WriteString("\n")
		sectionsRendered++
	}

	// BY PROVIDER section
	if filter.ShowProvider && len(result.ByProvider) > 0 {
		if sectionsRendered > 0 {
			content.WriteString("\n")
		}
		content.WriteString(sectionStyle.Render("BY PROVIDER"))
		content.WriteString("\n")
		content.WriteString(renderProviderSection(result.ByProvider, filter.ProviderFilter))
		sectionsRendered++
	}

	// BY TAG section
	if filter.ShowTag && len(result.ByTag) > 0 {
		if sectionsRendered > 0 {
			content.WriteString("\n")
		}
		content.WriteString(sectionStyle.Render("BY TAG"))
		content.WriteString("\n")
		content.WriteString(renderTagSection(result.ByTag))
		sectionsRendered++
	}

	// BY TYPE section
	if filter.ShowType && len(result.ByType) > 0 {
		if sectionsRendered > 0 {
			content.WriteString("\n")
		}
		content.WriteString(sectionStyle.Render("BY TYPE"))
		content.WriteString("\n")
		content.WriteString(renderTypeSection(result.ByType))
	}

	// Critical scopes warning
	if len(result.CriticalScopes) > 0 {
		content.WriteString("\n")
		content.WriteString(renderCriticalScopesWarning(result.CriticalScopes))
	}

	// Warnings
	if len(result.Warnings) > 0 {
		content.WriteString("\n")
		content.WriteString(renderScopedWarnings(result.Warnings))
	}

	box := borderStyle.Render(content.String())
	_, err := fmt.Fprintln(w, box)
	return err
}

// renderPlainScopedBudget renders the scoped budget result as plain text to w.
// It writes the header and overall health, then conditionally writes the GLOBAL,
// BY PROVIDER, BY TAG, and BY TYPE sections according to the provided filter,
// and finally writes any critical scopes.
// w is the destination writer, result contains the scoped budget data, and filter
// selects which sections to include (if nil, all sections are rendered).
// It returns any error encountered while writing output.
func renderPlainScopedBudget(
	w io.Writer,
	result *engine.ScopedBudgetResult,
	filter *BudgetScopeFilter,
) error {
	p := message.NewPrinter(language.English)

	if err := writePlainHeader(w, p, result.OverallHealth); err != nil {
		return err
	}

	if err := writePlainGlobalSection(w, filter, result.Global); err != nil {
		return err
	}

	if err := writePlainProviderSectionWrapper(w, filter, result.ByProvider); err != nil {
		return err
	}

	if err := writePlainTagSectionWrapper(w, filter, result.ByTag); err != nil {
		return err
	}

	if err := writePlainTypeSectionWrapper(w, filter, result.ByType); err != nil {
		return err
	}

	return writePlainCriticalScopes(w, result.CriticalScopes)
}

// writePlainHeader writes the "BUDGET STATUS" header, an underline, and an
// "Overall Health" line to the provided writer.
// 
// w is the destination writer. p is a language printer used to format the
// overall health label. health is the budget health status to display.
// 
// It returns an error if any of the write operations fail.
func writePlainHeader(w io.Writer, p *message.Printer, health pbc.BudgetHealthStatus) error {
	if _, err := fmt.Fprintln(w, "BUDGET STATUS"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "============="); err != nil {
		return err
	}
	_, err := p.Fprintf(w, "Overall Health: %s\n\n", healthStatusLabel(health))
	return err
}

// writePlainGlobalSection writes the GLOBAL section when the provided filter enables it and a global scoped status is available.
// If the filter disables the global section or the global status is nil, it performs no output.
// It returns any error encountered while rendering the section.
func writePlainGlobalSection(
	w io.Writer,
	filter *BudgetScopeFilter,
	global *engine.ScopedBudgetStatus,
) error {
	if !filter.ShowGlobal || global == nil {
		return nil
	}
	return writePlainSection(w, "GLOBAL", "------", func() error {
		return renderPlainScopedStatusLine(w, global)
	})
}

// writePlainProviderSectionWrapper writes the "BY PROVIDER" section to w when the
// filter enables provider sections and providers contains entries. If the filter
// disables provider output or providers is empty, it does nothing. It returns any
// error produced while writing the section content.
func writePlainProviderSectionWrapper(
	w io.Writer,
	filter *BudgetScopeFilter,
	providers map[string]*engine.ScopedBudgetStatus,
) error {
	if !filter.ShowProvider || len(providers) == 0 {
		return nil
	}
	return writePlainSection(w, "BY PROVIDER", "-----------", func() error {
		return renderPlainProviderSection(w, providers, filter.ProviderFilter)
	})
}

// writePlainTagSectionWrapper writes the tag section if enabled and has data.
func writePlainTagSectionWrapper(
	w io.Writer,
	filter *BudgetScopeFilter,
	tags []*engine.ScopedBudgetStatus,
) error {
	if !filter.ShowTag || len(tags) == 0 {
		return nil
	}
	return writePlainSection(w, "BY TAG", "------", func() error {
		return renderPlainTagSection(w, tags)
	})
}

// writePlainTypeSectionWrapper writes the BY TYPE section to w when the provided
// filter enables type sections and the types map is non-empty. It returns any
// error encountered while writing the section.
func writePlainTypeSectionWrapper(
	w io.Writer,
	filter *BudgetScopeFilter,
	types map[string]*engine.ScopedBudgetStatus,
) error {
	if !filter.ShowType || len(types) == 0 {
		return nil
	}
	return writePlainSection(w, "BY TYPE", "-------", func() error {
		return renderPlainTypeSection(w, types)
	})
}

// writePlainSection writes a section header and underline, invokes renderContent to write
// the section body, and appends a trailing newline.
// It returns the first non-nil error encountered while writing the header, writing the
// underline, executing renderContent, or writing the trailing newline.
func writePlainSection(w io.Writer, header, underline string, renderContent func() error) error {
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, underline); err != nil {
		return err
	}
	if err := renderContent(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w)
	return err
}

// writePlainCriticalScopes writes a "CRITICAL SCOPES:" section and lists each scope on its own line
// prefixed with "  - ". If `criticalScopes` is empty, it does nothing. It returns any error encountered
// while writing to `w`.
func writePlainCriticalScopes(w io.Writer, criticalScopes []string) error {
	if len(criticalScopes) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "CRITICAL SCOPES:"); err != nil {
		return err
	}
	for _, scope := range criticalScopes {
		if _, err := fmt.Fprintf(w, "  - %s\n", scope); err != nil {
			return err
		}
	}
	return nil
}

// renderOverallHealthSummary builds a formatted "Overall Health" line for the given scoped budget result.
// It reads result.OverallHealth to produce a labeled, styled health indicator used for display.
// The result must be non-nil; calling this with a nil result will cause a panic.
// It returns the complete "Overall Health: <label>" string with the label styled.
func renderOverallHealthSummary(result *engine.ScopedBudgetResult) string {
	p := message.NewPrinter(language.English)

	label := healthStatusLabel(result.OverallHealth)
	color := healthStatusColor(result.OverallHealth)

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(color)

	return p.Sprintf("Overall Health: %s", style.Render(label))
}

// renderScopedStatusLine builds a multiline string describing a scoped budget status,
// including budget and spend amounts, the percentage used, a horizontal progress bar,
// and a styled health label.
// 
// The `status` parameter provides the scoped budget values and health to format.
// 
// Returns the formatted multiline string containing the budget line, progress bar,
// and colored health label.
func renderScopedStatusLine(status *engine.ScopedBudgetStatus) string {
	p := message.NewPrinter(language.English)
	var content strings.Builder

	// Budget and spend info
	budgetLine := p.Sprintf("  Budget: %s%.2f  |  Spend: %s%.2f (%.1f%%)",
		currencySymbol(status.Currency),
		status.Budget.Amount,
		currencySymbol(status.Currency),
		status.CurrentSpend,
		status.Percentage)
	content.WriteString(budgetLine)
	content.WriteString("\n")

	// Progress bar
	bar := renderScopedProgressBar(status.Percentage, scopedMinProgressBar)
	content.WriteString("  ")
	content.WriteString(bar)

	// Health status
	healthLabel := healthStatusLabel(status.Health)
	healthColor := healthStatusColor(status.Health)
	healthStyle := lipgloss.NewStyle().Bold(true).Foreground(healthColor)
	content.WriteString("  ")
	content.WriteString(healthStyle.Render(healthLabel))

	return content.String()
}

// renderPlainScopedStatusLine writes a single-line plain-text summary of the given scoped budget status to w.
// The output includes the budget amount (with currency symbol), current spend, percentage, and health label.
// w is the destination writer and status provides the values to render.
// It returns any write error encountered.
func renderPlainScopedStatusLine(w io.Writer, status *engine.ScopedBudgetStatus) error {
	p := message.NewPrinter(language.English)

	if _, err := p.Fprintf(w, "  Budget: %s%.2f | Spend: %s%.2f (%.1f%%) | Status: %s\n",
		currencySymbol(status.Currency),
		status.Budget.Amount,
		currencySymbol(status.Currency),
		status.CurrentSpend,
		status.Percentage,
		healthStatusLabel(status.Health)); err != nil {
		return err
	}
	return nil
}

// renderProviderSection builds the "BY PROVIDER" section content for the given providers.
// It includes an uppercase provider label followed by the provider's scoped status for each entry.
// If filterProviders is non-empty, only providers whose name appears in filterProviders (case-insensitive) are included.
// The returned string contains the concatenated section lines with trailing newlines for each provider block.
func renderProviderSection(providers map[string]*engine.ScopedBudgetStatus, filterProviders []string) string {
	var content strings.Builder

	// Get sorted provider keys
	keys := make([]string, 0, len(providers))
	for key := range providers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		// Apply provider filter if specified
		if len(filterProviders) > 0 && !containsIgnoreCase(filterProviders, key) {
			continue
		}

		status := providers[key]
		labelStyle := lipgloss.NewStyle().Bold(true)
		content.WriteString(labelStyle.Render(strings.ToUpper(key)))
		content.WriteString("\n")
		content.WriteString(renderScopedStatusLine(status))
		content.WriteString("\n")
	}

	return content.String()
}

// renderPlainProviderSection writes the "BY PROVIDER" plain-text section to w.
// It lists providers in alphabetical order and, if filterProviders is non-empty,
// restricts output to providers whose names match any entry in filterProviders
// (case-insensitive). For each included provider it writes the provider name in
// uppercase followed by the provider's plain scoped status line.
//
// w is the destination writer. providers maps provider names to their scoped
// status. filterProviders, when non-empty, limits which providers are rendered.
//
// It returns any write or rendering error encountered.
func renderPlainProviderSection(
	w io.Writer,
	providers map[string]*engine.ScopedBudgetStatus,
	filterProviders []string,
) error {
	keys := make([]string, 0, len(providers))
	for key := range providers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if len(filterProviders) > 0 && !containsIgnoreCase(filterProviders, key) {
			continue
		}

		status := providers[key]
		if _, err := fmt.Fprintf(w, "%s:\n", strings.ToUpper(key)); err != nil {
			return err
		}
		if err := renderPlainScopedStatusLine(w, status); err != nil {
			return err
		}
	}
	return nil
}

// renderTagSection builds the BY TAG section content from the provided scoped statuses.
// It returns a formatted string containing each tag's scope key and its corresponding rendered scoped status.
// The tags parameter supplies the scoped budget statuses to include, in the given order.
func renderTagSection(tags []*engine.ScopedBudgetStatus) string {
	var content strings.Builder

	for _, status := range tags {
		labelStyle := lipgloss.NewStyle().Bold(true)
		content.WriteString(labelStyle.Render(status.ScopeKey))
		content.WriteString("\n")
		content.WriteString(renderScopedStatusLine(status))
		content.WriteString("\n")
	}

	return content.String()
}

// renderPlainTagSection writes the BY TAG section in plain text to w.
// For each ScopedBudgetStatus in tags it writes a header line with the
// status.ScopeKey followed by the scoped status line rendered by
// renderPlainScopedStatusLine.
// w is the destination writer and tags is the list of tag-scoped statuses to render.
// It returns any error encountered while writing to w or while rendering a scoped status.
func renderPlainTagSection(w io.Writer, tags []*engine.ScopedBudgetStatus) error {
	for _, status := range tags {
		if _, err := fmt.Fprintf(w, "%s:\n", status.ScopeKey); err != nil {
			return err
		}
		if err := renderPlainScopedStatusLine(w, status); err != nil {
			return err
		}
	}
	return nil
}

// renderTypeSection builds the "BY TYPE" section content for the provided map of scoped statuses.
// For each type key (sorted alphabetically) it writes the key as a bold label followed by the
// corresponding rendered scoped status line, each separated by newlines. The returned string
// contains the concatenated section content.
func renderTypeSection(types map[string]*engine.ScopedBudgetStatus) string {
	var content strings.Builder

	keys := make([]string, 0, len(types))
	for key := range types {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		status := types[key]
		labelStyle := lipgloss.NewStyle().Bold(true)
		content.WriteString(labelStyle.Render(key))
		content.WriteString("\n")
		content.WriteString(renderScopedStatusLine(status))
		content.WriteString("\n")
	}

	return content.String()
}

// renderPlainTypeSection writes the plain-text BY TYPE section to w.
// It lists type keys in alphabetical order and for each key writes a header
// line "<type>:" followed by the plain scoped status line for that type.
// It returns any write error encountered or any error returned by
// renderPlainScopedStatusLine.
func renderPlainTypeSection(w io.Writer, types map[string]*engine.ScopedBudgetStatus) error {
	keys := make([]string, 0, len(types))
	for key := range types {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		status := types[key]
		if _, err := fmt.Fprintf(w, "%s:\n", key); err != nil {
			return err
		}
		if err := renderPlainScopedStatusLine(w, status); err != nil {
			return err
		}
	}
	return nil
}

// renderCriticalScopesWarning renders the critical scopes warning section.
func renderCriticalScopesWarning(scopes []string) string {
	warningStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(progressExceededColor())

	var content strings.Builder
	content.WriteString(warningStyle.Render("⚠ CRITICAL SCOPES:"))
	content.WriteString("\n")

	for _, scope := range scopes {
		content.WriteString("  • ")
		content.WriteString(scope)
		content.WriteString("\n")
	}

	return content.String()
}

// renderScopedWarnings renders a warnings block for scoped budget output.
// It returns a string containing a "Warnings:" header followed by each warning
// on its own line prefixed with a bullet ("•") and two-space indent.
func renderScopedWarnings(warnings []string) string {
	warningStyle := lipgloss.NewStyle().
		Foreground(colorWarning())

	var content strings.Builder
	content.WriteString(warningStyle.Render("Warnings:"))
	content.WriteString("\n")

	for _, warning := range warnings {
		content.WriteString("  • ")
		content.WriteString(warning)
		content.WriteString("\n")
	}

	return content.String()
}

// renderScopedProgressBar renders a horizontal progress bar that visually represents
// the given percentage within a fixed character width.
//
// The displayed fill is capped at 100% for the bar length calculation; the bar's
// color reflects the original (uncapped) percentage. The returned string contains
// the styled filled and empty segments ready for printing.
func renderScopedProgressBar(percentage float64, width int) string {
	// Cap percentage at 100% for bar display
	cappedPercent := percentage
	if cappedPercent > maxPercentageForBarCap {
		cappedPercent = maxPercentageForBarCap
	}

	filledWidth := int(cappedPercent / maxPercentageForBarCap * float64(width))
	emptyWidth := width - filledWidth

	// Determine color based on percentage
	barColor := determineProgressBarColor(percentage)

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	filled := filledStyle.Render(strings.Repeat(progressFilledChar, filledWidth))
	empty := emptyStyle.Render(strings.Repeat(progressEmptyChar, emptyWidth))

	return filled + empty
}

// healthStatusLabel maps a BudgetHealthStatus to its human-readable label.
// It returns the `UNSPECIFIED` label for unknown or default values.
func healthStatusLabel(health pbc.BudgetHealthStatus) string {
	switch health {
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK:
		return healthOKLabel
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING:
		return healthWarningLabel
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL:
		return healthCriticalLabel
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED:
		return healthExceededLabel
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED:
		return healthUnspecified
	default:
		return healthUnspecified
	}
}

// healthStatusColor returns the lipgloss color corresponding to the given BudgetHealthStatus.
// It maps:
//  - BUDGET_HEALTH_STATUS_OK to progressOKColor()
//  - BUDGET_HEALTH_STATUS_WARNING to colorWarning()
//  - BUDGET_HEALTH_STATUS_CRITICAL to orange ("208")
//  - BUDGET_HEALTH_STATUS_EXCEEDED to progressExceededColor()
//  - BUDGET_HEALTH_STATUS_UNSPECIFIED (and any unknown value) to gray ("246").
func healthStatusColor(health pbc.BudgetHealthStatus) lipgloss.Color {
	switch health {
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK:
		return progressOKColor()
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING:
		return colorWarning()
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL:
		return lipgloss.Color("208") // Orange
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED:
		return progressExceededColor()
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED:
		return lipgloss.Color("246") // Gray
	default:
		return lipgloss.Color("246") // Gray
	}
}

// containsIgnoreCase reports whether the slice contains target using a case-insensitive comparison.
// It returns true if any element of slice equals target when compared ignoring case, false otherwise.
func containsIgnoreCase(slice []string, target string) bool {
	targetLower := strings.ToLower(target)
	for _, s := range slice {
		if strings.ToLower(s) == targetLower {
			return true
		}
	}
	return false
}