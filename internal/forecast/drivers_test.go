package forecast

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExclude(t *testing.T) {
	t.Parallel()
	got, err := ParseExclude(" 2025-12-01:2025-12-31 ")
	require.NoError(t, err)
	assert.Equal(t, DateRange{From: day(2025, 12, 1), To: day(2025, 12, 31)}, got)

	for raw, want := range map[string]string{
		"2025-12-01":            `invalid exclude range "2025-12-01": use FROM:TO with dates as 2006-01-02`,
		"2025-12-01:bad":        `invalid exclude range "2025-12-01:bad": use FROM:TO with dates as 2006-01-02`,
		"2025-12-31:2025-12-01": `invalid exclude range "2025-12-31:2025-12-01": FROM is after TO`,
	} {
		_, err = ParseExclude(raw)
		require.EqualError(t, err, want, raw)
		assert.True(t, IsInputError(err))
	}
}

func TestParseStep(t *testing.T) {
	t.Parallel()
	got, err := ParseStep("2027-06-15=-250.5")
	require.NoError(t, err)
	assert.Equal(t, Step{Month: day(2027, 6, 1), Amount: -250.5}, got)

	for raw, want := range map[string]string{
		"2027-06-01":      `invalid step "2027-06-01": use DATE=AMOUNT with DATE as 2006-01-02`,
		"June=500":        `invalid step "June=500": use DATE=AMOUNT with DATE as 2006-01-02`,
		"2027-06-01=lots": `invalid step "2027-06-01=lots": amount must be a number`,
		"2027-06-01=NaN":  `invalid step "2027-06-01=NaN": amount must be a number`,
	} {
		_, err = ParseStep(raw)
		require.EqualError(t, err, want, raw)
		assert.True(t, IsInputError(err))
	}
}

func TestValidateGrowthCap(t *testing.T) {
	t.Parallel()
	require.NoError(t, ValidateGrowthCap(1.25))
	for _, bad := range []float64{0, -1, math.Inf(1), math.NaN()} {
		err := ValidateGrowthCap(bad)
		require.Error(t, err)
		assert.True(t, IsInputError(err))
	}
}

func dailyHistory(values ...float64) SpendHistory {
	points := make([]Point, len(values))
	for i, v := range values {
		points[i] = Point{Time: day(2025, 12, 1).AddDate(0, 0, i), Value: v}
	}
	return SpendHistory{Cadence: CadenceDaily, Points: points}
}

func TestDriversExclude_InterpolatesInside(t *testing.T) {
	t.Parallel()
	history := dailyHistory(10, 20, 900, 800, 50, 60)
	drivers := Drivers{Excludes: []DateRange{{From: day(2025, 12, 3), To: day(2025, 12, 4)}}}

	got, notes := drivers.Exclude(history)
	assert.Equal(t, []float64{10, 20, 30, 40, 50, 60}, valuesOf(got.Points))
	assert.Equal(t, history.Points[2].Time, got.Points[2].Time)
	assert.Equal(t, []string{"spend history: interpolated 2 points in excluded ranges"}, notes)
	assert.InDelta(t, 900, history.Points[2].Value, 0, "the input is not modified")
}

func TestDriversExclude_DropsEnds(t *testing.T) {
	t.Parallel()
	history := dailyHistory(900, 20, 30, 40, 900)
	drivers := Drivers{Excludes: []DateRange{
		{From: day(2025, 11, 1), To: day(2025, 12, 1)},
		{From: day(2025, 12, 5), To: day(2025, 12, 9)},
	}}

	got, notes := drivers.Exclude(history)
	assert.Equal(t, []float64{20, 30, 40}, valuesOf(got.Points))
	assert.Equal(t, day(2025, 12, 2), got.Points[0].Time)
	assert.Equal(t, []string{"spend history: dropped 2 points in excluded ranges at its start or end"}, notes)
}

func TestDriversExclude_MonthlyOverlap(t *testing.T) {
	t.Parallel()
	history := SpendHistory{Cadence: CadenceMonthly, Points: []Point{
		{Time: day(2025, 10, 1), Value: 100},
		{Time: day(2025, 11, 1), Value: 200},
		{Time: day(2025, 12, 1), Value: 9000},
		{Time: day(2026, 1, 1), Value: 400},
	}}
	got, _ := Drivers{Excludes: []DateRange{{From: day(2025, 12, 20), To: day(2025, 12, 24)}}}.Exclude(history)
	assert.Equal(t, []float64{100, 200, 300, 400}, valuesOf(got.Points))
}

func statProjection() Projection {
	start := day(2026, 10, 1)
	return Projection{
		Forecast: Series{Name: SeriesForecast, Points: stamp(start, []float64{100, 110, 120, 130})},
		Lower:    Series{Name: SeriesLower, Points: stamp(start, []float64{90, 60, -40, -20})},
		Upper:    Series{Name: SeriesUpper, Points: stamp(start, []float64{110, 160, 200, 240})},
	}
}

func TestDriversApply_StepsCapFloor(t *testing.T) {
	t.Parallel()
	p := statProjection()
	drivers := Drivers{
		Steps: []Step{
			{Month: day(2026, 12, 1), Amount: 10},
			{Month: day(2026, 1, 1), Amount: -5},
		},
		GrowthCap: 1.2,
	}

	notes := drivers.Apply(&p)
	// Month 0 after steps is 95, so the cap is 114.
	assert.Equal(t, []float64{95, 105, 114, 114}, valuesOf(p.Forecast.Points))
	assert.Equal(t, []float64{85, 55, 0, 0}, valuesOf(p.Lower.Points))
	assert.Equal(t, []float64{105, 114, 114, 114}, valuesOf(p.Upper.Points))
	assert.Equal(t, []string{"growth cap 1.2 limited 2 months to 114.00"}, notes)
}

func TestDriversApply_FloorsForecast(t *testing.T) {
	t.Parallel()
	p := statProjection()
	notes := Drivers{Steps: []Step{{Month: day(2026, 11, 1), Amount: -125}}}.Apply(&p)
	assert.Equal(t, []float64{100, 0, 0, 5}, valuesOf(p.Forecast.Points))
	assert.Equal(t, []string{"2 forecast months were below zero and are shown as zero"}, notes)
}

func TestDriversApply_GrowthLeavesProvidersAlone(t *testing.T) {
	t.Parallel()
	start := day(2026, 10, 1)
	p := Projection{
		Forecast:   Series{Name: SeriesForecast, Points: stamp(start, []float64{100, 200})},
		ByProvider: []Series{{Name: "aws", Points: stamp(start, []float64{100, 200})}},
	}
	notes := Drivers{GrowthCap: 1.5}.Apply(&p)
	assert.Equal(t, []float64{100, 150}, valuesOf(p.Forecast.Points))
	assert.Equal(t, []float64{100, 200}, valuesOf(p.ByProvider[0].Points))
	assert.Empty(t, p.Lower.Points)
	assert.Len(t, notes, 1)
}

func TestDriversApply_NoDriversIsNoOp(t *testing.T) {
	t.Parallel()
	p := statProjection()
	p.Lower.Points[2].Value = 1
	p.Lower.Points[3].Value = 2
	want := statProjection()
	want.Lower.Points[2].Value = 1
	want.Lower.Points[3].Value = 2
	assert.Empty(t, Drivers{}.Apply(&p))
	assert.Equal(t, want, p)
	assert.True(t, Drivers{}.Empty())
	assert.False(t, Drivers{GrowthCap: 2}.Empty())
}

func TestDriversApply_CapIgnoredWhenMonthZeroNotPositive(t *testing.T) {
	t.Parallel()
	p := statProjection()
	notes := Drivers{Steps: []Step{{Month: day(2026, 10, 1), Amount: -100}}, GrowthCap: 1.5}.Apply(&p)
	assert.Equal(t, []float64{0, 10, 20, 30}, valuesOf(p.Forecast.Points))
	assert.Contains(t, notes, "growth cap ignored: the current month is not above zero")
}
