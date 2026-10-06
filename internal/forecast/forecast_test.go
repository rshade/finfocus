package forecast

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProject_LinearAndFlat(t *testing.T) {
	t.Parallel()
	rate := 0.10
	got, err := Project([]Resource{
		{ID: "ec2", Provider: "aws", Monthly: 100, Currency: "USD", GrowthType: "none"},
		{ID: "bucket", Provider: "aws", Monthly: 100, Currency: "usd", GrowthType: "linear", GrowthRate: &rate},
	}, 1, time.Date(2026, 10, 15, 12, 0, 0, 0, time.FixedZone("EDT", -4*3600)))
	require.NoError(t, err)
	assert.Equal(t, "USD", got.Currency)
	assert.Equal(t, 1, got.Months)
	require.Len(t, got.Forecast.Points, 2)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), got.Forecast.Points[0].Time)
	assert.InDelta(t, 200, got.Forecast.Points[0].Value, 1e-9)
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), got.Forecast.Points[1].Time)
	assert.InDelta(t, 210, got.Forecast.Points[1].Value, 1e-9)
	require.Len(t, got.ByProvider, 1)
	assert.Equal(t, "aws", got.ByProvider[0].Name)
	assert.InDelta(t, 210, got.ByProvider[0].Points[1].Value, 1e-9)
	assert.Empty(t, got.Warnings)
}

func TestProject_Exponential(t *testing.T) {
	t.Parallel()
	rate := 0.10
	got, err := Project([]Resource{
		{ID: "logs", Provider: "gcp", Monthly: 100, Currency: "USD", GrowthType: "exponential", GrowthRate: &rate},
	}, 2, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, got.Forecast.Points, 3)
	assert.InDelta(t, 100, got.Forecast.Points[0].Value, 1e-9)
	assert.InDelta(t, 110, got.Forecast.Points[1].Value, 1e-9)
	assert.InDelta(t, 121, got.Forecast.Points[2].Value, 1e-9)
}

func TestProject_ProvidersStaySeparate(t *testing.T) {
	t.Parallel()
	rate := 0.50
	got, err := Project([]Resource{
		{ID: "a", Provider: "aws-native", Monthly: 10, Currency: "USD", GrowthType: "linear", GrowthRate: &rate},
		{ID: "g", Provider: "google", Monthly: 20, Currency: "USD", GrowthType: "none"},
	}, 1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, got.ByProvider, 2)
	assert.Equal(t, "aws", got.ByProvider[0].Name)
	assert.Equal(t, "gcp", got.ByProvider[1].Name)
	assert.InDelta(t, 15, got.ByProvider[0].Points[1].Value, 1e-9)
	assert.InDelta(t, 20, got.ByProvider[1].Points[1].Value, 1e-9)
	assert.InDelta(t, 35, got.Forecast.Points[1].Value, 1e-9)
}

func TestProject_MixedCurrency(t *testing.T) {
	t.Parallel()
	_, err := Project([]Resource{
		{ID: "a", Monthly: 1, Currency: "USD"},
		{ID: "b", Monthly: 1, Currency: "eur"},
	}, 1, time.Now())
	require.ErrorIs(t, err, ErrMixedCurrencies)
	assert.Contains(t, err.Error(), "EUR")
	assert.Contains(t, err.Error(), "USD")
}

func TestProject_BlankCurrencyAdoptsExplicit(t *testing.T) {
	t.Parallel()
	got, err := Project([]Resource{
		{ID: "a", Monthly: 1, Currency: "gbp"},
		{ID: "b", Monthly: 2},
	}, 1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, "GBP", got.Currency)
	assert.InDelta(t, 3, got.Forecast.Points[0].Value, 1e-9)
}

func TestProject_NoPricedResource(t *testing.T) {
	t.Parallel()
	_, err := Project(nil, 1, time.Now())
	require.ErrorIs(t, err, ErrNoCostData)
}

func TestProject_MonthsBounds(t *testing.T) {
	t.Parallel()
	for _, months := range []int{0, -1, 37} {
		_, err := Project([]Resource{{Monthly: 1, Currency: "USD"}}, months, time.Now())
		require.ErrorIs(t, err, ErrInvalidMonths)
	}
}

func TestProject_MissingRateStaysFlat(t *testing.T) {
	t.Parallel()
	got, err := Project([]Resource{
		{ID: "bucket", Monthly: 40, Currency: "USD", GrowthType: "linear"},
	}, 3, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	for _, point := range got.Forecast.Points {
		assert.InDelta(t, 40, point.Value, 1e-9)
	}
	require.NotEmpty(t, got.Warnings)
	assert.Contains(t, got.Warnings[0], "bucket")
	assert.Contains(t, got.Warnings[0], "no rate")
}

func TestProject_ClampsNegativeLinear(t *testing.T) {
	t.Parallel()
	rate := -1.0
	got, err := Project([]Resource{
		{ID: "shrink", Monthly: 100, Currency: "USD", GrowthType: "linear", GrowthRate: &rate},
	}, 2, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.InDelta(t, 100, got.Forecast.Points[0].Value, 1e-9)
	assert.InDelta(t, 0, got.Forecast.Points[1].Value, 1e-9)
	assert.InDelta(t, 0, got.Forecast.Points[2].Value, 1e-9)
	assert.Contains(t, strings.Join(got.Warnings, "\n"), "clamped at zero")
}

func TestProject_HighRateWarns(t *testing.T) {
	t.Parallel()
	rate := 1.5
	got, err := Project([]Resource{
		{ID: "burst", Monthly: 10, Currency: "USD", GrowthType: "linear", GrowthRate: &rate},
	}, 1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.InDelta(t, 25, got.Forecast.Points[1].Value, 1e-9)
	assert.Contains(t, strings.Join(got.Warnings, "\n"), "exceeds 100%")
}

func TestProject_NonFiniteBaseClamped(t *testing.T) {
	t.Parallel()
	got, err := Project([]Resource{
		{ID: "bad", Monthly: math.NaN(), Currency: "USD"},
		{ID: "ok", Monthly: 5, Currency: "USD"},
	}, 1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.InDelta(t, 5, got.Forecast.Points[0].Value, 1e-9)
	assert.Contains(t, strings.Join(got.Warnings, "\n"), "bad")
}

func TestProject_UnknownGrowthType(t *testing.T) {
	t.Parallel()
	got, err := Project([]Resource{
		{ID: "x", Monthly: 8, Currency: "USD", GrowthType: "quadratic"},
	}, 1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.InDelta(t, 8, got.Forecast.Points[1].Value, 1e-9)
	assert.Contains(t, strings.Join(got.Warnings, "\n"), "unknown growth type")
}

func TestProject_InvalidRateStaysFlat(t *testing.T) {
	t.Parallel()
	rate := -1.5
	got, err := Project([]Resource{
		{ID: "x", Monthly: 8, Currency: "USD", GrowthType: "linear", GrowthRate: &rate},
	}, 1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.InDelta(t, 8, got.Forecast.Points[1].Value, 1e-9)
	assert.Contains(t, strings.Join(got.Warnings, "\n"), "cost stays flat")
}
