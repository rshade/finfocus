package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverviewRowResult_SpotSavings(t *testing.T) {
	t.Parallel()

	row := spotOverviewRow("urn:a::saved", PricingCategoryDynamic, 0.12)
	onDemand := 65.79
	row.ProjectedCost.OnDemandEquivalentMonthly = &onDemand
	row.ProjectedCost.SpotSavingsPercent = SpotSavingsPercent(PricingCategoryDynamic, 25, &onDemand)

	result := ComputeOverviewRowResult(row)
	assert.Equal(t, "SPOT · risk 0.12 · -62% vs on-demand", result.SpotDisplay)

	data, err := json.Marshal(result)
	require.NoError(t, err)
	var decoded struct {
		ProjectedCost struct {
			OnDemand *float64 `json:"onDemandEquivalentMonthlyCost"`
			Percent  *float64 `json:"spotSavingsPercent"`
		} `json:"projectedCost"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.NotNil(t, decoded.ProjectedCost.OnDemand)
	require.NotNil(t, decoded.ProjectedCost.Percent)
	assert.InDelta(t, 65.79, *decoded.ProjectedCost.OnDemand, 1e-9)
	assert.InDelta(t, 62.0, *decoded.ProjectedCost.Percent, 0.01)

	var out bytes.Buffer
	overview := ComputeOverviewResult([]OverviewRow{row}, testDayOfMonth)
	require.NoError(t, RenderOverviewAsTable(&out, overview, StackContext{}))
	assert.Contains(t, out.String(), "$25.00 SPOT · risk 0.12 · -62% vs on-demand")
}

func TestEnrichProjectedCost_CopiesOnDemandEquivalent(t *testing.T) {
	t.Parallel()

	row := &OverviewRow{URN: "urn:spot", Type: "aws:ec2:Instance", Status: StatusActive}
	resource := ResourceDescriptor{Type: row.Type, ID: row.URN, Provider: "aws"}
	cost := CostResult{Monthly: 38, Currency: "USD", PricingCategory: PricingCategoryDynamic}
	cost.applyOnDemandEquivalent(floatPtr(100))
	failure := enrichProjectedCost(context.Background(), row, &mockEnricher{
		projectedResult: &CostResultWithErrors{Results: []CostResult{cost}},
	}, resource)
	require.Nil(t, failure)
	require.NotNil(t, row.ProjectedCost.OnDemandEquivalentMonthly)
	require.NotNil(t, row.ProjectedCost.SpotSavingsPercent)
	assert.InDelta(t, 62, *row.ProjectedCost.SpotSavingsPercent, 1e-9)
}

func TestDiffRowNote_SpotSavings(t *testing.T) {
	t.Parallel()

	spot := &CostResult{Monthly: 38, PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.12}
	spot.applyOnDemandEquivalent(floatPtr(100))
	entry := DiffEntry{Operation: DiffOperationCreate, After: spot}
	assert.Equal(t, "SPOT · risk 0.12 · -62% vs on-demand", diffRowNote(&entry))
}
