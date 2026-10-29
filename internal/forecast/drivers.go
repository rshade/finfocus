package forecast

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// DateRange is an inclusive range of UTC days.
type DateRange struct {
	From time.Time
	To   time.Time
}

// Step adds Amount to every forecast month from Month (a month start) on.
type Step struct {
	Month  time.Time
	Amount float64
}

// Drivers are known future changes applied on top of a forecast. Excludes
// apply to spend history before fitting; Steps and GrowthCap apply to the
// forecast afterwards. A GrowthCap of zero is no cap.
type Drivers struct {
	Excludes  []DateRange
	Steps     []Step
	GrowthCap float64
}

// Empty reports whether no driver is set.
func (d Drivers) Empty() bool {
	return len(d.Excludes) == 0 && len(d.Steps) == 0 && d.GrowthCap == 0
}

// ParseExclude reads FROM:TO with both dates as 2006-01-02.
func ParseExclude(raw string) (DateRange, error) {
	trimmed := strings.TrimSpace(raw)
	fromRaw, toRaw, ok := strings.Cut(trimmed, ":")
	from, fromErr := time.Parse(spendDateLayout, fromRaw)
	to, toErr := time.Parse(spendDateLayout, toRaw)
	if !ok || fromErr != nil || toErr != nil {
		return DateRange{}, inputError(fmt.Errorf(
			"invalid exclude range %q: use FROM:TO with dates as 2006-01-02", trimmed))
	}
	if from.After(to) {
		return DateRange{}, inputError(fmt.Errorf("invalid exclude range %q: FROM is after TO", trimmed))
	}
	return DateRange{From: from, To: to}, nil
}

// ParseStep reads DATE=AMOUNT with DATE as 2006-01-02. The step starts in
// DATE's month.
func ParseStep(raw string) (Step, error) {
	trimmed := strings.TrimSpace(raw)
	dateRaw, amountRaw, ok := strings.Cut(trimmed, "=")
	when, err := time.Parse(spendDateLayout, strings.TrimSpace(dateRaw))
	if !ok || err != nil {
		return Step{}, inputError(fmt.Errorf("invalid step %q: use DATE=AMOUNT with DATE as 2006-01-02", trimmed))
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(amountRaw), 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return Step{}, inputError(fmt.Errorf("invalid step %q: amount must be a number", trimmed))
	}
	return Step{Month: monthStart(when), Amount: amount}, nil
}

// ValidateGrowthCap rejects a cap that is not a positive, finite factor.
func ValidateGrowthCap(factor float64) error {
	if math.IsNaN(factor) || math.IsInf(factor, 0) || factor <= 0 {
		return inputError(fmt.Errorf("growth cap must be a positive factor such as 1.25, got %v", factor))
	}
	return nil
}

// Exclude replaces spend inside the excluded ranges with a straight line
// between the nearest kept points, and drops excluded points at either end,
// which have no neighbor. The input is not modified.
func (d Drivers) Exclude(history SpendHistory) (SpendHistory, []string) {
	if len(d.Excludes) == 0 {
		return history, nil
	}
	points := append([]Point(nil), history.Points...)
	excluded := make([]bool, len(points))
	for i, point := range points {
		excluded[i] = d.excludes(history.Cadence, point.Time)
	}
	first, last := 0, len(points)-1
	for first <= last && excluded[first] {
		first++
	}
	for last >= first && excluded[last] {
		last--
	}
	dropped := len(points) - (last - first + 1)
	interpolated := 0
	for i := first; i <= last; i++ {
		if !excluded[i] {
			continue
		}
		left := i - 1
		right := i
		for excluded[right] {
			right++
		}
		for j := i; j < right; j++ {
			share := float64(j-left) / float64(right-left)
			points[j].Value = points[left].Value + share*(points[right].Value-points[left].Value)
			interpolated++
		}
		i = right
	}
	var notes []string
	if interpolated > 0 {
		notes = append(notes, fmt.Sprintf("spend history: interpolated %d points in excluded ranges", interpolated))
	}
	if dropped > 0 {
		notes = append(notes, fmt.Sprintf(
			"spend history: dropped %d points in excluded ranges at its start or end", dropped))
	}
	if first > last {
		return SpendHistory{Cadence: history.Cadence}, notes
	}
	return SpendHistory{Cadence: history.Cadence, Points: points[first : last+1]}, notes
}

// excludes reports whether a point falls in a range. A monthly point covers
// its whole month, so any overlap excludes it.
func (d Drivers) excludes(cadence Cadence, when time.Time) bool {
	end := when
	if cadence == CadenceMonthly {
		end = when.AddDate(0, 1, -1)
	}
	for _, r := range d.Excludes {
		if !when.After(r.To) && !end.Before(r.From) {
			return true
		}
	}
	return false
}

// Apply adds the steps (a step before month 0 applies from month 0), caps every month at GrowthCap times month 0 (after the
// steps), and floors at zero. It changes the forecast and, when present, its
// bounds; provider series stay the undriven projection.
func (d Drivers) Apply(p *Projection) []string {
	series := []*Series{&p.Forecast, &p.Lower, &p.Upper}
	for _, step := range d.Steps {
		forEachPoint(series, func(_ *Series, point *Point) {
			if !point.Time.Before(step.Month) {
				point.Value += step.Amount
			}
		})
	}
	var notes []string
	if note := d.applyCap(p, series); note != "" {
		notes = append(notes, note)
	}
	if note := floorAtZero(p, series); note != "" {
		notes = append(notes, note)
	}
	return notes
}

// applyCap limits every point to GrowthCap times month 0 of the forecast.
func (d Drivers) applyCap(p *Projection, series []*Series) string {
	if d.GrowthCap <= 0 || len(p.Forecast.Points) == 0 {
		return ""
	}
	if p.Forecast.Points[0].Value <= 0 {
		return "growth cap ignored: the current month is not above zero"
	}
	limit := d.GrowthCap * p.Forecast.Points[0].Value
	capped := 0
	forEachPoint(series, func(s *Series, point *Point) {
		if point.Value <= limit {
			return
		}
		if s == &p.Forecast {
			capped++
		}
		point.Value = limit
	})
	if capped == 0 {
		return ""
	}
	return fmt.Sprintf("growth cap %v limited %d months to %.2f", d.GrowthCap, capped, limit)
}

// floorAtZero shows every negative point as zero.
func floorAtZero(p *Projection, series []*Series) string {
	negative := 0
	forEachPoint(series, func(s *Series, point *Point) {
		if point.Value >= 0 {
			return
		}
		if s == &p.Forecast {
			negative++
		}
		point.Value = 0
	})
	if negative == 0 {
		return ""
	}
	return fmt.Sprintf("%d forecast months were below zero and are shown as zero", negative)
}

func forEachPoint(series []*Series, fn func(*Series, *Point)) {
	for _, s := range series {
		for i := range s.Points {
			fn(s, &s.Points[i])
		}
	}
}
