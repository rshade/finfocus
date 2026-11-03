package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/proto"
)

func TestSpotDisplay(t *testing.T) {
	t.Parallel()

	const unavailable = " · spot savings unavailable"
	tests := []struct {
		name     string
		category PricingCategory
		risk     float64
		want     string
	}{
		{"dynamic with risk", PricingCategoryDynamic, 0.12, "SPOT · risk 0.12" + unavailable},
		{"dynamic rounds to two places", PricingCategoryDynamic, 0.456, "SPOT · risk 0.46" + unavailable},
		{"dynamic zero risk is unknown", PricingCategoryDynamic, 0, "SPOT · risk unknown" + unavailable},
		{"standard is not spot", PricingCategoryStandard, 0.5, ""},
		{"tiny risk is not shown as zero", PricingCategoryDynamic, 0.001, "SPOT · risk <0.01" + unavailable},
		{"risk at the display floor", PricingCategoryDynamic, 0.005, "SPOT · risk 0.01" + unavailable},
		{"risk of one", PricingCategoryDynamic, 1, "SPOT · risk 1.00" + unavailable},
		{"committed is not spot", PricingCategoryCommitted, 0, ""},
		{"unspecified is not spot", "", 0.9, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, SpotDisplay(tt.category, tt.risk, nil))
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
		{"dynamic", "DYNAMIC", 0.12, PricingCategoryDynamic, "SPOT · risk 0.12 · spot savings unavailable"},
		{"dynamic zero risk", "DYNAMIC", 0, PricingCategoryDynamic, "SPOT · risk unknown · spot savings unavailable"},
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

func TestEstimateResponseToCostResult_DropsInvalidRisk(t *testing.T) {
	t.Parallel()

	dynamic := pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
	standard := pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
	tests := []struct {
		name     string
		category pbc.FocusPricingCategory
		risk     float64
		want     float64
	}{
		{"NaN", dynamic, math.NaN(), 0},
		{"infinity", dynamic, math.Inf(1), 0},
		{"negative", dynamic, -0.2, 0},
		{"above one", dynamic, 1.7, 0},
		{"standard with a score", standard, 0.5, 0},
		{"zero", dynamic, 0, 0},
		{"one", dynamic, 1, 1},
	}
	resource := &ResourceDescriptor{Type: "aws:ec2:Instance", ID: "i-1"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := estimateResponseToCostResult(&pbc.EstimateCostResponse{
				CostMonthly: 1, PricingCategory: tt.category, SpotInterruptionRiskScore: tt.risk,
			}, resource)
			assert.InDelta(t, tt.want, got.SpotInterruptionRisk, 1e-12)
			_, err := json.Marshal(got)
			require.NoError(t, err)
		})
	}
}

func TestNonSpotJSONGainsOnlyPricingCategory(t *testing.T) {
	t.Parallel()

	got := estimateResponseToCostResult(&pbc.EstimateCostResponse{
		CostMonthly: 1, PricingCategory: pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD,
		SpotInterruptionRiskScore: 0.5,
	}, &ResourceDescriptor{Type: "aws:ec2:Instance", ID: "i-1"})
	data, err := json.Marshal(got)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "STANDARD", decoded["pricingCategory"])
	assert.NotContains(t, decoded, "spotInterruptionRisk")
	assert.Empty(t, decoded["notes"], "the old Pricing: note string is gone")
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
	assert.Equal(t, "SPOT · risk 0.12 · spot savings unavailable", got[0].SpotDisplay())
}

func TestProjectedCacheKeepsOnDemandEquivalent(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{
		ID:         "i-spot",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"instanceType": "t3.micro"},
	}
	onDemand := 100.0
	cached := CostResult{
		ResourceType:         resource.Type,
		Monthly:              38,
		Currency:             "USD",
		PricingCategory:      PricingCategoryDynamic,
		SpotInterruptionRisk: 0.12,
	}
	cached.applyOnDemandEquivalent(&onDemand)
	eng := &Engine{cache: newMockCache(true)}
	eng.storeProjectedCostCache(context.Background(), resource, []CostResult{cached})

	got := eng.tryProjectedCostCache(context.Background(), resource)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].OnDemandEquivalentMonthly)
	assert.InDelta(t, 100, *got[0].OnDemandEquivalentMonthly, 1e-9)
	require.NotNil(t, got[0].SpotSavingsPercent)
	assert.InDelta(t, 62, *got[0].SpotSavingsPercent, 1e-9)
	assert.Equal(t, "SPOT · risk 0.12 · -62% vs on-demand", got[0].SpotDisplay())
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
	assert.Contains(t, text, "SPOT · risk 0.12 · spot savings unavailable; billed hourly")
	assert.Contains(t, text, "SPOT · risk unknown · spot savings unavailable")
	assert.Equal(t, 2, strings.Count(text, "SPOT"))
	assert.Contains(t, text, "on-demand")
}
