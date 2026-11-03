package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func largeSpotResults() []engine.CostResult {
	return []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance", ResourceID: "big-spot", Currency: "USD", Monthly: 1234567.89,
			PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.12,
		},
		{ResourceType: "aws:ec2:Instance", ResourceID: "std", Currency: "USD", Monthly: 73},
	}
}

func TestNewResultTable_LargeSpotCostKeepsTag(t *testing.T) {
	t.Parallel()

	tbl := NewResultTable(largeSpotResults(), 10)
	rows := tbl.Rows()
	require.Len(t, rows, 2)
	assert.Equal(t, "$1234567.89 SPOT", rows[0][3])
	assert.Equal(t, "$73.00", rows[1][3])

	cols := tbl.Columns()
	require.Len(t, cols, 6)
	assert.Equal(t, len("$1234567.89 SPOT"), cols[3].Width)
	assert.Equal(t, 40, cols[0].Width)
	assert.Equal(t, 30, cols[1].Width)
	assert.Equal(t, 10, cols[2].Width)
	assert.Equal(t, 15, cols[4].Width)
	assert.Equal(t, 15, cols[5].Width)
}

func TestNewResultTable_SmallCostKeepsDefaultWidth(t *testing.T) {
	t.Parallel()

	tbl := NewResultTable(largeSpotResults()[1:], 10)
	assert.Equal(t, 15, tbl.Columns()[3].Width)
}

func TestGolden_CostTable_LargeSpotCost(t *testing.T) {
	t.Parallel()

	tbl := NewResultTable(largeSpotResults(), 10)
	output := tbl.View()
	testGolden(t, "cost_table_large_spot", output)
	assert.Contains(t, ansi.Strip(output), "$1234567.89 SPOT")
}
