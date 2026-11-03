package proto

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestSpotRisk(t *testing.T) {
	t.Parallel()

	dynamic := pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
	standard := pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
	tests := []struct {
		name     string
		category pbc.FocusPricingCategory
		score    float64
		want     float64
	}{
		{"valid", dynamic, 0.12, 0.12},
		{"lower bound", dynamic, 0, 0},
		{"upper bound", dynamic, 1, 1},
		{"NaN", dynamic, math.NaN(), 0},
		{"positive infinity", dynamic, math.Inf(1), 0},
		{"negative infinity", dynamic, math.Inf(-1), 0},
		{"negative", dynamic, -0.1, 0},
		{"above one", dynamic, 1.7, 0},
		{"standard with a score", standard, 0.5, 0},
		{"unspecified with a score", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_UNSPECIFIED, 0.5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, SpotRisk(tt.category, tt.score), 1e-12)
		})
	}
}

func TestMapProjectedProtoDropsInvalidRisk(t *testing.T) {
	t.Parallel()

	mapped := mapProjectedProto(&pbc.GetProjectedCostResponse{
		CostPerMonth:              1,
		PricingCategory:           pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC,
		SpotInterruptionRiskScore: math.NaN(),
	})
	assert.Equal(t, "DYNAMIC", mapped.PricingCategory)
	assert.Zero(t, mapped.SpotInterruptionRisk)
}

func TestOnDemandEquivalent(t *testing.T) {
	t.Parallel()

	dynamic := pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
	ptr := func(v float64) *float64 { return &v }
	tests := []struct {
		name     string
		category pbc.FocusPricingCategory
		value    *float64
		want     *float64
	}{
		{"present", dynamic, ptr(100), ptr(100)},
		{"explicit zero is kept", dynamic, ptr(0), ptr(0)},
		{"absent", dynamic, nil, nil},
		{"NaN", dynamic, ptr(math.NaN()), nil},
		{"positive infinity", dynamic, ptr(math.Inf(1)), nil},
		{"negative", dynamic, ptr(-1), nil},
		{"standard category", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD, ptr(100), nil},
		{"unspecified category", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_UNSPECIFIED, ptr(100), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := OnDemandEquivalent(tt.category, tt.value)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.InDelta(t, *tt.want, *got, 1e-12)
			assert.NotSame(t, tt.value, got)
		})
	}
}

func TestMapProjectedProtoCarriesOnDemandEquivalent(t *testing.T) {
	t.Parallel()

	onDemand := 100.0
	mapped := mapProjectedProto(&pbc.GetProjectedCostResponse{
		CostPerMonth:                  38,
		PricingCategory:               pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC,
		OnDemandEquivalentMonthlyCost: &onDemand,
	})
	require.NotNil(t, mapped.OnDemandEquivalentMonthly)
	assert.InDelta(t, 100, *mapped.OnDemandEquivalentMonthly, 1e-12)

	unset := mapProjectedProto(&pbc.GetProjectedCostResponse{
		CostPerMonth:    38,
		PricingCategory: pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC,
	})
	assert.Nil(t, unset.OnDemandEquivalentMonthly)
}
