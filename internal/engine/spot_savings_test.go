package engine

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/proto"
)

func floatPtr(v float64) *float64 { return &v }

func TestSpotSavingsPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category PricingCategory
		spot     float64
		onDemand *float64
		want     *float64
	}{
		{"cheaper than on-demand", PricingCategoryDynamic, 38, floatPtr(100), floatPtr(62)},
		{"same price", PricingCategoryDynamic, 100, floatPtr(100), floatPtr(0)},
		{"dearer than on-demand", PricingCategoryDynamic, 115, floatPtr(100), floatPtr(-15)},
		{"free spot", PricingCategoryDynamic, 0, floatPtr(100), floatPtr(100)},
		{"absent on-demand", PricingCategoryDynamic, 38, nil, nil},
		{"zero on-demand is not a percentage", PricingCategoryDynamic, 38, floatPtr(0), nil},
		{"zero on-demand and zero spot", PricingCategoryDynamic, 0, floatPtr(0), nil},
		{"negative on-demand", PricingCategoryDynamic, 38, floatPtr(-1), nil},
		{"NaN on-demand", PricingCategoryDynamic, 38, floatPtr(math.NaN()), nil},
		{"infinite on-demand", PricingCategoryDynamic, 38, floatPtr(math.Inf(1)), nil},
		{"negative spot", PricingCategoryDynamic, -1, floatPtr(100), nil},
		{"NaN spot", PricingCategoryDynamic, math.NaN(), floatPtr(100), nil},
		{"standard ignores the field", PricingCategoryStandard, 38, floatPtr(100), nil},
		{"unspecified ignores the field", "", 38, floatPtr(100), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SpotSavingsPercent(tt.category, tt.spot, tt.onDemand)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.InDelta(t, *tt.want, *got, 1e-9)
		})
	}
}

func TestSpotDisplayWithSavings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		risk    float64
		percent *float64
		want    string
	}{
		{"savings", 0.12, floatPtr(62), "SPOT · risk 0.12 · -62% vs on-demand"},
		{"savings rounded", 0.12, floatPtr(61.6), "SPOT · risk 0.12 · -62% vs on-demand"},
		{"unknown risk with savings", 0, floatPtr(40), "SPOT · risk unknown · -40% vs on-demand"},
		{"dearer than on-demand", 0.12, floatPtr(-15), "SPOT · risk 0.12 · +15% vs on-demand"},
		{"exactly equal", 0.12, floatPtr(0), "SPOT · risk 0.12 · 0% vs on-demand"},
		{"rounds to equal", 0.12, floatPtr(0.2), "SPOT · risk 0.12 · 0% vs on-demand"},
		{"negative zero stays plain", 0.12, floatPtr(math.Copysign(0, -1)), "SPOT · risk 0.12 · 0% vs on-demand"},
		{"unavailable is explicit, not 0%", 0.12, nil, "SPOT · risk 0.12 · spot savings unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, SpotDisplay(PricingCategoryDynamic, tt.risk, tt.percent))
		})
	}
	assert.Empty(t, SpotDisplay(PricingCategoryStandard, 0.1, floatPtr(50)))
}

func TestMapProtoCostResultToEngine_OnDemandEquivalent(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"}
	tests := []struct {
		name        string
		category    string
		onDemand    *float64
		wantOD      *float64
		wantPercent *float64
		wantDisplay string
	}{
		{"spot with field", "DYNAMIC", floatPtr(100), floatPtr(100), floatPtr(62),
			"SPOT · risk 0.12 · -62% vs on-demand"},
		{"spot without field", "DYNAMIC", nil, nil, nil,
			"SPOT · risk 0.12 · spot savings unavailable"},
		{"spot with zero on-demand", "DYNAMIC", floatPtr(0), floatPtr(0), nil,
			"SPOT · risk 0.12 · spot savings unavailable"},
		{"non-spot drops the field", "STANDARD", floatPtr(100), nil, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mapProtoCostResultToEngine(resource, "plugin", &proto.CostResult{
				Currency:                  "USD",
				MonthlyCost:               38,
				PricingCategory:           tt.category,
				SpotInterruptionRisk:      0.12,
				OnDemandEquivalentMonthly: tt.onDemand,
			})
			assert.Equal(t, tt.wantOD, got.OnDemandEquivalentMonthly)
			assert.Equal(t, tt.wantPercent, got.SpotSavingsPercent)
			assert.Equal(t, tt.wantDisplay, got.SpotDisplay())
		})
	}
}

func TestEstimateResponseToCostResult_OnDemandEquivalent(t *testing.T) {
	t.Parallel()

	resource := &ResourceDescriptor{Type: "aws:ec2:Instance", ID: "i-1"}
	onDemand := 100.0
	zero := 0.0
	bad := math.NaN()
	dynamic := pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC

	spot := estimateResponseToCostResult(&pbc.EstimateCostResponse{
		CostMonthly: 38, PricingCategory: dynamic, OnDemandEquivalentMonthlyCost: &onDemand,
	}, resource)
	require.NotNil(t, spot.SpotSavingsPercent)
	assert.InDelta(t, 62, *spot.SpotSavingsPercent, 1e-9)

	zeroOD := estimateResponseToCostResult(&pbc.EstimateCostResponse{
		CostMonthly: 38, PricingCategory: dynamic, OnDemandEquivalentMonthlyCost: &zero,
	}, resource)
	require.NotNil(t, zeroOD.OnDemandEquivalentMonthly)
	assert.Nil(t, zeroOD.SpotSavingsPercent)

	invalid := estimateResponseToCostResult(&pbc.EstimateCostResponse{
		CostMonthly: 38, PricingCategory: dynamic, OnDemandEquivalentMonthlyCost: &bad,
	}, resource)
	assert.Nil(t, invalid.OnDemandEquivalentMonthly)
	assert.Nil(t, invalid.SpotSavingsPercent)

	standard := estimateResponseToCostResult(&pbc.EstimateCostResponse{
		CostMonthly:                   38,
		PricingCategory:               pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD,
		OnDemandEquivalentMonthlyCost: &onDemand,
	}, resource)
	assert.Nil(t, standard.OnDemandEquivalentMonthly)
	assert.Nil(t, standard.SpotSavingsPercent)
}

func TestCostResultJSONCarriesSavings(t *testing.T) {
	t.Parallel()

	result := CostResult{Monthly: 38, PricingCategory: PricingCategoryDynamic}
	result.applyOnDemandEquivalent(floatPtr(100))
	data, err := json.Marshal(result)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.InDelta(t, 100, decoded["onDemandEquivalentMonthlyCost"], 1e-9)
	assert.InDelta(t, 62, decoded["spotSavingsPercent"], 1e-9)

	zero := CostResult{Monthly: 38, PricingCategory: PricingCategoryDynamic}
	zero.applyOnDemandEquivalent(floatPtr(0))
	data, err = json.Marshal(zero)
	require.NoError(t, err)
	decoded = map[string]any{}
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Contains(t, decoded, "onDemandEquivalentMonthlyCost",
		"an explicit zero on-demand price is kept, not dropped as unset")
	assert.InDelta(t, 0, decoded["onDemandEquivalentMonthlyCost"], 1e-9)
	assert.NotContains(t, decoded, "spotSavingsPercent")

	plain, err := json.Marshal(CostResult{Monthly: 1})
	require.NoError(t, err)
	assert.NotContains(t, string(plain), "onDemandEquivalentMonthlyCost")
	assert.NotContains(t, string(plain), "spotSavingsPercent")
}
