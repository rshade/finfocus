package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func spotOverviewRow(urn string, category PricingCategory, risk float64) OverviewRow {
	return OverviewRow{
		URN:    urn,
		Type:   "aws:ec2/instance:Instance",
		Status: StatusCreating,
		ProjectedCost: &ProjectedCostData{
			MonthlyCost:          25,
			Currency:             "USD",
			PricingCategory:      category,
			SpotInterruptionRisk: risk,
		},
	}
}

func TestOverviewRowResult_SpotDisplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  OverviewRow
		want string
	}{
		{
			"dynamic with risk",
			spotOverviewRow("urn:a::spot", PricingCategoryDynamic, 0.12),
			"SPOT · risk 0.12 · spot savings unavailable",
		},
		{
			"dynamic zero risk",
			spotOverviewRow("urn:a::unknown", PricingCategoryDynamic, 0),
			"SPOT · risk unknown · spot savings unavailable",
		},
		{"standard", spotOverviewRow("urn:a::std", PricingCategoryStandard, 0.4), ""},
		{"unspecified", spotOverviewRow("urn:a::none", "", 0), ""},
		{"no projected cost", OverviewRow{URN: "urn:a::bare", Type: "aws:s3/bucket:Bucket"}, ""},
		{
			"error row hides the tag",
			func() OverviewRow {
				row := spotOverviewRow("urn:a::err", PricingCategoryDynamic, 0.2)
				row.Error = &OverviewRowError{URN: row.URN, ErrorType: ErrorTypeAuth, Message: "boom"}
				return row
			}(),
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ComputeOverviewRowResult(tt.row)
			assert.Equal(t, tt.want, result.SpotDisplay)
			assert.NotContains(t, result.ProjectedDisplay, "SPOT", "the tag never joins the amount cell")
		})
	}
}

func TestRenderOverviewAsTable_SpotTag(t *testing.T) {
	t.Parallel()

	rows := []OverviewRow{
		spotOverviewRow("urn:x::spot", PricingCategoryDynamic, 0.12),
		spotOverviewRow("urn:x::plain", PricingCategoryStandard, 0),
	}
	result := ComputeOverviewResult(rows, testDayOfMonth)

	var out bytes.Buffer
	require.NoError(t, RenderOverviewAsTable(&out, result, StackContext{}))

	var spotLines, plainLines []string
	for _, line := range strings.Split(out.String(), "\n") {
		switch {
		case strings.Contains(line, "::spot"):
			spotLines = append(spotLines, line)
		case strings.Contains(line, "::plain"):
			plainLines = append(plainLines, line)
		}
	}
	require.Len(t, spotLines, 1)
	require.Len(t, plainLines, 1)
	assert.Contains(t, spotLines[0], "$25.00 SPOT · risk 0.12 · spot savings unavailable")
	assert.NotContains(t, plainLines[0], "SPOT")
}

func TestEnrichProjectedCost_CopiesPricingFields(t *testing.T) {
	t.Parallel()

	row := &OverviewRow{URN: "urn:spot", Type: "aws:ec2:Instance", Status: StatusActive}
	resource := ResourceDescriptor{Type: row.Type, ID: row.URN, Provider: "aws"}
	failure := enrichProjectedCost(context.Background(), row, &mockEnricher{
		projectedResult: &CostResultWithErrors{Results: []CostResult{{
			Monthly: 20, Currency: "USD",
			PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.3,
		}}},
	}, resource)
	require.Nil(t, failure)
	require.NotNil(t, row.ProjectedCost)
	assert.Equal(t, PricingCategoryDynamic, row.ProjectedCost.PricingCategory)
	assert.InDelta(t, 0.3, row.ProjectedCost.SpotInterruptionRisk, 1e-9)
}

func TestOverviewJSON_CarriesPricingFields(t *testing.T) {
	t.Parallel()

	row := spotOverviewRow("urn:a::spot", PricingCategoryDynamic, 0.12)
	data, err := json.Marshal(ComputeOverviewRowResult(row))
	require.NoError(t, err)

	var decoded struct {
		SpotDisplay   string `json:"spotDisplay"`
		ProjectedCost struct {
			PricingCategory      string  `json:"pricingCategory"`
			SpotInterruptionRisk float64 `json:"spotInterruptionRisk"`
		} `json:"projectedCost"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "SPOT · risk 0.12 · spot savings unavailable", decoded.SpotDisplay)
	assert.Equal(t, "DYNAMIC", decoded.ProjectedCost.PricingCategory)
	assert.InDelta(t, 0.12, decoded.ProjectedCost.SpotInterruptionRisk, 1e-9)

	plain, err := json.Marshal(ComputeOverviewRowResult(spotOverviewRow("urn:a::p", PricingCategoryStandard, 0)))
	require.NoError(t, err)
	assert.NotContains(t, string(plain), "spotDisplay")
}
