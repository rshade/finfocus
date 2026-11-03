package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestMapProjectedProtoPricing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category pbc.FocusPricingCategory
		risk     float64
		wantCat  string
		wantRisk float64
	}{
		{"dynamic with risk", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC, 0.12, "DYNAMIC", 0.12},
		{"dynamic zero risk", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC, 0, "DYNAMIC", 0},
		{"standard", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD, 0, "STANDARD", 0},
		{"committed", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_COMMITTED, 0, "COMMITTED", 0},
		{"other", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_OTHER, 0, "OTHER", 0},
		{"unspecified", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_UNSPECIFIED, 0, "", 0},
		{"unknown enum value", pbc.FocusPricingCategory(99), 0, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mapped := mapProjectedProto(&pbc.GetProjectedCostResponse{
				CostPerMonth:              1,
				PricingCategory:           tt.category,
				SpotInterruptionRiskScore: tt.risk,
			})
			assert.Equal(t, tt.wantCat, mapped.PricingCategory)
			assert.InDelta(t, tt.wantRisk, mapped.SpotInterruptionRisk, 1e-9)
		})
	}
}
