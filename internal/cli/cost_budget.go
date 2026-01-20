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

// boxBorderColor returns the lipgloss.Color used for budget box borders.
func boxBorderColor() lipgloss.Color   { return lipgloss.Color("240") }
// boxTitleColor returns the Lip Gloss color used for budget box titles.
func boxTitleColor() lipgloss.Color    { return lipgloss.Color("39") }
// colorWarning returns the lipgloss color used for warning-level (approaching) budget states.
func colorWarning() lipgloss.Color     { return lipgloss.Color("214") }
// colorApproaching returns the Lip Gloss color used to style alerts and progress indicators when a budget is approaching its threshold.
func colorApproaching() lipgloss.Color { return lipgloss.Color("220") }
// progressOKColor returns the color used for progress bars that indicate an OK (below warning) budget level.
func progressOKColor() lipgloss.Color  { return lipgloss.Color("42") }

// progressWarningColor returns the lipgloss.Color used to represent a warning state in the progress bar.
// It is retained for completeness and may be unused in the current presentation logic.
func progressWarningColor() lipgloss.Color  { return lipgloss.Color("214") }
// progressExceededColor returns the Lip Gloss color used for progress bars and alerts when a budget has been exceeded.
func progressExceededColor() lipgloss.Color { return lipgloss.Color("196") }

// RenderBudgetStatus renders the budget status to the writer.
// It automatically detects if the output is a TTY and renders appropriately:
// - TTY: Styled output with colors and borders using Lip Gloss.
// RenderBudgetStatus renders the given budget status to w using a terminal-styled
// bordered box when w is a TTY or plain text suitable for non-interactive outputs
// otherwise.
//
// w is the destination for the rendered output. status is the budget status to
// render; if status is nil, the function performs no output and returns nil.
//
// The function returns any error produced by the chosen rendering implementation.
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
// isWriterTerminal reports whether the provided io.Writer refers to a terminal.
// It returns true when w is an *os.File whose file descriptor is a terminal, and false for any other writer.
func isWriterTerminal(w io.Writer) bool {
	// If w is a file (like os.Stdout), check if it's a terminal
	if f, ok := w.(*os.File); ok {
		return isTerminal(f)
	}
	// For other writers (like bytes.Buffer in tests), check if w wraps stdout
	return false
}

// renderStyledBudget renders a styled budget status box using Lip Gloss.
// renderStyledBudget writes a styled, bordered budget status box to the provided writer for TTY output.
// 
// It renders title, budget and current spend lines, a colored progress bar, alert messages, and an
// optional forecasted spend line using terminal styling. If `status` contains forecast or alert
// information those are included in the box.
// 
// Parameters:
//   - w: the destination io.Writer (expected to be a terminal/TTY).
//   - status: the budget status to render; its fields drive the amounts, percentages, alerts, and forecast.
//
// Returns an error if writing the rendered box to `w` fails.
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
// renderPlainBudget writes a plain-text budget summary to w suitable for non-TTY environments such as CI/CD pipelines and log files.
// It writes a header, the configured budget, current spend with percentage, a status message, and an optional forecasted spend line based on the provided status.
// The status parameter supplies currency, budget amounts, current and forecasted spends, and percentage values used in the output.
// It always returns nil.
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

// renderProgressBar renders a colored ASCII progress bar representing the given budget status.
// It uses status.CappedPercentage to compute the filled portion and `width` as the total bar length.
// The returned string combines colored filled and empty segments and appends a numeric percentage
// label; the percentage label is highlighted with the exceeded style when the status is over budget.
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

// determineProgressBarColor chooses a progress bar color based on the percentage of budget used.
// It returns the exceeded color for percentages >= 100%, a warning color for percentages >= 80% and < 100%, and the OK color for percentages < 80%.
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

// renderAlertMessages formats alert messages for the thresholds contained in the provided BudgetStatus.
// It returns the formatted alert lines joined with newline separators; if there are no alerts to show, an empty string is returned.
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

// formatAlertMessage returns a formatted alert string that combines the provided prefix with the alert's
// type ("spend" or "forecasted spend") and the alert threshold as a whole percentage. The returned string
// has the form "<prefix> - <type> exceeds <threshold>%".
func formatAlertMessage(alert engine.ThresholdStatus, prefix string) string {
	typeStr := "spend"
	if alert.Type == config.AlertTypeForecasted {
		typeStr = "forecasted spend"
	}
	return fmt.Sprintf("%s - %s exceeds %.0f%% threshold", prefix, typeStr, alert.Threshold)
}

// getStatusMessage returns a plain-text status label for non-TTY output.
// It returns one of:
// - "WARNING - Exceeds X% threshold" when any threshold has been exceeded (X is the highest exceeded threshold),
// - "APPROACHING - Near budget threshold" when a threshold is being approached,
// - "OK - Within budget" when the spend is within configured thresholds.
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

// currencySymbol maps a three-letter currency code to its typographic symbol.
// For unrecognized codes the function returns the original code followed by a space.
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

// getTerminalWidth returns the current terminal width in columns.
// If the terminal size cannot be determined or yields a non-positive width,
// it returns a sensible fallback width (defaultBoxWidth + boxPaddingWidth).
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

// calculateProgressBarWidth returns the width available for the progress bar given the containing box width.
// It subtracts barPaddingWidth from boxWidth and then clamps the result to be at least minProgressBarWidth
// and at most progressBarWidth.
func calculateProgressBarWidth(boxWidth int) int {
	barWidth := boxWidth - barPaddingWidth
	barWidth = max(barWidth, minProgressBarWidth)
	barWidth = min(barWidth, progressBarWidth)
	return barWidth
}

// renderBudgetIfConfigured checks if a budget is configured and renders status if so.
// It evaluates the current spend against the configured budget and displays the result.
// renderBudgetIfConfigured checks whether a budget is configured in the global
// configuration and, if so, evaluates the configured budgets against the
// provided totalCost and currency and renders the resulting budget status to
// the command's stdout.
//
// cmd is the cobra command used for output. totalCost is the current total
// cost to evaluate. currency is the currency code used for evaluation.
//
// If no global configuration exists or no budget is configured, renderBudgetIfConfigured
// does nothing and returns nil. It returns an error if budget evaluation fails
// or if rendering the budget status fails. A blank line is printed to the
// command output immediately before the rendered status when a budget is shown.
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