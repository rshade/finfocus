package history

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDailyFileNameFor(t *testing.T) {
	t.Parallel()

	name, err := DailyFileNameFor("demo", "dev")
	require.NoError(t, err)
	assert.Equal(t, "demo@dev.daily.db", name)
	assert.True(t, IsDailyActualsFile(name))
	assert.False(t, IsDailyActualsFile("demo@dev.history.db"))
	assert.False(t, IsCostHistoryFile(name))

	legacy, err := DailyFileNameFor("", "dev")
	require.NoError(t, err)
	assert.Equal(t, "dev.daily.db", legacy)

	_, err = DailyFileNameFor("demo", "../dev")
	require.Error(t, err)
}

func TestPointsFromObservations_GroupsDaysAndCurrencies(t *testing.T) {
	t.Parallel()

	written := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	day := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	points, err := PointsFromObservations("urn:a", []ObservedCost{
		{At: day, Amount: 1, Currency: "USD"},
		{At: day.Add(time.Hour), Amount: 2, Currency: "USD"},
		{At: day, Amount: 0, Currency: "EUR"},
		{At: time.Time{}, Amount: 9, Currency: "USD"},
	}, written)
	require.NoError(t, err)
	require.Len(t, points, 2)
	assert.InDelta(t, 3, points[0].Amount, 0)
	assert.Equal(t, "USD", points[0].Currency)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), points[0].Day)
	assert.InDelta(t, 0, points[1].Amount, 0)
	assert.Equal(t, "EUR", points[1].Currency)

	absent, err := PointsFromObservations("urn:a", nil, written)
	require.NoError(t, err)
	assert.Empty(t, absent)

	_, err = PointsFromObservations("urn:a", []ObservedCost{{
		At: day, Amount: math.NaN(), Currency: "USD",
	}}, written)
	require.ErrorContains(t, err, "not a finite amount")
}

func TestPointsForSeries_DenseZeroAndCurrencies(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	written := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	points, err := PointsForSeries("urn:a", "USD", start, end, []float64{1, 0, 3}, written)
	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), points[0].Day)
	assert.Equal(t, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), points[1].Day)
	assert.InDelta(t, 0, points[1].Amount, 0)
	assert.Equal(t, "USD", points[2].Currency)

	blank, err := PointsForSeries("urn:a", "  ", start, end, []float64{1, 2, 3}, written)
	require.NoError(t, err)
	assert.Equal(t, "USD", blank[0].Currency)

	eur, err := PointsForSeries("urn:a", "EUR", start, end, []float64{4, 5, 6}, written)
	require.NoError(t, err)
	assert.Equal(t, "EUR", eur[0].Currency)
	assert.NotEqual(t, points[0].Currency, eur[0].Currency)
}

func TestPointsForSeries_AbsentAndMismatch(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	written := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	absent, err := PointsForSeries("urn:a", "USD", start, end, nil, written)
	require.NoError(t, err)
	assert.Empty(t, absent)

	empty, err := PointsForSeries("urn:a", "USD", start, end, []float64{}, written)
	require.NoError(t, err)
	assert.Empty(t, empty)

	_, err = PointsForSeries("urn:a", "USD", start, end, []float64{1, 2}, written)
	require.ErrorContains(t, err, "2 points for a 3-day window")

	_, err = PointsForSeries("urn:a", "USD", time.Time{}, end, []float64{1, 2, 3}, written)
	require.ErrorContains(t, err, "missing a start or end")

	_, err = PointsForSeries("urn:a", "USD", start, end, []float64{1, 2, math.NaN()}, written)
	require.ErrorContains(t, err, "not a finite amount")
}
