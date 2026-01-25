package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestDefaultBudgetEngine_Evaluate(t *testing.T) {
	tests := []struct {
		name         string
		budget       config.BudgetConfig
		currentSpend float64
		currency     string
		wantErr      bool
		errContains  string
		check        func(t *testing.T, status *BudgetStatus)
	}{
		{
			name: "basic evaluation - 50% of budget",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "monthly",
			},
			currentSpend: 500.0,
			currency:     "USD",
			wantErr:      false,
			check: func(t *testing.T, status *BudgetStatus) {
				assert.Equal(t, 500.0, status.CurrentSpend)
				assert.Equal(t, 50.0, status.Percentage)
				assert.Equal(t, "USD", status.Currency)
			},
		},
		{
			name: "over budget - 150%",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
			},
			currentSpend: 1500.0,
			currency:     "USD",
			wantErr:      false,
			check: func(t *testing.T, status *BudgetStatus) {
				assert.Equal(t, 150.0, status.Percentage)
				assert.True(t, status.IsOverBudget())
				assert.Equal(t, 100.0, status.CappedPercentage())
			},
		},
		{
			name: "zero spend",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
			},
			currentSpend: 0.0,
			currency:     "USD",
			wantErr:      false,
			check: func(t *testing.T, status *BudgetStatus) {
				assert.Equal(t, 0.0, status.Percentage)
				assert.False(t, status.IsOverBudget())
			},
		},
		{
			name: "currency mismatch",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
			},
			currentSpend: 500.0,
			currency:     "EUR",
			wantErr:      true,
			errContains:  "currency mismatch",
		},
		{
			name: "disabled budget",
			budget: config.BudgetConfig{
				Amount:   0.0,
				Currency: "USD",
			},
			currentSpend: 500.0,
			currency:     "USD",
			wantErr:      true,
			errContains:  "budget is disabled",
		},
		{
			name: "negative spend",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
			},
			currentSpend: -100.0,
			currency:     "USD",
			wantErr:      true,
			errContains:  "negative spend not allowed",
		},
	}

	engine := NewBudgetEngine()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, err := engine.Evaluate(tc.budget, tc.currentSpend, tc.currency)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, status)
			if tc.check != nil {
				tc.check(t, status)
			}
		})
	}
}

func TestDefaultBudgetEngine_EvaluateAlerts(t *testing.T) {
	tests := []struct {
		name            string
		budget          config.BudgetConfig
		currentSpend    float64
		wantExceeded    []float64 // thresholds that should be exceeded
		wantApproaching []float64 // thresholds that should be approaching
	}{
		{
			name: "no alerts configured",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts:   []config.AlertConfig{},
			},
			currentSpend:    500.0,
			wantExceeded:    nil,
			wantApproaching: nil,
		},
		{
			name: "all alerts OK - 20% spend",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: 50.0, Type: config.AlertTypeActual},
					{Threshold: 80.0, Type: config.AlertTypeActual},
				},
			},
			currentSpend:    200.0,
			wantExceeded:    nil,
			wantApproaching: nil,
		},
		{
			name: "one threshold exceeded - 60% spend",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: 50.0, Type: config.AlertTypeActual},
					{Threshold: 80.0, Type: config.AlertTypeActual},
				},
			},
			currentSpend:    600.0,
			wantExceeded:    []float64{50.0},
			wantApproaching: nil,
		},
		{
			name: "approaching threshold - 77% spend",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: 50.0, Type: config.AlertTypeActual},
					{Threshold: 80.0, Type: config.AlertTypeActual},
				},
			},
			currentSpend:    770.0, // 77% - within 5% of 80%
			wantExceeded:    []float64{50.0},
			wantApproaching: []float64{80.0},
		},
		{
			name: "all thresholds exceeded - 150% spend",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: 50.0, Type: config.AlertTypeActual},
					{Threshold: 80.0, Type: config.AlertTypeActual},
					{Threshold: 100.0, Type: config.AlertTypeActual},
				},
			},
			currentSpend:    1500.0,
			wantExceeded:    []float64{50.0, 80.0, 100.0},
			wantApproaching: nil,
		},
	}

	engine := NewBudgetEngine()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, err := engine.Evaluate(tc.budget, tc.currentSpend, "USD")
			require.NoError(t, err)

			// Check exceeded alerts
			exceeded := status.GetExceededAlerts()
			var exceededThresholds []float64
			for _, a := range exceeded {
				exceededThresholds = append(exceededThresholds, a.Threshold)
			}
			assert.ElementsMatch(t, tc.wantExceeded, exceededThresholds, "exceeded thresholds mismatch")

			// Check approaching alerts
			var approachingThresholds []float64
			for _, alert := range status.Alerts {
				if alert.Status == ThresholdStatusApproaching {
					approachingThresholds = append(approachingThresholds, alert.Threshold)
				}
			}
			assert.ElementsMatch(t, tc.wantApproaching, approachingThresholds, "approaching thresholds mismatch")
		})
	}
}

func TestDefaultBudgetEngine_Forecasting(t *testing.T) {
	// Fixed time: January 15th (day 15 of 31 days)
	fixedTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	engine := NewBudgetEngineWithTime(func() time.Time { return fixedTime })

	budget := config.BudgetConfig{
		Amount:   1000.0,
		Currency: "USD",
		Alerts: []config.AlertConfig{
			{Threshold: 100.0, Type: config.AlertTypeForecasted},
		},
	}

	t.Run("forecast under budget", func(t *testing.T) {
		// Spend $300 by day 15 = $20/day * 31 days = $620 forecast (62%)
		status, err := engine.Evaluate(budget, 300.0, "USD")
		require.NoError(t, err)

		// Expected: 300/15 * 31 = 620
		assert.InDelta(t, 620.0, status.ForecastedSpend, 0.1)
		assert.InDelta(t, 62.0, status.ForecastPercentage, 0.1)
		assert.False(t, status.IsForecastOverBudget())

		// 100% threshold should be OK
		assert.Len(t, status.Alerts, 1)
		assert.Equal(t, ThresholdStatusOK, status.Alerts[0].Status)
	})

	t.Run("forecast over budget", func(t *testing.T) {
		// Spend $600 by day 15 = $40/day * 31 days = $1240 forecast (124%)
		status, err := engine.Evaluate(budget, 600.0, "USD")
		require.NoError(t, err)

		assert.InDelta(t, 1240.0, status.ForecastedSpend, 0.1)
		assert.InDelta(t, 124.0, status.ForecastPercentage, 0.1)
		assert.True(t, status.IsForecastOverBudget())

		// 100% threshold should be EXCEEDED
		assert.Len(t, status.Alerts, 1)
		assert.Equal(t, ThresholdStatusExceeded, status.Alerts[0].Status)
	})

	t.Run("forecast approaching threshold", func(t *testing.T) {
		// Spend $475 by day 15 = ~$31.67/day * 31 days = ~$981.67 forecast (~98.2%)
		// This is within 5% of 100% threshold
		status, err := engine.Evaluate(budget, 475.0, "USD")
		require.NoError(t, err)

		assert.InDelta(t, 98.2, status.ForecastPercentage, 0.5)
		assert.False(t, status.IsForecastOverBudget())

		// 100% threshold should be APPROACHING
		assert.Len(t, status.Alerts, 1)
		assert.Equal(t, ThresholdStatusApproaching, status.Alerts[0].Status)
	})
}

func TestDefaultBudgetEngine_MixedAlertTypes(t *testing.T) {
	// Fixed time: January 15th (day 15 of 31 days)
	fixedTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	engine := NewBudgetEngineWithTime(func() time.Time { return fixedTime })

	budget := config.BudgetConfig{
		Amount:   1000.0,
		Currency: "USD",
		Alerts: []config.AlertConfig{
			{Threshold: 50.0, Type: config.AlertTypeActual},      // Check actual spend
			{Threshold: 80.0, Type: config.AlertTypeActual},      // Check actual spend
			{Threshold: 100.0, Type: config.AlertTypeForecasted}, // Check forecast
		},
	}

	// Spend $450 by day 15 = 45% actual, but 93% forecast (450/15*31 = 930)
	status, err := engine.Evaluate(budget, 450.0, "USD")
	require.NoError(t, err)

	assert.Equal(t, 45.0, status.Percentage)                // 45% actual
	assert.InDelta(t, 93.0, status.ForecastPercentage, 0.5) // ~93% forecast

	// Check alert statuses
	require.Len(t, status.Alerts, 3)

	// 50% actual: APPROACHING (we're at 45%, which is exactly at boundary 50-5=45, and 45 >= 45)
	assert.Equal(t, 50.0, status.Alerts[0].Threshold)
	assert.Equal(t, config.AlertTypeActual, status.Alerts[0].Type)
	assert.Equal(t, ThresholdStatusApproaching, status.Alerts[0].Status)

	// 80% actual: OK (we're at 45%)
	assert.Equal(t, 80.0, status.Alerts[1].Threshold)
	assert.Equal(t, config.AlertTypeActual, status.Alerts[1].Type)
	assert.Equal(t, ThresholdStatusOK, status.Alerts[1].Status)

	// 100% forecasted: OK (we're at ~93%, which is < 95% threshold for approaching)
	assert.Equal(t, 100.0, status.Alerts[2].Threshold)
	assert.Equal(t, config.AlertTypeForecasted, status.Alerts[2].Type)
	assert.Equal(t, ThresholdStatusOK, status.Alerts[2].Status)
}

func TestBudgetStatus_Methods(t *testing.T) {
	t.Run("HasExceededAlerts", func(t *testing.T) {
		status := &BudgetStatus{
			Alerts: []ThresholdStatus{
				{Threshold: 50.0, Status: ThresholdStatusExceeded},
				{Threshold: 80.0, Status: ThresholdStatusOK},
			},
		}
		assert.True(t, status.HasExceededAlerts())

		statusNoExceeded := &BudgetStatus{
			Alerts: []ThresholdStatus{
				{Threshold: 50.0, Status: ThresholdStatusOK},
				{Threshold: 80.0, Status: ThresholdStatusApproaching},
			},
		}
		assert.False(t, statusNoExceeded.HasExceededAlerts())
	})

	t.Run("HasApproachingAlerts", func(t *testing.T) {
		status := &BudgetStatus{
			Alerts: []ThresholdStatus{
				{Threshold: 50.0, Status: ThresholdStatusExceeded},
				{Threshold: 80.0, Status: ThresholdStatusApproaching},
			},
		}
		assert.True(t, status.HasApproachingAlerts())

		statusNoApproaching := &BudgetStatus{
			Alerts: []ThresholdStatus{
				{Threshold: 50.0, Status: ThresholdStatusOK},
				{Threshold: 80.0, Status: ThresholdStatusExceeded},
			},
		}
		assert.False(t, statusNoApproaching.HasApproachingAlerts())
	})

	t.Run("GetHighestExceededThreshold", func(t *testing.T) {
		status := &BudgetStatus{
			Alerts: []ThresholdStatus{
				{Threshold: 50.0, Status: ThresholdStatusExceeded},
				{Threshold: 80.0, Status: ThresholdStatusExceeded},
				{Threshold: 100.0, Status: ThresholdStatusOK},
			},
		}
		assert.Equal(t, 80.0, status.GetHighestExceededThreshold())

		statusNoneExceeded := &BudgetStatus{
			Alerts: []ThresholdStatus{
				{Threshold: 50.0, Status: ThresholdStatusOK},
			},
		}
		assert.Equal(t, 0.0, statusNoneExceeded.GetHighestExceededThreshold())
	})

	t.Run("CappedPercentage", func(t *testing.T) {
		tests := []struct {
			percentage float64
			expected   float64
		}{
			{50.0, 50.0},
			{100.0, 100.0},
			{150.0, 100.0},
			{200.0, 100.0},
		}

		for _, tc := range tests {
			status := &BudgetStatus{Percentage: tc.percentage}
			assert.Equal(t, tc.expected, status.CappedPercentage())
		}
	})
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected int
	}{
		{"January 2025", time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), 31},
		{"February 2025 (non-leap)", time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC), 28},
		{"February 2024 (leap)", time.Date(2024, 2, 10, 0, 0, 0, 0, time.UTC), 29},
		{"April 2025", time.Date(2025, 4, 20, 0, 0, 0, 0, time.UTC), 30},
		{"December 2025", time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC), 31},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, daysInMonth(tc.time))
		})
	}
}

func TestEvaluateThreshold(t *testing.T) {
	tests := []struct {
		name       string
		threshold  float64
		percentage float64
		expected   ThresholdStatusValue
	}{
		// Normal thresholds (> 5%)
		{"exceeded - exactly at threshold", 80.0, 80.0, ThresholdStatusExceeded},
		{"exceeded - above threshold", 80.0, 85.0, ThresholdStatusExceeded},
		{"approaching - within 5%", 80.0, 76.0, ThresholdStatusApproaching},
		{"approaching - exactly 5% below", 80.0, 75.0, ThresholdStatusApproaching},
		{"OK - more than 5% below", 80.0, 74.0, ThresholdStatusOK},
		{"OK - well below", 80.0, 50.0, ThresholdStatusOK},

		// Small thresholds (at or below 5% buffer)
		// Threshold at buffer value (5%) - no approaching state possible
		{"small threshold 5% - exceeded", 5.0, 5.0, ThresholdStatusExceeded},
		{"small threshold 5% - above", 5.0, 6.0, ThresholdStatusExceeded},
		{"small threshold 5% - below, no approaching", 5.0, 4.0, ThresholdStatusOK},
		{"small threshold 5% - zero", 5.0, 0.0, ThresholdStatusOK},

		// Threshold below buffer (3%)
		{"small threshold 3% - exceeded", 3.0, 3.0, ThresholdStatusExceeded},
		{"small threshold 3% - above", 3.0, 4.0, ThresholdStatusExceeded},
		{"small threshold 3% - below, no approaching", 3.0, 2.0, ThresholdStatusOK},
		{"small threshold 3% - zero", 3.0, 0.0, ThresholdStatusOK},

		// Threshold 0%
		{"zero threshold - exceeded", 0.0, 0.0, ThresholdStatusExceeded},
		{"zero threshold - above", 0.0, 1.0, ThresholdStatusExceeded},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, evaluateThreshold(tc.threshold, tc.percentage))
		})
	}
}

// =============================================================================
// Budget Exit Code Tests (Issue #219)
// =============================================================================

// T013: Unit test for BudgetStatus.ShouldExit() returns false when disabled.
func TestBudgetStatus_ShouldExit_Disabled(t *testing.T) {
	status := &BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: false, // Disabled
		},
		Alerts: []ThresholdStatus{
			{Threshold: 80.0, Status: ThresholdStatusExceeded},
		},
	}

	assert.False(t, status.ShouldExit(), "should not exit when exit_on_threshold is disabled")
}

// T014: Unit test for BudgetStatus.ShouldExit() returns false when no thresholds exceeded.
func TestBudgetStatus_ShouldExit_NoExceeded(t *testing.T) {
	status := &BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true, // Enabled
		},
		Alerts: []ThresholdStatus{
			{Threshold: 80.0, Status: ThresholdStatusOK},
			{Threshold: 100.0, Status: ThresholdStatusApproaching},
		},
	}

	assert.False(t, status.ShouldExit(), "should not exit when no thresholds are exceeded")
}

// T015: Unit test for BudgetStatus.ShouldExit() returns true when enabled AND exceeded.
func TestBudgetStatus_ShouldExit_EnabledAndExceeded(t *testing.T) {
	status := &BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true, // Enabled
		},
		Alerts: []ThresholdStatus{
			{Threshold: 80.0, Status: ThresholdStatusExceeded},
		},
	}

	assert.True(t, status.ShouldExit(), "should exit when exit_on_threshold is enabled and threshold is exceeded")
}

// T016: Unit test for BudgetStatus.GetExitCode() returns 0 when should not exit.
func TestBudgetStatus_GetExitCode_NoExit(t *testing.T) {
	tests := []struct {
		name   string
		status *BudgetStatus
	}{
		{
			name: "exit disabled, no exceeded",
			status: &BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:          1000.0,
					Currency:        "USD",
					ExitOnThreshold: false,
					ExitCode:        2,
				},
				Alerts: []ThresholdStatus{
					{Threshold: 80.0, Status: ThresholdStatusOK},
				},
			},
		},
		{
			name: "exit enabled but no exceeded",
			status: &BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:          1000.0,
					Currency:        "USD",
					ExitOnThreshold: true,
					ExitCode:        2,
				},
				Alerts: []ThresholdStatus{
					{Threshold: 80.0, Status: ThresholdStatusOK},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, 0, tc.status.GetExitCode(), "should return 0 when should not exit")
		})
	}
}

// T017: Unit test for BudgetStatus.GetExitCode() returns configured code on exit.
func TestBudgetStatus_GetExitCode_ShouldExit(t *testing.T) {
	tests := []struct {
		name         string
		exitCode     int
		expectedCode int
	}{
		// Note: When ExitOnThreshold=true and ExitCode=0, it's warning-only mode (exit code 0)
		// The "default exit code 1" behavior is handled by BudgetConfig.GetExitCode() when
		// exit is NOT enabled. When exit IS enabled with ExitCode=0, that's explicit warning mode.
		{"custom exit code 2", 2, 2},
		{"max exit code 255", 255, 255},
		{"explicit exit code 1", 1, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status := &BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:          1000.0,
					Currency:        "USD",
					ExitOnThreshold: true,
					ExitCode:        tc.exitCode,
				},
				Alerts: []ThresholdStatus{
					{Threshold: 80.0, Status: ThresholdStatusExceeded},
				},
			}

			assert.Equal(t, tc.expectedCode, status.GetExitCode())
		})
	}
}

// T018: Unit test for BudgetStatus.ExitReason() returns empty string when no exit.
func TestBudgetStatus_ExitReason_NoExit(t *testing.T) {
	status := &BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: false,
		},
		Alerts: []ThresholdStatus{
			{Threshold: 80.0, Status: ThresholdStatusExceeded},
		},
	}

	assert.Empty(t, status.ExitReason(), "should return empty string when should not exit")
}

// T019: Unit test for BudgetStatus.ExitReason() returns descriptive message on exit.
func TestBudgetStatus_ExitReason_OnExit(t *testing.T) {
	status := &BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true,
			ExitCode:        2,
		},
		Alerts: []ThresholdStatus{
			{Threshold: 80.0, Status: ThresholdStatusExceeded},
			{Threshold: 100.0, Status: ThresholdStatusExceeded},
		},
	}

	reason := status.ExitReason()
	assert.NotEmpty(t, reason)
	assert.Contains(t, reason, "budget threshold exceeded")
	assert.Contains(t, reason, "100") // Should mention highest threshold
}

// T020a: Unit test for exit code 1 when budget evaluation error occurs (FR-009).
func TestBudgetStatus_ExitCodeOnError(t *testing.T) {
	// When evaluation fails, the error exit code should be 1 (not the configured code)
	// This is tested at the CLI level, but we verify the constant here
	assert.Equal(t, 1, ExitCodeBudgetEvaluationError,
		"budget evaluation error exit code should be 1")
}

// Additional exit code tests for edge cases.
func TestBudgetStatus_ShouldExit_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		status     *BudgetStatus
		shouldExit bool
	}{
		{
			name: "empty alerts list",
			status: &BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:          1000.0,
					Currency:        "USD",
					ExitOnThreshold: true,
				},
				Alerts: []ThresholdStatus{},
			},
			shouldExit: false,
		},
		{
			name: "nil budget status",
			status: &BudgetStatus{
				Budget: config.BudgetConfig{
					ExitOnThreshold: true,
				},
				Alerts: nil,
			},
			shouldExit: false,
		},
		{
			name: "warning-only mode (exit code 0)",
			status: &BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:          1000.0,
					Currency:        "USD",
					ExitOnThreshold: true,
					ExitCode:        0, // Warning-only mode
				},
				Alerts: []ThresholdStatus{
					{Threshold: 80.0, Status: ThresholdStatusExceeded},
				},
			},
			shouldExit: true, // Still returns true, but exit code will be 0
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.shouldExit, tc.status.ShouldExit())
		})
	}
}

// Test exit code with warning-only mode (exit_code: 0).
func TestBudgetStatus_GetExitCode_WarningOnly(t *testing.T) {
	status := &BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true,
			ExitCode:        0, // Warning-only mode
		},
		Alerts: []ThresholdStatus{
			{Threshold: 80.0, Status: ThresholdStatusExceeded},
		},
	}

	// Should return 0 even when threshold is exceeded (warning-only mode)
	assert.Equal(t, 0, status.GetExitCode())
}
