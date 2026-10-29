package forecast

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// weeklySpend is a daily spend with a trend and a weekend dip, plus a small
// deterministic wobble so the series is not perfectly regular.
func weeklySpend(first time.Time, days int) (SpendHistory, func(time.Time) float64) {
	truth := func(d time.Time) float64 {
		i := d.Sub(first).Hours() / 24
		v := 50 + 0.1*i
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			v -= 15
		}
		return v
	}
	points := make([]Point, days)
	for i := range points {
		d := first.AddDate(0, 0, i)
		points[i] = Point{Time: d, Value: truth(d) + float64((i*37)%11)/10 - 0.5}
	}
	return SpendHistory{Cadence: CadenceDaily, Points: points}, truth
}

func monthTruth(truth func(time.Time) float64, month time.Time) float64 {
	total := 0.0
	for d := month; d.Before(month.AddDate(0, 1, 0)); d = d.AddDate(0, 0, 1) {
		total += truth(d)
	}
	return total
}

func assertBoundsContain(t *testing.T, p Projection) {
	t.Helper()
	require.Len(t, p.Lower.Points, len(p.Forecast.Points))
	require.Len(t, p.Upper.Points, len(p.Forecast.Points))
	for i, point := range p.Forecast.Points {
		assert.LessOrEqual(t, p.Lower.Points[i].Value, point.Value+1e-9, "lower at %s", point.Time)
		assert.GreaterOrEqual(t, p.Upper.Points[i].Value, point.Value-1e-9, "upper at %s", point.Time)
		assert.Equal(t, point.Time, p.Lower.Points[i].Time)
	}
}

func TestProjectStatistical_DailyWeeklyTrend(t *testing.T) {
	t.Parallel()
	history, truth := weeklySpend(day(2026, 4, 4), 180) // through 2026-09-30
	require.Equal(t, day(2026, 9, 30), history.Points[len(history.Points)-1].Time)

	p, err := ProjectStatistical(history, Drivers{}, 12, day(2026, 10, 15), DefaultConfidence, "")
	require.NoError(t, err)
	assert.Equal(t, ModelARIMA, p.Model)
	assert.Contains(t, p.ModelOrder, "[7]", "weekday pattern needs a weekly seasonal model")
	assert.Equal(t, "USD", p.Currency)
	assert.InDelta(t, DefaultConfidence, p.Confidence, 1e-12)
	require.Len(t, p.Forecast.Points, 13)
	assert.Equal(t, day(2026, 10, 1), p.Forecast.Points[0].Time)
	assert.Equal(t, day(2027, 10, 1), p.Forecast.Points[12].Time)
	for m := range p.Forecast.Points {
		point := p.Forecast.Points[m]
		want := monthTruth(truth, point.Time)
		assert.InEpsilon(t, want, point.Value, 0.05, "month %s", point.Time.Format("2006-01"))
	}
	assertBoundsContain(t, p)
	assert.Equal(t, history.Points, p.Spend.Points)
	assert.Equal(t, SeriesSpend, p.Spend.Name)
}

func TestProjectStatistical_PartialCurrentMonthUsesRecordedDays(t *testing.T) {
	t.Parallel()
	history, _ := weeklySpend(day(2026, 5, 14), 150) // through 2026-10-10
	last := history.Points[len(history.Points)-1].Time
	require.Equal(t, day(2026, 10, 10), last)
	recorded := 0.0
	for _, point := range history.Points {
		if !point.Time.Before(day(2026, 10, 1)) {
			recorded += point.Value
		}
	}

	p, err := ProjectStatistical(history, Drivers{}, 1, day(2026, 10, 15), DefaultConfidence, "USD")
	require.NoError(t, err)
	october := p.Forecast.Points[0].Value
	assert.Greater(t, october, recorded)
	perDay := (october - recorded) / 21
	assert.InDelta(t, recorded/10, perDay, recorded/10*0.25, "21 forecast days resemble the recorded days")
	assertBoundsContain(t, p)
}

func TestProjectStatistical_Monthly(t *testing.T) {
	t.Parallel()
	points := make([]Point, 24)
	for i := range points {
		points[i] = Point{Time: day(2024, 10, 1).AddDate(0, i, 0), Value: 1000 + 25*float64(i) + float64((i*7)%5)}
	}
	history := SpendHistory{Cadence: CadenceMonthly, Points: points} // through 2026-09

	p, err := ProjectStatistical(history, Drivers{}, 3, day(2026, 10, 2), 0.8, "EUR")
	require.NoError(t, err)
	assert.Equal(t, "EUR", p.Currency)
	require.Len(t, p.Forecast.Points, 4)
	assert.InEpsilon(t, 1000+25*24.0, p.Forecast.Points[0].Value, 0.05)
	assert.Greater(t, p.Forecast.Points[3].Value, p.Forecast.Points[0].Value)
	assertBoundsContain(t, p)
}

func TestProjectStatistical_ConstantIsFlat(t *testing.T) {
	t.Parallel()
	points := make([]Point, 30)
	for i := range points {
		points[i] = Point{Time: day(2026, 9, 1).AddDate(0, 0, i), Value: 250}
	}

	p, err := ProjectStatistical(
		SpendHistory{Cadence: CadenceDaily, Points: points},
		Drivers{},
		2,
		day(2026, 10, 3),
		0.95,
		"",
	)
	require.NoError(t, err)
	assert.Equal(t, "constant", p.ModelOrder)
	assert.Contains(t, p.Warnings, "spend history is constant; forecasting a flat line")
	assert.InDelta(t, 31*250.0, p.Forecast.Points[0].Value, 1e-9)
	assert.InDelta(t, 30*250.0, p.Forecast.Points[1].Value, 1e-9)
	assert.InDelta(t, 31*250.0, p.Upper.Points[2].Value, 1e-9)
}

func TestProjectStatistical_Deterministic(t *testing.T) {
	t.Parallel()
	history, _ := weeklySpend(day(2026, 4, 4), 180)
	a, err := ProjectStatistical(history, Drivers{}, 6, day(2026, 10, 15), 0.9, "USD")
	require.NoError(t, err)
	b, err := ProjectStatistical(history, Drivers{}, 6, day(2026, 10, 15), 0.9, "USD")
	require.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestProjectStatistical_InputErrors(t *testing.T) {
	t.Parallel()
	short := SpendHistory{Cadence: CadenceDaily, Points: make([]Point, 9)}
	for i := range short.Points {
		short.Points[i] = Point{Time: day(2026, 9, 1).AddDate(0, 0, i), Value: float64(i)}
	}
	_, err := ProjectStatistical(short, Drivers{}, 3, day(2026, 10, 1), 0.95, "")
	require.EqualError(t, err, "spend history has 9 usable points; at least 10 are required")
	assert.True(t, IsInputError(err))

	history, _ := weeklySpend(day(2026, 4, 4), 60)
	for _, confidence := range []float64{0, 1, -0.1, math.NaN()} {
		_, err = ProjectStatistical(history, Drivers{}, 3, day(2026, 10, 1), confidence, "")
		require.Error(t, err)
		assert.True(t, IsInputError(err))
	}
	_, err = ProjectStatistical(history, Drivers{}, 37, day(2026, 10, 1), 0.9, "")
	require.ErrorIs(t, err, ErrInvalidMonths)
}

func TestValidateConfidence(t *testing.T) {
	t.Parallel()
	require.NoError(t, ValidateConfidence(0.5))
	require.EqualError(t, ValidateConfidence(1), "confidence must be between 0 and 1 (exclusive), got 1")
}

func TestProjectStatistical_ExcludedRecordedDaysKeepTheirValue(t *testing.T) {
	t.Parallel()
	history, _ := weeklySpend(day(2026, 5, 14), 150)   // through 2026-10-10
	history.Points[len(history.Points)-3].Value = 5000 // a spike on 2026-10-08
	recorded := 0.0
	for _, point := range history.Points {
		if !point.Time.Before(day(2026, 10, 1)) {
			recorded += point.Value
		}
	}
	excludeSpike := Drivers{Excludes: []DateRange{{From: day(2026, 10, 8), To: day(2026, 10, 8)}}}
	excludeTail := Drivers{Excludes: []DateRange{{From: day(2026, 10, 8), To: day(2026, 10, 10)}}}

	spike, err := ProjectStatistical(history, excludeSpike, 1, day(2026, 10, 15), 0.9, "USD")
	require.NoError(t, err)
	assert.Contains(t, spike.Warnings, "spend history: interpolated 1 points in excluded ranges")
	assert.Greater(t, spike.Forecast.Points[0].Value, recorded, "the spike still happened this month")

	tail, err := ProjectStatistical(history, excludeTail, 1, day(2026, 10, 15), 0.9, "USD")
	require.NoError(t, err)
	assert.Contains(t, tail.Warnings, "spend history: dropped 3 points in excluded ranges at its start or end")
	assert.Greater(t, tail.Forecast.Points[0].Value, recorded)
	assert.Less(t, tail.Forecast.Points[0].Value, recorded+21*100,
		"recorded tail days are not forecast a second time")
	assert.Len(t, tail.Spend.Points, 150)
}

func TestProjectStatistical_CoverageNotes(t *testing.T) {
	t.Parallel()
	lateStart, _ := weeklySpend(day(2026, 10, 3), 12)
	p, err := ProjectStatistical(lateStart, Drivers{}, 1, day(2026, 10, 20), 0.9, "")
	require.NoError(t, err)
	assert.Contains(t, p.Warnings, "spend history starts 2026-10-03; Oct 2026 covers only the days from then on")

	stale, _ := weeklySpend(day(2026, 1, 1), 60) // through 2026-03-01
	p, err = ProjectStatistical(stale, Drivers{}, 1, day(2026, 10, 20), 0.9, "")
	require.NoError(t, err)
	assert.Contains(
		t,
		p.Warnings,
		"spend history ends 2026-03-01, 6 months before Oct 2026; the forecast spans that gap",
	)
}

func TestValidateSpendHistory_CountsAfterExclusion(t *testing.T) {
	t.Parallel()
	history, _ := weeklySpend(day(2026, 9, 1), 12)
	require.NoError(t, ValidateSpendHistory(history, Drivers{}))
	err := ValidateSpendHistory(history, Drivers{Excludes: []DateRange{{From: day(2026, 9, 1), To: day(2026, 9, 3)}}})
	require.EqualError(t, err, "spend history has 9 usable points; at least 10 are required")
	assert.True(t, IsInputError(err))
}
