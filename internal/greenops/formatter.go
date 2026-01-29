package greenops

import (
	"fmt"
	"math"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// printer is the locale-aware message printer for number formatting.
// Uses English locale for consistent thousand separators.
//
//nolint:gochecknoglobals // Global printer is idiomatic for x/text/message usage.
var printer = message.NewPrinter(language.English)

// FormatNumber formats an integer with thousand separators.
// Example: FormatNumber(18248) returns "18,248".
func FormatNumber(n int64) string {
	return printer.Sprintf("%d", n)
}

// FormatFloat formats a float with the specified precision and thousand separators.
// Example: FormatFloat(1234.567, 2) returns "1,234.57".
func FormatFloat(f float64, precision int) string {
	// Round to the specified precision.
	const base = 10
	multiplier := math.Pow(base, float64(precision))
	rounded := math.Round(f*multiplier) / multiplier

	if precision == 0 {
		return FormatNumber(int64(rounded))
	}

	// Format with precision.
	format := fmt.Sprintf("%%.%df", precision)
	formatted := fmt.Sprintf(format, rounded)

	// Add thousand separators to the integer part.
	const decimalPartCount = 2
	parts := splitDecimal(formatted)
	if len(parts) == decimalPartCount {
		intPart, err := parseIntPart(parts[0])
		if err == nil {
			return printer.Sprintf("%d", intPart) + "." + parts[1]
		}
	}

	return formatted
}

// splitDecimal splits a formatted number string into integer and decimal parts.
func splitDecimal(s string) []string {
	for i, c := range s {
		if c == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// parseIntPart parses an integer string, handling negative numbers.
func parseIntPart(s string) (int64, error) {
	const base = 10
	var n int64
	negative := false

	for i, c := range s {
		if i == 0 && c == '-' {
			negative = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid character: %c", c)
		}
		n = n*base + int64(c-'0')
	}

	if negative {
		n = -n
	}
	return n, nil
}

// FormatLarge formats large numbers with abbreviated notation.
//
// Values below LargeNumberThreshold (1 million) use comma-separated format.
// Values at or above LargeNumberThreshold use "~X.X million" format.
// Values at or above BillionThreshold use "~X.X billion" format.
//
// Example: FormatLarge(1500000000) returns "~1.5 billion".
func FormatLarge(n float64) string {
	if n >= BillionThreshold {
		billions := n / BillionThreshold
		return fmt.Sprintf("~%.1f billion", billions)
	}

	if n >= LargeNumberThreshold {
		millions := n / LargeNumberThreshold
		return fmt.Sprintf("~%.1f million", millions)
	}

	// Below threshold, use comma-separated integer format
	return FormatNumber(int64(math.Round(n)))
}
