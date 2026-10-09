package viewmodel

import (
	"fmt"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/engine"
)

// BudgetThresholdDisplay contains safe alert presentation with no notification destinations.
type BudgetThresholdDisplay struct {
	Text     string `json:"text"`
	Critical bool   `json:"critical"`
}

// BudgetDetailDisplay is the safe, formatted subset of a budget used in both UIs.
type BudgetDetailDisplay struct {
	Name                string                   `json:"name"`
	Health              pbc.BudgetHealthStatus   `json:"-"`
	HealthDisplay       string                   `json:"healthDisplay"`
	LimitDisplay        string                   `json:"limitDisplay"`
	CurrentSpendDisplay string                   `json:"currentSpendDisplay"`
	ForecastedDisplay   string                   `json:"forecastedDisplay"`
	UtilizationDisplay  string                   `json:"utilizationDisplay"`
	Thresholds          []BudgetThresholdDisplay `json:"thresholds"`
}

// BudgetDisplay shares footer aggregation and safe detail formatting with the TUI.
type BudgetDisplay struct {
	Health         pbc.BudgetHealthStatus `json:"-"`
	HealthDisplay  string                 `json:"healthDisplay"`
	AmountsDisplay string                 `json:"amountsDisplay"`
	FooterDisplay  string                 `json:"footerDisplay"`
	Details        []BudgetDetailDisplay  `json:"details"`
}

// BuildBudgetDisplay projects safe numerical and status fields, excluding notification configuration.
func BuildBudgetDisplay(result *engine.BudgetResult) BudgetDisplay {
	display := BudgetDisplay{Details: []BudgetDetailDisplay{}}
	if result == nil || len(result.Budgets) == 0 {
		return display
	}
	display.Health = pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK
	if result.Summary != nil {
		display.Health = result.Summary.OverallHealth
	}
	display.HealthDisplay = engine.HealthStatusLabel(display.Health)
	var spend, limit float64
	for _, budget := range result.Budgets {
		amount, status := budget.GetAmount(), budget.GetStatus()
		if amount == nil || status == nil {
			continue
		}
		if amount.GetLimit() > 0 {
			spend += status.GetCurrentSpend()
			limit += amount.GetLimit()
		}
		detail := budgetDetailDisplay(budget)
		display.Details = append(display.Details, detail)
	}
	mixed := result.Summary != nil && len(result.Summary.ByCurrency) > 1
	if !mixed && limit > 0 {
		display.AmountsDisplay = fmt.Sprintf(
			"%s / %s (%.0f%%)",
			engine.FormatOverviewCurrency(spend),
			engine.FormatOverviewCurrency(limit),
			spend/limit*percentScale,
		)
	}
	display.FooterDisplay = "Budget: " + display.HealthDisplay
	if display.AmountsDisplay != "" {
		display.FooterDisplay += " " + display.AmountsDisplay
	}
	return display
}

func budgetDetailDisplay(budget *pbc.Budget) BudgetDetailDisplay {
	amount, status := budget.GetAmount(), budget.GetStatus()
	name := budget.GetName()
	if name == "" {
		name = budget.GetId()
	}
	detail := BudgetDetailDisplay{
		Name:                name,
		Health:              status.GetHealth(),
		HealthDisplay:       engine.HealthStatusLabel(status.GetHealth()),
		LimitDisplay:        engine.FormatOverviewCurrency(amount.GetLimit()),
		CurrentSpendDisplay: engine.FormatOverviewCurrency(status.GetCurrentSpend()),
		UtilizationDisplay:  fmt.Sprintf("%.1f%%", status.GetPercentageUsed()),
		Thresholds:          []BudgetThresholdDisplay{},
	}
	if status.GetForecastedSpend() > 0 {
		detail.ForecastedDisplay = engine.FormatOverviewCurrency(status.GetForecastedSpend())
	}
	for _, threshold := range budget.GetThresholds() {
		if threshold.GetTriggered() {
			detail.Thresholds = append(
				detail.Thresholds,
				BudgetThresholdDisplay{
					Text: fmt.Sprintf(
						"%.0f%% threshold triggered (%s)",
						threshold.GetPercentage(),
						threshold.GetType().String(),
					),
					Critical: threshold.GetPercentage() >= percentScale,
				},
			)
		}
	}
	return detail
}
