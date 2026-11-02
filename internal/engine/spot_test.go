package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/proto"
)

func TestSpotDisplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category PricingCategory
		risk     float64
		want     string
	}{
		{"dynamic with risk", PricingCategoryDynamic, 0.12, "SPOT · risk 0.12"},
		{"dynamic rounds to two places", PricingCategoryDynamic, 0.456, "SPOT · risk 0.46"},
		{"dynamic zero risk is unknown", PricingCategoryDynamic, 0, "SPOT · risk unknown"},
		{"standard is not spot", PricingCategoryStandard, 0.5, ""},
		{"committed is not spot", PricingCategoryCommitted, 0, ""},
		{"unspecified is not spot", "", 0.9, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, SpotDisplay(tt.category, tt.risk))
		})
	}

	var nilResult *CostResult
	assert.Empty(t, nilResult.SpotDisplay())
}

func TestMapProtoCostResultToEngine_Pricing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		category     string
		risk         float64
		wantCategory PricingCategory
		wantDisplay  string
	}{
		{"dynamic", "DYNAMIC", 0.12, PricingCategoryDynamic, "SPOT · risk 0.12"},
		{"dynamic zero risk", "DYNAMIC", 0, PricingCategoryDynamic, "SPOT · risk unknown"},
		{"standard", "STANDARD", 0, PricingCategoryStandard, ""},
		{"unspecified", "", 0, "", ""},
	}
	resource := ResourceDescriptor{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mapProtoCostResultToEngine(resource, "plugin", &proto.CostResult{
				Currency:             "USD",
				MonthlyCost:          10,
				PricingCategory:      tt.category,
				SpotInterruptionRisk: tt.risk,
			})
			assert.Equal(t, tt.wantCategory, got.PricingCategory)
			assert.InDelta(t, tt.risk, got.SpotInterruptionRisk, 1e-9)
			assert.Equal(t, tt.wantDisplay, got.SpotDisplay())
		})
	}
}

func TestEstimateResponseToCostResult_Pricing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		category     pbc.FocusPricingCategory
		risk         float64
		wantCategory PricingCategory
	}{
		{"dynamic", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC, 0.3, PricingCategoryDynamic},
		{"dynamic zero risk", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC, 0, PricingCategoryDynamic},
		{"standard", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD, 0, PricingCategoryStandard},
		{"unspecified", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_UNSPECIFIED, 0, ""},
	}
	resource := &ResourceDescriptor{Type: "aws:ec2:Instance", ID: "i-1"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := estimateResponseToCostResult(&pbc.EstimateCostResponse{
				Currency:                  "USD",
				CostMonthly:               73,
				PricingCategory:           tt.category,
				SpotInterruptionRiskScore: tt.risk,
			}, resource)
			assert.Equal(t, tt.wantCategory, got.PricingCategory)
			assert.InDelta(t, tt.risk, got.SpotInterruptionRisk, 1e-9)
			assert.Empty(t, got.Notes, "pricing is a structured field, not a note")
		})
	}
}

func TestProjectedCacheKeepsSpotFields(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{
		ID:         "i-spot",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"instanceType": "t3.micro"},
	}
	eng := &Engine{cache: newMockCache(true)}
	eng.storeProjectedCostCache(context.Background(), resource, []CostResult{{
		ResourceType:         resource.Type,
		Monthly:              20,
		Currency:             "USD",
		PricingCategory:      PricingCategoryDynamic,
		SpotInterruptionRisk: 0.12,
	}})

	got := eng.tryProjectedCostCache(context.Background(), resource)
	require.Len(t, got, 1)
	assert.Equal(t, PricingCategoryDynamic, got[0].PricingCategory)
	assert.InDelta(t, 0.12, got[0].SpotInterruptionRisk, 1e-9)
	assert.Equal(t, "SPOT · risk 0.12", got[0].SpotDisplay())
}

func TestProjectedCacheMissesPrePricingEntries(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{
		ID:         "i-old",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"instanceType": "t3.micro"},
	}
	key, err := ProjectedResourceCacheKey(resource)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(key, projectedPricingCacheSuffix), key)

	store := newMockCache(true)
	oldKey := strings.TrimSuffix(key, projectedPricingCacheSuffix)
	require.NoError(t, store.Set(oldKey, json.RawMessage(
		`[{"resourceType":"aws:ec2:Instance","monthly":10,"currency":"USD","growthType":"none"}]`,
	)))
	eng := &Engine{cache: store}
	assert.Nil(t, eng.tryProjectedCostCache(context.Background(), resource))
}

func TestCostResultJSONCarriesPricing(t *testing.T) {
	t.Parallel()

	spot, err := json.Marshal(CostResult{PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.12})
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(spot, &decoded))
	assert.Equal(t, "DYNAMIC", decoded["pricingCategory"])
	assert.InDelta(t, 0.12, decoded["spotInterruptionRisk"], 1e-9)

	plain, err := json.Marshal(CostResult{Monthly: 1})
	require.NoError(t, err)
	assert.NotContains(t, string(plain), "pricingCategory")
	assert.NotContains(t, string(plain), "spotInterruptionRisk")
}

func TestRenderCostTableShowsSpotTag(t *testing.T) {
	t.Parallel()

	results := []CostResult{
		{
			ResourceType: "aws:ec2:Instance", ResourceID: "spot", Adapter: "p", Currency: "USD", Monthly: 20,
			PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.12, Notes: "billed hourly",
		},
		{
			ResourceType: "aws:ec2:Instance", ResourceID: "unknown", Adapter: "p", Currency: "USD", Monthly: 20,
			PricingCategory: PricingCategoryDynamic,
		},
		{
			ResourceType: "aws:ec2:Instance", ResourceID: "std", Adapter: "p", Currency: "USD", Monthly: 30,
			PricingCategory: PricingCategoryStandard, Notes: "on-demand",
		},
	}
	var out bytes.Buffer
	require.NoError(t, RenderCostTable(&out, results, CostTableOptions{}))
	text := out.String()
	assert.Contains(t, text, "SPOT · risk 0.12; billed hourly")
	assert.Contains(t, text, "SPOT · risk unknown")
	assert.Equal(t, 2, strings.Count(text, "SPOT"))
	assert.Contains(t, text, "on-demand")
}
