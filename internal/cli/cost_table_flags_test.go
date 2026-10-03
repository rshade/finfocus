package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
)

func TestCostCommands_TableFlags(t *testing.T) {
	t.Parallel()

	projected := cli.NewCostProjectedCmd()
	breakdown := projected.Flags().Lookup("show-breakdown")
	require.NotNil(t, breakdown)
	assert.Equal(t, "bool", breakdown.Value.Type())
	assert.Equal(t, "false", breakdown.DefValue)

	actual := cli.NewCostActualCmd()
	for _, name := range []string{"show-breakdown", "show-confidence"} {
		flag := actual.Flags().Lookup(name)
		require.NotNil(t, flag, name)
		assert.Equal(t, "bool", flag.Value.Type())
		assert.Equal(t, "false", flag.DefValue)
	}
	estimate := actual.Flags().Lookup("estimate-confidence")
	require.NotNil(t, estimate)
}

func TestRenderCostOutput_ShowBreakdownTable(t *testing.T) {
	t.Parallel()

	output := renderProjected(t, "table", true)
	assert.Contains(t, output, "  ├─ compute")
	assert.Contains(t, output, "  └─ storage")
	assert.Contains(t, output, "60.00")
	assert.NotContains(t, output, "9.00")
}

func TestRenderCostOutput_JSONIgnoresShowBreakdown(t *testing.T) {
	t.Parallel()

	withFlag := renderProjected(t, "json", true)
	withoutFlag := renderProjected(t, "json", false)
	assert.Equal(t, withoutFlag, withFlag)
	assert.NotContains(t, withFlag, "├─")
	assert.NotContains(t, withFlag, "└─")
	assert.Contains(t, withFlag, `"compute"`)

	ndjson := renderProjected(t, "ndjson", true)
	assert.NotContains(t, ndjson, "├─")
	assert.Contains(t, ndjson, `"compute"`)
}

func TestRenderActualCostOutput_TableFlags(t *testing.T) {
	t.Parallel()

	output := renderActual(t, "table", false, true, true)
	assert.Contains(t, output, "Confidence")
	assert.Contains(t, output, "HIGH")
	assert.Contains(t, output, "  ├─ compute")
	assert.Contains(t, output, "  └─ storage")

	confidenceOnly := renderActual(t, "table", false, false, true)
	assert.Contains(t, confidenceOnly, "Confidence")
	assert.Contains(t, confidenceOnly, "HIGH")
	assert.NotContains(t, confidenceOnly, "├─")

	breakdownOnly := renderActual(t, "table", false, true, false)
	assert.NotContains(t, breakdownOnly, "Confidence")
	assert.NotContains(t, breakdownOnly, "HIGH")
	assert.Contains(t, breakdownOnly, "  └─ storage")
}

func TestRenderActualCostOutput_EstimateConfidenceStillFillsTableColumn(t *testing.T) {
	t.Parallel()

	output := renderActual(t, "table", true, true, false)
	assert.Contains(t, output, "Confidence")
	assert.Contains(t, output, "HIGH")
	assert.Contains(t, output, "  ├─ compute")
}

func TestRenderActualCostOutput_JSONIgnoresTableFlags(t *testing.T) {
	t.Parallel()

	withFlags := renderActual(t, "json", false, true, true)
	withoutFlags := renderActual(t, "json", false, false, false)
	assert.Equal(t, withoutFlags, withFlags)
	assert.NotContains(t, withFlags, "├─")
	assert.NotContains(t, withFlags, `"high"`)

	kept := renderActual(t, "json", true, false, false)
	assert.Contains(t, kept, `"high"`)
	assert.NotContains(t, kept, "├─")

	withTableFlags := renderActual(t, "ndjson", false, true, true)
	baseline := renderActual(t, "ndjson", false, false, false)
	assert.Equal(t, baseline, withTableFlags)
	assert.NotContains(t, withTableFlags, "├─")
}

func renderProjected(t *testing.T, format string, showBreakdown bool) string {
	t.Helper()
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cli.RenderCostOutput(context.Background(), cmd, format, projectedView(), showBreakdown)
	require.NoError(t, err)
	return buf.String()
}

func renderActual(t *testing.T, format string, estimate, showBreakdown, showConfidence bool) string {
	t.Helper()
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cli.RenderActualCostOutput(
		context.Background(), cmd, format, projectedView(), "", estimate, showBreakdown, showConfidence,
	)
	require.NoError(t, err)
	return buf.String()
}

func projectedView() *engine.CostResultWithErrors {
	return &engine.CostResultWithErrors{
		Results: []engine.CostResult{{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "web",
			Adapter:      "aws-plugin",
			Currency:     "USD",
			Monthly:      100,
			TotalCost:    100,
			CostPeriod:   "1 month",
			Confidence:   engine.ConfidenceHigh,
			Breakdown: map[string]float64{
				"storage": 40,
				"compute": 60,
				"":        9,
			},
		}},
	}
}
