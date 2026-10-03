package engine_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func projectedBreakdownResults() []engine.CostResult {
	return []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "web",
			Adapter:      "aws-plugin",
			Currency:     "USD",
			Monthly:      100,
			Hourly:       0.137,
			Breakdown: map[string]float64{
				"storage": 40,
				"compute": 60,
				"":        9,
			},
		},
		{
			ResourceType: "aws:s3/bucket:Bucket",
			ResourceID:   "logs",
			Adapter:      "aws-plugin",
			Currency:     "USD",
			Monthly:      5,
			Hourly:       0.0068,
		},
	}
}

func TestRenderCostTable_BreakdownSubrows(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := engine.RenderCostTable(&buf, projectedBreakdownResults(), engine.CostTableOptions{ShowBreakdown: true})
	require.NoError(t, err)

	output := buf.String()
	compute := strings.Index(output, "  ├─ compute")
	storage := strings.Index(output, "  └─ storage")
	require.NotEqual(t, -1, compute)
	require.NotEqual(t, -1, storage)
	assert.Less(t, compute, storage)
	assert.NotContains(t, output, "├─ storage")
	assert.Contains(t, lineContaining(t, output, "  ├─ compute"), "60.00")
	assert.Contains(t, lineContaining(t, output, "  └─ storage"), "40.00")
	assert.NotContains(t, output, "9.00")

	logs := strings.Index(output, "aws:s3/bucket:Bucket/logs")
	require.NotEqual(t, -1, logs)
	assert.Less(t, storage, logs)
}

func TestRenderCostTable_OmitsEmptyBreakdown(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		breakdown map[string]float64
	}{
		{name: "nil"},
		{name: "empty", breakdown: map[string]float64{}},
		{name: "blank key", breakdown: map[string]float64{"": 4}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := []engine.CostResult{{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "web",
				Currency:     "USD",
				Monthly:      1,
				Breakdown:    tt.breakdown,
			}}
			var buf bytes.Buffer
			err := engine.RenderCostTable(&buf, results, engine.CostTableOptions{ShowBreakdown: true})
			require.NoError(t, err)
			assert.NotContains(t, buf.String(), "├─")
			assert.NotContains(t, buf.String(), "└─")
		})
	}
}

func TestRenderCostTable_SingleComponentUsesLastPrefix(t *testing.T) {
	t.Parallel()

	results := []engine.CostResult{{
		ResourceType: "aws:lambda/function:Function",
		ResourceID:   "fn",
		Currency:     "USD",
		Monthly:      1,
		Breakdown:    map[string]float64{"idle": 0},
	}}
	var buf bytes.Buffer
	err := engine.RenderCostTable(&buf, results, engine.CostTableOptions{ShowBreakdown: true})
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "  └─ idle")
	assert.Contains(t, lineContaining(t, output, "  └─ idle"), "0.00")
	assert.NotContains(t, output, "├─")
}

func TestRenderCostTable_FlagOffOmitsSubrows(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := engine.RenderCostTable(&buf, projectedBreakdownResults(), engine.CostTableOptions{})
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "RESOURCE DETAILS")
	assert.Contains(t, output, "100.00")
	assert.NotContains(t, output, "├─")
	assert.NotContains(t, output, "└─")
}

func TestRenderActualCostTable_BreakdownAndConfidence(t *testing.T) {
	t.Parallel()

	results := []engine.CostResult{{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "i-001",
		Adapter:      "aws-plugin",
		Currency:     "USD",
		TotalCost:    100,
		CostPeriod:   "1 month",
		Confidence:   engine.ConfidenceHigh,
		Breakdown: map[string]float64{
			"network": 10,
			"compute": 70,
			"storage": 20,
		},
	}}

	var shown bytes.Buffer
	err := engine.RenderActualCostTable(&shown, results, engine.CostTableOptions{
		ShowBreakdown:  true,
		ShowConfidence: true,
	})
	require.NoError(t, err)
	output := shown.String()
	assert.Contains(t, output, "Confidence")
	assert.Contains(t, output, "HIGH")
	compute := strings.Index(output, "  ├─ compute")
	network := strings.Index(output, "  ├─ network")
	storage := strings.Index(output, "  └─ storage")
	require.NotEqual(t, -1, compute)
	require.NotEqual(t, -1, network)
	require.NotEqual(t, -1, storage)
	assert.Less(t, compute, network)
	assert.Less(t, network, storage)
	assert.Contains(t, lineContaining(t, output, "  ├─ compute"), "70.00")
	assert.Contains(t, lineContaining(t, output, "  └─ storage"), "20.00")

	var hidden bytes.Buffer
	err = engine.RenderActualCostTable(&hidden, results, engine.CostTableOptions{})
	require.NoError(t, err)
	plain := hidden.String()
	assert.NotContains(t, plain, "Confidence")
	assert.NotContains(t, plain, "HIGH")
	assert.NotContains(t, plain, "├─")
	assert.NotContains(t, plain, "└─")
}

func TestRenderActualCostResults_DoesNotAddBreakdownSubrows(t *testing.T) {
	t.Parallel()

	results := []engine.CostResult{{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "i-001",
		Currency:     "USD",
		TotalCost:    10,
		Confidence:   engine.ConfidenceMedium,
		Breakdown:    map[string]float64{"compute": 10},
	}}
	var buf bytes.Buffer
	err := engine.RenderActualCostResults(&buf, engine.OutputTable, results, true)
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "Confidence")
	assert.Contains(t, output, "MEDIUM")
	assert.NotContains(t, output, "├─")
	assert.NotContains(t, output, "└─")
}

func TestRenderResults_JSONIgnoresComponentSubrows(t *testing.T) {
	t.Parallel()

	results := projectedBreakdownResults()
	var asJSON bytes.Buffer
	err := engine.RenderResults(&asJSON, engine.OutputJSON, results)
	require.NoError(t, err)
	var asNDJSON bytes.Buffer
	err = engine.RenderResults(&asNDJSON, engine.OutputNDJSON, results)
	require.NoError(t, err)

	for _, output := range []string{asJSON.String(), asNDJSON.String()} {
		assert.NotContains(t, output, "├─")
		assert.NotContains(t, output, "└─")
		assert.Contains(t, output, `"compute"`)
	}
}

func TestRenderCostTable_TrendColumn(t *testing.T) {
	t.Parallel()

	results := []engine.CostResult{{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "web",
		Adapter:      "aws-plugin",
		Currency:     "USD",
		Monthly:      10,
		Hourly:       0.0137,
		Breakdown:    map[string]float64{"compute": 10},
	}}
	var plain bytes.Buffer
	err := engine.RenderCostTable(&plain, results, engine.CostTableOptions{})
	require.NoError(t, err)
	assert.NotContains(t, plain.String(), "Trend")

	var shown bytes.Buffer
	err = engine.RenderCostTable(&shown, results, engine.CostTableOptions{
		ShowBreakdown: true,
		Trends:        map[string]string{"web": "▁▂▃▄▅▆▇"},
		TotalTrend:    "▂▃▄▅▆▇█",
	})
	require.NoError(t, err)
	output := shown.String()
	assert.Contains(t, output, "▂▃▄▅▆▇█")
	assert.Contains(t, output, "▁▂▃▄▅▆▇")
	header := lineContaining(t, output, "Recommendations")
	assert.Contains(t, header, "Trend")
	assert.Contains(t, header, "Hourly")
	assert.Contains(t, output, "  └─ compute")

	var actual bytes.Buffer
	err = engine.RenderActualCostTable(&actual, []engine.CostResult{{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "web",
		Adapter:      "aws-plugin",
		Currency:     "USD",
		TotalCost:    10,
		CostPeriod:   "30 days",
	}}, engine.CostTableOptions{
		Trends:     map[string]string{"missing": "▁"},
		TotalTrend: "█▇▅",
	})
	require.NoError(t, err)
	actualOut := actual.String()
	assert.Contains(t, actualOut, "█▇▅")
	actualHeader := lineContaining(t, actualOut, "Total Cost")
	assert.Contains(t, actualHeader, "Trend")
	assert.Contains(t, actualHeader, "Period")
	row := lineContaining(t, actualOut, "aws:ec2/instance:Instance/web")
	assert.Contains(t, row, "30 days")
	assert.NotContains(t, row, "▁")
}

func lineContaining(t *testing.T, output, fragment string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, fragment) {
			return line
		}
	}
	t.Fatalf("output missing %q", fragment)
	return ""
}
