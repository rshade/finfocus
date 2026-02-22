package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
)

// healthBadgeLabel maps a BudgetHealthStatus to its human-readable label such as "OK", "WARNING", "CRITICAL", "EXCEEDED", or "UNKNOWN".
func healthBadgeLabel(health pbc.BudgetHealthStatus) string {
	switch health {
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK:
		return "OK"
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING:
		return "WARNING"
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL:
		return "CRITICAL"
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED:
		return "EXCEEDED"
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED:
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}

// healthBadgeStyle returns the lipgloss style for a budget health status badge.
func healthBadgeStyle(health pbc.BudgetHealthStatus) lipgloss.Style {
	switch health {
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK:
		return OKStyle
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING:
		return WarningStyle
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED:
		return CriticalStyle
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED:
		return SubtleStyle
	default:
		return SubtleStyle
	}
}

// renderBudgetFooter renders the budget health footer for the list view.
// It displays a color-coded health badge with aggregated spend/limit
// information when budget data is available.
//
// renderBudgetFooter renders the footer for the budget list view, showing a colored health badge and, when possible, an aggregated spend/limit with utilization percentage.
//
// If budgets are not loaded, budgetResult is nil, or no budgets exist, it returns an empty string. If multiple currencies are present it returns only the badge prefixed with "Budget: ". When budgets share a single currency it sums current spend and limits across budgets (skipping budgets with nil amount/status or with limit <= 0); if the aggregated limit is <= 0 it returns only the badge. Otherwise it returns a string of the form `Budget: <badge> <spent> / <limit> (X%)` where the amounts are formatted for overview display and the percentage is totalSpend/totalLimit rounded to the nearest integer.
func renderBudgetFooter(m OverviewModel) string {
	if !m.budgetLoaded || m.budgetResult == nil || len(m.budgetResult.Budgets) == 0 {
		return ""
	}

	summary := m.budgetResult.Summary
	overallHealth := pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK
	if summary != nil {
		overallHealth = summary.OverallHealth
	}

	badge := healthBadgeStyle(overallHealth).Render(healthBadgeLabel(overallHealth))

	// Check if we have mixed currencies.
	isMixed := summary != nil && len(summary.ByCurrency) > 1

	if isMixed {
		// Mixed currencies: show badge and status label only, no dollar amounts.
		return fmt.Sprintf("Budget: %s", badge)
	}

	// Same currency (or single budget): aggregate spend and limit.
	var totalSpend, totalLimit float64
	for _, b := range m.budgetResult.Budgets {
		amount := b.GetAmount()
		status := b.GetStatus()
		if amount == nil || status == nil {
			continue
		}
		// Exclude disabled budgets (Limit <= 0).
		if amount.GetLimit() <= 0 {
			continue
		}
		totalSpend += status.GetCurrentSpend()
		totalLimit += amount.GetLimit()
	}

	if totalLimit <= 0 {
		return fmt.Sprintf("Budget: %s", badge)
	}

	//nolint:mnd // Percentage calculation.
	pct := (totalSpend / totalLimit) * 100

	return fmt.Sprintf("Budget: %s %s / %s (%.0f%%)",
		badge,
		engine.FormatOverviewCurrency(totalSpend),
		engine.FormatOverviewCurrency(totalLimit),
		pct,
	)
}

// renderDetailBudgetStatus renders the "BUDGET STATUS" section for the detail
// view. It shows per-budget breakdown with name, limit, current spend,
// forecasted spend, utilization percentage, health badge, and triggered alerts.
//
// renderDetailBudgetStatus renders the "BUDGET STATUS" section for the detail view.
// If budget data is not loaded, unavailable, or contains no budgets, it returns an empty string.
// For each budget it appends a health badge, the budget name (or ID if name is empty), limit,
// current spend, an optional forecasted spend (only when > 0), utilization percentage, and any
// triggered threshold alerts. Currency values are formatted for the overview display. The
// returned string contains the assembled lines including the section header and spacing.
func renderDetailBudgetStatus(m OverviewModel) string {
	if !m.budgetLoaded || m.budgetResult == nil || len(m.budgetResult.Budgets) == 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString(HeaderStyle.Render("BUDGET STATUS"))
	content.WriteString("\n")

	for _, b := range m.budgetResult.Budgets {
		amount := b.GetAmount()
		status := b.GetStatus()
		if amount == nil || status == nil {
			continue
		}

		health := status.GetHealth()
		badge := healthBadgeStyle(health).Render(healthBadgeLabel(health))
		name := b.GetName()
		if name == "" {
			name = b.GetId()
		}

		content.WriteString(fmt.Sprintf("  %s  %s\n", badge, LabelStyle.Render(name)))
		content.WriteString(fmt.Sprintf("    Limit:       %s\n",
			ValueStyle.Render(engine.FormatOverviewCurrency(amount.GetLimit()))))
		content.WriteString(fmt.Sprintf("    Spend:       %s\n",
			ValueStyle.Render(engine.FormatOverviewCurrency(status.GetCurrentSpend()))))

		if status.GetForecastedSpend() > 0 {
			content.WriteString(fmt.Sprintf("    Forecasted:  %s\n",
				ValueStyle.Render(engine.FormatOverviewCurrency(status.GetForecastedSpend()))))
		}

		content.WriteString(fmt.Sprintf("    Utilization: %s\n",
			ValueStyle.Render(fmt.Sprintf("%.1f%%", status.GetPercentageUsed()))))

		// Show triggered threshold alerts.
		for _, threshold := range b.GetThresholds() {
			if threshold.GetTriggered() {
				alertStyle := WarningStyle
				if threshold.GetPercentage() >= 100 { //nolint:mnd // 100% threshold.
					alertStyle = CriticalStyle
				}
				content.WriteString(fmt.Sprintf("    %s %.0f%% threshold triggered (%s)\n",
					alertStyle.Render("ALERT:"),
					threshold.GetPercentage(),
					threshold.GetType().String(),
				))
			}
		}

		content.WriteString("\n")
	}

	return content.String()
}