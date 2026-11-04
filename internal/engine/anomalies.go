package engine

import (
	"math"
	"slices"
	"sort"

	"github.com/rshade/finfocus/internal/history"
)

const (
	// AnomalySeverityWarning is the only severity: a row is emitted only when
	// its evidence already crosses a documented cutoff.
	AnomalySeverityWarning = "warning"
	// AnomalyMethodDrift marks an anomaly derived from overview run-rate drift.
	AnomalyMethodDrift = "drift"
	// AnomalyMethodRobustMedian marks an anomaly derived from one resource's
	// own recent daily actuals: the latest complete day against the median of
	// the days before it.
	AnomalyMethodRobustMedian = "robust-median"
)

const (
	// robustMedianMinBaseline is the number of qualifying earlier daily points
	// in one currency a resource needs before its latest day is scored. A
	// missing day is a gap and does not count; it is never filled with zero.
	robustMedianMinBaseline = 14
	// robustMedianZCutoff is the absolute robust z-score above which a day is
	// reported, on either side of the median. It is a documented cutoff, not a
	// probability or confidence, and deliberately not a config key.
	robustMedianZCutoff = 3.5
	// robustZNormalScore scales a MAD-based deviation onto the scale of a
	// standard z-score for a normal sample (0.6745 is the MAD of the standard
	// normal distribution in formula terms). Cost days are not normal.
	robustZNormalScore = 0.6745
	// medianHalfDivisor splits an even-length sorted sample between its two
	// middle values.
	medianHalfDivisor = 2
)

// Anomaly is one resource whose monthly run-rate or daily cost has diverged
// from its own reference. For drift rows Baseline and Observed are monthly
// figures and Observed is the extrapolated run-rate, never the raw
// month-to-date spend; for robust-median rows they are one day's amount and
// the median of the earlier days.
type Anomaly struct {
	Resource     string    `json:"resource"`
	Type         string    `json:"type"`
	Period       DateRange `json:"period"`
	Baseline     float64   `json:"baseline"`
	Observed     float64   `json:"observed"`
	Deviation    float64   `json:"deviation"`
	PercentDrift float64   `json:"percentDrift,omitempty"`
	Severity     string    `json:"severity"`
	Method       string    `json:"method"`
	Currency     string    `json:"currency"`
	// BaselineWindow is the first and last day inside the median for
	// robust-median rows, so a reader can see what was compared. Drift rows
	// leave it unset.
	BaselineWindow *DateRange `json:"baselineWindow,omitempty"`
}

// AnomalyReport is the anomaly list together with the number of rows the
// overview produced, so an empty list differs from a failed load.
type AnomalyReport struct {
	Considered int       `json:"considered"`
	Anomalies  []Anomaly `json:"anomalies"`
}

// SelectAnomalies projects overview rows onto the rows that carry drift data
// or a qualifying daily series. It applies its own thresholds in two places
// only: the drift path reuses the overview's own picks, and the daily path
// needs robustMedianMinBaseline earlier points and a robust z-score of at
// least robustMedianZCutoff.
//
// Series holds complete UTC days inside the requested window, one point per
// resource, day, and currency; the loaders drop the current partial day, and
// neither here nor anywhere else is a missing day turned into a zero. A
// resource's series is split by currency and each side is scored on its own;
// the row reports the qualifying currency with the larger absolute deviation.
//
// Per resource URN a statistical row replaces the drift row when it
// qualifies, and otherwise the drift row stands, so a resource appears at
// most once. A row with an error is never an anomaly, and neither is a row a
// preview marks for deletion: it has no forward projection, but enrichment
// still prices it and a web preview can change a row's status after its drift
// was set, so the status is checked here. The result is ordered by absolute
// deviation, largest first, then by resource URN. Neither input is modified.
func SelectAnomalies(rows []OverviewRow, series []history.DailyPoint) AnomalyReport {
	report := AnomalyReport{Considered: len(rows), Anomalies: []Anomaly{}}
	statistical := scoreDailyAnomalies(series)
	presented := make(map[string]struct{}, len(rows))
	for i := range rows {
		row := &rows[i]
		if row.Error != nil || row.Status == StatusDeleting {
			continue
		}
		urn := row.URN
		if _, dup := presented[urn]; dup {
			// One row per resource: the first occurrence stands.
			continue
		}
		stat, ok := statistical.pick(urn)
		if ok {
			stat.Type = row.Type
			report.Anomalies = append(report.Anomalies, stat)
			presented[urn] = struct{}{}
			continue
		}
		if row.CostDrift == nil {
			continue
		}
		anomaly := Anomaly{
			Resource:     urn,
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
		presented[urn] = struct{}{}
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

// statisticalSeries scores every series in points and keeps the
// qualifying rows keyed by resource URN. Points reaching one currency are
// scored on their own, and the same currency is never compared across
// resources.
//
// pick returns the row to emit for one resource: the qualified currency with
// the larger absolute deviation, ties broken by currency code, or no row when
// no currency qualifies.
type statisticalSeries map[string][]Anomaly

func (s statisticalSeries) pick(urn string) (Anomaly, bool) {
	candidates := s[urn]
	if len(candidates) == 0 {
		return Anomaly{}, false
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if strongerDeviation(candidate, best) {
			best = candidate
		}
	}
	return best, true
}

// strongerDeviation reports whether candidate's absolute deviation exceeds
// best's, or matches it with the alphabetically smaller currency.
func strongerDeviation(candidate, best Anomaly) bool {
	if da, db := math.Abs(candidate.Deviation), math.Abs(best.Deviation); da != db {
		return da > db
	}
	return candidate.Currency < best.Currency
}

// scoreDailyAnomalies evaluates, per resource and currency, whether the
// latest stored day diverges from the median of the days before it by
// robustMedianZCutoff robust z-scores or more.
func scoreDailyAnomalies(series []history.DailyPoint) statisticalSeries {
	if len(series) == 0 {
		return nil
	}
	// Group by resource, then currency, so two currencies are never one
	// series.
	type grouped struct {
		byCurrency map[string][]history.DailyPoint
	}
	byResource := make(map[string]*grouped)
	for _, point := range series {
		group, ok := byResource[point.ResourceID]
		if !ok {
			group = &grouped{byCurrency: make(map[string][]history.DailyPoint)}
			byResource[point.ResourceID] = group
		}
		group.byCurrency[point.Currency] = append(group.byCurrency[point.Currency], point)
	}

	result := make(statisticalSeries, len(byResource))
	for urn, group := range byResource {
		for currency, points := range group.byCurrency {
			slices.SortFunc(points, func(a, b history.DailyPoint) int {
				return a.Day.Compare(b.Day)
			})
			if candidate, ok := dailySeriesAnomaly(urn, currency, points); ok {
				result[urn] = append(result[urn], candidate)
			}
		}
	}
	return result
}

// dailySeriesAnomaly scores one resource-currency series: the latest point is
// the observation, every earlier point is the baseline, and the row is
// proposed only when the baseline reaches robustMedianMinBaseline points, the
// MAD is not zero, and the absolute robust z crosses robustMedianZCutoff.
func dailySeriesAnomaly(urn, currency string, points []history.DailyPoint) (Anomaly, bool) {
	observed := points[len(points)-1]
	baseline := points[:len(points)-1]
	if len(baseline) < robustMedianMinBaseline {
		return Anomaly{}, false
	}
	amounts := make([]float64, len(baseline))
	for i, point := range baseline {
		amounts[i] = point.Amount
	}
	median := medianOf(amounts)
	deviations := make([]float64, len(baseline))
	for i, amount := range amounts {
		deviations[i] = math.Abs(amount - median)
	}
	deviation := medianOf(deviations)
	if deviation == 0 {
		// A flat series hides a spike from the MAD; no second threshold
		// catches that here.
		return Anomaly{}, false
	}
	z := robustZNormalScore * (observed.Amount - median) / deviation
	if math.Abs(z) < robustMedianZCutoff {
		return Anomaly{}, false
	}

	anomaly := Anomaly{
		Resource:  urn,
		Period:    DateRange{Start: observed.Day, End: observed.Day},
		Baseline:  median,
		Observed:  observed.Amount,
		Deviation: observed.Amount - median,
		Severity:  AnomalySeverityWarning,
		Method:    AnomalyMethodRobustMedian,
		Currency:  currency,
	}
	// A zero median leaves the percent drift undefined; the deviation still
	// reports the day's gap.
	if median != 0 {
		anomaly.PercentDrift = anomaly.Deviation / median * driftPercentMultiplier
	}
	first, last := baseline[0], baseline[len(baseline)-1]
	anomaly.BaselineWindow = &DateRange{Start: first.Day, End: last.Day}
	return anomaly, true
}

// medianOf returns the median of values: the middle value for an odd count,
// the average of the two middle values for an even count. It does not modify
// the input.
func medianOf(values []float64) float64 {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	middle := len(sorted) / medianHalfDivisor
	if len(sorted)%medianHalfDivisor != 0 {
		// An odd count leaves one middle value.
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / medianHalfDivisor
}
