package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func spotEstimateResult() *engine.EstimateResult {
	return &engine.EstimateResult{
		Resource: &engine.ResourceDescriptor{Provider: "aws", Type: "aws:ec2:Instance", ID: "i-1"},
		Baseline: &engine.CostResult{Monthly: 73, Currency: "USD"},
		Modified: &engine.CostResult{
			Monthly: 21.9, Currency: "USD",
			PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.12,
		},
		TotalChange: -51.1,
	}
}

func TestRenderEstimateResultTable_SpotTag(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, renderEstimateResultTable(&out, spotEstimateResult()))
	text := out.String()
	assert.Contains(t, text, "Modified:  $21.90/mo (USD)  SPOT · risk 0.12")
	assert.Contains(t, text, "Baseline:  $73.00/mo (USD)\n")
}

func TestRenderEstimateResultTable_NoSpotTagWhenNotDynamic(t *testing.T) {
	t.Parallel()

	result := spotEstimateResult()
	result.Modified.PricingCategory = engine.PricingCategoryStandard
	result.Modified.SpotInterruptionRisk = 0

	var out bytes.Buffer
	require.NoError(t, renderEstimateResultTable(&out, result))
	assert.NotContains(t, out.String(), "SPOT")
}

func TestRenderEstimateResultJSON_CarriesPricingFields(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, renderEstimateResultJSON(&out, spotEstimateResult()))

	var decoded struct {
		Modified struct {
			PricingCategory      string  `json:"pricingCategory"`
			SpotInterruptionRisk float64 `json:"spotInterruptionRisk"`
			Notes                string  `json:"notes"`
		} `json:"modified"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))
	assert.Equal(t, "DYNAMIC", decoded.Modified.PricingCategory)
	assert.InDelta(t, 0.12, decoded.Modified.SpotInterruptionRisk, 1e-9)
	assert.NotContains(t, decoded.Modified.Notes, "Pricing:")
}
