package cli

import (
	"fmt"
	"io"
	"slices"
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
	// TagFilter limits tag display to specific tag selectors.
	TagFilter []string
	// TypeFilter limits type display to specific resource types.
	TypeFilter []string
}

// NewBudgetScopeFilter creates a filter from a --budget-scope flag value.
// Empty string means show all scopes. Otherwise, accepts comma-separated values:
// - "global" - show global budget only
// - "provider" - show BY PROVIDER section
// - "provider=aws" - show only AWS provider budget
// - "tag" - show BY TAG section
// - "tag=team:platform" - show only the specific tag budget
// - "type" - show BY TYPE section
// - "type=aws:ec2/instance" - show only the specific resource type budget.
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
		part = strings.TrimSpace(part)
		partLower := strings.ToLower(part)

		switch {
		case partLower == "global":
			filter.ShowGlobal = true
		case strings.HasPrefix(partLower, "provider="):
			filter.ShowProvider = true
			provider := strings.TrimPrefix(part, "provider=")
			provider = strings.TrimPrefix(provider, "Provider=")
			provider = strings.TrimPrefix(provider, "PROVIDER=")
			if provider != "" {
				filter.ProviderFilter = append(filter.ProviderFilter, provider)
			}
		case partLower == "provider":
			filter.ShowProvider = true
		case strings.HasPrefix(partLower, "tag="):
			filter.ShowTag = true
			// Preserve original case for tag selectors (key:value format)
			tagSelector := strings.TrimPrefix(part, "tag=")
			tagSelector = strings.TrimPrefix(tagSelector, "Tag=")
			tagSelector = strings.TrimPrefix(tagSelector, "TAG=")
			if tagSelector != "" {
				filter.TagFilter = append(filter.TagFilter, tagSelector)
			}
		case partLower == "tag":
			filter.ShowTag = true
		case strings.HasPrefix(partLower, "type="):
			filter.ShowType = true
			// Preserve original case for resource types (aws:ec2/instance format)
			resourceType := strings.TrimPrefix(part, "type=")
			resourceType = strings.TrimPrefix(resourceType, "Type=")
			resourceType = strings.TrimPrefix(resourceType, "TYPE=")
			if resourceType != "" {
				filter.TypeFilter = append(filter.TypeFilter, resourceType)
			}
		case partLower == "type":
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
// It automatically detects if the output is a TTY and renders appropriately.
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

// renderStyledScopedBudget renders a styled hierarchical budget status box using Lip Gloss.
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
		content.WriteString(renderTagSection(result.ByTag, filter.TagFilter))
		sectionsRendered++
	}

	// BY TYPE section
	if filter.ShowType && len(result.ByType) > 0 {
		if sectionsRendered > 0 {
			content.WriteString("\n")
		}
		content.WriteString(sectionStyle.Render("BY TYPE"))
		content.WriteString("\n")
		content.WriteString(renderTypeSection(result.ByType, filter.TypeFilter))
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

// renderPlainScopedBudget renders a plain text hierarchical budget status.
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

	if err := writePlainCriticalScopes(w, result.CriticalScopes); err != nil {
		return err
	}

	return writePlainWarnings(w, result.Warnings)
}

// writePlainHeader writes the budget status header with overall health.
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

// writePlainGlobalSection writes the global budget section if enabled.
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

// writePlainProviderSectionWrapper writes the provider section if enabled and has data.
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
		return renderPlainTagSection(w, tags, filter.TagFilter)
	})
}

// writePlainTypeSectionWrapper writes the type section if enabled and has data.
func writePlainTypeSectionWrapper(
	w io.Writer,
	filter *BudgetScopeFilter,
	types map[string]*engine.ScopedBudgetStatus,
) error {
	if !filter.ShowType || len(types) == 0 {
		return nil
	}
	return writePlainSection(w, "BY TYPE", "-------", func() error {
		return renderPlainTypeSection(w, types, filter.TypeFilter)
	})
}

// writePlainSection writes a section with header, underline, content, and trailing newline.
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

// writePlainCriticalScopes writes the critical scopes section if any exist.
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

// writePlainWarnings writes budget evaluation warnings in plain text.
func writePlainWarnings(w io.Writer, warnings []string) error {
	if len(warnings) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "WARNINGS:"); err != nil {
		return err
	}
	for _, warning := range warnings {
		if _, err := fmt.Fprintf(w, "  - %s\n", warning); err != nil {
			return err
		}
	}
	return nil
}

// renderOverallHealthSummary renders the overall health status line.
func renderOverallHealthSummary(result *engine.ScopedBudgetResult) string {
	p := message.NewPrinter(language.English)

	label := healthStatusLabel(result.OverallHealth)
	color := healthStatusColor(result.OverallHealth)

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(color)

	return p.Sprintf("Overall Health: %s", style.Render(label))
}

// renderScopedStatusLine renders a single scoped budget status line with progress bar.
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

// renderPlainScopedStatusLine renders a plain text scoped budget status line.
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

// renderProviderSection renders the BY PROVIDER section content.
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

// renderPlainProviderSection renders the plain text BY PROVIDER section.
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

// renderTagSection renders the BY TAG section content.
func renderTagSection(tags []*engine.ScopedBudgetStatus, filterTags []string) string {
	var content strings.Builder

	for _, status := range tags {
		// Apply tag filter if specified
		if len(filterTags) > 0 && !containsIgnoreCase(filterTags, status.ScopeKey) {
			continue
		}

		labelStyle := lipgloss.NewStyle().Bold(true)
		content.WriteString(labelStyle.Render(status.ScopeKey))
		content.WriteString("\n")
		content.WriteString(renderScopedStatusLine(status))
		content.WriteString("\n")
	}

	return content.String()
}

// renderPlainTagSection renders the plain text BY TAG section.
func renderPlainTagSection(w io.Writer, tags []*engine.ScopedBudgetStatus, filterTags []string) error {
	for _, status := range tags {
		// Apply tag filter if specified
		if len(filterTags) > 0 && !containsIgnoreCase(filterTags, status.ScopeKey) {
			continue
		}

		if _, err := fmt.Fprintf(w, "%s:\n", status.ScopeKey); err != nil {
			return err
		}
		if err := renderPlainScopedStatusLine(w, status); err != nil {
			return err
		}
	}
	return nil
}

// renderTypeSection renders the BY TYPE section content.
func renderTypeSection(types map[string]*engine.ScopedBudgetStatus, filterTypes []string) string {
	var content strings.Builder

	keys := make([]string, 0, len(types))
	for key := range types {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		// Apply type filter if specified
		if len(filterTypes) > 0 && !containsIgnoreCase(filterTypes, key) {
			continue
		}

		status := types[key]
		labelStyle := lipgloss.NewStyle().Bold(true)
		content.WriteString(labelStyle.Render(key))
		content.WriteString("\n")
		content.WriteString(renderScopedStatusLine(status))
		content.WriteString("\n")
	}

	return content.String()
}

// renderPlainTypeSection renders the plain text BY TYPE section.
func renderPlainTypeSection(
	w io.Writer,
	types map[string]*engine.ScopedBudgetStatus,
	filterTypes []string,
) error {
	keys := make([]string, 0, len(types))
	for key := range types {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		// Apply type filter if specified
		if len(filterTypes) > 0 && !containsIgnoreCase(filterTypes, key) {
			continue
		}

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

// renderScopedWarnings renders budget evaluation warnings.
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

// renderScopedProgressBar renders a progress bar for scoped budgets.
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

// healthStatusLabel returns a human-readable label for a health status.
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

// healthStatusColor returns the appropriate color for a health status.
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

// containsIgnoreCase checks if a slice contains a string (case-insensitive).
func containsIgnoreCase(slice []string, target string) bool {
	return slices.ContainsFunc(slice, func(s string) bool {
		return strings.EqualFold(s, target)
	})
}
