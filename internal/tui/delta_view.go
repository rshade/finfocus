package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/rshade/finfocus/internal/engine"
)

// Column width constants for property table formatting.
const (
	propertyKeyWidth   = 20 // Width for property name column
	propertyValueWidth = 15 // Width for original/modified value columns
	separatorWidth     = 60 // Width for horizontal separator lines
	deltaSeparatorLen  = 50 // Width for delta section separator
	minTruncateLen     = 3  // Minimum length before truncation with ellipsis
)

// defaultEstimateCurrency is the default currency for cost estimates.
const defaultEstimateCurrency = "USD"

// RenderEstimateDelta renders a styled cost delta with sign and directional arrow.
//
// Parameters:
//   - delta: The cost change (positive = increase, negative = savings)
//
// Returns a styled string with:
//   - "+" prefix for positive values
//   - ↑ arrow for increases (warning color)
//   - ↓ arrow for decreases (OK color)
//   - → arrow for no change (muted color)
func RenderEstimateDelta(delta float64) string {
	// Round to cents for display consistency
	rounded := math.Round(delta*centsMultiplier) / centsMultiplier

	var icon, sign string
	var color lipgloss.Color

	switch {
	case rounded > 0:
		icon = IconArrowUp
		sign = "+"
		color = ColorWarning
	case rounded < 0:
		icon = IconArrowDown
		sign = ""
		color = ColorOK
	default:
		icon = IconArrowRight
		sign = ""
		color = ColorMuted
	}

	formatted := fmt.Sprintf("$%.2f", math.Abs(rounded))
	style := lipgloss.NewStyle().Foreground(color).Bold(true)
	return style.Render(fmt.Sprintf("%s%s %s", sign, formatted, icon))
}

// RenderEstimateHeader renders the header for the estimate TUI.
//
// Parameters:
//   - provider: Cloud provider name (e.g., "aws")
//   - resourceType: Resource type (e.g., "ec2:Instance")
//   - resourceID: Optional resource ID
//
// Returns a styled header string.
func RenderEstimateHeader(provider, resourceType, resourceID string) string {
	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorHeader).
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	sb.WriteString(titleStyle.Render("What-If Cost Analysis"))
	sb.WriteString("\n\n")

	// Resource info
	labelStyle := lipgloss.NewStyle().Foreground(ColorLabel)
	valueStyle := lipgloss.NewStyle().Foreground(ColorValue).Bold(true)

	sb.WriteString(labelStyle.Render("Provider: "))
	sb.WriteString(valueStyle.Render(provider))
	sb.WriteString("\n")

	sb.WriteString(labelStyle.Render("Type: "))
	sb.WriteString(valueStyle.Render(resourceType))

	if resourceID != "" {
		sb.WriteString("\n")
		sb.WriteString(labelStyle.Render("ID: "))
		sb.WriteString(valueStyle.Render(resourceID))
	}

	return sb.String()
}

// RenderCostComparison renders the cost comparison section.
//
// Parameters:
//   - baseline: The baseline monthly cost
//   - modified: The modified monthly cost
//   - currency: The currency code (e.g., "USD")
//
// Returns a styled cost comparison view.
func RenderCostComparison(baseline, modified float64, currency string) string {
	var sb strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(ColorLabel)
	valueStyle := lipgloss.NewStyle().Foreground(ColorValue).Bold(true)

	symbol := getCurrencySymbol(currency)

	// Baseline cost
	sb.WriteString(labelStyle.Render("Baseline:  "))
	sb.WriteString(valueStyle.Render(fmt.Sprintf("%s%.2f/mo (%s)", symbol, baseline, currency)))
	sb.WriteString("\n")

	// Modified cost
	sb.WriteString(labelStyle.Render("Modified:  "))
	sb.WriteString(valueStyle.Render(fmt.Sprintf("%s%.2f/mo (%s)", symbol, modified, currency)))
	sb.WriteString("\n\n")

	// Total change
	change := modified - baseline
	sb.WriteString(labelStyle.Render("Change:    "))
	sb.WriteString(RenderEstimateDelta(change))
	sb.WriteString("/mo")

	return sb.String()
}

// RenderPropertyTable renders the editable property table.
//
// Parameters:
//   - properties: List of property rows to display
//   - focusedRow: Index of the currently focused row
//   - editing: Whether we're currently in edit mode
//
// Returns a styled property table.
func RenderPropertyTable(properties []PropertyRow, focusedRow int, editing bool) string {
	if len(properties) == 0 {
		muted := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
		return muted.Render("No properties to edit")
	}

	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().Foreground(ColorHeader).Bold(true)
	sb.WriteString(headerStyle.Render("Property Changes:"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("-", separatorWidth))
	sb.WriteString("\n")

	// Column headers
	labelStyle := lipgloss.NewStyle().Foreground(ColorLabel)
	sb.WriteString(labelStyle.Render(fmt.Sprintf("  %-*s %-*s %-*s %s\n",
		propertyKeyWidth, "Property", propertyValueWidth, "Original", propertyValueWidth, "Modified", "Δ Cost")))
	sb.WriteString("\n")

	// Property rows
	for i, prop := range properties {
		row := renderPropertyRow(prop, i == focusedRow, editing && i == focusedRow)
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderPropertyRow renders a single property row.
func renderPropertyRow(prop PropertyRow, focused, editing bool) string {
	var sb strings.Builder

	// Focus indicator
	if focused {
		if editing {
			sb.WriteString("> ")
		} else {
			sb.WriteString("→ ")
		}
	} else {
		sb.WriteString("  ")
	}

	// Styles based on state
	keyStyle := lipgloss.NewStyle().Foreground(ColorLabel)
	valueStyle := lipgloss.NewStyle().Foreground(ColorValue)
	modifiedStyle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)

	// Property name
	sb.WriteString(keyStyle.Render(fmt.Sprintf("%-*s", propertyKeyWidth, truncate(prop.Key, propertyKeyWidth))))

	// Original value
	origTrunc := truncate(prop.OriginalValue, propertyValueWidth)
	sb.WriteString(valueStyle.Render(fmt.Sprintf("%-*s", propertyValueWidth, origTrunc)))

	// Modified value (highlighted if changed)
	currTrunc := truncate(prop.CurrentValue, propertyValueWidth)
	currFormatted := fmt.Sprintf("%-*s", propertyValueWidth, currTrunc)
	if prop.CurrentValue != prop.OriginalValue {
		sb.WriteString(modifiedStyle.Render(currFormatted))
	} else {
		sb.WriteString(valueStyle.Render(currFormatted))
	}

	// Delta
	if prop.CostDelta != 0 {
		sb.WriteString(RenderEstimateDelta(prop.CostDelta))
	} else if prop.CurrentValue != prop.OriginalValue {
		// Show pending indicator if value changed but delta not calculated
		muted := lipgloss.NewStyle().Foreground(ColorMuted)
		sb.WriteString(muted.Render("(pending)"))
	}

	return sb.String()
}

// truncate truncates a string to the specified length with ellipsis.
// Uses rune-aware counting to properly handle multi-byte UTF-8 characters.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= minTruncateLen {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-minTruncateLen]) + "..."
}

// RenderEstimateResultView renders a complete estimate result for display.
//
// Parameters:
//   - result: The estimate result to render
//   - width: The available width for rendering
//
// Returns a styled view of the estimate result.
func RenderEstimateResultView(result *engine.EstimateResult, width int) string {
	if result == nil {
		muted := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
		return muted.Render("No estimate result available")
	}

	var sb strings.Builder

	// Header
	provider := ""
	resourceType := ""
	resourceID := ""
	if result.Resource != nil {
		provider = result.Resource.Provider
		resourceType = result.Resource.Type
		resourceID = result.Resource.ID
	}
	sb.WriteString(RenderEstimateHeader(provider, resourceType, resourceID))
	sb.WriteString("\n\n")

	// Cost comparison
	baseline := 0.0
	modified := 0.0
	currency := defaultEstimateCurrency
	if result.Baseline != nil {
		baseline = result.Baseline.Monthly
		if result.Baseline.Currency != "" {
			currency = result.Baseline.Currency
		}
	}
	if result.Modified != nil {
		modified = result.Modified.Monthly
	}
	sb.WriteString(RenderCostComparison(baseline, modified, currency))
	sb.WriteString("\n\n")

	// Property deltas
	if len(result.Deltas) > 0 {
		sb.WriteString(renderDeltaList(result.Deltas))
		sb.WriteString("\n")
	}

	// Fallback note
	if result.UsedFallback {
		noteStyle := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
		sb.WriteString(noteStyle.Render("Note: Estimated using fallback strategy (EstimateCost RPC not available)"))
		sb.WriteString("\n")
	}

	// Apply width constraint
	if width > 0 {
		boxStyle := lipgloss.NewStyle().MaxWidth(width)
		return boxStyle.Render(sb.String())
	}

	return sb.String()
}

// renderDeltaList renders the list of property deltas.
func renderDeltaList(deltas []engine.CostDelta) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().Foreground(ColorHeader).Bold(true)
	sb.WriteString(headerStyle.Render("Property Changes:"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("-", deltaSeparatorLen))
	sb.WriteString("\n")

	for _, delta := range deltas {
		sb.WriteString(renderDeltaRow(delta))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderDeltaRow renders a single delta row.
func renderDeltaRow(delta engine.CostDelta) string {
	labelStyle := lipgloss.NewStyle().Foreground(ColorLabel)
	valueStyle := lipgloss.NewStyle().Foreground(ColorValue)
	arrowStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	return fmt.Sprintf("  %s: %s %s %s (%s)",
		labelStyle.Render(delta.Property),
		valueStyle.Render(delta.OriginalValue),
		arrowStyle.Render("→"),
		valueStyle.Render(delta.NewValue),
		RenderEstimateDelta(delta.CostChange),
	)
}

// RenderEstimateHelp renders the keyboard shortcut help text.
func RenderEstimateHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	shortcuts := []string{
		"↑/↓: Navigate",
		"Enter: Edit property",
		"Esc: Cancel edit",
		"q: Quit",
	}

	return helpStyle.Render(strings.Join(shortcuts, " | "))
}

// RenderLoadingIndicator renders a loading indicator for cost calculation.
func RenderLoadingIndicator() string {
	loadingStyle := lipgloss.NewStyle().
		Foreground(ColorSpinner).
		Bold(true)

	return loadingStyle.Render("Calculating costs...")
}
