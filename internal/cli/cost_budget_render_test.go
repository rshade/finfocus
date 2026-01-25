package cli

import (
	"bytes"
	"io"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func TestRenderBudgetStatus_Nil(t *testing.T) {
	err := RenderBudgetStatus(io.Discard, nil)
	assert.NoError(t, err)
}

func TestRenderPlainBudget(t *testing.T) {
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:   1000.0,
			Currency: "USD",
			Period:   "monthly",
		},
		CurrentSpend:       500.0,
		Percentage:         50.0,
		Currency:           "USD",
		ForecastedSpend:    1200.0,
		ForecastPercentage: 120.0,
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Status: engine.ThresholdStatusOK},
		},
	}

	var buf bytes.Buffer
	err := renderPlainBudget(&buf, status)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "BUDGET STATUS")
	assert.Contains(t, output, "Budget: $1000.00/monthly")
	assert.Contains(t, output, "Current Spend: $500.00 (50.0%)")
	assert.Contains(t, output, "Status: OK - Within budget")
	assert.Contains(t, output, "Forecasted: $1200.00 (120.0%)")
}

func TestGetStatusMessage(t *testing.T) {
	tests := []struct {
		name   string
		status *engine.BudgetStatus
		want   string
	}{
		{
			name: "OK",
			status: &engine.BudgetStatus{
				Alerts: []engine.ThresholdStatus{},
			},
			want: "OK - Within budget",
		},
		{
			name: "Exceeded",
			status: &engine.BudgetStatus{
				Alerts: []engine.ThresholdStatus{
					{Threshold: 100.0, Status: engine.ThresholdStatusExceeded},
				},
			},
			want: "WARNING - Exceeds 100% threshold",
		},
		{
			name: "Approaching",
			status: &engine.BudgetStatus{
				Alerts: []engine.ThresholdStatus{
					{Threshold: 80.0, Status: engine.ThresholdStatusApproaching},
				},
			},
			want: "APPROACHING - Near budget threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStatusMessage(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCurrencySymbol(t *testing.T) {
	assert.Equal(t, "$", currencySymbol("USD"))
	assert.Equal(t, "€", currencySymbol("EUR"))
	assert.Equal(t, "£", currencySymbol("GBP"))
	assert.Equal(t, "¥", currencySymbol("JPY"))
	assert.Equal(t, "¥", currencySymbol("CNY"))
	assert.Equal(t, "C$", currencySymbol("CAD"))
	assert.Equal(t, "A$", currencySymbol("AUD"))
	assert.Equal(t, "₹", currencySymbol("INR"))
	assert.Equal(t, "₩", currencySymbol("KRW"))
	assert.Equal(t, "BRL ", currencySymbol("BRL"))
}

func TestCalculateBoxWidth(t *testing.T) {
	assert.Equal(t, minBoxWidth, calculateBoxWidth(20))
	assert.Equal(t, defaultBoxWidth, calculateBoxWidth(100))
	assert.Equal(t, 32, calculateBoxWidth(40))
}

func TestCalculateProgressBarWidth(t *testing.T) {
	assert.Equal(t, minProgressBarWidth, calculateProgressBarWidth(20))
	assert.Equal(t, progressBarWidth, calculateProgressBarWidth(100))
}

func TestIsWriterTerminal(t *testing.T) {
	var buf bytes.Buffer
	assert.False(t, isWriterTerminal(&buf))
}

func TestRenderProgressBar(t *testing.T) {
	status := &engine.BudgetStatus{
		Percentage: 50.0,
	}
	// Use a small width to make it easy to verify
	bar := renderProgressBar(status, 10)
	assert.Contains(t, bar, "50%")
}

func TestDetermineProgressBarColor(t *testing.T) {
	assert.Equal(t, progressOKColor(), determineProgressBarColor(50.0))
	assert.Equal(t, lipgloss.Color("214"), determineProgressBarColor(85.0))
	assert.Equal(t, progressExceededColor(), determineProgressBarColor(110.0))
}

func TestFormatAlertMessage(t *testing.T) {
	alert := engine.ThresholdStatus{
		Threshold: 80.0,
		Type:      config.AlertTypeActual,
	}
	assert.Equal(t, "WARNING - spend exceeds 80% threshold", formatAlertMessage(alert, "WARNING"))

	alert.Type = config.AlertTypeForecasted
	assert.Equal(t, "WARNING - forecasted spend exceeds 80% threshold", formatAlertMessage(alert, "WARNING"))
}

func TestRenderAlertMessages(t *testing.T) {
	status := &engine.BudgetStatus{
		Alerts: []engine.ThresholdStatus{
			{Threshold: 100.0, Status: engine.ThresholdStatusExceeded, Type: config.AlertTypeActual},
			{Threshold: 80.0, Status: engine.ThresholdStatusApproaching, Type: config.AlertTypeActual},
		},
	}
	messages := renderAlertMessages(status)
	assert.Contains(t, messages, "WARNING - spend exceeds 100% threshold")
	assert.Contains(t, messages, "APPROACHING - spend exceeds 80% threshold")
}

func TestRenderStyledBudget(t *testing.T) {
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:   1000.0,
			Currency: "USD",
			Period:   "monthly",
		},
		CurrentSpend:       500.0,
		Percentage:         50.0,
		Currency:           "USD",
		ForecastedSpend:    1200.0,
		ForecastPercentage: 120.0,
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Status: engine.ThresholdStatusOK},
		},
	}

	var buf bytes.Buffer
	err := renderStyledBudget(&buf, status)
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())

	// Verify key data is present in output
	output := buf.String()
	assert.Contains(t, output, "1,000") // Budget amount (formatted)
	assert.Contains(t, output, "$")     // Currency symbol (for USD)
	assert.Contains(t, output, "50")    // Percentage
}
