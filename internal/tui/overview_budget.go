package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/viewmodel"
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

	display := viewmodel.BuildBudgetDisplay(m.budgetResult)
	footer := "Budget: " + healthBadgeStyle(display.Health).Render(display.HealthDisplay)
	if display.AmountsDisplay != "" {
		footer += " " + display.AmountsDisplay
	}
	return footer
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

	for _, budget := range viewmodel.BuildBudgetDisplay(m.budgetResult).Details {
		badge := healthBadgeStyle(budget.Health).Render(budget.HealthDisplay)
		fmt.Fprintf(&content, "  %s  %s\n", badge, LabelStyle.Render(budget.Name))
		fmt.Fprintf(&content, "    Limit:       %s\n", ValueStyle.Render(budget.LimitDisplay))
		fmt.Fprintf(&content, "    Spend:       %s\n", ValueStyle.Render(budget.CurrentSpendDisplay))
		if budget.ForecastedDisplay != "" {
			fmt.Fprintf(&content, "    Forecasted:  %s\n", ValueStyle.Render(budget.ForecastedDisplay))
		}
		fmt.Fprintf(&content, "    Utilization: %s\n", ValueStyle.Render(budget.UtilizationDisplay))
		for _, threshold := range budget.Thresholds {
			style := WarningStyle
			if threshold.Critical {
				style = CriticalStyle
			}
			fmt.Fprintf(&content, "    %s %s\n", style.Render("ALERT:"), threshold.Text)
		}
		content.WriteString("\n")
	}

	return content.String()
}
