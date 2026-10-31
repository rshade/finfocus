package engine

import (
	"math"
	"sort"
)

const (
	// AnomalySeverityWarning is the only severity: a row is emitted only when
	// its absolute drift already crossed the overview warning threshold.
	AnomalySeverityWarning = "warning"
	// AnomalyMethodDrift marks an anomaly derived from overview run-rate drift.
	AnomalyMethodDrift = "drift"
)

// Anomaly is one resource whose month-to-date run-rate has diverged from the
// projected month. Baseline and Observed are both monthly figures; Observed is
// the extrapolated run-rate, never the raw month-to-date spend.
type Anomaly struct {
	Resource     string    `json:"resource"`
	Type         string    `json:"type"`
	Period       DateRange `json:"period"`
	Baseline     float64   `json:"baseline"`
	Observed     float64   `json:"observed"`
	Deviation    float64   `json:"deviation"`
	PercentDrift float64   `json:"percentDrift"`
	Severity     string    `json:"severity"`
	Method       string    `json:"method"`
	Currency     string    `json:"currency"`
}

// AnomalyReport is the anomaly list together with the number of rows the
// overview produced, so an empty list differs from a failed load.
type AnomalyReport struct {
	Considered int       `json:"considered"`
	Anomalies  []Anomaly `json:"anomalies"`
}

// SelectAnomalies projects overview rows onto the rows that carry drift data.
// It applies no threshold of its own: CalculateCostDrift has already skipped
// drift within the warning threshold, the first two days, new resources, and
// deleted resources. A row with an error is never an anomaly. The result is
// ordered by absolute deviation, largest first, then by resource URN. The
// input slice is not modified.
func SelectAnomalies(rows []OverviewRow) AnomalyReport {
	report := AnomalyReport{Considered: len(rows), Anomalies: []Anomaly{}}
	for i := range rows {
		row := &rows[i]
		if row.Error != nil || row.CostDrift == nil {
			continue
		}
		anomaly := Anomaly{
			Resource:     row.URN,
			Type:         row.Type,
			Baseline:     row.CostDrift.Projected,
			Observed:     row.CostDrift.ExtrapolatedMonthly,
			Deviation:    row.CostDrift.Delta,
			PercentDrift: row.CostDrift.PercentDrift,
			Severity:     AnomalySeverityWarning,
			Method:       AnomalyMethodDrift,
		}
		if row.ActualCost != nil {
			anomaly.Period = row.ActualCost.Period
			anomaly.Currency = row.ActualCost.Currency
		}
		report.Anomalies = append(report.Anomalies, anomaly)
	}
	sort.SliceStable(report.Anomalies, func(i, j int) bool {
		a, b := report.Anomalies[i], report.Anomalies[j]
		if da, db := math.Abs(a.Deviation), math.Abs(b.Deviation); da != db {
			return da > db
		}
		return a.Resource < b.Resource
	})
	return report
}
