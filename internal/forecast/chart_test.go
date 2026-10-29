package forecast

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_Golden(t *testing.T) {
	t.Parallel()
	rate := 0.10
	projection, err := Project([]Resource{
		{ID: "ec2", Provider: "aws", Monthly: 100, Currency: "USD", GrowthType: "none"},
		{ID: "bucket", Provider: "aws", Monthly: 50, Currency: "USD", GrowthType: "linear", GrowthRate: &rate},
	}, 6, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	out := Render(projection, ChartOptions{
		Height:         8,
		Width:          40,
		SplitProviders: true,
		Budget:         200,
		NoColor:        true,
	})
	path := filepath.Join("testdata", "chart.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(out), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), out)
	assert.Contains(t, out, "Forecast monthly cost (USD)")
	assert.Contains(t, out, "Jan 2026")
	assert.Contains(t, out, "Jul 2026")
	assert.Contains(t, out, "Legend: Forecast  AWS  Budget")
	assert.NotContains(t, out, "\x1b[")
}

func TestRender_EmptyAndSingle(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "No forecast points.\n", Render(Projection{}, ChartOptions{}))
	single := Render(Projection{
		Currency: "USD",
		Forecast: Series{Name: SeriesForecast, Points: []Point{{
			Time:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			Value: 12.5,
		}}},
	}, ChartOptions{})
	assert.Contains(t, single, "Mar 2026")
	assert.Contains(t, single, "12.50")
	assert.NotContains(t, single, "Legend:")
}

func TestRender_NoProviderSplitOmitsProvider(t *testing.T) {
	t.Parallel()
	projection, err := Project([]Resource{
		{ID: "a", Provider: "aws", Monthly: 10, Currency: "USD"},
		{ID: "b", Provider: "gcp", Monthly: 10, Currency: "USD"},
	}, 1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	out := Render(projection, ChartOptions{Height: 6, Width: 30, NoColor: true})
	assert.Contains(t, out, "Legend: Forecast")
	assert.NotContains(t, out, "AWS")
	assert.NotContains(t, out, "GCP")
}

func TestRender_StatisticalGolden(t *testing.T) {
	t.Parallel()
	start := day(2026, 10, 1)
	projection := Projection{
		Currency:   "USD",
		Months:     6,
		Model:      ModelARIMA,
		ModelOrder: "SARIMA(1,1,1)(0,1,1)[7]",
		Confidence: 0.8,
		Forecast: Series{
			Name:   SeriesForecast,
			Points: stamp(start, []float64{2000, 2050, 2110, 2160, 2230, 2280, 2350}),
		},
		Lower: Series{
			Name:   SeriesLower,
			Points: stamp(start, []float64{1950, 1900, 1850, 1780, 1720, 1650, 1590}),
		},
		Upper: Series{
			Name:   SeriesUpper,
			Points: stamp(start, []float64{2050, 2200, 2370, 2540, 2740, 2910, 3110}),
		},
		Baseline: Series{
			Name:   SeriesBaseline,
			Points: stamp(start, []float64{1900, 1900, 1900, 1900, 1900, 1900, 1900}),
		},
	}

	out := Render(projection, ChartOptions{Height: 10, Width: 50, SplitProviders: true, Budget: 2600, NoColor: true})
	path := filepath.Join("testdata", "chart_statistical.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(path, []byte(out), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), out)
	assert.Contains(t, out, "Forecast monthly cost (USD, "+projection.ModelOrder+")")
	assert.Contains(t, out, "Legend: Forecast  Lower  Upper  Baseline  Budget")
	assert.NotContains(t, out, "\x1b[")
}
