package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func spotCostResults() []CostResult {
	return []CostResult{
		{
			ResourceType: "aws:ec2:Instance", ResourceID: "a", Adapter: "p", Currency: "USD", Monthly: 300,
			PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.18,
		},
		{ResourceType: "aws:ec2:Instance", ResourceID: "b", Adapter: "p", Currency: "USD", Monthly: 700},
	}
}

func TestRenderCostTableShowsSpotExposure(t *testing.T) {
	t.Parallel()

	var spot bytes.Buffer
	require.NoError(t, RenderCostTable(&spot, spotCostResults(), CostTableOptions{}))
	assert.Contains(t, spot.String(), "Spot exposure:")
	assert.Contains(t, spot.String(), "300.00 USD (30.0% of 1000.00 USD), weighted risk 0.18")

	var none bytes.Buffer
	require.NoError(t, RenderCostTable(&none, spotCostResults()[1:], CostTableOptions{}))
	assert.NotContains(t, none.String(), "Spot exposure")
}

func TestRenderResultsJSONSpotExposure(t *testing.T) {
	t.Parallel()

	var spot bytes.Buffer
	require.NoError(t, RenderResults(&spot, OutputJSON, spotCostResults()))
	var doc struct {
		Finfocus struct {
			Summary struct {
				Spot map[string]any `json:"spot"`
			} `json:"summary"`
		} `json:"finfocus"`
	}
	require.NoError(t, json.Unmarshal(spot.Bytes(), &doc))
	got := doc.Finfocus.Summary.Spot
	assert.InDelta(t, 300.0, got["spotCost"], 1e-9)
	assert.InDelta(t, 0.3, got["spotShare"], 1e-9)
	assert.InDelta(t, 0.18, got["weightedRisk"], 1e-9)
	assert.InDelta(t, 1.0, got["spotResources"], 1e-9)
	assert.InDelta(t, 0.0, got["riskUnknownResources"], 1e-9)

	var none bytes.Buffer
	require.NoError(t, RenderResults(&none, OutputJSON, spotCostResults()[1:]))
	assert.NotContains(t, none.String(), `"spot"`)
}

func TestRenderResultsJSONOmitsWeightedRiskWhenUnknown(t *testing.T) {
	t.Parallel()

	results := spotCostResults()
	results[0].SpotInterruptionRisk = 0
	var out bytes.Buffer
	require.NoError(t, RenderResults(&out, OutputJSON, results))
	assert.NotContains(t, out.String(), "weightedRisk")
	assert.Contains(t, out.String(), `"riskUnknownResources": 1`)
}

func spotDiff() *DiffResult {
	std := &CostResult{ResourceType: "aws:ec2:Instance", ResourceID: "std", Monthly: 100, Currency: "USD"}
	spotBefore := spotCost(100, 0.1)
	spotAfter := spotCost(200, 0.1)
	return &DiffResult{
		Entries: []DiffEntry{
			{
				Operation: DiffOperationSame, ResourceType: "aws:ec2:Instance", ResourceID: "std",
				Before: std, After: std,
			},
			{
				Operation: DiffOperationUpdate, ResourceType: "aws:ec2:Instance", ResourceID: "spot",
				Before: spotBefore, After: spotAfter, DeltaMonthly: 100,
			},
		},
		Summary: DiffSummary{TotalBefore: 200, TotalAfter: 300, TotalDelta: 100, Currency: "USD"},
	}
}

func TestRenderDiffSpotExposure(t *testing.T) {
	t.Parallel()

	var table bytes.Buffer
	require.NoError(t, renderDiffTable(&table, spotDiff(), false))
	assert.Contains(t, table.String(), "Spot exposure")
	assert.Contains(t, table.String(), "200.00 USD (66.7% of 300.00 USD), weighted risk 0.10")
	assert.Contains(t, table.String(), "Spot change")
	assert.Contains(t, table.String(), "+100.00 USD (before 100.00, after 200.00)")

	var out bytes.Buffer
	require.NoError(t, renderDiffJSON(&out, spotDiff()))
	var doc struct {
		Finfocus struct {
			Summary struct {
				Spot map[string]any `json:"spot"`
			} `json:"summary"`
			SpotChange map[string]any `json:"spotChange"`
		} `json:"finfocus"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	assert.InDelta(t, 200.0, doc.Finfocus.Summary.Spot["spotCost"], 1e-9)
	assert.InDelta(t, 100.0, doc.Finfocus.SpotChange["deltaSpotCost"], 1e-9)
	assert.InDelta(t, 100.0, doc.Finfocus.SpotChange["beforeSpotCost"], 1e-9)
	assert.InDelta(t, 200.0, doc.Finfocus.SpotChange["afterSpotCost"], 1e-9)
}

func TestRenderDiffWithoutSpotIsUnchanged(t *testing.T) {
	t.Parallel()

	diff := spotDiff()
	diff.Entries = diff.Entries[:1]

	var table bytes.Buffer
	require.NoError(t, renderDiffTable(&table, diff, false))
	assert.NotContains(t, table.String(), "Spot")

	var out bytes.Buffer
	require.NoError(t, renderDiffJSON(&out, diff))
	assert.NotContains(t, out.String(), "spot")
}

func TestOverviewTotalsSpotExposure(t *testing.T) {
	t.Parallel()

	rows := []OverviewRow{
		spotOverviewRow("urn:a::spot", PricingCategoryDynamic, 0.2),
		spotOverviewRow("urn:a::unknown", PricingCategoryDynamic, 0),
		spotOverviewRow("urn:a::std", PricingCategoryStandard, 0),
		spotOverviewRow("urn:a::bare", "", 0),
	}
	errRow := spotOverviewRow("urn:a::err", PricingCategoryDynamic, 0.9)
	errRow.Error = &OverviewRowError{URN: errRow.URN, ErrorType: ErrorTypeAuth, Message: "boom"}
	rows = append(rows, errRow)

	result := ComputeOverviewResult(rows, 15)
	spot := result.Summary.Spot
	require.NotNil(t, spot)
	assert.InDelta(t, 50.0, spot.SpotCost, 1e-9)
	assert.InDelta(t, 100.0, spot.TotalCost, 1e-9)
	assert.InDelta(t, 100.0, result.Summary.TotalProjected, 1e-9)
	assert.Equal(t, 2, spot.SpotResources)
	assert.Equal(t, 1, spot.RiskUnknownResources)
	require.NotNil(t, spot.WeightedRisk)
	assert.InDelta(t, 0.2, *spot.WeightedRisk, 1e-9)

	var table bytes.Buffer
	require.NoError(t, RenderOverviewAsTable(&table, result, StackContext{StackName: "dev"}))
	assert.Contains(t, table.String(), "Spot exposure:")
	assert.Contains(t, table.String(), "(1 of 2 spot resources have no risk score and are excluded)")

	var out bytes.Buffer
	require.NoError(t, RenderOverviewAsJSON(context.Background(), &out, result, StackContext{StackName: "dev"}, nil))
	var doc struct {
		Summary struct {
			Spot map[string]any `json:"spot"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	assert.InDelta(t, 50.0, doc.Summary.Spot["spotCost"], 1e-9)
}

func TestOverviewTotalsWithoutSpotOmitBlock(t *testing.T) {
	t.Parallel()

	result := ComputeOverviewResult([]OverviewRow{spotOverviewRow("urn:a::std", PricingCategoryStandard, 0)}, 15)
	assert.Nil(t, result.Summary.Spot)

	var table bytes.Buffer
	require.NoError(t, RenderOverviewAsTable(&table, result, StackContext{StackName: "dev"}))
	assert.NotContains(t, table.String(), "Spot exposure")

	var out bytes.Buffer
	require.NoError(t, RenderOverviewAsJSON(context.Background(), &out, result, StackContext{StackName: "dev"}, nil))
	assert.NotContains(t, out.String(), `"spot"`)
}

func TestOverviewTotalsMixedCurrenciesCarryNoSpot(t *testing.T) {
	t.Parallel()

	eur := spotOverviewRow("urn:a::eur", PricingCategoryStandard, 0)
	eur.ProjectedCost.Currency = "EUR"
	result := ComputeOverviewResult([]OverviewRow{spotOverviewRow("urn:a::spot", PricingCategoryDynamic, 0.2), eur}, 15)
	assert.True(t, result.Summary.MixedCurrencies)
	assert.Nil(t, result.Summary.Spot)
}
