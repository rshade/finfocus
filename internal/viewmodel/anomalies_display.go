package viewmodel

import (
	"fmt"
	"math"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

const (
	anomalyDateLayout     = "Jan 2"
	anomalyDateYearLayout = "Jan 2, 2006"
	anomalyPeriodJoiner   = " – "
)

// AnomalyDisplay is the text the web page shows for one anomaly. The browser
// formats nothing, so every amount, the drift, and the period are built here.
type AnomalyDisplay struct {
	Baseline  string `json:"baseline"`
	Observed  string `json:"observed"`
	Deviation string `json:"deviation"`
	Drift     string `json:"drift"`
	Period    string `json:"period"`
}

// BuildAnomalyDisplay formats one anomaly. Amounts use the symbol of the
// anomaly's own currency, so a euro row never reads as dollars.
func BuildAnomalyDisplay(a engine.Anomaly) AnomalyDisplay {
	return AnomalyDisplay{
		Baseline:  anomalyAmount(a.Baseline, a.Currency, false),
		Observed:  anomalyAmount(a.Observed, a.Currency, false),
		Deviation: anomalyAmount(a.Deviation, a.Currency, true),
		Drift:     fmt.Sprintf("%+.1f%%", a.PercentDrift),
		Period:    anomalyPeriod(a.Period),
	}
}

// AnomalySummary is the count line shared by the web status line.
func AnomalySummary(shown, considered int) string {
	return fmt.Sprintf("%d of %d resources", shown, considered)
}

func anomalyAmount(amount float64, currency string, signed bool) string {
	if currency == "" {
		currency = defaultCurrency
	}
	sign := ""
	switch {
	case amount < 0:
		sign = "-"
	case signed && amount > 0:
		sign = "+"
	}
	digits := strings.TrimPrefix(FormatMoneyShort(math.Abs(amount)), "$")
	return sign + CurrencySymbol(currency) + digits
}

func anomalyPeriod(p engine.DateRange) string {
	if p.Start.IsZero() && p.End.IsZero() {
		return ""
	}
	if p.Start.Year() == p.End.Year() {
		return p.Start.Format(anomalyDateLayout) + anomalyPeriodJoiner + p.End.Format(anomalyDateYearLayout)
	}
	return p.Start.Format(anomalyDateYearLayout) + anomalyPeriodJoiner + p.End.Format(anomalyDateYearLayout)
}
