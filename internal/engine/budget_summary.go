package engine

import (
	"context"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/logging"
)

// CalculateBudgetSummary computes a BudgetSummary for the given budgets.
// It sets TotalBudgets to the number of provided budgets and increments
// the appropriate health counters (BudgetsOk, BudgetsWarning, BudgetsCritical,
// BudgetsExceeded) based on each budget's status.Health. Budgets with a nil
// status or a health of UNSPECIFIED are logged and excluded from the health
// counts. It returns a pointer to the populated BudgetSummary.
func CalculateBudgetSummary(ctx context.Context, budgets []*pbc.Budget) *pbc.BudgetSummary {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "CalculateBudgetSummary").
		Logger()

	summary := &pbc.BudgetSummary{
		TotalBudgets: int32(len(budgets)), //nolint:gosec // length is bounded by practical budget limits
	}

	for _, b := range budgets {
		if b == nil {
			logger.Warn().Msg("nil budget skipped")
			continue
		}
		status := b.GetStatus()
		if status == nil || status.GetHealth() == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED {
			logger.Warn().Str("budget_id", b.GetId()).Msg("Budget missing health status, excluded from health metrics")
			continue
		}

		switch status.GetHealth() {
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED:
			// Already handled above, no-op for exhaustive switch
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK:
			summary.BudgetsOk++
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING:
			summary.BudgetsWarning++
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL:
			summary.BudgetsCritical++
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED:
			summary.BudgetsExceeded++
		}
	}

	return summary
}

// updateSummaryCount updates counts in a summary struct based on budget status.
func updateSummaryCount(s *pbc.BudgetSummary, b *pbc.Budget) {
	s.TotalBudgets++
	status := b.GetStatus()
	if status == nil {
		return
	}
	switch status.GetHealth() {
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK:
		s.BudgetsOk++
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING:
		s.BudgetsWarning++
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL:
		s.BudgetsCritical++
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED:
		s.BudgetsExceeded++
	case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED:
		// No-op
	}
}

// processBudgetForExtendedSummary updates the extended summary with data from a single budget.
func processBudgetForExtendedSummary(b *pbc.Budget, ext *ExtendedBudgetSummary) pbc.BudgetHealthStatus {
	// By Provider
	provider := b.GetSource()
	if _, exists := ext.ByProvider[provider]; !exists {
		ext.ByProvider[provider] = &pbc.BudgetSummary{}
	}
	updateSummaryCount(ext.ByProvider[provider], b)

	// By Currency
	if amt := b.GetAmount(); amt != nil {
		currency := amt.GetCurrency()
		if currency != "" {
			if _, exists := ext.ByCurrency[currency]; !exists {
				ext.ByCurrency[currency] = &pbc.BudgetSummary{}
			}
			updateSummaryCount(ext.ByCurrency[currency], b)
		}
	}

	// Return health for aggregation
	if status := b.GetStatus(); status != nil {
		health := status.GetHealth()
		if health == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL ||
			health == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED {
			ext.CriticalBudgets = append(ext.CriticalBudgets, b.GetId())
		}
		return health
	}
	return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED
}

// CalculateExtendedSummary provides detailed breakdown by provider and currency.
// Includes list of critical/exceeded budget IDs for immediate attention.
func CalculateExtendedSummary(ctx context.Context, budgets []*pbc.Budget) *ExtendedBudgetSummary {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "CalculateExtendedSummary").
		Logger()

	// Base summary
	baseSummary := CalculateBudgetSummary(ctx, budgets)

	extended := &ExtendedBudgetSummary{
		BudgetSummary:   baseSummary,
		ByProvider:      make(map[string]*pbc.BudgetSummary),
		ByCurrency:      make(map[string]*pbc.BudgetSummary),
		CriticalBudgets: []string{},
		OverallHealth:   pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
	}

	worstHealth := pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED

	for _, b := range budgets {
		if b == nil {
			continue
		}

		health := processBudgetForExtendedSummary(b, extended)
		if healthSeverity(health) > healthSeverity(worstHealth) {
			worstHealth = health
		}
	}

	extended.OverallHealth = worstHealth

	logger.Debug().
		Int("critical_count", len(extended.CriticalBudgets)).
		Int("provider_count", len(extended.ByProvider)).
		Int("currency_count", len(extended.ByCurrency)).
		Str("overall_health", extended.OverallHealth.String()).
		Msg("extended summary calculated")

	return extended
}
