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
