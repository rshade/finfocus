package integration_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

// TestBudgetConfig_YAMLIntegration tests loading budget config from YAML.
func TestBudgetConfig_YAMLIntegration(t *testing.T) {
	configContent := `
amount: 1000.0
currency: USD
period: monthly
alerts:
  - threshold: 50
    type: actual
  - threshold: 80
    type: actual
  - threshold: 100
    type: forecasted
`
	var budgetCfg config.BudgetConfig
	err := yaml.Unmarshal([]byte(configContent), &budgetCfg)
	require.NoError(t, err)

	// Verify budget config
	assert.True(t, budgetCfg.IsEnabled())
	assert.Equal(t, 1000.0, budgetCfg.Amount)
	assert.Equal(t, "USD", budgetCfg.Currency)
	assert.Equal(t, "monthly", budgetCfg.Period)
	assert.Len(t, budgetCfg.Alerts, 3)

	// Verify alert types
	actualAlerts := budgetCfg.GetActualAlerts()
	assert.Len(t, actualAlerts, 2)

	forecastedAlerts := budgetCfg.GetForecastedAlerts()
	assert.Len(t, forecastedAlerts, 1)
}

// TestBudgetConfig_CostConfigYAMLIntegration tests full CostConfig YAML parsing.
func TestBudgetConfig_CostConfigYAMLIntegration(t *testing.T) {
	configContent := `
budgets:
  amount: 1000.0
  currency: USD
  period: monthly
  alerts:
    - threshold: 50
      type: actual
    - threshold: 80
      type: actual
`
	var costCfg config.CostConfig
	err := yaml.Unmarshal([]byte(configContent), &costCfg)
	require.NoError(t, err)

	// Verify cost config
	assert.True(t, costCfg.HasBudget())
	assert.Equal(t, 1000.0, costCfg.Budgets.Amount)
	assert.Equal(t, "USD", costCfg.Budgets.Currency)
}

// TestBudgetEngine_EvaluationIntegration tests full budget evaluation workflow.
func TestBudgetEngine_EvaluationIntegration(t *testing.T) {
	tests := []struct {
		name           string
		budget         config.BudgetConfig
		currentSpend   float64
		wantPercentage float64
		wantExceeded   bool
		wantAlertCount int
	}{
		{
			name: "within budget - no alerts triggered",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "monthly",
				Alerts: []config.AlertConfig{
					{Threshold: 80, Type: config.AlertTypeActual},
				},
			},
			currentSpend:   500.0,
			wantPercentage: 50.0,
			wantExceeded:   false,
			wantAlertCount: 1, // One alert configured, but status is OK
		},
		{
			name: "over threshold - alert exceeded",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "monthly",
				Alerts: []config.AlertConfig{
					{Threshold: 80, Type: config.AlertTypeActual},
				},
			},
			currentSpend:   850.0,
			wantPercentage: 85.0,
			wantExceeded:   true,
			wantAlertCount: 1,
		},
		{
			name: "multiple thresholds",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "monthly",
				Alerts: []config.AlertConfig{
					{Threshold: 50, Type: config.AlertTypeActual},
					{Threshold: 80, Type: config.AlertTypeActual},
					{Threshold: 100, Type: config.AlertTypeActual},
				},
			},
			currentSpend:   900.0,
			wantPercentage: 90.0,
			wantExceeded:   true,
			wantAlertCount: 3,
		},
	}

	budgetEngine := engine.NewBudgetEngine()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, err := budgetEngine.Evaluate(tc.budget, tc.currentSpend, "USD")
			require.NoError(t, err)
			require.NotNil(t, status)

			assert.Equal(t, tc.wantPercentage, status.Percentage)
			assert.Equal(t, tc.wantExceeded, status.HasExceededAlerts())
			assert.Len(t, status.Alerts, tc.wantAlertCount)
		})
	}
}

// TestBudgetEngine_ForecastingIntegration tests forecasting logic with controlled time.
func TestBudgetEngine_ForecastingIntegration(t *testing.T) {
	// Fixed time: January 15th (day 15 of 31 days)
	fixedTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	budgetEngine := engine.NewBudgetEngineWithTime(func() time.Time { return fixedTime })

	budget := config.BudgetConfig{
		Amount:   1000.0,
		Currency: "USD",
		Period:   "monthly",
		Alerts: []config.AlertConfig{
			{Threshold: 100, Type: config.AlertTypeForecasted},
		},
	}

	tests := []struct {
		name                string
		currentSpend        float64
		wantForecastAbove   float64
		wantForecastBelow   float64
		wantForecastExceeds bool
	}{
		{
			name:                "low spend - forecast under budget",
			currentSpend:        300.0, // 300/15 * 31 = 620 forecast
			wantForecastAbove:   600.0,
			wantForecastBelow:   650.0,
			wantForecastExceeds: false,
		},
		{
			name:                "high spend - forecast over budget",
			currentSpend:        600.0, // 600/15 * 31 = 1240 forecast
			wantForecastAbove:   1200.0,
			wantForecastBelow:   1300.0,
			wantForecastExceeds: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, err := budgetEngine.Evaluate(budget, tc.currentSpend, "USD")
			require.NoError(t, err)

			assert.Greater(t, status.ForecastedSpend, tc.wantForecastAbove)
			assert.Less(t, status.ForecastedSpend, tc.wantForecastBelow)
			assert.Equal(t, tc.wantForecastExceeds, status.IsForecastOverBudget())
		})
	}
}

// TestBudgetRendering_PlainText tests plain text budget rendering for CI/CD.
func TestBudgetRendering_PlainText(t *testing.T) {
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:   1000.0,
			Currency: "USD",
			Period:   "monthly",
		},
		CurrentSpend: 850.0,
		Percentage:   85.0,
		Currency:     "USD",
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Type: config.AlertTypeActual, Status: engine.ThresholdStatusExceeded},
		},
	}

	var buf bytes.Buffer
	err := cli.RenderBudgetStatus(&buf, status)
	require.NoError(t, err)

	output := buf.String()

	// Plain text output (buffer is not a TTY)
	assert.Contains(t, output, "BUDGET STATUS")
	assert.Contains(t, output, "Budget: $1000.00/monthly")
	assert.Contains(t, output, "Current Spend: $850.00")
	assert.Contains(t, output, "85.0%")
	assert.Contains(t, output, "WARNING")
}

// TestBudgetRendering_WithForecasting tests budget rendering includes forecast.
func TestBudgetRendering_WithForecasting(t *testing.T) {
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:   1000.0,
			Currency: "USD",
			Period:   "monthly",
		},
		CurrentSpend:       450.0,
		Percentage:         45.0,
		ForecastedSpend:    930.0,
		ForecastPercentage: 93.0,
		Currency:           "USD",
	}

	var buf bytes.Buffer
	err := cli.RenderBudgetStatus(&buf, status)
	require.NoError(t, err)

	output := buf.String()

	// Verify forecast is displayed
	assert.Contains(t, output, "Forecasted:")
	assert.Contains(t, output, "930.00")
	assert.Contains(t, output, "93.0%")
}

// TestBudgetRendering_MultipleCurrencies tests different currency symbols.
func TestBudgetRendering_MultipleCurrencies(t *testing.T) {
	tests := []struct {
		currency       string
		expectedSymbol string
	}{
		{"USD", "$"},
		{"EUR", "€"},
		{"GBP", "£"},
		{"JPY", "¥"},
	}

	for _, tc := range tests {
		t.Run(tc.currency, func(t *testing.T) {
			status := &engine.BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:   1000.0,
					Currency: tc.currency,
					Period:   "monthly",
				},
				CurrentSpend: 500.0,
				Percentage:   50.0,
				Currency:     tc.currency,
			}

			var buf bytes.Buffer
			err := cli.RenderBudgetStatus(&buf, status)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, tc.expectedSymbol)
		})
	}
}

// TestBudgetRendering_ApproachingThreshold tests "APPROACHING" status display.
func TestBudgetRendering_ApproachingThreshold(t *testing.T) {
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:   1000.0,
			Currency: "USD",
			Period:   "monthly",
		},
		CurrentSpend: 770.0,
		Percentage:   77.0,
		Currency:     "USD",
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Type: config.AlertTypeActual, Status: engine.ThresholdStatusApproaching},
		},
	}

	var buf bytes.Buffer
	err := cli.RenderBudgetStatus(&buf, status)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "APPROACHING")
}

// TestBudgetRendering_OverBudget tests over-budget display (>100%).
func TestBudgetRendering_OverBudget(t *testing.T) {
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:   1000.0,
			Currency: "USD",
			Period:   "monthly",
		},
		CurrentSpend: 1200.0,
		Percentage:   120.0,
		Currency:     "USD",
		Alerts: []engine.ThresholdStatus{
			{Threshold: 100.0, Type: config.AlertTypeActual, Status: engine.ThresholdStatusExceeded},
		},
	}

	var buf bytes.Buffer
	err := cli.RenderBudgetStatus(&buf, status)
	require.NoError(t, err)

	output := buf.String()

	// Verify over-budget handling
	assert.Contains(t, output, "1200.00")
	assert.Contains(t, output, "120.0%")
	assert.Contains(t, output, "WARNING")

	// Progress bar should be capped at 100%
	cappedPercent := status.CappedPercentage()
	assert.Equal(t, 100.0, cappedPercent)
}

// TestBudgetRendering_NilStatus tests nil status handling.
func TestBudgetRendering_NilStatus(t *testing.T) {
	var buf bytes.Buffer
	err := cli.RenderBudgetStatus(&buf, nil)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

// TestBudgetConfig_ValidationIntegration tests budget configuration validation.
func TestBudgetConfig_ValidationIntegration(t *testing.T) {
	tests := []struct {
		name    string
		config  config.BudgetConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "monthly",
				Alerts: []config.AlertConfig{
					{Threshold: 80, Type: config.AlertTypeActual},
				},
			},
			wantErr: false,
		},
		{
			name: "disabled budget (zero amount) - valid",
			config: config.BudgetConfig{
				Amount:   0,
				Currency: "USD",
			},
			wantErr: false, // Zero amount is valid (disabled budget)
		},
		{
			name: "negative amount - invalid",
			config: config.BudgetConfig{
				Amount:   -100.0,
				Currency: "USD",
			},
			wantErr: true,
		},
		{
			name: "extreme overspend threshold (150%) - valid",
			config: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: 150, Type: config.AlertTypeActual}, // >100% allowed for extreme overspend detection
				},
			},
			wantErr: false, // Thresholds up to 1000% are allowed
		},
		{
			name: "threshold at boundary (0%) - valid edge case",
			config: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: 0, Type: config.AlertTypeActual}, // Edge case: alert at 0%
				},
			},
			wantErr: false, // 0% is valid (alerts immediately on any spend)
		},
		{
			name: "threshold above max (>1000%) - invalid",
			config: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: 1001, Type: config.AlertTypeActual}, // Exceeds max 1000%
				},
			},
			wantErr: true,
		},
		{
			name: "negative threshold - invalid",
			config: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: -10, Type: config.AlertTypeActual},
				},
			},
			wantErr: true,
		},
		{
			name: "missing currency with enabled budget - invalid",
			config: config.BudgetConfig{
				Amount: 1000.0,
				// Currency missing
			},
			wantErr: true,
		},
		{
			name: "invalid alert type - invalid",
			config: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: 80, Type: "invalid"},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestBudgetEngine_ErrorHandling tests error cases in budget evaluation.
func TestBudgetEngine_ErrorHandling(t *testing.T) {
	budgetEngine := engine.NewBudgetEngine()

	tests := []struct {
		name        string
		budget      config.BudgetConfig
		spend       float64
		currency    string
		errContains string
	}{
		{
			name: "currency mismatch",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
			},
			spend:       500.0,
			currency:    "EUR",
			errContains: "currency mismatch",
		},
		{
			name: "disabled budget",
			budget: config.BudgetConfig{
				Amount:   0,
				Currency: "USD",
			},
			spend:       500.0,
			currency:    "USD",
			errContains: "budget is disabled",
		},
		{
			name: "negative spend",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
			},
			spend:       -100.0,
			currency:    "USD",
			errContains: "negative spend",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := budgetEngine.Evaluate(tc.budget, tc.spend, tc.currency)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}
