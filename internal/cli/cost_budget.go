package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

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
func boxBorderColor() lipgloss.Color { return lipgloss.Color("240") }

// boxTitleColor returns the Lip Gloss color used for budget box titles.
func boxTitleColor() lipgloss.Color { return lipgloss.Color("39") }

// colorWarning returns the lipgloss color used for warning-level (approaching) budget states.
func colorWarning() lipgloss.Color { return lipgloss.Color("214") }

// colorApproaching returns the Lip Gloss color used to style alerts and progress indicators when a budget is approaching its threshold.
func colorApproaching() lipgloss.Color { return lipgloss.Color("220") }

// progressOKColor returns the color used for progress bars that indicate an OK (below warning) budget level.
func progressOKColor() lipgloss.Color { return lipgloss.Color("42") }

// progressWarningColor returns the lipgloss.Color used to represent a warning state in the progress bar.
// It is retained for completeness and may be unused in the current presentation logic.
//
//nolint:unused // Retained for future use and API completeness
func progressWarningColor() lipgloss.Color { return lipgloss.Color("214") }

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
	width := getTerminalWidth(w)
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
// It returns an error if any write operation fails.
func renderPlainBudget(w io.Writer, status *engine.BudgetStatus) error {
	p := message.NewPrinter(language.English)

	if _, err := fmt.Fprintln(w, "BUDGET STATUS"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "============="); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Budget: %s%.2f/%s\n",
		currencySymbol(status.Currency),
		status.Budget.Amount,
		status.Budget.GetPeriod()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Current Spend: %s%.2f (%.1f%%)\n",
		currencySymbol(status.Currency),
		status.CurrentSpend,
		status.Percentage); err != nil {
		return err
	}

	// Status line
	statusMsg := getStatusMessage(status)
	if _, err := p.Fprintf(w, "Status: %s\n", statusMsg); err != nil {
		return err
	}

	// Forecasted spend (if applicable)
	if status.ForecastedSpend > 0 {
		if _, err := fmt.Fprintf(w, "Forecasted: %s%.2f (%.1f%%)\n",
			currencySymbol(status.Currency),
			status.ForecastedSpend,
			status.ForecastPercentage); err != nil {
			return err
		}
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

// currencySymbols maps ISO currency codes to their typographic symbols.
// Package-level to avoid repeated allocation on each call.
//
//nolint:gochecknoglobals // Intentional: immutable lookup table for performance
var currencySymbols = map[string]string{
	defaultCurrency: "$",
	"EUR":           "€",
	"GBP":           "£",
	"JPY":           "¥",
	"CNY":           "¥",
	"CAD":           "C$",
	"AUD":           "A$",
	"INR":           "₹",
	"KRW":           "₩",
}

// currencySymbol maps a three-letter currency code to its typographic symbol.
// For unrecognized codes the function returns the original code followed by a space.
func currencySymbol(currency string) string {
	if sym, ok := currencySymbols[currency]; ok {
		return sym
	}
	return currency + " "
}

// getTerminalWidth returns the terminal width in columns for the given io.Writer.
// If the writer is an *os.File, it queries the terminal size using GetSize.
// If the terminal size cannot be determined or yields a non-positive width,
// it returns a sensible fallback width (defaultBoxWidth + boxPaddingWidth).
func getTerminalWidth(w io.Writer) int {
	// Try to detect if the writer is an *os.File with a file descriptor
	if f, ok := w.(*os.File); ok {
		width, _, err := term.GetSize(int(f.Fd()))
		if err == nil && width > 0 {
			return width
		}
	}
	// Fallback: try os.Stdout as a last resort
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
// does nothing and returns (nil, nil). It returns an error if budget evaluation fails
// or if rendering the budget status fails. A blank line is printed to the
// command output immediately before the rendered status when a budget is shown.
//
// The returned BudgetStatus can be used for exit code evaluation.
func renderBudgetIfConfigured(cmd *cobra.Command, totalCost float64, currency string) (*engine.BudgetStatus, error) {
	// Get the global configuration
	cfg := config.GetGlobalConfig()
	if cfg == nil || !cfg.Cost.HasBudget() {
		return nil, nil //nolint:nilnil // intentionally returns nil,nil when no budget configured
	}

	// Extract global budget from hierarchical config for legacy evaluation
	budgetsCfg := cfg.Cost.Budgets
	if budgetsCfg == nil || budgetsCfg.Global == nil {
		return nil, nil //nolint:nilnil // no global budget configured
	}

	// Convert ScopedBudget to BudgetConfig for evaluation
	globalBudget := budgetsCfg.Global
	budgetConfig := config.BudgetConfig{
		Amount:   globalBudget.Amount,
		Currency: globalBudget.Currency,
		Period:   globalBudget.Period,
		Alerts:   globalBudget.Alerts,
	}
	if globalBudget.ExitOnThreshold != nil {
		budgetConfig.ExitOnThreshold = *globalBudget.ExitOnThreshold
	}
	if globalBudget.ExitCode != nil {
		budgetConfig.ExitCode = *globalBudget.ExitCode
	}

	// Create budget engine and evaluate
	budgetEngine := engine.NewBudgetEngine()
	status, err := budgetEngine.Evaluate(budgetConfig, totalCost, currency)
	if err != nil {
		// Budget evaluation failed (e.g., currency mismatch)
		return nil, fmt.Errorf("evaluating budget: %w", err)
	}

	// Add a blank line before budget status
	cmd.Println()

	// Render the budget status
	if renderErr := RenderBudgetStatus(cmd.OutOrStdout(), status); renderErr != nil {
		return status, renderErr
	}

	return status, nil
}

// BudgetRenderResult holds the result of budget rendering for exit code evaluation.
// It can contain either a legacy BudgetStatus or a ScopedBudgetResult.
type BudgetRenderResult struct {
	// LegacyStatus is set when using legacy budget configuration.
	LegacyStatus *engine.BudgetStatus
	// ScopedResult is set when using scoped budget configuration.
	ScopedResult *engine.ScopedBudgetResult
}

// renderBudgetWithScope renders budget status using either scoped or legacy budgets.
// It automatically detects which configuration style is in use and renders appropriately.
// The scopeFilter parameter is only used when scoped budgets are configured.
//
// This is the main entry point for budget rendering in cost commands.
func renderBudgetWithScope(
	cmd *cobra.Command,
	costs []engine.CostResult,
	totalCost float64,
	currency string,
	scopeFilter string,
) (*BudgetRenderResult, error) {
	cfg := config.GetGlobalConfig()
	if cfg == nil {
		return nil, nil //nolint:nilnil // intentionally returns nil,nil when no config
	}

	// Check if scoped budgets are configured (provider/tag/type)
	budgetsCfg := cfg.Cost.Budgets
	if budgetsCfg != nil && budgetsCfg.HasScopedBudgets() {
		// Use scoped budget rendering
		result, err := renderScopedBudgetIfConfigured(cmd, costs, scopeFilter)
		if err != nil {
			return nil, err
		}
		return &BudgetRenderResult{ScopedResult: result}, nil
	}

	// Fall back to legacy budget rendering
	status, err := renderBudgetIfConfigured(cmd, totalCost, currency)
	if err != nil {
		return nil, err
	}
	return &BudgetRenderResult{LegacyStatus: status}, nil
}

// checkBudgetExitFromResult evaluates whether the CLI should exit based on budget result.
// It handles both legacy and scoped budget results.
func checkBudgetExitFromResult(cmd *cobra.Command, result *BudgetRenderResult, evalErr error) error {
	if result == nil {
		return checkBudgetExit(cmd, nil, evalErr)
	}

	if result.LegacyStatus != nil {
		return checkBudgetExit(cmd, result.LegacyStatus, evalErr)
	}

	// For scoped budgets, check if any scope is critical/exceeded
	if result.ScopedResult != nil && result.ScopedResult.HasCriticalBudgets() {
		return checkScopedBudgetExit(cmd, result.ScopedResult)
	}

	return nil
}

// checkScopedBudgetExit checks whether any critical/exceeded scoped budget should trigger a non-zero exit.
func checkScopedBudgetExit(cmd *cobra.Command, scopedResult *engine.ScopedBudgetResult) error {
	isDebug := cmd.Flag("debug") != nil && cmd.Flag("debug").Changed
	reason := fmt.Sprintf("budget exceeded: %d critical scope(s)", len(scopedResult.CriticalScopes))

	if isDebug {
		cmd.PrintErrf("DEBUG: %s: %v\n", reason, scopedResult.CriticalScopes)
	}

	cfg := config.GetGlobalConfig()
	if cfg == nil {
		return nil
	}

	budgetsCfg := cfg.Cost.Budgets
	if budgetsCfg == nil {
		return nil
	}

	for _, status := range scopedResult.AllScopes() {
		if status == nil {
			continue
		}
		if status.Health != pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL &&
			status.Health != pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED {
			continue
		}
		if budgetsCfg.GetEffectiveExitOnThreshold(status.Budget.ShouldExitOnThreshold()) {
			exitCode := budgetsCfg.GetEffectiveExitCode(status.Budget.GetExitCode())
			return &BudgetExitError{
				ExitCode: exitCode,
				Reason:   reason,
			}
		}
	}

	return nil
}

// renderScopedBudgetIfConfigured renders scoped budget status if configured.
// It uses the hierarchical budget configuration with provider, tag, and type scopes.
// The scopeFilter parameter controls which scopes are displayed.
//
// This function is used when scoped budgets (provider/tag/type) are configured.
// For global-only budgets, use renderBudgetIfConfigured instead.
//
// Returns the ScopedBudgetResult for exit code evaluation, or nil if no budgets configured.
func renderScopedBudgetIfConfigured(
	cmd *cobra.Command,
	costs []engine.CostResult,
	scopeFilter string,
) (*engine.ScopedBudgetResult, error) {
	cfg := config.GetGlobalConfig()
	if cfg == nil {
		return nil, nil //nolint:nilnil // intentionally returns nil,nil when no config
	}

	// Get budgets configuration
	budgetsCfg := cfg.Cost.Budgets
	if budgetsCfg == nil || !budgetsCfg.IsEnabled() {
		return nil, nil //nolint:nilnil // intentionally returns nil,nil when no budget configured
	}

	// Check if we have scoped budgets (provider/tag/type)
	if !budgetsCfg.HasScopedBudgets() {
		// Fall back to legacy rendering for global-only budgets
		return nil, nil //nolint:nilnil // use legacy renderBudgetIfConfigured instead
	}

	// Create scoped budget evaluator
	eval := engine.NewScopedBudgetEvaluator(budgetsCfg)

	// Allocate costs and evaluate all scopes
	result := evaluateScopedBudgets(cmd.Context(), eval, budgetsCfg, costs)

	// Add a blank line before budget status
	cmd.Println()

	// Render the scoped budget status
	filter := NewBudgetScopeFilter(scopeFilter)
	if renderErr := RenderScopedBudgetStatus(cmd.OutOrStdout(), result, filter); renderErr != nil {
		return result, renderErr
	}

	return result, nil
}

// evaluateScopedBudgets allocates costs to scopes and calculates budget statuses.
func evaluateScopedBudgets(
	ctx context.Context,
	eval *engine.ScopedBudgetEvaluator,
	cfg *config.BudgetsConfig,
	costs []engine.CostResult,
) *engine.ScopedBudgetResult {
	result := &engine.ScopedBudgetResult{
		ByProvider: make(map[string]*engine.ScopedBudgetStatus),
		ByType:     make(map[string]*engine.ScopedBudgetStatus),
	}

	// Track spend per scope
	globalSpend := 0.0
	providerSpend := make(map[string]float64)
	tagSpend := make(map[string]float64)
	typeSpend := make(map[string]float64)

	// Allocate each cost result to appropriate scopes
	for _, cost := range costs {
		// Check for context cancellation to support graceful shutdown
		if err := ctx.Err(); err != nil {
			return result // Return partial result on cancellation
		}

		// All costs count toward global
		globalSpend += cost.Monthly

		// Allocate to provider
		allocation := eval.AllocateCostToProvider(ctx, cost.ResourceType, cost.Monthly)
		if allocation.Provider != "" {
			providerSpend[allocation.Provider] += cost.Monthly
		}

		// NOTE: Tag budget allocation is not implemented here because CostResult
		// does not carry tag information. Tag-based budgets require tags to be
		// passed through from the original ResourceDescriptor.
		// Future enhancement: Add Tags field to CostResult or pass resources alongside costs.
		_ = tagSpend // silence unused warning until tag budgets are fully implemented

		// Allocate to type (if type budget exists)
		if eval.GetTypeBudget(cost.ResourceType) != nil {
			typeSpend[cost.ResourceType] += cost.Monthly
		}
	}

	// Calculate global status
	if cfg.Global != nil {
		result.Global = engine.CalculateProviderBudgetStatus("", cfg.Global, globalSpend)
		result.Global.ScopeType = engine.ScopeTypeGlobal
		result.Global.ScopeKey = ""
	}

	// Calculate provider statuses (skip nil budgets)
	for provider, budget := range cfg.Providers {
		if budget == nil {
			continue
		}
		spend := providerSpend[provider]
		status := engine.CalculateProviderBudgetStatus(provider, budget, spend)
		result.ByProvider[provider] = status
	}

	// Calculate tag statuses
	for _, tagBudget := range cfg.Tags {
		if tagBudget.IsDisabled() {
			continue
		}
		spend := tagSpend[tagBudget.Selector]
		status := engine.CalculateTagBudgetStatus(&tagBudget, spend)
		result.ByTag = append(result.ByTag, status)
	}

	// Calculate type statuses (skip nil budgets)
	for resourceType, budget := range cfg.Types {
		if budget == nil {
			continue
		}
		spend := typeSpend[resourceType]
		status := engine.CalculateProviderBudgetStatus(resourceType, budget, spend)
		status.ScopeType = engine.ScopeTypeType
		status.ScopeKey = resourceType
		result.ByType[resourceType] = status
	}

	// Calculate overall health (worst wins)
	healthStatuses := collectHealthStatuses(result)
	result.OverallHealth = engine.AggregateHealthStatuses(healthStatuses)

	// Identify critical scopes
	result.CriticalScopes = identifyCriticalScopes(result)

	return result
}

// collectHealthStatuses gathers all health statuses from a scoped budget result.
func collectHealthStatuses(result *engine.ScopedBudgetResult) []pbc.BudgetHealthStatus {
	var statuses []pbc.BudgetHealthStatus

	if result.Global != nil {
		statuses = append(statuses, result.Global.Health)
	}

	for _, status := range result.ByProvider {
		statuses = append(statuses, status.Health)
	}

	for _, status := range result.ByTag {
		statuses = append(statuses, status.Health)
	}

	for _, status := range result.ByType {
		statuses = append(statuses, status.Health)
	}

	return statuses
}

// identifyCriticalScopes returns scope identifiers with CRITICAL or EXCEEDED status.
func identifyCriticalScopes(result *engine.ScopedBudgetResult) []string {
	var critical []string

	isCritical := func(health pbc.BudgetHealthStatus) bool {
		return health == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL ||
			health == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED
	}

	if result.Global != nil && isCritical(result.Global.Health) {
		critical = append(critical, "global")
	}

	for key, status := range result.ByProvider {
		if isCritical(status.Health) {
			critical = append(critical, "provider:"+key)
		}
	}

	for _, status := range result.ByTag {
		if isCritical(status.Health) {
			critical = append(critical, "tag:"+status.ScopeKey)
		}
	}

	for key, status := range result.ByType {
		if isCritical(status.Health) {
			critical = append(critical, "type:"+key)
		}
	}

	return critical
}

// BudgetExitError is a sentinel error that carries an exit code for budget threshold violations.
// It is used to communicate the exit code from budget evaluation to the CLI layer.
type BudgetExitError struct {
	ExitCode int
	Reason   string
}

func (e *BudgetExitError) Error() string {
	return e.Reason
}

// checkBudgetExit evaluates whether the CLI should exit based on budget status.
// It returns a BudgetExitError with the appropriate exit code if a threshold was exceeded
// and exit_on_threshold is enabled, or nil if no exit is needed.
//
// For budget evaluation errors (FR-009), it returns a BudgetExitError with code 1.
func checkBudgetExit(cmd *cobra.Command, status *engine.BudgetStatus, evalErr error) error {
	isDebug := cmd.Flag("debug") != nil && cmd.Flag("debug").Changed

	// FR-009: Budget evaluation errors return exit code 1
	if evalErr != nil {
		// Log the error for debugging
		if isDebug {
			cmd.PrintErrf("DEBUG: budget evaluation error: %v\n", evalErr)
		}
		return &BudgetExitError{
			ExitCode: engine.ExitCodeBudgetEvaluationError,
			Reason:   fmt.Sprintf("budget evaluation failed: %v", evalErr),
		}
	}

	// No status means no budget configured - no exit needed
	if status == nil {
		return nil
	}

	// Check if we should exit based on threshold violation
	if status.ShouldExit() {
		exitCode := status.GetExitCode()
		reason := status.ExitReason()

		// Log exit reason when debug is enabled
		if isDebug {
			cmd.PrintErrf("DEBUG: %s\n", reason)
		}

		// Warning-only mode: exit code 0 means log warning but don't fail
		if exitCode == 0 {
			cmd.PrintErrf("WARNING: %s\n", reason)
			return nil
		}

		return &BudgetExitError{
			ExitCode: exitCode,
			Reason:   reason,
		}
	}

	return nil
}
