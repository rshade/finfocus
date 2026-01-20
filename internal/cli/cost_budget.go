package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

// Budget rendering constants.
const (
	defaultBoxWidth     = 44
	minBoxWidth         = 30
	progressBarWidth    = 30
	minProgressBarWidth = 10
	progressFilledChar  = "█"
	progressEmptyChar   = "░"
	narrowTerminalWidth = 40
	boxPaddingWidth     = 4 // Padding for box borders.
	layoutWidthPercent  = 0.8
	barPaddingWidth     = 14 // Account for borders, padding, and percentage label.

	// Percentage thresholds.
	thresholdPercent100 = 100
	thresholdPercent80  = 80
)

// Budget color definitions (as functions to avoid global variables).
func boxBorderColor() lipgloss.Color   { return lipgloss.Color("240") }
func boxTitleColor() lipgloss.Color    { return lipgloss.Color("39") }
func colorWarning() lipgloss.Color     { return lipgloss.Color("214") }
func colorApproaching() lipgloss.Color { return lipgloss.Color("220") }
func progressOKColor() lipgloss.Color  { return lipgloss.Color("42") }

//nolint:unused // Used for warning color in progress bar (kept for completeness).
func progressWarningColor() lipgloss.Color  { return lipgloss.Color("214") }
func progressExceededColor() lipgloss.Color { return lipgloss.Color("196") }

// RenderBudgetStatus renders the budget status to the writer.
// It automatically detects if the output is a TTY and renders appropriately:
// - TTY: Styled output with colors and borders using Lip Gloss.
// - Non-TTY: Plain text output suitable for CI/CD pipelines.
func RenderBudgetStatus(w io.Writer, status *engine.BudgetStatus) error {
	if status == nil {
		return nil
	}

	// Detect if we're writing to a TTY
	if isWriterTerminal(w) {
		return renderStyledBudget(w, status)
	}
	return renderPlainBudget(w, status)
}

// isWriterTerminal checks if the writer is a terminal (TTY).
// This wraps the package-level isTerminal function for io.Writer compatibility.
func isWriterTerminal(w io.Writer) bool {
	// If w is a file (like os.Stdout), check if it's a terminal
	if f, ok := w.(*os.File); ok {
		return isTerminal(f)
	}
	// For other writers (like bytes.Buffer in tests), check if w wraps stdout
	return false
}

// renderStyledBudget renders a styled budget status box using Lip Gloss.
// This is displayed when output is to a TTY terminal.
func renderStyledBudget(w io.Writer, status *engine.BudgetStatus) error {
	// Get terminal width for responsive layout
	width := getTerminalWidth()
	boxWidth := calculateBoxWidth(width)
	barWidth := calculateProgressBarWidth(boxWidth)

	// Create styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(boxTitleColor())

	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(boxBorderColor()).
		Padding(0, 1).
		Width(boxWidth)

	// Build content
	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("BUDGET STATUS"))
	content.WriteString("\n")
	content.WriteString(strings.Repeat("─", boxWidth-boxPaddingWidth))
	content.WriteString("\n\n")

	// Budget amount and period
	p := message.NewPrinter(language.English)
	budgetLine := p.Sprintf("Budget: %s%.2f/%s",
		currencySymbol(status.Currency),
		status.Budget.Amount,
		status.Budget.GetPeriod())
	content.WriteString(budgetLine)
	content.WriteString("\n")

	// Current spend with percentage
	spendLine := p.Sprintf("Current Spend: %s%.2f (%.1f%%)",
		currencySymbol(status.Currency),
		status.CurrentSpend,
		status.Percentage)
	content.WriteString(spendLine)
	content.WriteString("\n\n")

	// Progress bar
	progressBar := renderProgressBar(status, barWidth)
	content.WriteString(progressBar)
	content.WriteString("\n")

	// Alert messages
	alertMessages := renderAlertMessages(status)
	if alertMessages != "" {
		content.WriteString("\n")
		content.WriteString(alertMessages)
	}

	// Forecasted spend (if applicable)
	if status.ForecastedSpend > 0 {
		content.WriteString("\n")
		forecastLine := p.Sprintf("Forecasted: %s%.2f (%.1f%%)",
			currencySymbol(status.Currency),
			status.ForecastedSpend,
			status.ForecastPercentage)
		forecastStyle := lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("246"))
		content.WriteString(forecastStyle.Render(forecastLine))
	}

	// Render the box
	box := borderStyle.Render(content.String())
	_, err := fmt.Fprintln(w, box)
	return err
}

// renderPlainBudget renders a plain text budget status for non-TTY environments.
// This is suitable for CI/CD pipelines and log files.
func renderPlainBudget(w io.Writer, status *engine.BudgetStatus) error {
	p := message.NewPrinter(language.English)

	fmt.Fprintln(w, "BUDGET STATUS")
	fmt.Fprintln(w, "=============")
	fmt.Fprintf(w, "Budget: %s%.2f/%s\n",
		currencySymbol(status.Currency),
		status.Budget.Amount,
		status.Budget.GetPeriod())
	fmt.Fprintf(w, "Current Spend: %s%.2f (%.1f%%)\n",
		currencySymbol(status.Currency),
		status.CurrentSpend,
		status.Percentage)

	// Status line
	statusMsg := getStatusMessage(status)
	_, _ = p.Fprintf(w, "Status: %s\n", statusMsg)

	// Forecasted spend (if applicable)
	if status.ForecastedSpend > 0 {
		fmt.Fprintf(w, "Forecasted: %s%.2f (%.1f%%)\n",
			currencySymbol(status.Currency),
			status.ForecastedSpend,
			status.ForecastPercentage)
	}

	return nil
}

// renderProgressBar creates an ASCII progress bar with color based on status.
func renderProgressBar(status *engine.BudgetStatus, width int) string {
	// Calculate filled portion (capped at 100%)
	cappedPercent := status.CappedPercentage()
	filledWidth := int(cappedPercent / thresholdPercent100 * float64(width))
	emptyWidth := width - filledWidth

	// Determine color based on percentage
	barColor := determineProgressBarColor(status.Percentage)

	// Build the bar
	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	filled := filledStyle.Render(strings.Repeat(progressFilledChar, filledWidth))
	empty := emptyStyle.Render(strings.Repeat(progressEmptyChar, emptyWidth))

	// Add percentage label
	percentLabel := fmt.Sprintf(" %.0f%%", status.Percentage)
	if status.IsOverBudget() {
		percentLabel = lipgloss.NewStyle().
			Foreground(progressExceededColor()).
			Bold(true).
			Render(percentLabel)
	}

	return filled + empty + percentLabel
}

// determineProgressBarColor returns the appropriate color based on percentage.
func determineProgressBarColor(percentage float64) lipgloss.Color {
	switch {
	case percentage >= thresholdPercent100:
		return progressExceededColor()
	case percentage >= thresholdPercent80:
		return lipgloss.Color("214") // Warning color.
	default:
		return progressOKColor()
	}
}

// renderAlertMessages creates alert message lines based on threshold statuses.
func renderAlertMessages(status *engine.BudgetStatus) string {
	var messages []string

	for _, alert := range status.Alerts {
		switch alert.Status {
		case engine.ThresholdStatusExceeded:
			msg := formatAlertMessage(alert, "WARNING")
			style := lipgloss.NewStyle().Foreground(colorWarning()).Bold(true)
			messages = append(messages, style.Render("⚠ "+msg))
		case engine.ThresholdStatusApproaching:
			msg := formatAlertMessage(alert, "APPROACHING")
			style := lipgloss.NewStyle().Foreground(colorApproaching())
			messages = append(messages, style.Render("◉ "+msg))
		case engine.ThresholdStatusOK:
			// OK status doesn't generate an alert message.
		}
	}

	return strings.Join(messages, "\n")
}

// formatAlertMessage creates a human-readable alert message.
func formatAlertMessage(alert engine.ThresholdStatus, prefix string) string {
	typeStr := "spend"
	if alert.Type == config.AlertTypeForecasted {
		typeStr = "forecasted spend"
	}
	return fmt.Sprintf("%s - %s exceeds %.0f%% threshold", prefix, typeStr, alert.Threshold)
}

// getStatusMessage returns a plain text status message for non-TTY output.
func getStatusMessage(status *engine.BudgetStatus) string {
	if status.HasExceededAlerts() {
		highest := status.GetHighestExceededThreshold()
		return fmt.Sprintf("WARNING - Exceeds %.0f%% threshold", highest)
	}
	if status.HasApproachingAlerts() {
		return "APPROACHING - Near budget threshold"
	}
	return "OK - Within budget"
}

// currencySymbol returns the symbol for a currency code.
func currencySymbol(currency string) string {
	symbols := map[string]string{
		"USD": "$",
		"EUR": "€",
		"GBP": "£",
		"JPY": "¥",
		"CAD": "C$",
		"AUD": "A$",
		"CHF": "CHF ",
	}
	if sym, ok := symbols[currency]; ok {
		return sym
	}
	return currency + " "
}

// getTerminalWidth returns the current terminal width.
func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return defaultBoxWidth + boxPaddingWidth
	}
	return width
}

// calculateBoxWidth calculates the appropriate box width based on terminal width.
func calculateBoxWidth(termWidth int) int {
	if termWidth < narrowTerminalWidth {
		return minBoxWidth
	}
	// Use ~80% of terminal width, capped at default.
	boxWidth := int(float64(termWidth) * layoutWidthPercent)
	boxWidth = min(boxWidth, defaultBoxWidth)
	boxWidth = max(boxWidth, minBoxWidth)
	return boxWidth
}

// calculateProgressBarWidth calculates the progress bar width based on box width.
func calculateProgressBarWidth(boxWidth int) int {
	barWidth := boxWidth - barPaddingWidth
	barWidth = max(barWidth, minProgressBarWidth)
	barWidth = min(barWidth, progressBarWidth)
	return barWidth
}

// renderBudgetIfConfigured checks if a budget is configured and renders status if so.
// It evaluates the current spend against the configured budget and displays the result.
// If no budget is configured, it returns nil without rendering anything.
func renderBudgetIfConfigured(cmd *cobra.Command, totalCost float64, currency string) error {
	// Get the global configuration
	cfg := config.GetGlobalConfig()
	if cfg == nil || !cfg.Cost.HasBudget() {
		return nil // No budget configured
	}

	// Create budget engine and evaluate
	budgetEngine := engine.NewBudgetEngine()
	status, err := budgetEngine.Evaluate(cfg.Cost.Budgets, totalCost, currency)
	if err != nil {
		// Budget evaluation failed (e.g., currency mismatch)
		return fmt.Errorf("evaluating budget: %w", err)
	}

	// Add a blank line before budget status
	cmd.Println()

	// Render the budget status
	return RenderBudgetStatus(cmd.OutOrStdout(), status)
}
