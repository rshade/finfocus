package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func TestRenderBudgetStatus_Nil(t *testing.T) {
	var buf bytes.Buffer
	err := RenderBudgetStatus(&buf, nil)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestRenderPlainBudget(t *testing.T) {
	tests := []struct {
		name     string
		status   *engine.BudgetStatus
		contains []string
	}{
		{
			name: "basic budget status",
			status: &engine.BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:   1000.0,
					Currency: "USD",
					Period:   "monthly",
				},
				CurrentSpend: 550.0,
				Percentage:   55.0,
				Currency:     "USD",
			},
			contains: []string{
				"BUDGET STATUS",
				"Budget: $1000.00/monthly",
				"Current Spend: $550.00 (55.0%)",
				"OK - Within budget",
			},
		},
		{
			name: "over budget with alerts",
			status: &engine.BudgetStatus{
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
			},
			contains: []string{
				"BUDGET STATUS",
				"Budget: $1000.00/monthly",
				"Current Spend: $850.00 (85.0%)",
				"WARNING - Exceeds 80% threshold",
			},
		},
		{
			name: "with forecasted spend",
			status: &engine.BudgetStatus{
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
			},
			contains: []string{
				"Forecasted: $930.00 (93.0%)",
			},
		},
		{
			name: "different currency - EUR",
			status: &engine.BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:   500.0,
					Currency: "EUR",
					Period:   "monthly",
				},
				CurrentSpend: 250.0,
				Percentage:   50.0,
				Currency:     "EUR",
			},
			contains: []string{
				"Budget: €500.00/monthly",
				"Current Spend: €250.00 (50.0%)",
			},
		},
		{
			name: "approaching threshold",
			status: &engine.BudgetStatus{
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
			},
			contains: []string{
				"APPROACHING - Near budget threshold",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := renderPlainBudget(&buf, tc.status)
			require.NoError(t, err)

			output := buf.String()
			for _, expected := range tc.contains {
				assert.Contains(t, output, expected, "expected output to contain: %s", expected)
			}
		})
	}
}

func TestRenderStyledBudget(t *testing.T) {
	// Note: renderStyledBudget includes ANSI escape codes, so we test that it
	// produces output with expected content (stripping color codes for comparison)

	tests := []struct {
		name     string
		status   *engine.BudgetStatus
		contains []string
	}{
		{
			name: "basic styled output",
			status: &engine.BudgetStatus{
				Budget: config.BudgetConfig{
					Amount:   1000.0,
					Currency: "USD",
					Period:   "monthly",
				},
				CurrentSpend: 550.0,
				Percentage:   55.0,
				Currency:     "USD",
			},
			contains: []string{
				"BUDGET STATUS",
				"Budget:",
				"1,000.00", // Uses thousand separators
				"monthly",
				"Current Spend:",
				"550.00",
				"55",
			},
		},
		{
			name: "over budget with warning",
			status: &engine.BudgetStatus{
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
			},
			contains: []string{
				"WARNING",
				"80",
				"threshold",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := renderStyledBudget(&buf, tc.status)
			require.NoError(t, err)

			// Strip ANSI codes for content verification
			output := stripANSI(buf.String())
			for _, expected := range tc.contains {
				assert.Contains(t, output, expected, "expected styled output to contain: %s", expected)
			}
		})
	}
}

func TestRenderProgressBar(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		width      int
		wantFilled int // Approximate filled characters
	}{
		{
			name:       "50% filled",
			percentage: 50.0,
			width:      20,
			wantFilled: 10,
		},
		{
			name:       "0% empty",
			percentage: 0.0,
			width:      20,
			wantFilled: 0,
		},
		{
			name:       "100% full",
			percentage: 100.0,
			width:      20,
			wantFilled: 20,
		},
		{
			name:       "over 100% capped",
			percentage: 150.0,
			width:      20,
			wantFilled: 20, // Should cap at 100%
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status := &engine.BudgetStatus{
				Percentage: tc.percentage,
			}
			bar := renderProgressBar(status, tc.width)

			// Strip ANSI codes and count filled characters
			stripped := stripANSI(bar)
			filledCount := strings.Count(stripped, progressFilledChar)
			assert.Equal(t, tc.wantFilled, filledCount, "expected %d filled chars", tc.wantFilled)
		})
	}
}

func TestRenderAlertMessages(t *testing.T) {
	tests := []struct {
		name     string
		alerts   []engine.ThresholdStatus
		contains []string
		empty    bool
	}{
		{
			name:   "no alerts",
			alerts: []engine.ThresholdStatus{},
			empty:  true,
		},
		{
			name: "all OK alerts",
			alerts: []engine.ThresholdStatus{
				{Threshold: 80.0, Status: engine.ThresholdStatusOK},
			},
			empty: true,
		},
		{
			name: "exceeded alert",
			alerts: []engine.ThresholdStatus{
				{Threshold: 80.0, Type: config.AlertTypeActual, Status: engine.ThresholdStatusExceeded},
			},
			contains: []string{"WARNING", "80", "threshold"},
		},
		{
			name: "approaching alert",
			alerts: []engine.ThresholdStatus{
				{Threshold: 80.0, Type: config.AlertTypeActual, Status: engine.ThresholdStatusApproaching},
			},
			contains: []string{"APPROACHING", "80", "threshold"},
		},
		{
			name: "forecasted alert",
			alerts: []engine.ThresholdStatus{
				{Threshold: 100.0, Type: config.AlertTypeForecasted, Status: engine.ThresholdStatusExceeded},
			},
			contains: []string{"WARNING", "forecasted", "100"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status := &engine.BudgetStatus{Alerts: tc.alerts}
			result := renderAlertMessages(status)

			if tc.empty {
				assert.Empty(t, result)
				return
			}

			stripped := stripANSI(result)
			for _, expected := range tc.contains {
				assert.Contains(t, stripped, expected)
			}
		})
	}
}

func TestGetStatusMessage(t *testing.T) {
	tests := []struct {
		name     string
		status   *engine.BudgetStatus
		expected string
	}{
		{
			name:     "OK status",
			status:   &engine.BudgetStatus{},
			expected: "OK - Within budget",
		},
		{
			name: "exceeded threshold",
			status: &engine.BudgetStatus{
				Alerts: []engine.ThresholdStatus{
					{Threshold: 50.0, Status: engine.ThresholdStatusExceeded},
					{Threshold: 80.0, Status: engine.ThresholdStatusExceeded},
				},
			},
			expected: "WARNING - Exceeds 80% threshold",
		},
		{
			name: "approaching threshold",
			status: &engine.BudgetStatus{
				Alerts: []engine.ThresholdStatus{
					{Threshold: 80.0, Status: engine.ThresholdStatusApproaching},
				},
			},
			expected: "APPROACHING - Near budget threshold",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, getStatusMessage(tc.status))
		})
	}
}

func TestCurrencySymbol(t *testing.T) {
	tests := []struct {
		currency string
		expected string
	}{
		{"USD", "$"},
		{"EUR", "€"},
		{"GBP", "£"},
		{"JPY", "¥"},
		{"CAD", "C$"},
		{"AUD", "A$"},
		{"CHF", "CHF "},
		{"XYZ", "XYZ "}, // Unknown currency
	}

	for _, tc := range tests {
		t.Run(tc.currency, func(t *testing.T) {
			assert.Equal(t, tc.expected, currencySymbol(tc.currency))
		})
	}
}

func TestCalculateBoxWidth(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		expected  int
	}{
		{"narrow terminal", 30, minBoxWidth},
		{"normal terminal", 80, defaultBoxWidth},
		{"wide terminal", 200, defaultBoxWidth},
		{"very narrow", 20, minBoxWidth},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateBoxWidth(tc.termWidth)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCalculateProgressBarWidth(t *testing.T) {
	tests := []struct {
		name     string
		boxWidth int
		min      int
		max      int
	}{
		{"narrow box", minBoxWidth, minProgressBarWidth, progressBarWidth},
		{"normal box", defaultBoxWidth, minProgressBarWidth, progressBarWidth},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateProgressBarWidth(tc.boxWidth)
			assert.GreaterOrEqual(t, result, tc.min)
			assert.LessOrEqual(t, result, tc.max)
		})
	}
}

func TestIsWriterTerminal(t *testing.T) {
	// Testing with a bytes.Buffer should return false (not a TTY)
	var buf bytes.Buffer
	assert.False(t, isWriterTerminal(&buf))

	// Note: We can't easily test true case without a real terminal
}

// stripANSI removes ANSI escape codes from a string for content comparison.
func stripANSI(s string) string {
	// Simple approach: remove escape sequences between \x1b[ and m
	result := s
	for {
		start := strings.Index(result, "\x1b[")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "m")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}
	return result
}
