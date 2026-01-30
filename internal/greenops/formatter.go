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
// FormatNumber formats n with comma thousand separators using the package's English-locale printer.
func FormatNumber(n int64) string {
	return printer.Sprintf("%d", n)
}

// FormatFloat formats a float with the specified precision and thousand separators.
// FormatFloat formats f rounded to precision decimal places and returns a string
// with English-style thousand separators applied to the integer part when possible.
// If precision is 0 the value is formatted via FormatNumber. The function rounds
// to the requested precision, preserves the decimal portion, and preserves the
// sign for negative values. If applying thousand separators to the integer
// portion fails, the plain decimal-formatted string is returned.
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

// splitDecimal splits s at the first '.' and returns a slice containing the integer and decimal parts.
// If a '.' is found the returned slice has two elements: the substring before the dot and the substring after the dot (the decimal part excludes the '.').
// If no '.' is present the returned slice contains a single element equal to s.
func splitDecimal(s string) []string {
	for i, c := range s {
		if c == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// ParseIntPart parses s as a base-10 integer and returns its int64 value.
// It accepts an optional leading '-' to denote a negative number and otherwise
// requires only digits 0-9. If s contains any non-digit (or an unexpected
// leading '+'), it returns an error describing the invalid rune.
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
// FormatLarge abbreviates large floating-point numbers using million and billion units.
// If n is greater than or equal to BillionThreshold it returns a string like "~X.X billion" with one decimal place.
// If n is greater than or equal to LargeNumberThreshold it returns a string like "~X.X million" with one decimal place.
// For values below those thresholds it returns a comma-separated integer representation of n rounded to the nearest integer.
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