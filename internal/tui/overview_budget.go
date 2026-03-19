package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
)

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

// renderBudgetFooter renders the budget health footer for the list view, showing a
// color-coded health badge with aggregated spend/limit and utilization percentage.
// Returns an empty string when budgets are not loaded or unavailable.
func renderBudgetFooter(m OverviewModel) string {
	if !m.budgetLoaded || m.budgetResult == nil || len(m.budgetResult.Budgets) == 0 {
		return ""
	}

	summary := m.budgetResult.Summary
	overallHealth := pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK
	if summary != nil {
		overallHealth = summary.OverallHealth
	}

	badge := healthBadgeStyle(overallHealth).Render(engine.HealthStatusLabel(overallHealth))

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

// renderDetailBudgetStatus renders the "BUDGET STATUS" section for the detail view,
// showing per-budget breakdown with health badge, name, limit, spend, forecasted spend,
// utilization, and triggered threshold alerts. Returns an empty string when unavailable.
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
		badge := healthBadgeStyle(health).Render(engine.HealthStatusLabel(health))
		name := b.GetName()
		if name == "" {
			name = b.GetId()
		}

		fmt.Fprintf(&content, "  %s  %s\n", badge, LabelStyle.Render(name))
		fmt.Fprintf(&content, "    Limit:       %s\n",
			ValueStyle.Render(engine.FormatOverviewCurrency(amount.GetLimit())))
		fmt.Fprintf(&content, "    Spend:       %s\n",
			ValueStyle.Render(engine.FormatOverviewCurrency(status.GetCurrentSpend())))

		if status.GetForecastedSpend() > 0 {
			fmt.Fprintf(&content, "    Forecasted:  %s\n",
				ValueStyle.Render(engine.FormatOverviewCurrency(status.GetForecastedSpend())))
		}

		fmt.Fprintf(&content, "    Utilization: %s\n",
			ValueStyle.Render(fmt.Sprintf("%.1f%%", status.GetPercentageUsed())))

		// Show triggered threshold alerts.
		for _, threshold := range b.GetThresholds() {
			if threshold.GetTriggered() {
				alertStyle := WarningStyle
				if threshold.GetPercentage() >= 100 { //nolint:mnd // 100% threshold.
					alertStyle = CriticalStyle
				}
				fmt.Fprintf(&content, "    %s %.0f%% threshold triggered (%s)\n",
				alertStyle.Render("ALERT:"),
				threshold.GetPercentage(),
				threshold.GetType().String(),
			)
			}
		}

		content.WriteString("\n")
	}

	return content.String()
}
