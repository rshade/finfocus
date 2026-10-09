package tui

import (
	"fmt"
	"math"

	"github.com/rshade/finfocus/internal/viewmodel"
)

// FormatMoney formats monetary values through the shared presentation helper.
func FormatMoney(amount float64, currency string) string {
	return viewmodel.FormatMoney(amount, currency)
}

// FormatMoneyShort formats monetary values without a currency code.
func FormatMoneyShort(amount float64) string { return viewmodel.FormatMoneyShort(amount) }

// FormatPercent formats a percentage value with one decimal place.
// Handles rounding and ensures consistent formatting for percentage displays.
//
// Usage:
//
//	FormatPercent(85.7)  // "85.7%"
//	FormatPercent(100)   // "100.0%"
//
// FormatPercent formats a percentage value with one decimal place and a trailing percent sign.
// If value is NaN, it returns "0.0%". The returned string contains the value rounded to one decimal place followed by "%".
func FormatPercent(value float64) string {
	// Handle special cases: NaN and Infinity
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "0.0%"
	}

	// Round to one decimal place
	rounded := fmt.Sprintf("%.1f", value)
	return rounded + "%"
}
