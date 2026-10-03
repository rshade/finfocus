package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChart_SingleSeries(t *testing.T) {
	t.Parallel()
	out := RenderChart([]CostSnapshot{
		sampleSnap(1, time.January, 800, map[string]float64{"aws": 800}),
		sampleSnap(2, time.June, 1200, map[string]float64{"aws": 1200}),
	}, nil, ChartOptions{Stack: "dev", Height: 8, Width: 40, NoBudget: true, NoAnnotations: true})
	assert.Contains(t, out, "Stack: dev")
	assert.Contains(t, out, "2 snapshots")
	assert.Contains(t, out, "Total")
	assert.Contains(t, out, "┤")
}

func TestChart_MultiProvider(t *testing.T) {
	t.Parallel()
	out := RenderChart([]CostSnapshot{
		sampleSnap(1, time.January, 100, map[string]float64{"aws": 80, "gcp": 20}),
		sampleSnap(2, time.March, 200, map[string]float64{"aws": 150, "gcp": 50}),
	}, nil, ChartOptions{Stack: "dev", Height: 8, Width: 40, SplitProviders: true, NoBudget: true})
	assert.Contains(t, out, "AWS")
	assert.Contains(t, out, "GCP")
}

func TestChart_BudgetOverlay(t *testing.T) {
	t.Parallel()
	out := RenderChart([]CostSnapshot{
		sampleSnap(1, time.January, 800, map[string]float64{"aws": 800}),
		sampleSnap(2, time.June, 2100, map[string]float64{"aws": 2100}),
	}, nil, ChartOptions{Stack: "dev", Height: 8, Width: 40, Budget: 2000})
	assert.Contains(t, out, "Budget")
}

func TestChart_Annotations(t *testing.T) {
	t.Parallel()
	snaps := []CostSnapshot{
		sampleSnap(12, time.January, 800, map[string]float64{"aws": 800}),
		sampleSnap(23, time.March, 1200, map[string]float64{"aws": 1200}),
	}
	out := RenderChart(snaps, []CostAnnotation{
		{Version: 12, Message: "Initial deployment", Kind: "update"},
		{Version: 23, Message: "Upgraded to m5.2xlarge", Kind: "update"},
	}, ChartOptions{Stack: "dev", Height: 8, Width: 40, NoBudget: true})
	assert.Contains(t, out, "Deployment Annotations:")
	assert.Contains(t, out, "Initial deployment")
	assert.Contains(t, out, "$800/mo")
	assert.Contains(t, out, "$1,200/mo")
	assert.Contains(t, out, "+$400")
}

func TestChart_EmptyData(t *testing.T) {
	t.Parallel()
	out := RenderChart(nil, nil, ChartOptions{Stack: "dev"})
	assert.Equal(t, "No cost history snapshots in the selected range.\n", out)
	filtered := RenderChart([]CostSnapshot{sampleSnap(1, time.January, 10, nil)}, nil, ChartOptions{
		Stack: "dev", From: date(2025, 6, 1),
	})
	assert.Equal(t, "No cost history snapshots in the selected range.\n", filtered)
}

func TestChart_SingleDataPoint(t *testing.T) {
	t.Parallel()
	out := RenderChart([]CostSnapshot{sampleSnap(12, time.January, 800, nil)}, nil, ChartOptions{Stack: "dev"})
	assert.Contains(t, out, "1 snapshot")
	assert.Contains(t, out, "v12")
	assert.NotContains(t, out, "Legend:")
	assert.NotContains(t, out, "┤")
}

func TestChart_ProviderFilter(t *testing.T) {
	t.Parallel()
	snaps := []CostSnapshot{
		sampleSnap(1, time.January, 100, map[string]float64{"aws": 80, "gcp": 20}),
		sampleSnap(2, time.February, 200, map[string]float64{"aws": 150, "gcp": 50}),
	}
	opt := ChartOptions{Stack: "dev", Provider: "aws", Height: 6, Width: 30, NoBudget: true, NoAnnotations: true}
	out := RenderChart(snaps, nil, opt)
	assert.Contains(t, out, "AWS")
	assert.NotContains(t, out, "GCP")
	opt.Provider = "AWS"
	assert.Equal(t, out, RenderChart(snaps, nil, opt))
}

func TestChart_SingleProvider(t *testing.T) {
	t.Parallel()
	out := RenderChart([]CostSnapshot{
		sampleSnap(1, time.January, 100, map[string]float64{"aws": 80, "gcp": 20}),
	}, nil, ChartOptions{Stack: "dev", Provider: "AWS"})
	assert.Contains(t, out, "$80")
	assert.NotContains(t, out, "$100")
	assert.NotContains(t, out, "$20")
}

func sampleSnap(version int, month time.Month, total float64, by map[string]float64) CostSnapshot {
	if by == nil {
		by = map[string]float64{}
	}
	return CostSnapshot{
		Timestamp:    date(2025, month, 15),
		Version:      version,
		TotalMonthly: total,
		Currency:     "USD",
		ByProvider:   by,
		ByType:       map[string]float64{},
		Resources:    []CostResource{},
	}
}
