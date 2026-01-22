package engine_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// TestRenderResults_TableFormat tests table output rendering.
func TestRenderResults_TableFormat(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      7.30,
			Hourly:       0.01,
			Notes:        "t3.micro instance",
		},
		{
			ResourceType: "aws:s3/bucket:Bucket",
			ResourceID:   "bucket-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      2.30,
			Hourly:       0.00315,
			Notes:        "Standard storage",
		},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "COST SUMMARY")
	assert.Contains(t, output, "Total Monthly Cost")
	assert.Contains(t, output, "9.60") // Sum of monthly costs
	assert.Contains(t, output, "USD")
	assert.Contains(t, output, "aws:ec2/instance:Instance")
	assert.Contains(t, output, "i-001")
	assert.Contains(t, output, "7.30")
	assert.Contains(t, output, "2.30")
}

// TestRenderResults_JSONFormat tests JSON output rendering.
func TestRenderResults_JSONFormat(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      7.30,
			Hourly:       0.01,
			Notes:        "t3.micro instance",
			Breakdown: map[string]float64{
				"compute": 7.30,
			},
		},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputJSON, results)
	require.NoError(t, err)

	output := buf.String()

	// Verify valid JSON - renderJSON wraps results in {"finfocus": ...}
	var wrapper struct {
		FinFocus engine.AggregatedResults `json:"finfocus"`
	}
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	// Verify structure
	aggregated := wrapper.FinFocus
	assert.Equal(t, 7.30, aggregated.Summary.TotalMonthly)
	assert.Equal(t, 0.01, aggregated.Summary.TotalHourly)
	assert.Equal(t, "USD", aggregated.Summary.Currency)
	assert.Len(t, aggregated.Resources, 1)
	assert.Equal(t, "i-001", aggregated.Resources[0].ResourceID)
}

// TestRenderResults_NDJSONFormat tests NDJSON output rendering.
func TestRenderResults_NDJSONFormat(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      7.30,
			Hourly:       0.01,
		},
		{
			ResourceType: "aws:s3/bucket:Bucket",
			ResourceID:   "bucket-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      2.30,
			Hourly:       0.00315,
		},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputNDJSON, results)
	require.NoError(t, err)

	output := buf.String()

	// Split into lines
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 2)

	// Verify each line is valid JSON
	for i, line := range lines {
		var result engine.CostResult
		err := json.Unmarshal([]byte(line), &result)
		require.NoError(t, err)
		assert.Equal(t, results[i].ResourceID, result.ResourceID)
	}
}

// TestRenderResults_EmptyResults tests rendering with no results.
func TestRenderResults_EmptyResults(t *testing.T) {
	results := []engine.CostResult{}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "COST SUMMARY")
	assert.Contains(t, output, "0.00") // Zero total
}

// TestRenderResults_UnsupportedFormat tests handling of invalid format.
func TestRenderResults_UnsupportedFormat(t *testing.T) {
	results := []engine.CostResult{
		{ResourceID: "i-001"},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, "invalid", results)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

// TestRenderResults_MultiProvider tests multi-provider aggregation in table.
func TestRenderResults_MultiProvider(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      10.0,
			Hourly:       0.014,
		},
		{
			ResourceType: "azure:compute/virtualMachine:VirtualMachine",
			ResourceID:   "vm-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      15.0,
			Hourly:       0.021,
		},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "BY PROVIDER")
	assert.Contains(t, output, "aws")
	assert.Contains(t, output, "azure")
	assert.Contains(t, output, "25.00") // Total
}

// TestRenderResults_WithBreakdown tests rendering with cost breakdowns.
func TestRenderResults_WithBreakdown(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:lambda/function:Function",
			ResourceID:   "func-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      5.0,
			Hourly:       0.007,
			Breakdown: map[string]float64{
				"compute":  3.0,
				"requests": 2.0,
			},
		},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputJSON, results)
	require.NoError(t, err)

	output := buf.String()

	// renderJSON wraps results in {"finfocus": ...}
	var wrapper struct {
		FinFocus engine.AggregatedResults `json:"finfocus"`
	}
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	aggregated := wrapper.FinFocus
	assert.Equal(t, 2, len(aggregated.Resources[0].Breakdown))
	assert.Equal(t, 3.0, aggregated.Resources[0].Breakdown["compute"])
	assert.Equal(t, 2.0, aggregated.Resources[0].Breakdown["requests"])
}

// TestRenderActualCostResults_TableFormat tests actual cost table rendering.
func TestRenderActualCostResults_TableFormat(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			TotalCost:    100.0,
			Monthly:      100.0,
			Hourly:       0.137,
			CostPeriod:   "1 month",
		},
	}

	var buf bytes.Buffer
	err := engine.RenderActualCostResults(&buf, engine.OutputTable, results, false)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "100.00")
	assert.Contains(t, output, "1 month")
}

// TestRenderActualCostResults_JSONFormat tests actual cost JSON rendering.
func TestRenderActualCostResults_JSONFormat(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-001",
			Adapter:      "test-plugin",
			Currency:     "USD",
			TotalCost:    100.0,
			CostPeriod:   "1 month",
		},
	}

	var buf bytes.Buffer
	err := engine.RenderActualCostResults(&buf, engine.OutputJSON, results, false)
	require.NoError(t, err)

	output := buf.String()

	var rendered []engine.CostResult
	err = json.Unmarshal([]byte(output), &rendered)
	require.NoError(t, err)
	assert.Len(t, rendered, 1)
	assert.Equal(t, 100.0, rendered[0].TotalCost)
}

// TestRenderActualCostResults_NDJSONFormat tests actual cost NDJSON rendering.
func TestRenderActualCostResults_NDJSONFormat(t *testing.T) {
	results := []engine.CostResult{
		{ResourceID: "i-001", TotalCost: 100.0},
		{ResourceID: "i-002", TotalCost: 200.0},
	}

	var buf bytes.Buffer
	err := engine.RenderActualCostResults(&buf, engine.OutputNDJSON, results, false)
	require.NoError(t, err)

	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 2)

	for i, line := range lines {
		var result engine.CostResult
		err := json.Unmarshal([]byte(line), &result)
		require.NoError(t, err)
		assert.Equal(t, results[i].ResourceID, result.ResourceID)
	}
}

// TestRenderResults_LongResourceName tests truncation of long resource names.
func TestRenderResults_LongResourceName(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-very-long-resource-id-that-should-be-truncated-for-display",
			Adapter:      "test-plugin",
			Currency:     "USD",
			Monthly:      7.30,
			Hourly:       0.01,
		},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()
	// Table should handle long names gracefully (may truncate or wrap)
	assert.Contains(t, output, "aws:ec2/instance:Instance")
}

// TestRenderResults_MultiAdapter tests multiple adapters in summary.
func TestRenderResults_MultiAdapter(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-001",
			Adapter:      "plugin1",
			Currency:     "USD",
			Monthly:      10.0,
			Hourly:       0.014,
		},
		{
			ResourceType: "aws:s3/bucket:Bucket",
			ResourceID:   "bucket-001",
			Adapter:      "plugin2",
			Currency:     "USD",
			Monthly:      5.0,
			Hourly:       0.007,
		},
		{
			ResourceType: "aws:rds/instance:Instance",
			ResourceID:   "db-001",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      20.0,
			Hourly:       0.027,
		},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "BY ADAPTER")
	assert.Contains(t, output, "plugin1")
	assert.Contains(t, output, "plugin2")
	assert.Contains(t, output, "local-spec")
}

// TestRenderResults_JSONPrettyPrint tests JSON pretty printing.
func TestRenderResults_JSONPrettyPrint(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceID: "i-001",
			Monthly:    7.30,
		},
	}

	var buf bytes.Buffer
	err := engine.RenderResults(&buf, engine.OutputJSON, results)
	require.NoError(t, err)

	output := buf.String()

	// Verify pretty printing (should have indentation)
	assert.Contains(t, output, "\n")
	assert.Contains(t, output, "  ") // Indentation

	// Verify it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)
}
