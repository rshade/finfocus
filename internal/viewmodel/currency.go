package viewmodel

const defaultCurrency = "USD"

// CurrencySymbol returns the currency symbol used by TUI price displays.
func CurrencySymbol(currency string) string {
	// Mapping of ISO 4217 currency codes to their symbols.
	switch currency {
	case defaultCurrency:
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY", "CNY":
		return "¥"
	case "CAD":
		return "C$"
	case "AUD":
		return "A$"
	case "CHF":
		return "CHF"
	case "INR":
		return "₹"
	case "KRW":
		return "₩"
	default:
		// Fall back to currency code for unknown currencies
		return currency
	}
}
