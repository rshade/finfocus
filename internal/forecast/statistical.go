package forecast

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/rshade/goarima/autoarima"
	"github.com/rshade/goarima/timeseries"
)

// DefaultConfidence is the bound level when --confidence is omitted.
const DefaultConfidence = 0.95

const (
	constantModelOrder = "constant"
	oneDay             = 24 * time.Hour
)

// ValidateConfidence rejects a bound level outside (0, 1).
func ValidateConfidence(confidence float64) error {
	if math.IsNaN(confidence) || confidence <= 0 || confidence >= 1 {
		return inputError(fmt.Errorf("confidence must be between 0 and 1 (exclusive), got %v", confidence))
	}
	return nil
}

// ProjectStatistical fits an automatically selected (seasonal) ARIMA model to
// spend history, after drivers.Exclude, and returns calendar-month totals from
// start's month through months ahead, with bounds at confidence. Daily spend
// is forecast per day and summed per month. Days the history records count at
// their recorded value, excluded or not: an exclusion keeps a value out of
// the fit, not out of spend that already happened. Monthly bounds from daily
// data are the sum of daily bounds, which can only overstate their width.
// Steps and the cap are applied separately, by Drivers.Apply.
func ProjectStatistical(
	history SpendHistory,
	drivers Drivers,
	months int,
	start time.Time,
	confidence float64,
	currency string,
) (Projection, error) {
	if err := ValidateMonths(months); err != nil {
		return Projection{}, err
	}
	if err := ValidateConfidence(confidence); err != nil {
		return Projection{}, err
	}
	fitHistory, notes := drivers.Exclude(history)
	n := len(fitHistory.Points)
	if n < autoarima.MinObservations {
		return Projection{}, inputError(fmt.Errorf(
			"spend history has %d usable points; at least %d are required", n, autoarima.MinObservations))
	}

	origin := monthStart(start)
	end := origin.AddDate(0, months+1, 0)
	steps := forecastSteps(fitHistory, end)
	fit, err := fitSpend(fitHistory.Cadence, valuesOf(fitHistory.Points), steps, confidence)
	if err != nil {
		return Projection{}, err
	}
	notes = append(notes, fit.warnings...)
	notes = append(notes, coverageNotes(history, origin)...)

	totals := make([]float64, months+1)
	lower := make([]float64, months+1)
	upper := make([]float64, months+1)
	add := func(when time.Time, value, lo, hi float64) {
		m := monthIndex(when) - monthIndex(origin)
		if m < 0 || m > months {
			return
		}
		totals[m] += value
		lower[m] += lo
		upper[m] += hi
	}
	for _, point := range history.Points {
		add(point.Time, point.Value, point.Value, point.Value)
	}
	recordedLast := history.Points[len(history.Points)-1].Time
	fitLast := fitHistory.Points[n-1].Time
	for i := range fit.forecast {
		when := stepTime(history.Cadence, fitLast, i+1)
		if !when.After(recordedLast) {
			continue
		}
		add(when, fit.forecast[i], fit.lower[i], fit.upper[i])
	}

	return Projection{
		Currency:   normalizeCurrency(currency),
		Months:     months,
		Forecast:   Series{Name: SeriesForecast, Points: stamp(origin, totals)},
		Lower:      Series{Name: SeriesLower, Points: stamp(origin, lower)},
		Upper:      Series{Name: SeriesUpper, Points: stamp(origin, upper)},
		Spend:      Series{Name: SeriesSpend, Points: append([]Point(nil), history.Points...)},
		Model:      ModelARIMA,
		ModelOrder: fit.order,
		Confidence: confidence,
		Warnings:   notes,
	}, nil
}

// coverageNotes warns when the history does not reach month 0 from both
// sides: it starts after month 0 begins, so the days before it are in no
// total, or it ends before the previous month, so the forecast spans a gap.
func coverageNotes(history SpendHistory, origin time.Time) []string {
	var notes []string
	first := history.Points[0].Time
	last := history.Points[len(history.Points)-1].Time
	if first.After(origin) && monthIndex(first) == monthIndex(origin) {
		notes = append(notes, fmt.Sprintf(
			"spend history starts %s; %s covers only the days from then on",
			first.Format(spendDateLayout), origin.Format("Jan 2006")))
	}
	if gap := monthIndex(origin) - monthIndex(last) - 1; gap > 0 {
		notes = append(notes, fmt.Sprintf(
			"spend history ends %s, %d months before %s; the forecast spans that gap",
			last.Format(spendDateLayout), gap, origin.Format("Jan 2006")))
	}
	return notes
}

type spendFit struct {
	forecast, lower, upper []float64
	order                  string
	warnings               []string
}

func fitSpend(cadence Cadence, values []float64, steps int, confidence float64) (spendFit, error) {
	cfg := autoarima.DefaultConfig()
	cfg.SeasonalPeriods = autoarima.DailyPeriods
	if cadence == CadenceMonthly {
		cfg.SeasonalPeriods = []int{12}
	}
	result, err := autoarima.AutoARIMA(timeseries.New(values), cfg)
	if errors.Is(err, autoarima.ErrNoModel) && constant(values) {
		flat := make([]float64, steps)
		for i := range flat {
			flat[i] = values[0]
		}
		return spendFit{
			forecast: flat, lower: flat, upper: flat,
			order:    constantModelOrder,
			warnings: []string{"spend history is constant; forecasting a flat line"},
		}, nil
	}
	if errors.Is(err, autoarima.ErrNoModel) {
		return spendFit{}, inputError(errors.New("no time-series model could be fitted to the spend history"))
	}
	if err != nil {
		return spendFit{}, fmt.Errorf("fitting spend history: %w", err)
	}
	fit := spendFit{order: result.Order()}
	if steps == 0 {
		return fit, nil
	}
	fit.forecast, fit.lower, fit.upper, err = result.PredictWithInterval(steps, confidence)
	if err != nil {
		return spendFit{}, fmt.Errorf("forecasting spend history: %w", err)
	}
	for i := range fit.forecast {
		for _, v := range []float64{fit.forecast[i], fit.lower[i], fit.upper[i]} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return spendFit{}, fmt.Errorf("the %s model produced a non-finite forecast", fit.order)
			}
		}
	}
	return fit, nil
}

// forecastSteps counts the periods after the last recorded point and before
// end, the first instant after the horizon.
func forecastSteps(history SpendHistory, end time.Time) int {
	last := history.Points[len(history.Points)-1].Time
	if history.Cadence == CadenceMonthly {
		return max(monthIndex(end)-monthIndex(last)-1, 0)
	}
	return max(int(end.Sub(last)/oneDay)-1, 0)
}

func stepTime(cadence Cadence, last time.Time, step int) time.Time {
	if cadence == CadenceMonthly {
		return last.AddDate(0, step, 0)
	}
	return last.AddDate(0, 0, step)
}

func constant(values []float64) bool {
	for _, v := range values[1:] {
		if v != values[0] {
			return false
		}
	}
	return true
}
