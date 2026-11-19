package history

import (
	"cmp"
	"strings"

	"github.com/rshade/finfocus/internal/config"
)

// ChartBudget is a monthly threshold for the total or a named provider.
type ChartBudget struct {
	Provider string  `json:"provider,omitempty"`
	Amount   float64 `json:"amount"`
	Display  string  `json:"display"`
}

// ChartBudgets returns enabled monthly thresholds in the selected history currency.
func ChartBudgets(budgets *config.BudgetsConfig, snapshots []CostSnapshot) []ChartBudget {
	if budgets == nil || len(snapshots) == 0 {
		return nil
	}
	currency := snapshots[0].Currency
	if currency == "" {
		currency = defaultCurrency
	}
	matches := func(budget *config.ScopedBudget, fallback string) bool {
		if budget == nil || !budget.IsEnabled() || budget.GetPeriod() != config.DefaultBudgetPeriod {
			return false
		}
		actual := cmp.Or(budget.Currency, fallback, defaultCurrency)
		return strings.EqualFold(actual, currency)
	}
	out := []ChartBudget{}
	add := func(provider string, amount float64) {
		out = append(out, ChartBudget{Provider: provider, Amount: amount, Display: FormatChartMoney(currency, amount)})
	}
	if matches(budgets.Global, "") {
		add("", budgets.Global.Amount)
	}
	providers := config.ProviderBudgetIndex(budgets.Providers)
	for _, name := range ProviderNames(snapshots) {
		if budget := providers[name]; matches(budget, budgets.GetGlobalCurrency()) {
			add(name, budget.Amount)
		}
	}
	return out
}
