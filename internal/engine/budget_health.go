package engine

import (
	"context"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
)

// Health threshold constants define the boundaries for budget health status.
// These thresholds match the proto enum semantics for BudgetHealthStatus.
const (
	// HealthThresholdWarning is the utilization percentage at which a budget transitions to WARNING.
	// Budgets at or above 80% utilization but below 90% are considered WARNING.
	HealthThresholdWarning = 80.0

	// HealthThresholdCritical is the utilization percentage at which a budget transitions to CRITICAL.
	// Budgets at or above 90% utilization but below 100% are considered CRITICAL.
	HealthThresholdCritical = 90.0

	// HealthThresholdExceeded is the utilization percentage at which a budget transitions to EXCEEDED.
	// Budgets at or above 100% utilization are considered EXCEEDED.
	HealthThresholdExceeded = 100.0
)

const (
	severityExceeded    = 4
	severityCritical    = 3
	severityWarning     = 2
	severityOK          = 1
	severityUnspecified = 0
)

// healthSeverityMap maps BudgetHealthStatus to numeric severity for comparison.
// Higher values indicate worse health (more critical).
var healthSeverityMap = map[pbc.BudgetHealthStatus]int{ //nolint:gochecknoglobals // Constant lookup table
	pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED:    severityExceeded,
	pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL:    severityCritical,
	pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING:     severityWarning,
	pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK:          severityOK,
	pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED: severityUnspecified,
}

// CalculateBudgetHealthFromPercentage calculates health status from a raw utilization percentage.
//
// Thresholds:
//   - OK: 0-79%
//   - WARNING: 80-89%
//   - CRITICAL: 90-99%
//   - EXCEEDED: 100%+
//
// Negative percentages are treated as 0% (OK status).
func CalculateBudgetHealthFromPercentage(percentageUsed float64) pbc.BudgetHealthStatus {
	switch {
	case percentageUsed >= HealthThresholdExceeded:
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED
	case percentageUsed >= HealthThresholdCritical:
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL
	case percentageUsed >= HealthThresholdWarning:
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING
	default:
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK
	}
}

// CalculateBudgetHealth determines the health status of a single budget
// based on its current utilization percentage.
//
// Thresholds:
//   - OK: 0-79%
//   - WARNING: 80-89%
//   - CRITICAL: 90-99%
//   - EXCEEDED: 100%+
//
// Returns UNSPECIFIED if budget has no status or invalid data.
func CalculateBudgetHealth(budget *pbc.Budget) pbc.BudgetHealthStatus {
	if budget == nil {
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED
	}

	status := budget.GetStatus()
	if status == nil {
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED
	}

	// If the status already has a health value set (not UNSPECIFIED), use it
	if status.GetHealth() != pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED {
		return status.GetHealth()
	}

	// Calculate health from percentage used
	return CalculateBudgetHealthFromPercentage(status.GetPercentageUsed())
}

// AggregateHealth returns the worst-case health status across all budgets.
// If budgets is empty, returns UNSPECIFIED.
//
// The aggregation uses "worst wins" logic:
//   - EXCEEDED > CRITICAL > WARNING > OK > UNSPECIFIED
func AggregateHealth(budgets []*pbc.Budget) pbc.BudgetHealthStatus {
	if len(budgets) == 0 {
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED
	}

	worst := pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED

	for _, budget := range budgets {
		health := CalculateBudgetHealth(budget)
		if healthSeverity(health) > healthSeverity(worst) {
			worst = health
		}
	}

	return worst
}

// healthSeverity returns a numeric severity for a health status.
// Higher values indicate worse health (more critical).
func healthSeverity(health pbc.BudgetHealthStatus) int {
	if severity, ok := healthSeverityMap[health]; ok {
		return severity
	}
	return severityUnspecified
}

// CalculateBudgetHealthResults calculates health results for multiple budgets.
// It returns a BudgetHealthResult for each budget with all relevant health information.
func CalculateBudgetHealthResults(ctx context.Context, budgets []*pbc.Budget) []BudgetHealthResult {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "CalculateBudgetHealthResults").
		Logger()

	results := make([]BudgetHealthResult, 0, len(budgets))

	for _, budget := range budgets {
		if budget == nil {
			continue
		}

		health := CalculateBudgetHealth(budget)
		result := BudgetHealthResult{
			BudgetID:   budget.GetId(),
			BudgetName: budget.GetName(),
			Provider:   budget.GetSource(),
			Health:     health,
		}

		// Extract amount information
		if amount := budget.GetAmount(); amount != nil {
			result.Currency = amount.GetCurrency()
			result.Limit = amount.GetLimit()
		}

		// Extract status information
		if status := budget.GetStatus(); status != nil {
			result.Utilization = status.GetPercentageUsed()
			result.Forecasted = status.GetPercentageForecasted()
			result.CurrentSpend = status.GetCurrentSpend()
		}

		logger.Debug().
			Str("budget_id", result.BudgetID).
			Str("health", result.Health.String()).
			Float64("utilization", result.Utilization).
			Msg("calculated budget health")

		results = append(results, result)
	}

	return results
}
