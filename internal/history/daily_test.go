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

func TestPointsFromObservations_RejectsASumThatOverflows(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	_, err := PointsFromObservations("urn:big", []ObservedCost{
		{At: day, Amount: math.MaxFloat64, Currency: "USD"},
		{At: day.Add(time.Hour), Amount: math.MaxFloat64, Currency: "USD"},
	}, day)
	require.Error(t, err, "each bucket is finite but the day's sum is +Inf and would fail the whole write")
	assert.Contains(t, err.Error(), "urn:big")
}
