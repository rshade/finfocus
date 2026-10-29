package forecast

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDocument_JSONShape(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	projection := Projection{
		Currency: "USD",
		Months:   1,
		Forecast: Series{Name: SeriesForecast, Points: []Point{{Time: start, Value: 10}}},
		ByProvider: []Series{
			{Name: "aws", Points: []Point{{Time: start, Value: 10}}},
		},
		Warnings: []string{"w"},
	}
	history := Series{Name: SeriesHistory, Points: []Point{{Time: start.AddDate(0, -1, 0), Value: 8}}}

	doc := NewDocument(projection, SeriesList(projection, history), 50)
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"currency": "USD",
		"months": 1,
		"budget": 50,
		"series": [
			{"name": "history", "points": [{"time": "2026-09-01T00:00:00Z", "value": 8}]},
			{"name": "forecast", "points": [{"time": "2026-10-01T00:00:00Z", "value": 10}]},
			{"name": "aws", "points": [{"time": "2026-10-01T00:00:00Z", "value": 10}]}
		],
		"warnings": ["w"]
	}`, string(raw))
}

func TestNewDocument_OmitsBudgetAndEmptyHistory(t *testing.T) {
	t.Parallel()
	projection := Projection{Currency: "USD", Months: 1, Forecast: Series{Name: SeriesForecast}}

	doc := NewDocument(projection, SeriesList(projection, Series{}), 0)
	assert.Nil(t, doc.Budget)
	require.Len(t, doc.Series, 1)
	assert.Equal(t, SeriesForecast, doc.Series[0].Name)
}

func TestNewDocument_StatisticalFieldsAndOrder(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	one := func(name string, v float64) Series {
		return Series{Name: name, Points: []Point{{Time: start, Value: v}}}
	}
	projection := Projection{
		Currency:   "USD",
		Months:     1,
		Model:      ModelARIMA,
		ModelOrder: "ARIMA(1,1,0)",
		Confidence: 0.9,
		Forecast:   one(SeriesForecast, 10),
		Lower:      one(SeriesLower, 8),
		Upper:      one(SeriesUpper, 12),
		Baseline:   one(SeriesBaseline, 9),
		Spend:      one(SeriesSpend, 7),
	}
	doc := NewDocument(projection, SeriesList(projection, one(SeriesHistory, 6)), 0)
	assert.Equal(t, ModelARIMA, doc.Model)
	assert.Equal(t, "ARIMA(1,1,0)", doc.ModelOrder)
	require.NotNil(t, doc.Confidence)
	assert.InDelta(t, 0.9, *doc.Confidence, 1e-12)
	names := make([]string, len(doc.Series))
	for i, s := range doc.Series {
		names[i] = s.Name
	}
	assert.Equal(
		t,
		[]string{SeriesHistory, SeriesForecast, SeriesLower, SeriesUpper, SeriesBaseline, SeriesSpend},
		names,
	)
}

func TestNewDocument_GrowthOmitsStatisticalFields(t *testing.T) {
	t.Parallel()
	projection := Projection{Currency: "USD", Months: 1, Forecast: Series{Name: SeriesForecast}}
	raw, err := json.Marshal(NewDocument(projection, SeriesList(projection, Series{}), 0))
	require.NoError(t, err)
	assert.JSONEq(t, `{"currency":"USD","months":1,"series":[{"name":"forecast","points":null}]}`, string(raw))
}

func TestValidateMonths(t *testing.T) {
	t.Parallel()
	for _, months := range []int{MinMonths, DefaultMonths, MaxMonths} {
		require.NoError(t, ValidateMonths(months))
	}
	for _, months := range []int{0, MaxMonths + 1, -3} {
		err := ValidateMonths(months)
		require.ErrorIs(t, err, ErrInvalidMonths)
		assert.True(t, IsInputError(err))
		assert.Equal(t, ErrInvalidMonths.Error(), err.Error())
	}
}

func TestParseGrowth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		kind     string
		rate     string
		wantKind string
		wantRate *float64
		wantErr  string
		wantIs   error
	}{
		{name: "plugin growth", kind: "", rate: ""},
		{name: "plugin growth with rate", kind: "", rate: "0.05", wantRate: ptr(0.05)},
		{name: "linear", kind: " Linear ", rate: "0.10", wantKind: GrowthLinear, wantRate: ptr(0.10)},
		{name: "none ignores missing rate", kind: "none", wantKind: GrowthNone},
		{
			name:    "unknown type",
			kind:    "cubic",
			wantErr: `invalid growth type "cubic" (supported: none, linear, exponential)`,
		},
		{name: "unparseable rate", kind: "linear", rate: "ten", wantErr: `invalid growth rate "ten"`},
		{name: "rate below -1", kind: "linear", rate: "-1.5", wantIs: ErrInvalidGrowthRate},
		{name: "linear without rate", kind: "linear", wantIs: ErrMissingGrowthRate},
		{name: "exponential without rate", kind: "exponential", wantIs: ErrMissingGrowthRate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kind, rate, err := ParseGrowth(tt.kind, tt.rate)
			if tt.wantErr != "" || tt.wantIs != nil {
				require.Error(t, err)
				assert.True(t, IsInputError(err))
				if tt.wantErr != "" {
					require.EqualError(t, err, tt.wantErr)
				}
				if tt.wantIs != nil {
					require.ErrorIs(t, err, tt.wantIs)
					assert.Equal(t, tt.wantIs.Error(), err.Error())
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantKind, kind)
			if tt.wantRate == nil {
				assert.Nil(t, rate)
				return
			}
			require.NotNil(t, rate)
			assert.InDelta(t, *tt.wantRate, *rate, 1e-12)
		})
	}
}

func TestIsInputError_OtherErrors(t *testing.T) {
	t.Parallel()
	assert.False(t, IsInputError(nil))
	assert.False(t, IsInputError(errors.New("plugin unavailable")))
}

func TestIsUnprojectable(t *testing.T) {
	t.Parallel()
	_, noData := Project(nil, 1, time.Now())
	assert.True(t, IsUnprojectable(noData))
	_, mixed := Project([]Resource{
		{ID: "a", Monthly: 1, Currency: "USD"},
		{ID: "b", Monthly: 1, Currency: "EUR"},
	}, 1, time.Now())
	assert.True(t, IsUnprojectable(mixed))
	assert.False(t, IsUnprojectable(ErrInvalidMonths))
	assert.False(t, IsUnprojectable(errors.New("plugin unavailable")))
}

func ptr(v float64) *float64 { return &v }
