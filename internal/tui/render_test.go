package tui_test

import (
	"math"
	"testing"

	"github.com/rshade/finfocus/internal/tui"
)

func TestFormatMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   float64
		currency string
		expected string
	}{
		{"Zero USD", 0, "USD", "$0.00 USD"},
		{"Positive USD", 1234.56, "USD", "$1,234.56 USD"},
		{"Negative USD", -999.99, "USD", "-$999.99 USD"},
		{"Large number USD", 1234567.89, "USD", "$1,234,567.89 USD"},
		{"Small decimals USD", 0.01, "USD", "$0.01 USD"},
		{"EUR currency", 1234.56, "EUR", "$1,234.56 EUR"},
		{"Empty currency", 1234.56, "", "$1,234.56"},
		{"Custom currency", 1234.56, "GBP", "$1,234.56 GBP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tui.FormatMoney(tt.amount, tt.currency)
			if result != tt.expected {
				t.Errorf(
					"FormatMoney(%.2f, %q) = %q, expected %q",
					tt.amount,
					tt.currency,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestFormatMoneyShort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   float64
		expected string
	}{
		{"Zero", 0, "$0.00"},
		{"Positive", 1234.56, "$1,234.56"},
		{"Negative", -999.99, "-$999.99"},
		{"Large number", 1234567.89, "$1,234,567.89"},
		{"Small decimals", 0.01, "$0.01"},
		{"One decimal place", 123.5, "$123.50"},
		{"No decimals", 100, "$100.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tui.FormatMoneyShort(tt.amount)
			if result != tt.expected {
				t.Errorf("FormatMoneyShort(%.2f) = %q, expected %q", tt.amount, result, tt.expected)
			}
		})
	}
}

func TestFormatPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{"Zero percent", 0, "0.0%"},
		{"Whole number", 85, "85.0%"},
		{"One decimal", 85.5, "85.5%"},
		{"Two decimals", 85.55, "85.5%"}, // Go uses banker's rounding
		{"Negative percent", -15.7, "-15.7%"},
		{"Large percent", 150.25, "150.2%"},
		{"Small percent", 0.123, "0.1%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tui.FormatPercent(tt.value)
			if result != tt.expected {
				t.Errorf("FormatPercent(%.3f) = %q, expected %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestMoneyFormatting_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		function func() string
		expected string
	}{
		{
			"FormatMoney with NaN",
			func() string { return tui.FormatMoney(math.NaN(), "USD") },
			"$0.00 USD",
		},
		{
			"FormatMoneyShort with NaN",
			func() string { return tui.FormatMoneyShort(math.NaN()) },
			"$0.00",
		},
		{"FormatPercent with NaN", func() string { return tui.FormatPercent(math.NaN()) }, "0.0%"},
		// +Inf and -Inf should be handled gracefully
		{
			"FormatMoney with +Inf",
			func() string { return tui.FormatMoney(math.Inf(1), "USD") },
			"$0.00 USD",
		},
		{
			"FormatMoneyShort with +Inf",
			func() string { return tui.FormatMoneyShort(math.Inf(1)) },
			"$0.00",
		},
		{"FormatPercent with +Inf", func() string { return tui.FormatPercent(math.Inf(1)) }, "0.0%"},
		{
			"FormatMoney with -Inf",
			func() string { return tui.FormatMoney(math.Inf(-1), "USD") },
			"$0.00 USD",
		},
		{
			"FormatMoneyShort with -Inf",
			func() string { return tui.FormatMoneyShort(math.Inf(-1)) },
			"$0.00",
		},
		{"FormatPercent with -Inf", func() string { return tui.FormatPercent(math.Inf(-1)) }, "0.0%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.function()
			if result != tt.expected {
				t.Errorf("%s = %q, expected %q", tt.name, result, tt.expected)
			}
		})
	}
}
